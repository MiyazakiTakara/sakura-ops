package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc/credentials"
)

// MaxMessageSize is the proposed 256 KiB gRPC message limit (docs/03-architecture.md).
const MaxMessageSize = 256 << 10

// Revoker holds an in-memory set of revoked certificate serials and the open
// streams of each serial, so a revocation takes effect both at the handshake
// and on connections that are already established.
type Revoker struct {
	mu      sync.Mutex
	revoked map[string]struct{}
	streams map[string]map[*stream]struct{}
	conns   map[string]map[*trackedConn]struct{}
}

type stream struct{ ch chan struct{} }

// NewRevoker returns an empty Revoker.
func NewRevoker() *Revoker {
	return &Revoker{
		revoked: make(map[string]struct{}),
		streams: make(map[string]map[*stream]struct{}),
		conns:   make(map[string]map[*trackedConn]struct{}),
	}
}

// IsRevoked reports whether the serial (lower-case hex) has been revoked.
func (r *Revoker) IsRevoked(serial string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.revoked[serial]
	return ok
}

// Revoke marks the serial revoked, signals every open stream authenticated with
// it and closes every transport connection registered for it, which also
// unblocks handlers stuck in Send or Recv under flow control. Other serials are
// untouched.
func (r *Revoker) Revoke(serial string) {
	r.mu.Lock()
	r.revoked[serial] = struct{}{}
	for s := range r.streams[serial] {
		close(s.ch)
		delete(r.streams[serial], s)
	}
	var toClose []*trackedConn
	for c := range r.conns[serial] {
		toClose = append(toClose, c)
	}
	delete(r.conns, serial)
	r.mu.Unlock()
	// Closed outside the lock: Close unregisters and takes the lock itself.
	for _, c := range toClose {
		_ = c.Close()
	}
}

// registerConn tracks c under serial. It reports false, and registers nothing,
// when the serial is already revoked; check and insert are atomic with Revoke, so
// a revocation racing with the handshake cannot be missed.
func (r *Revoker) registerConn(serial string, c *trackedConn) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.revoked[serial]; ok {
		return false
	}
	if r.conns[serial] == nil {
		r.conns[serial] = make(map[*trackedConn]struct{})
	}
	r.conns[serial][c] = struct{}{}
	return true
}

func (r *Revoker) unregisterConn(serial string, c *trackedConn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.conns[serial], c)
}

// trackedConn is a server-side connection that unregisters itself when closed.
type trackedConn struct {
	net.Conn
	rev    *Revoker
	serial string
	once   sync.Once
}

func (c *trackedConn) Close() error {
	var err error
	c.once.Do(func() {
		c.rev.unregisterConn(c.serial, c)
		err = c.Conn.Close()
	})
	return err
}

// revocableCreds wraps the server TLS credentials so that every verified
// connection is registered with the Revoker.
type revocableCreds struct {
	credentials.TransportCredentials
	rev *Revoker
}

// NewServerCredentials returns gRPC server credentials for the mutual-TLS
// configuration whose connections are closed when their certificate is revoked.
func NewServerCredentials(cfg *tls.Config, rev *Revoker) credentials.TransportCredentials {
	return &revocableCreds{TransportCredentials: credentials.NewTLS(cfg), rev: rev}
}

func (c *revocableCreds) ServerHandshake(raw net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, info, err := c.TransportCredentials.ServerHandshake(raw)
	if err != nil {
		return nil, nil, err
	}
	ti, ok := info.(credentials.TLSInfo)
	if !ok || len(ti.State.PeerCertificates) == 0 {
		_ = conn.Close()
		return nil, nil, errors.New("mtls: handshake produced no verified client certificate")
	}
	serial := serialOf(ti.State.PeerCertificates[0])
	tc := &trackedConn{Conn: conn, rev: c.rev, serial: serial}
	if !c.rev.registerConn(serial, tc) {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("mtls: client certificate %s is revoked", serial)
	}
	return tc, info, nil
}

func (c *revocableCreds) Clone() credentials.TransportCredentials {
	return &revocableCreds{TransportCredentials: c.TransportCredentials.Clone(), rev: c.rev}
}

// Track registers an open stream for the serial. The returned channel is closed
// when the serial is revoked (it is already closed if the serial was revoked
// before Track); done must be called when the stream ends.
func (r *Revoker) Track(serial string) (revoked <-chan struct{}, done func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := &stream{ch: make(chan struct{})}
	if _, ok := r.revoked[serial]; ok {
		close(s.ch)
		return s.ch, func() {}
	}
	if r.streams[serial] == nil {
		r.streams[serial] = make(map[*stream]struct{})
	}
	r.streams[serial][s] = struct{}{}
	return s.ch, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.streams[serial], s)
	}
}

// ServerTLS builds the server configuration: TLS 1.3 only, client certificate
// required and verified against clientCAs, ALPN h2, no session resumption (so
// every connection is verified and checked for revocation). The leaf must carry
// an explicit clientAuth extended key usage; certificates without any EKU are
// rejected too, which is stricter than the default chain verification.
func ServerTLS(cert tls.Certificate, clientCAs *x509.CertPool, rev *Revoker) (*tls.Config, error) {
	if clientCAs == nil || rev == nil {
		return nil, errors.New("mtls: client CA pool and revoker are required")
	}
	return &tls.Config{
		MinVersion:             tls.VersionTLS13,
		Certificates:           []tls.Certificate{cert},
		ClientAuth:             tls.RequireAndVerifyClientCert,
		ClientCAs:              clientCAs,
		NextProtos:             []string{"h2"},
		SessionTicketsDisabled: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("mtls: no client certificate")
			}
			leaf := cs.PeerCertificates[0]
			if !hasClientAuth(leaf) {
				return errors.New("mtls: client certificate lacks the clientAuth extended key usage")
			}
			if rev.IsRevoked(serialOf(leaf)) {
				return fmt.Errorf("mtls: client certificate %s is revoked", serialOf(leaf))
			}
			return nil
		},
	}, nil
}

// ClientTLS builds the client configuration: TLS 1.3 only, the server must chain
// to serverCAs and present serverName. Verification is never disabled.
func ClientTLS(cert tls.Certificate, serverCAs *x509.CertPool, serverName string) (*tls.Config, error) {
	if serverCAs == nil || serverName == "" {
		return nil, errors.New("mtls: server CA pool and server name are required")
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		RootCAs:      serverCAs,
		ServerName:   serverName,
		NextProtos:   []string{"h2"},
	}, nil
}

func hasClientAuth(c *x509.Certificate) bool {
	for _, u := range c.ExtKeyUsage {
		if u == x509.ExtKeyUsageClientAuth {
			return true
		}
	}
	return false
}

func serialOf(c *x509.Certificate) string { return c.SerialNumber.Text(16) }
