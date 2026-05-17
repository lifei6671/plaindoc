package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestStdioCodecReadJSONAllowsLargeResponseWithinConfiguredLimit(t *testing.T) {
	payload := struct {
		HTML string `json:"html"`
	}{
		HTML: strings.Repeat("a", 2*1024*1024),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}
	encoded = append(encoded, '\n')

	codec := newStdioCodec(bytes.NewReader(encoded), io.Discard, 4*1024*1024)
	var decoded struct {
		HTML string `json:"html"`
	}
	if err := codec.ReadJSON(&decoded); err != nil {
		t.Fatalf("ReadJSON() returned error: %v", err)
	}
	if decoded.HTML != payload.HTML {
		t.Fatalf("expected decoded html length %d, got %d", len(payload.HTML), len(decoded.HTML))
	}
}

func TestStdioCodecReadJSONRejectsResponseOverConfiguredLimit(t *testing.T) {
	payload := struct {
		HTML string `json:"html"`
	}{
		HTML: strings.Repeat("a", 2*1024*1024),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}
	encoded = append(encoded, '\n')

	codec := newStdioCodec(bytes.NewReader(encoded), io.Discard, 1024*1024)
	var decoded struct {
		HTML string `json:"html"`
	}
	err = codec.ReadJSON(&decoded)
	if !errors.Is(err, errCodecLineTooLarge) {
		t.Fatalf("expected errCodecLineTooLarge, got %v", err)
	}
}
