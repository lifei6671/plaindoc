package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHandleMetaCommand_Version(t *testing.T) {
	var outputBuffer bytes.Buffer

	handled := handleMetaCommand([]string{"version"}, &outputBuffer)
	if !handled {
		t.Fatal("expected version command to be handled")
	}

	outputText := outputBuffer.String()
	if !strings.Contains(outputText, "version: ") {
		t.Fatalf("expected version line in output, got: %s", outputText)
	}
	if !strings.Contains(outputText, "commit_sha: ") {
		t.Fatalf("expected commit_sha line in output, got: %s", outputText)
	}
	if !strings.Contains(outputText, "build_time_utc: ") {
		t.Fatalf("expected build_time_utc line in output, got: %s", outputText)
	}
	if !strings.Contains(outputText, "go_version: ") {
		t.Fatalf("expected go_version line in output, got: %s", outputText)
	}
}

func TestHandleMetaCommand_UnknownCommand(t *testing.T) {
	var outputBuffer bytes.Buffer

	handled := handleMetaCommand([]string{"start"}, &outputBuffer)
	if handled {
		t.Fatal("expected unknown command not to be handled")
	}
	if outputBuffer.Len() != 0 {
		t.Fatalf("expected no output for unknown command, got: %s", outputBuffer.String())
	}
}
