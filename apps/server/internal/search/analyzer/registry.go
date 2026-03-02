package analyzer

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
)

var (
	// ErrProviderNotFound 表示分词器未注册。
	ErrProviderNotFound = errors.New("analyzer provider not found")
)

// Registry 维护 analyzer provider 注册与查询。
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry 创建 analyzer registry。
func NewRegistry(providers ...Provider) (*Registry, error) {
	registry := &Registry{
		providers: make(map[string]Provider, len(providers)),
	}
	for _, provider := range providers {
		if err := registry.Register(provider); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// Register 注册 provider。
func (registry *Registry) Register(provider Provider) error {
	if registry == nil {
		return errors.New("analyzer registry is nil")
	}
	if provider == nil {
		return errors.New("analyzer provider is nil")
	}
	normalizedName := strings.ToLower(strings.TrimSpace(provider.Name()))
	if normalizedName == "" {
		return errors.New("analyzer provider name is required")
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.providers[normalizedName]; exists {
		return fmt.Errorf("analyzer provider %q already registered", normalizedName)
	}
	registry.providers[normalizedName] = provider
	return nil
}

// Get 返回指定名称的 provider。
func (registry *Registry) Get(name string) (Provider, error) {
	if registry == nil {
		return nil, errors.New("analyzer registry is nil")
	}
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	if normalizedName == "" {
		return nil, fmt.Errorf("%w: empty name", ErrProviderNotFound)
	}

	registry.mu.RLock()
	provider, exists := registry.providers[normalizedName]
	registry.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, normalizedName)
	}
	return provider, nil
}

// Names 返回当前已注册 provider 名称集合。
func (registry *Registry) Names() []string {
	if registry == nil {
		return []string{}
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	names := make([]string, 0, len(registry.providers))
	for name := range registry.providers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
