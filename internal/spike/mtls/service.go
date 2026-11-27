package mtls

import (
	"context"
	"crypto/tls"
	"errors"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/MiyazakiTakara/sakura-ops/internal/spike/mtls/spikepb"
)

// PeerInfo is what the server verified about the calling connection.
type PeerInfo struct {
	Name       string // client certificate common name
	Serial     string // lower-case hex
	TLSVersion uint16
	ALPN       string
}

// PeerFromContext extracts the verified identity of the caller. It fails for
// anything but a TLS connection with a verified client certificate chain.
func PeerFromContext(ctx context.Context) (PeerInfo, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return PeerInfo{}, errors.New("mtls: no peer")
	}
	ti, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(ti.State.VerifiedChains) == 0 || len(ti.State.PeerCertificates) == 0 {
		return PeerInfo{}, errors.New("mtls: no verified client certificate")
	}
	leaf := ti.State.PeerCertificates[0]
	return PeerInfo{
		Name:       leaf.Subject.CommonName,
		Serial:     serialOf(leaf),
		TLSVersion: ti.State.Version,
		ALPN:       ti.State.NegotiatedProtocol,
	}, nil
}

// Service implements spikepb.SpikeServiceServer. It only echoes.
type Service struct {
	spikepb.UnimplementedSpikeServiceServer
	rev *Revoker

	// Test hooks (nil in production use).
	onCall   func(PeerInfo)
	echoWait func(ctx context.Context)
}

// NewService returns a Service that terminates streams of revoked certificates.
func NewService(rev *Revoker) *Service { return &Service{rev: rev} }

func (s *Service) Echo(ctx context.Context, req *spikepb.EchoRequest) (*spikepb.EchoResponse, error) {
	pi, err := PeerFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	if s.rev.IsRevoked(pi.Serial) { // the connection may predate the revocation
		return nil, status.Error(codes.Unauthenticated, "client certificate revoked")
	}
	if s.onCall != nil {
		s.onCall(pi)
	}
	if s.echoWait != nil {
		s.echoWait(ctx)
	}
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	return &spikepb.EchoResponse{PayloadSize: uint64(len(req.GetPayload())), Peer: pi.Name}, nil
}

func (s *Service) Connect(stream grpc.BidiStreamingServer[spikepb.Ping, spikepb.Pong]) error {
	pi, err := PeerFromContext(stream.Context())
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}
	if s.onCall != nil {
		s.onCall(pi)
	}
	revoked, done := s.rev.Track(pi.Serial)
	defer done()

	type recv struct {
		ping *spikepb.Ping
		err  error
	}
	// Receiving is done in its own goroutine so a revocation can end the handler
	// even while Recv blocks. The channel is buffered: after the handler returns
	// gRPC cancels the stream, Recv fails and the goroutine exits without Send.
	in := make(chan recv, 1)
	go func() {
		for {
			p, err := stream.Recv()
			in <- recv{p, err}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-revoked:
			return status.Error(codes.Unauthenticated, "client certificate revoked")
		case r := <-in:
			if errors.Is(r.err, io.EOF) {
				return nil
			}
			if r.err != nil {
				return r.err
			}
			if err := stream.Send(&spikepb.Pong{Seq: r.ping.GetSeq(), Peer: pi.Name}); err != nil {
				return err
			}
		}
	}
}

// NewServer builds a gRPC server over the mutual-TLS configuration with the
// 256 KiB receive limit.
func NewServer(tlsCfg *tls.Config, svc spikepb.SpikeServiceServer) *grpc.Server {
	srv := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsCfg)),
		grpc.MaxRecvMsgSize(MaxMessageSize),
		grpc.MaxSendMsgSize(MaxMessageSize),
	)
	spikepb.RegisterSpikeServiceServer(srv, svc)
	return srv
}
