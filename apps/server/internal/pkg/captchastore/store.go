package captchastore

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	// DefaultTTL 定义未显式指定 TTL 时的默认有效期。
	DefaultTTL = 10 * time.Minute
)

// Base64Store 定义与 base64Captcha.Store 兼容的方法集合。
type Base64Store interface {
	Set(id string, value string) error
	Get(id string, clear bool) string
	Verify(id string, answer string, clear bool) bool
}

// SetInput 描述写入验证码答案的输入参数。
type SetInput struct {
	ID       string
	Value    string
	TTL      time.Duration
	Metadata map[string]string
}

// GetInput 描述读取验证码答案的输入参数。
type GetInput struct {
	ID    string
	Clear bool
}

// GetResult 描述读取验证码答案的输出。
type GetResult struct {
	Found     bool
	Value     string
	ExpiresAt time.Time
	Metadata  map[string]string
}

// VerifyInput 描述验证码答案校验输入。
type VerifyInput struct {
	ID     string
	Answer string
	Clear  bool
}

// Record 是后端持久化的标准结构。
type Record struct {
	ID        string
	Value     string
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	Metadata  map[string]string
}

// Backend 定义验证码存储后端最小能力，便于扩展 DB/Redis/Memory 等实现。
type Backend interface {
	Set(ctx context.Context, record Record) error
	Get(ctx context.Context, id string) (Record, bool, error)
	Delete(ctx context.Context, id string) error
}

// Store 在兼容 base64Captcha.Store 的基础上，补充了 context 版本的接口。
type Store interface {
	Base64Store
	SetWithContext(ctx context.Context, input SetInput) error
	GetWithContext(ctx context.Context, input GetInput) (GetResult, error)
	VerifyWithContext(ctx context.Context, input VerifyInput) (bool, error)
	DeleteWithContext(ctx context.Context, id string) error
}

// StandardStore 是统一验证码存储门面，实现了 Store 接口。
type StandardStore struct {
	backend    Backend
	defaultTTL time.Duration
	nowFn      func() time.Time
}

// StandardStoreOption 定义 StandardStore 的构造参数。
type StandardStoreOption func(*standardStoreOptions)

type standardStoreOptions struct {
	defaultTTL time.Duration
	nowFn      func() time.Time
}

var errBackendRequired = errors.New("captcha store backend is required")

// WithDefaultTTL 配置 Set() 默认有效期。
func WithDefaultTTL(ttl time.Duration) StandardStoreOption {
	return func(options *standardStoreOptions) {
		if options == nil {
			return
		}
		options.defaultTTL = ttl
	}
}

// WithNowFunc 注入当前时间函数，便于测试。
func WithNowFunc(nowFn func() time.Time) StandardStoreOption {
	return func(options *standardStoreOptions) {
		if options == nil || nowFn == nil {
			return
		}
		options.nowFn = nowFn
	}
}

// New 创建统一验证码存储。
func New(backend Backend, opts ...StandardStoreOption) (*StandardStore, error) {
	if backend == nil {
		return nil, errBackendRequired
	}
	options := standardStoreOptions{
		defaultTTL: DefaultTTL,
		nowFn: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return &StandardStore{
		backend:    backend,
		defaultTTL: options.defaultTTL,
		nowFn:      options.nowFn,
	}, nil
}

// Set 兼容 base64Captcha.Store.Set。
func (s *StandardStore) Set(id string, value string) error {
	return s.SetWithContext(context.Background(), SetInput{
		ID:    id,
		Value: value,
	})
}

// Get 兼容 base64Captcha.Store.Get。
func (s *StandardStore) Get(id string, clear bool) string {
	result, err := s.GetWithContext(context.Background(), GetInput{
		ID:    id,
		Clear: clear,
	})
	if err != nil || !result.Found {
		return ""
	}
	return result.Value
}

// Verify 兼容 base64Captcha.Store.Verify。
func (s *StandardStore) Verify(id string, answer string, clear bool) bool {
	ok, err := s.VerifyWithContext(context.Background(), VerifyInput{
		ID:     id,
		Answer: answer,
		Clear:  clear,
	})
	if err != nil {
		return false
	}
	return ok
}

// SetWithContext 写入验证码答案。
func (s *StandardStore) SetWithContext(ctx context.Context, input SetInput) error {
	if s == nil || s.backend == nil {
		return errBackendRequired
	}
	id := strings.TrimSpace(input.ID)
	value := strings.TrimSpace(input.Value)
	if id == "" {
		return errors.New("captcha id is required")
	}
	if value == "" {
		return errors.New("captcha value is required")
	}

	now := s.currentTime()
	ttl := input.TTL
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	expiresAt := time.Time{}
	if ttl > 0 {
		expiresAt = now.Add(ttl)
	}

	return s.backend.Set(ctx, Record{
		ID:        id,
		Value:     value,
		ExpiresAt: expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  cloneMetadata(input.Metadata),
	})
}

// GetWithContext 读取验证码答案。
func (s *StandardStore) GetWithContext(ctx context.Context, input GetInput) (GetResult, error) {
	if s == nil || s.backend == nil {
		return GetResult{}, errBackendRequired
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return GetResult{}, nil
	}

	record, found, err := s.backend.Get(ctx, id)
	if err != nil {
		return GetResult{}, err
	}
	if !found {
		return GetResult{}, nil
	}

	now := s.currentTime()
	if !record.ExpiresAt.IsZero() && !record.ExpiresAt.After(now) {
		_ = s.backend.Delete(ctx, id)
		return GetResult{}, nil
	}
	if input.Clear {
		if err := s.backend.Delete(ctx, id); err != nil {
			return GetResult{}, err
		}
	}

	return GetResult{
		Found:     true,
		Value:     record.Value,
		ExpiresAt: record.ExpiresAt,
		Metadata:  cloneMetadata(record.Metadata),
	}, nil
}

// VerifyWithContext 校验验证码答案。
func (s *StandardStore) VerifyWithContext(ctx context.Context, input VerifyInput) (bool, error) {
	id := strings.TrimSpace(input.ID)
	answer := strings.TrimSpace(input.Answer)
	if id == "" || answer == "" {
		return false, nil
	}

	result, err := s.GetWithContext(ctx, GetInput{
		ID:    id,
		Clear: input.Clear,
	})
	if err != nil {
		return false, err
	}
	if !result.Found {
		return false, nil
	}
	return strings.EqualFold(strings.TrimSpace(result.Value), answer), nil
}

// DeleteWithContext 删除验证码答案。
func (s *StandardStore) DeleteWithContext(ctx context.Context, id string) error {
	if s == nil || s.backend == nil {
		return errBackendRequired
	}
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return nil
	}
	return s.backend.Delete(ctx, normalizedID)
}

func (s *StandardStore) currentTime() time.Time {
	if s == nil || s.nowFn == nil {
		return time.Now().UTC()
	}
	return s.nowFn().UTC()
}

func cloneMetadata(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		result[normalizedKey] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
