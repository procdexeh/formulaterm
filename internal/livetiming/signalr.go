package livetiming

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const RecordSeparator = "\x1e"

type MessageType int

const (
	MessageInvocation MessageType = 1
	MessageStreamItem MessageType = 2
	MessageCompletion MessageType = 3
	MessagePing       MessageType = 6
	MessageClose      MessageType = 6
)

type Envelope struct {
	Type         MessageType                `json:"type"`
	Target       string                     `json:"target,omitempty"`
	InvocationID string                     `json:"invocationId,omitempty"`
	Error        string                     `json:"error,omitempty"`
	Result       map[string]json.RawMessage `json:"Result,omitempty"`
	Arguments    []json.RawMessage          `json:"Arguments,omitempty"`
}

type NegotiateRespnse struct {
	ConnectionID    string `json:"connectionId"`
	ConnectionToken string `json:"connectionToken"`
}

// splits a payload with 0x1e, discards empty trailing segments
func SplitRecords(data []byte) [][]byte {
	parts := bytes.Split(data, []byte(RecordSeparator))
	out := make([][]byte, 0, len(parts))

	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		out = append(out, p)
	}

	return out
}

func DecodeEnvelope(record []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(record, &env); err != nil {
		return &Envelope{}, fmt.Errorf("signalr envelope decode: %w", err)
	}

	return &env, nil
}

// one ws message -> zero or more envelopes
func DecodeFrame(data []byte) ([]*Envelope, error) {
	records := SplitRecords(data)
	out := make([]*Envelope, 0, len(records))

	for _, r := range records {
		env, err := DecodeEnvelope(r)
		if err != nil {
			return nil, err
		}
		out = append(out, env)
	}

	return out, nil
}

func EncodeRecord(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return append(b, RecordSeparator...), nil
}

func HandshakeRequest() []byte {
	return []byte(`{"protocol":"json", "version":1}` + RecordSeparator)
}

func IsHandshakeResponse(msg []byte) bool {
	msg = bytes.TrimSuffix(msg, []byte(RecordSeparator))
	var obj map[string]any
	return json.Unmarshal(msg, &obj) == nil && len(obj) == 0
}

func SubscribeInvocation(invocationId string, topics []string) (map[string]any, error) {
	return map[string]any{
		"type":         MessageInvocation,
		"invocationId": invocationId,
		"target":       "Subscribe",
		"arguments":    [][]string{topics},
	}, nil
}

func PingPayload() []byte {
	return fmt.Appendf(nil, `{"type":%d}%s`, MessagePing, RecordSeparator)
}
