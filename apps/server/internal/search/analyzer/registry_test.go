package analyzer

import (
	"context"
	"testing"
)

type stubAnalyzerProvider struct {
	name string
}

func (provider stubAnalyzerProvider) Name() string {
	return provider.name
}

func (provider stubAnalyzerProvider) Health(ctx context.Context) error {
	return ctx.Err()
}

func (provider stubAnalyzerProvider) AnalyzeForIndex(ctx context.Context, input AnalyzeInput) (AnalyzeOutput, error) {
	return AnalyzeOutput{}, ctx.Err()
}

func (provider stubAnalyzerProvider) AnalyzeForQuery(ctx context.Context, input AnalyzeInput) (AnalyzeOutput, error) {
	return AnalyzeOutput{}, ctx.Err()
}

func (provider stubAnalyzerProvider) Reload(ctx context.Context, dictVersion string) error {
	return ctx.Err()
}

func (provider stubAnalyzerProvider) Capabilities() Capabilities {
	return Capabilities{}
}

func TestRegistry_GetRegisteredProvider(t *testing.T) {
	registry, err := NewRegistry(NewSimpleAnalyzer("v1"))
	if err != nil {
		t.Fatalf("new registry failed: %v", err)
	}

	provider, err := registry.Get("simple")
	if err != nil {
		t.Fatalf("get provider failed: %v", err)
	}
	if provider.Name() != "simple" {
		t.Fatalf("expected provider simple, got %q", provider.Name())
	}
}

func TestRegistry_RegisterRejectsDuplicatedName(t *testing.T) {
	registry, err := NewRegistry(stubAnalyzerProvider{name: "dup"})
	if err != nil {
		t.Fatalf("new registry failed: %v", err)
	}

	err = registry.Register(stubAnalyzerProvider{name: "dup"})
	if err == nil {
		t.Fatalf("expected duplicate register error")
	}
}
