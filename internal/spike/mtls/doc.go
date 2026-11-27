// Package mtls is a NON-NORMATIVE technical spike (NEXT-002/NEXT-004): gRPC over
// mutual TLS 1.3 between two Go processes, with certificate revocation that also
// terminates open streams. It is not the agent protocol and carries no task
// execution; its wire schema (spike.proto) must not be reused by NEXT-006.
package mtls
