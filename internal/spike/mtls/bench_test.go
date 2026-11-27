package mtls

import (
	"crypto/tls"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
)

// BenchmarkMutualTLSHandshake measures one full TLS 1.3 mutual-auth handshake
// (TCP connect included) over loopback, client and server in one process.
func BenchmarkMutualTLSHandshake(b *testing.B) {
	serverCA, deviceCA := newCA(b, "server CA"), newCA(b, "device CA")
	srvCfg, err := ServerTLS(serverCA.server(b).tls, deviceCA.pool, NewRevoker())
	if err != nil {
		b.Fatal(err)
	}
	cliCfg, err := ClientTLS(deviceCA.client(b, "bench").tls, serverCA.pool, serverName)
	if err != nil {
		b.Fatal(err)
	}
	lis, err := tls.Listen("tcp", "127.0.0.1:0", srvCfg)
	if err != nil {
		b.Fatal(err)
	}
	defer lis.Close()
	go func() {
		for {
			c, err := lis.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				_ = c.(*tls.Conn).Handshake()
				buf := make([]byte, 1)
				_, _ = c.Read(buf) // wait for the client to close
			}()
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := net.Dialer{Timeout: 5 * time.Second}
		c, err := tls.DialWithDialer(&d, "tcp", lis.Addr().String(), cliCfg)
		if err != nil {
			b.Fatal(err)
		}
		_ = c.Close()
	}
}

// BenchmarkUnaryEchoOverMutualTLS measures a small unary call on an established
// mTLS gRPC connection (loopback, one process).
func BenchmarkUnaryEchoOverMutualTLS(b *testing.B) {
	serverCA, deviceCA := newCA(b, "server CA"), newCA(b, "device CA")
	rev := NewRevoker()
	srvCfg, err := ServerTLS(serverCA.server(b).tls, deviceCA.pool, rev)
	if err != nil {
		b.Fatal(err)
	}
	cliCfg, err := ClientTLS(deviceCA.client(b, "bench").tls, serverCA.pool, serverName)
	if err != nil {
		b.Fatal(err)
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	srv := NewServer(srvCfg, NewService(rev))
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()
	conn, err := Dial(lis.Addr().String(), cliCfg, backoffBase, backoffMax)
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()
	if _, err := echo(conn, 16, grpc.WaitForReady(true)); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := echo(conn, 16); err != nil {
			b.Fatal(err)
		}
	}
}
