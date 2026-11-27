package mtls

import (
	"crypto/tls"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
)

// Dial creates a client connection over the mutual-TLS configuration. Reconnects
// use exponential backoff with jitter bounded by [base, maxDelay]; the proposed
// production range is 2s-60s (docs/03-architecture.md), tests pass smaller
// values. The connection is lazy: it dials on first use.
func Dial(target string, tlsCfg *tls.Config, base, maxDelay time.Duration) (*grpc.ClientConn, error) {
	return grpc.NewClient(target,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  base,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   maxDelay,
			},
			MinConnectTimeout: 2 * time.Second,
		}),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize)),
	)
}
