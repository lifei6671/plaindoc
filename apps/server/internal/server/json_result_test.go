package server

import (
	"encoding/json"
	"testing"
)

type jsonResultEnvelopeForTest struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	RequestID string          `json:"requestId"`
	Data      json.RawMessage `json:"data"`
}

func decodeJSONResultData[T any](t *testing.T, raw []byte) T {
	t.Helper()

	var envelope jsonResultEnvelopeForTest
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode json result failed: %v body=%s", err, string(raw))
	}

	var payload T
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return payload
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("decode json result data failed: %v body=%s", err, string(raw))
	}
	return payload
}

func decodeJSONResultCode(t *testing.T, raw []byte) int {
	t.Helper()

	var envelope jsonResultEnvelopeForTest
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode json result code failed: %v body=%s", err, string(raw))
	}
	return envelope.Code
}
