package agentv1_test

import (
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/MiyazakiTakara/sakura-ops/internal/agentproto/agentv1"
)

var ver = &agentv1.ProtocolVersion{Major: 1, Minor: 0}

func ts() *timestamppb.Timestamp {
	return timestamppb.New(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
}

func agentBodies() map[string]*agentv1.AgentMessage {
	return map[string]*agentv1.AgentMessage{
		"hello": {Version: ver, Body: &agentv1.AgentMessage_Hello{Hello: &agentv1.Hello{
			AgentId: "a", AgentVersion: "0.1", BuildDigest: "sha256:00", BootId: "b",
			ConnectionNonce: []byte{1, 2, 3}, Capabilities: []string{"action/x@1"},
			Resume: []*agentv1.ResumeEntry{{CommandId: "c", LastEventSeq: 4, ResultPending: true}},
		}}},
		"heartbeat": {Version: ver, Body: &agentv1.AgentMessage_Heartbeat{Heartbeat: &agentv1.Heartbeat{
			Epoch: 2, Seq: 3, BootId: "b", ActiveCommandIds: []string{"c1", "c2"}, SentAt: ts(),
		}}},
		"command_ack": {Version: ver, Body: &agentv1.AgentMessage_CommandAck{CommandAck: &agentv1.CommandAck{
			CommandId: "c", Disposition: agentv1.CommandAck_DISPOSITION_DUPLICATE, Code: "dup",
		}}},
		"command_event": {Version: ver, Body: &agentv1.AgentMessage_CommandEvent{CommandEvent: &agentv1.CommandEvent{
			CommandId: "c", EventSeq: 7, Kind: "step", Metadata: []byte(`{"a":1}`), MetadataSchema: "s/1", ObservedAt: ts(),
		}}},
		"command_result": {Version: ver, Body: &agentv1.AgentMessage_CommandResult{CommandResult: &agentv1.CommandResult{
			CommandId: "c", EventSeq: 8, Outcome: agentv1.CommandResult_OUTCOME_SUCCEEDED,
			Result: []byte(`{"ok":true}`), ResultSchema: "r/1", Effect: agentv1.CommandResult_EFFECT_NONE, ObservedAt: ts(),
		}}},
		"agent_protocol_error": {Version: ver, Body: &agentv1.AgentMessage_ProtocolError{ProtocolError: &agentv1.ProtocolError{
			Code: agentv1.ProtocolError_CODE_INVALID_FIELD, CorrelationId: "x",
		}}},
	}
}

func hubBodies() map[string]*agentv1.HubMessage {
	return map[string]*agentv1.HubMessage{
		"server_hello": {Version: ver, Body: &agentv1.HubMessage_ServerHello{ServerHello: &agentv1.ServerHello{
			Status: agentv1.ServerHello_STATUS_OK, Negotiated: ver, ConnectionEpoch: 9,
			HeartbeatIntervalSeconds: 30, MaxFrameBytes: 256 * 1024, ServerTime: ts(),
		}}},
		"command_offer": {Version: ver, Body: &agentv1.HubMessage_CommandOffer{CommandOffer: &agentv1.CommandOffer{
			Envelope: []byte(`{"command_id":"c"}`), Signature: []byte{9, 9}, KeyId: "k1", Epoch: 9,
		}}},
		"cancel_request": {Version: ver, Body: &agentv1.HubMessage_CancelRequest{CancelRequest: &agentv1.CancelRequest{
			Envelope: []byte(`{"command_id":"c"}`), Signature: []byte{8}, KeyId: "k1",
		}}},
		"result_ack": {Version: ver, Body: &agentv1.HubMessage_ResultAck{ResultAck: &agentv1.ResultAck{
			CommandId: "c", HighestCommittedSeq: 8,
		}}},
		"drain": {Version: ver, Body: &agentv1.HubMessage_Drain{Drain: &agentv1.Drain{ReasonCode: "maintenance"}}},
		"hub_protocol_error": {Version: ver, Body: &agentv1.HubMessage_ProtocolError{ProtocolError: &agentv1.ProtocolError{
			Code: agentv1.ProtocolError_CODE_STALE_EPOCH, CorrelationId: "y",
		}}},
	}
}

func TestAgentMessageRoundTrip(t *testing.T) {
	for name, in := range agentBodies() {
		t.Run(name, func(t *testing.T) {
			b, err := proto.Marshal(in)
			if err != nil {
				t.Fatal(err)
			}
			out := &agentv1.AgentMessage{}
			if err := proto.Unmarshal(b, out); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(in, out) {
				t.Fatalf("round trip changed the message:\n in=%v\nout=%v", in, out)
			}
			if out.Body == nil {
				t.Fatal("body lost")
			}
		})
	}
}

func TestHubMessageRoundTrip(t *testing.T) {
	for name, in := range hubBodies() {
		t.Run(name, func(t *testing.T) {
			b, err := proto.Marshal(in)
			if err != nil {
				t.Fatal(err)
			}
			out := &agentv1.HubMessage{}
			if err := proto.Unmarshal(b, out); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(in, out) {
				t.Fatalf("round trip changed the message:\n in=%v\nout=%v", in, out)
			}
		})
	}
}

// A newer minor may add oneof cases. An old receiver parses the frame but sees no known
// case, so a switch on Body hits its default branch and can fail closed.
func TestUnknownOneofCaseIsRepresentable(t *testing.T) {
	var frame []byte
	frame = protowire.AppendTag(frame, 1, protowire.BytesType) // version
	vb, _ := proto.Marshal(ver)
	frame = protowire.AppendBytes(frame, vb)
	frame = protowire.AppendTag(frame, 28, protowire.BytesType) // case unknown to v1.0
	frame = protowire.AppendBytes(frame, []byte("future"))

	for name, msg := range map[string]interface {
		proto.Message
		GetVersion() *agentv1.ProtocolVersion
	}{"agent": &agentv1.AgentMessage{}, "hub": &agentv1.HubMessage{}} {
		if err := proto.Unmarshal(frame, msg); err != nil {
			t.Fatalf("%s: unknown field must still parse: %v", name, err)
		}
		if msg.GetVersion().GetMajor() != 1 {
			t.Fatalf("%s: version lost", name)
		}
		if len(msg.ProtoReflect().GetUnknown()) == 0 {
			t.Fatalf("%s: unknown case not preserved", name)
		}
	}
	a := &agentv1.AgentMessage{}
	_ = proto.Unmarshal(frame, a)
	switch a.Body.(type) {
	case nil:
		// Fail-closed path: no known body.
	default:
		t.Fatalf("unknown case must not map to a known body, got %T", a.Body)
	}
	// An empty envelope has no body either.
	if (&agentv1.HubMessage{Version: ver}).Body != nil {
		t.Fatal("unset oneof must be nil")
	}
}

// Zero values of every enum are the *_UNSPECIFIED sentinel, so an omitted field is never
// mistaken for a real decision.
func TestEnumZeroValuesAreUnspecified(t *testing.T) {
	zeros := map[string]string{
		"ServerHello.Status":     agentv1.ServerHello_Status(0).String(),
		"CommandAck.Disposition": agentv1.CommandAck_Disposition(0).String(),
		"CommandResult.Outcome":  agentv1.CommandResult_Outcome(0).String(),
		"CommandResult.Effect":   agentv1.CommandResult_Effect(0).String(),
		"ProtocolError.Code":     agentv1.ProtocolError_Code(0).String(),
	}
	for n, v := range zeros {
		if len(v) < 12 || v[len(v)-11:] != "UNSPECIFIED" {
			t.Errorf("%s zero value = %q, want *_UNSPECIFIED", n, v)
		}
	}
}
