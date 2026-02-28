package captchastore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mojocn/base64Captcha"
)

var _ base64Captcha.Store = (*StandardStore)(nil)

func TestStandardStore_WithMemoryBackend(t *testing.T) {
	current := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return current }

	backend := NewMemoryBackend(MemoryBackendOptions{
		CollectEvery: 1,
		NowFunc:      nowFn,
	})
	store, err := New(
		backend,
		WithDefaultTTL(10*time.Minute),
		WithNowFunc(nowFn),
	)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	t.Run("SetAndGet", func(t *testing.T) {
		if err := store.Set("captcha-1", "AbCd"); err != nil {
			t.Fatalf("set failed: %v", err)
		}
		value := store.Get("captcha-1", false)
		if value != "AbCd" {
			t.Fatalf("expected value AbCd, got %q", value)
		}
	})

	t.Run("VerifyIgnoreCaseKeep", func(t *testing.T) {
		if err := store.Set("captcha-2", "QwEr"); err != nil {
			t.Fatalf("set failed: %v", err)
		}
		if ok := store.Verify("captcha-2", "qwer", false); !ok {
			t.Fatal("expected verify success")
		}
		if value := store.Get("captcha-2", false); value != "QwEr" {
			t.Fatalf("expected value QwEr, got %q", value)
		}
	})

	t.Run("VerifyMismatchClear", func(t *testing.T) {
		if err := store.Set("captcha-3", "ZXCV"); err != nil {
			t.Fatalf("set failed: %v", err)
		}
		if ok := store.Verify("captcha-3", "wrong", true); ok {
			t.Fatal("expected verify mismatch")
		}
		if value := store.Get("captcha-3", false); value != "" {
			t.Fatalf("expected value cleared, got %q", value)
		}
	})

	t.Run("TTLExpired", func(t *testing.T) {
		if err := store.SetWithContext(context.Background(), SetInput{
			ID:    "captcha-4",
			Value: "1234",
			TTL:   30 * time.Second,
		}); err != nil {
			t.Fatalf("set with context failed: %v", err)
		}
		current = current.Add(31 * time.Second)

		result, getErr := store.GetWithContext(context.Background(), GetInput{
			ID: "captcha-4",
		})
		if getErr != nil {
			t.Fatalf("get with context failed: %v", getErr)
		}
		if result.Found {
			t.Fatalf("expected expired captcha not found, got %+v", result)
		}
	})
}

func TestDatabaseBackend(t *testing.T) {
	repository := &fakeDatabaseRepository{
		records: map[string]Record{},
	}
	backend, err := NewDatabaseBackend(repository)
	if err != nil {
		t.Fatalf("new database backend failed: %v", err)
	}

	ctx := context.Background()
	if err := backend.Set(ctx, Record{ID: "db-1", Value: "A1B2"}); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	record, found, err := backend.Get(ctx, "db-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !found || record.Value != "A1B2" {
		t.Fatalf("unexpected get result: found=%v record=%+v", found, record)
	}

	if err := backend.Delete(ctx, "db-1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	_, found, err = backend.Get(ctx, "db-1")
	if err != nil {
		t.Fatalf("get after delete failed: %v", err)
	}
	if found {
		t.Fatal("expected db record deleted")
	}
}

func TestRedisBackend(t *testing.T) {
	current := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return current }

	client := &fakeRedisClient{
		values: map[string]fakeRedisValue{},
		nowFn:  nowFn,
	}
	backend, err := NewRedisBackend(client, RedisBackendOptions{
		KeyPrefix: "risk-captcha",
		NowFunc:   nowFn,
	})
	if err != nil {
		t.Fatalf("new redis backend failed: %v", err)
	}

	ctx := context.Background()
	if err := backend.Set(ctx, Record{
		ID:        "redis-1",
		Value:     "QWER",
		ExpiresAt: current.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	record, found, err := backend.Get(ctx, "redis-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !found || record.Value != "QWER" {
		t.Fatalf("unexpected redis get result: found=%v record=%+v", found, record)
	}

	current = current.Add(3 * time.Minute)
	_, found, err = backend.Get(ctx, "redis-1")
	if err != nil {
		t.Fatalf("get expired failed: %v", err)
	}
	if found {
		t.Fatal("expected redis key expired")
	}
}

type fakeDatabaseRepository struct {
	records map[string]Record
}

func (f *fakeDatabaseRepository) UpsertCaptchaRecord(_ context.Context, record Record) error {
	if f == nil {
		return errors.New("fake database repository is nil")
	}
	if f.records == nil {
		f.records = map[string]Record{}
	}
	f.records[record.ID] = record
	return nil
}

func (f *fakeDatabaseRepository) GetCaptchaRecordByID(_ context.Context, id string) (Record, bool, error) {
	if f == nil {
		return Record{}, false, errors.New("fake database repository is nil")
	}
	record, found := f.records[id]
	return record, found, nil
}

func (f *fakeDatabaseRepository) DeleteCaptchaRecordByID(_ context.Context, id string) error {
	if f == nil {
		return errors.New("fake database repository is nil")
	}
	delete(f.records, id)
	return nil
}

type fakeRedisValue struct {
	value     string
	expiresAt time.Time
}

type fakeRedisClient struct {
	values map[string]fakeRedisValue
	nowFn  func() time.Time
}

func (f *fakeRedisClient) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	if f == nil {
		return errors.New("fake redis client is nil")
	}
	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return errors.New("redis key is required")
	}
	expiresAt := time.Time{}
	if ttl > 0 {
		expiresAt = f.currentTime().Add(ttl)
	}
	if f.values == nil {
		f.values = map[string]fakeRedisValue{}
	}
	f.values[normalizedKey] = fakeRedisValue{
		value:     value,
		expiresAt: expiresAt,
	}
	return nil
}

func (f *fakeRedisClient) Get(_ context.Context, key string) (string, bool, error) {
	if f == nil {
		return "", false, errors.New("fake redis client is nil")
	}
	normalizedKey := strings.TrimSpace(key)
	value, found := f.values[normalizedKey]
	if !found {
		return "", false, nil
	}
	if !value.expiresAt.IsZero() && !value.expiresAt.After(f.currentTime()) {
		delete(f.values, normalizedKey)
		return "", false, nil
	}
	return value.value, true, nil
}

func (f *fakeRedisClient) Del(_ context.Context, key string) error {
	if f == nil {
		return errors.New("fake redis client is nil")
	}
	delete(f.values, strings.TrimSpace(key))
	return nil
}

func (f *fakeRedisClient) currentTime() time.Time {
	if f == nil || f.nowFn == nil {
		return time.Now().UTC()
	}
	return f.nowFn().UTC()
}
