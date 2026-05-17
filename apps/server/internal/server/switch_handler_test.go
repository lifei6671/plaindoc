package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSwitchHandlerServesInitialAndSwappedHandler(t *testing.T) {
	handler := NewSwitchHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("initial"))
	}))

	firstRecorder := httptest.NewRecorder()
	handler.ServeHTTP(firstRecorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if firstRecorder.Body.String() != "initial" {
		t.Fatalf("expected initial handler response, got %q", firstRecorder.Body.String())
	}

	handler.Set(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ready"))
	}))

	secondRecorder := httptest.NewRecorder()
	handler.ServeHTTP(secondRecorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if secondRecorder.Body.String() != "ready" {
		t.Fatalf("expected swapped handler response, got %q", secondRecorder.Body.String())
	}
}

func TestSwitchHandlerSwapsDifferentConcreteHandlerTypes(t *testing.T) {
	handler := NewSwitchHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("initial"))
	}))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ready"))
	})

	handler.Set(mux)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Body.String() != "ready" {
		t.Fatalf("expected swapped mux response, got %q", recorder.Body.String())
	}
}
