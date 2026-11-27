package mtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/MiyazakiTakara/sakura-ops/internal/spike/mtls/spikepb"
)

const (
	backoffBase = 50 * time.Millisecond
	backoffMax  = 400 * time.Millisecond
)

// hub is a running test server plus the PKI it trusts.
type hub struct {
	serverCA *testCA // signs the server certificate; clients trust it
	deviceCA *testCA // signs client certificates; the server trusts it
	srvCert  issued
	rev      *Revoker
	svc      *Service
	addr     string
	srv      *grpc.Server

	mu    sync.Mutex
	calls []PeerInfo
}

func (h *hub) callCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.calls)
}

func newHub(t *testing.T) *hub {
	t.Helper()
	h := &hub{serverCA: newCA(t, "test server CA"), deviceCA: newCA(t, "test device CA"), rev: NewRevoker()}
	h.srvCert = h.serverCA.server(t)
	h.svc = NewService(h.rev)
	h.svc.onCall = func(p PeerInfo) {
		h.mu.Lock()
		h.calls = append(h.calls, p)
		h.mu.Unlock()
	}
	h.start(t, "127.0.0.1:0")
	return h
}

func (h *hub) start(t *testing.T, addr string) {
	t.Helper()
	cfg, err := ServerTLS(h.srvCert.tls, h.deviceCA.pool, h.rev)
	if err != nil {
		t.Fatal(err)
	}
	var lis net.Listener
	for i := 0; i < 50; i++ { // the port may need a moment to be released on restart
		if lis, err = net.Listen("tcp", addr); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	h.addr = lis.Addr().String()
	h.srv = NewServer(cfg, h.svc)
	go func() { _ = h.srv.Serve(lis) }()
	t.Cleanup(h.srv.Stop)
}

func (h *hub) stop() { h.srv.Stop() }

func (h *hub) dial(t *testing.T, c issued) *grpc.ClientConn {
	t.Helper()
	return h.dialWith(t, c, h.serverCA.pool, serverName)
}

func (h *hub) dialWith(t *testing.T, c issued, roots *x509.CertPool, name string) *grpc.ClientConn {
	t.Helper()
	cfg, err := ClientTLS(c.tls, roots, name)
	if err != nil {
		t.Fatal(err)
	}
	return h.dialCfg(t, cfg)
}

func (h *hub) dialCfg(t *testing.T, cfg *tls.Config) *grpc.ClientConn {
	t.Helper()
	conn, err := Dial(h.addr, cfg, backoffBase, backoffMax)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func echo(conn *grpc.ClientConn, payload int, opts ...grpc.CallOption) (*spikepb.EchoResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return spikepb.NewSpikeServiceClient(conn).Echo(ctx, &spikepb.EchoRequest{Payload: make([]byte, payload)}, opts...)
}

func mustCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %v, got success", want)
	}
	if got := status.Code(err); got != want {
		t.Fatalf("code = %v (%v), want %v", got, err, want)
	}
}

func TestTwoClientsHaveDistinctVerifiedIdentities(t *testing.T) {
	h := newHub(t)
	a, b := h.deviceCA.client(t, "agent-a"), h.deviceCA.client(t, "agent-b")

	ra, err := echo(h.dial(t, a), 10)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := echo(h.dial(t, b), 10)
	if err != nil {
		t.Fatal(err)
	}
	if ra.Peer != "agent-a" || rb.Peer != "agent-b" {
		t.Fatalf("peers = %q, %q", ra.Peer, rb.Peer)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.calls) != 2 || h.calls[0].Serial == h.calls[1].Serial || h.calls[0].Name == h.calls[1].Name {
		t.Fatalf("identities not distinct: %+v", h.calls)
	}
}

func TestNegotiatedTLS13AndH2(t *testing.T) {
	h := newHub(t)
	if _, err := echo(h.dial(t, h.deviceCA.client(t, "agent-a")), 1); err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if p := h.calls[0]; p.TLSVersion != tls.VersionTLS13 || p.ALPN != "h2" {
		t.Fatalf("server saw TLS %#x ALPN %q, want TLS 1.3 and h2", p.TLSVersion, p.ALPN)
	}
}

func TestRejectedConnections(t *testing.T) {
	foreignCA := newCA(t, "foreign CA")
	now := time.Now()
	clientAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	type tc struct {
		name   string
		client func(h *hub, t *testing.T) issued
		// optional overrides
		roots  func(h *hub) *x509.CertPool
		srvDNS string
		maxVer uint16
	}
	cases := []tc{
		{name: "control: valid client is accepted", client: func(h *hub, t *testing.T) issued { return h.deviceCA.client(t, "ok") }},
		{name: "client cert from foreign CA", client: func(h *hub, t *testing.T) issued { return foreignCA.client(t, "intruder") }},
		{name: "no client certificate", client: func(h *hub, t *testing.T) issued { return issued{} }},
		{name: "client cert with serverAuth only", client: func(h *hub, t *testing.T) issued {
			return h.deviceCA.issue(t, leafOpts{cn: "wrong-eku", eku: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
		}},
		{name: "client cert without any EKU", client: func(h *hub, t *testing.T) issued {
			return h.deviceCA.issue(t, leafOpts{cn: "no-eku"})
		}},
		{name: "expired client cert", client: func(h *hub, t *testing.T) issued {
			return h.deviceCA.issue(t, leafOpts{cn: "expired", eku: clientAuth, notBefore: now.Add(-2 * time.Hour), notAfter: now.Add(-time.Hour)})
		}},
		{name: "not yet valid client cert", client: func(h *hub, t *testing.T) issued {
			return h.deviceCA.issue(t, leafOpts{cn: "future", eku: clientAuth, notBefore: now.Add(time.Hour), notAfter: now.Add(2 * time.Hour)})
		}},
		{name: "server cert for the wrong DNS name", srvDNS: "other.sakuraops.test",
			client: func(h *hub, t *testing.T) issued { return h.deviceCA.client(t, "ok") }},
		{name: "client does not trust the server CA", roots: func(h *hub) *x509.CertPool { return foreignCA.pool },
			client: func(h *hub, t *testing.T) issued { return h.deviceCA.client(t, "ok") }},
		{name: "client limited to TLS 1.2", maxVer: tls.VersionTLS12,
			client: func(h *hub, t *testing.T) issued { return h.deviceCA.client(t, "ok") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := &hub{serverCA: newCA(t, "test server CA"), deviceCA: newCA(t, "test device CA"), rev: NewRevoker()}
			if c.srvDNS != "" {
				h.srvCert = h.serverCA.issue(t, leafOpts{cn: "hub", dns: []string{c.srvDNS}, eku: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
			} else {
				h.srvCert = h.serverCA.server(t)
			}
			h.svc = NewService(h.rev)
			h.svc.onCall = func(p PeerInfo) { h.mu.Lock(); h.calls = append(h.calls, p); h.mu.Unlock() }
			h.start(t, "127.0.0.1:0")

			roots := h.serverCA.pool
			if c.roots != nil {
				roots = c.roots(h)
			}
			cert := c.client(h, t)
			cfg, err := ClientTLS(cert.tls, roots, serverName)
			if err != nil {
				t.Fatal(err)
			}
			if c.maxVer != 0 {
				cfg.MinVersion, cfg.MaxVersion = tls.VersionTLS12, c.maxVer
			}
			_, err = echo(h.dialCfg(t, cfg), 1)
			if strings.HasPrefix(c.name, "control") {
				if err != nil {
					t.Fatalf("control failed: %v", err)
				}
				return
			}
			mustCode(t, err, codes.Unavailable)
			if n := h.callCount(); n != 0 {
				t.Fatalf("handler ran %d times for a rejected connection", n)
			}
		})
	}
}

func TestRevokedSerialRejectedAtHandshake(t *testing.T) {
	h := newHub(t)
	c := h.deviceCA.client(t, "agent-revoked")
	h.rev.Revoke(c.serial)
	_, err := echo(h.dial(t, c), 1)
	mustCode(t, err, codes.Unavailable)
	if h.callCount() != 0 {
		t.Fatal("handler ran for a revoked certificate")
	}
	// An unrelated certificate still works.
	if _, err := echo(h.dial(t, h.deviceCA.client(t, "agent-ok")), 1); err != nil {
		t.Fatal(err)
	}
}

type liveStream struct {
	s spikepb.SpikeService_ConnectClient
}

func openStream(t *testing.T, conn *grpc.ClientConn) liveStream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s, err := spikepb.NewSpikeServiceClient(conn).Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return liveStream{s}
}

func (l liveStream) roundTrip(seq uint64) (*spikepb.Pong, error) {
	if err := l.s.Send(&spikepb.Ping{Seq: seq}); err != nil {
		return nil, err
	}
	return l.s.Recv()
}

func TestRevokingClosesOnlyThatClientsStream(t *testing.T) {
	h := newHub(t)
	a, b := h.deviceCA.client(t, "agent-a"), h.deviceCA.client(t, "agent-b")
	connA := h.dial(t, a)
	sa, sb := openStream(t, connA), openStream(t, h.dial(t, b))

	for i, s := range []liveStream{sa, sb} {
		p, err := s.roundTrip(uint64(i + 1))
		if err != nil || p.Seq != uint64(i+1) {
			t.Fatalf("stream %d warm-up: %v %v", i, p, err)
		}
	}

	h.rev.Revoke(a.serial)

	done := make(chan error, 1)
	go func() {
		_, err := sa.s.Recv()
		done <- err
	}()
	select {
	case err := <-done:
		mustCode(t, err, codes.Unauthenticated)
	case <-time.After(5 * time.Second):
		t.Fatal("revoked stream was not closed")
	}

	// Client B is unaffected.
	if p, err := sb.roundTrip(99); err != nil || p.Seq != 99 || p.Peer != "agent-b" {
		t.Fatalf("stream B broken after revoking A: %v %v", p, err)
	}
	// A cannot use the still-open connection for new calls either...
	_, err := echo(connA, 1)
	mustCode(t, err, codes.Unauthenticated)
	// ...nor open a new connection.
	_, err = echo(h.dial(t, a), 1)
	mustCode(t, err, codes.Unavailable)
}

func TestDeadlineExceeded(t *testing.T) {
	h := newHub(t)
	h.svc.echoWait = func(ctx context.Context) { <-ctx.Done() }
	conn := h.dial(t, h.deviceCA.client(t, "agent-a"))
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := spikepb.NewSpikeServiceClient(conn).Echo(ctx, &spikepb.EchoRequest{})
	mustCode(t, err, codes.DeadlineExceeded)
}

func TestMaxMessageSize(t *testing.T) {
	h := newHub(t)
	conn := h.dial(t, h.deviceCA.client(t, "agent-a"))
	if r, err := echo(conn, MaxMessageSize-1024); err != nil || r.PayloadSize != MaxMessageSize-1024 {
		t.Fatalf("message just under the limit: %v %v", r, err)
	}
	_, err := echo(conn, MaxMessageSize+1)
	mustCode(t, err, codes.ResourceExhausted)
	// The connection remains usable.
	if _, err := echo(conn, 10); err != nil {
		t.Fatalf("connection unusable after oversize message: %v", err)
	}
}

func TestClientReconnectsAfterServerRestart(t *testing.T) {
	h := newHub(t)
	conn := h.dial(t, h.deviceCA.client(t, "agent-a"))
	s := openStream(t, conn)
	if _, err := s.roundTrip(1); err != nil {
		t.Fatal(err)
	}
	if _, err := echo(conn, 1); err != nil {
		t.Fatal(err)
	}

	addr := h.addr
	h.stop()
	if _, err := s.s.Recv(); status.Code(err) != codes.Unavailable {
		t.Fatalf("open stream after server stop: %v, want Unavailable", err)
	}
	_, err := echo(conn, 1)
	mustCode(t, err, codes.Unavailable)

	restart := time.AfterFunc(300*time.Millisecond, func() {
		// start uses t.Fatal on failure, which must not run off the test goroutine.
		cfg, err := ServerTLS(h.srvCert.tls, h.deviceCA.pool, h.rev)
		if err != nil {
			return
		}
		var lis net.Listener
		for i := 0; i < 50; i++ {
			if lis, err = net.Listen("tcp", addr); err == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if err != nil {
			return
		}
		srv := NewServer(cfg, h.svc)
		t.Cleanup(srv.Stop)
		go func() { _ = srv.Serve(lis) }()
	})
	defer restart.Stop()

	begin := time.Now()
	r, err := echo(conn, 1, grpc.WaitForReady(true))
	if err != nil || r.Peer != "agent-a" {
		t.Fatalf("no reconnect after restart: %v %v", r, err)
	}
	if d := time.Since(begin); d > 4*time.Second {
		t.Fatalf("reconnect took %v with max backoff %v", d, backoffMax)
	}
	s2 := openStream(t, conn)
	if _, err := s2.roundTrip(7); err != nil {
		t.Fatalf("new stream after reconnect: %v", err)
	}
}

// TestClientInSeparateProcess re-executes this test binary as the client so the
// server and the client are two different OS processes.
func TestClientInSeparateProcess(t *testing.T) {
	h := newHub(t)
	client := h.deviceCA.client(t, "agent-proc")
	caPath, certPath, keyPath := writeClientFiles(t, t.TempDir(), h.serverCA, client)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHelperClientProcess$", "-test.v")
	cmd.Env = append(os.Environ(),
		"SAKURA_SPIKE_HELPER=1",
		"SAKURA_SPIKE_ADDR="+h.addr,
		"SAKURA_SPIKE_CA="+caPath,
		"SAKURA_SPIKE_CERT="+certPath,
		"SAKURA_SPIKE_KEY="+keyPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper process failed: %v\n%s", err, out)
	}
	var pid, pongs int
	var peer string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "HELPER-RESULT ") {
			if _, err := fmt.Sscanf(line, "HELPER-RESULT pid=%d peer=%s pongs=%d", &pid, &peer, &pongs); err != nil {
				t.Fatalf("bad result line %q: %v", line, err)
			}
		}
	}
	if pid == 0 || pid == os.Getpid() {
		t.Fatalf("client pid = %d, server pid = %d: not a separate process\n%s", pid, os.Getpid(), out)
	}
	if peer != "agent-proc" || pongs != 3 {
		t.Fatalf("peer=%q pongs=%d\n%s", peer, pongs, out)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.calls) != 2 || h.calls[0].Name != "agent-proc" {
		t.Fatalf("server did not see the helper identity: %+v", h.calls)
	}
}

// TestHelperClientProcess is the client side of TestClientInSeparateProcess. It
// does nothing unless launched by that test.
func TestHelperClientProcess(t *testing.T) {
	if os.Getenv("SAKURA_SPIKE_HELPER") != "1" {
		return // not a real test: only meaningful when re-executed by the parent
	}
	caPEM, err := os.ReadFile(os.Getenv("SAKURA_SPIKE_CA"))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("bad CA file")
	}
	cert, err := tls.LoadX509KeyPair(os.Getenv("SAKURA_SPIKE_CERT"), os.Getenv("SAKURA_SPIKE_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ClientTLS(cert, roots, serverName)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := Dial(os.Getenv("SAKURA_SPIKE_ADDR"), cfg, backoffBase, backoffMax)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	resp, err := echo(conn, 4)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := spikepb.NewSpikeServiceClient(conn).Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pongs := 0
	for i := uint64(1); i <= 3; i++ {
		if err := s.Send(&spikepb.Ping{Seq: i}); err != nil {
			t.Fatal(err)
		}
		p, err := s.Recv()
		if err != nil || p.Seq != i {
			t.Fatalf("pong %d: %v %v", i, p, err)
		}
		pongs++
	}
	_ = s.CloseSend()
	if _, err := s.Recv(); !errors.Is(err, io.EOF) {
		t.Fatalf("stream end: %v", err)
	}
	fmt.Printf("HELPER-RESULT pid=%d peer=%s pongs=%d\n", os.Getpid(), resp.Peer, pongs)
}
