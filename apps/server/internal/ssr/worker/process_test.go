package worker

import (
	"reflect"
	"testing"
)

func TestBuildSSRWorkerCommandArgsDisablesNodeOptimization(t *testing.T) {
	t.Parallel()

	args := buildSSRWorkerCommandArgs(Config{
		Exec:  "node",
		Entry: "worker-entry.js",
	})

	expected := []string{"--no-opt", "worker-entry.js"}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("expected node worker args %#v, got %#v", expected, args)
	}
}

func TestBuildSSRWorkerCommandArgsKeepsScriptArgsAfterEntry(t *testing.T) {
	t.Parallel()

	args := buildSSRWorkerCommandArgs(Config{
		Exec:  "/usr/bin/node",
		Entry: "worker-entry.js",
		Args:  []string{"--debug-payload"},
	})

	expected := []string{"--no-opt", "worker-entry.js", "--debug-payload"}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("expected node worker args %#v, got %#v", expected, args)
	}
}

func TestBuildSSRWorkerCommandArgsDoesNotInjectNodeFlagsForCustomRuntime(t *testing.T) {
	t.Parallel()

	args := buildSSRWorkerCommandArgs(Config{
		Exec:  "custom-js-runtime",
		Entry: "worker-entry.js",
		Args:  []string{"--debug-payload"},
	})

	expected := []string{"worker-entry.js", "--debug-payload"}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("expected custom runtime args %#v, got %#v", expected, args)
	}
}
