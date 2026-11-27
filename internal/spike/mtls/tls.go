package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
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
}

type stream struct{ ch chan struct{} }

// NewRevoker returns an empty Revoker.
func NewRevoker() *Revoker {
	return &Revoker{
		revoked: make(map[string]struct{}),
		streams: make(map[string]map[*stream]struct{}),
	}
}

// IsRevoked reports whether the serial (lower-case hex) has been revoked.
func (r *Revoker) IsRevoked(serial string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.revoked[serial]
	return ok
}

// Revoke marks the serial revoked and signals every open stream authenticated
// with it. Streams of other serials are untouched.
func (r *Revoker) Revoke(serial string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revoked[serial] = struct{}{}
	for s := range r.streams[serial] {
		close(s.ch)
		delete(r.streams[serial], s)
	}
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
