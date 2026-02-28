package captchastore

import (
	"context"
	"errors"
)

// DatabaseRepository 定义数据库持久化扩展点。
type DatabaseRepository interface {
	UpsertCaptchaRecord(ctx context.Context, record Record) error
	GetCaptchaRecordByID(ctx context.Context, id string) (Record, bool, error)
	DeleteCaptchaRecordByID(ctx context.Context, id string) error
}

// DatabaseBackend 是数据库后端实现，适用于多实例共享状态场景。
type DatabaseBackend struct {
	repository DatabaseRepository
}

// NewDatabaseBackend 创建数据库后端。
func NewDatabaseBackend(repository DatabaseRepository) (*DatabaseBackend, error) {
	if repository == nil {
		return nil, errors.New("captcha database repository is required")
	}
	return &DatabaseBackend{repository: repository}, nil
}

// Set 写入数据库。
func (b *DatabaseBackend) Set(ctx context.Context, record Record) error {
	if b == nil || b.repository == nil {
		return errors.New("captcha database backend is not initialized")
	}
	return b.repository.UpsertCaptchaRecord(ctx, record)
}

// Get 查询数据库。
func (b *DatabaseBackend) Get(ctx context.Context, id string) (Record, bool, error) {
	if b == nil || b.repository == nil {
		return Record{}, false, errors.New("captcha database backend is not initialized")
	}
	return b.repository.GetCaptchaRecordByID(ctx, id)
}

// Delete 删除数据库记录。
func (b *DatabaseBackend) Delete(ctx context.Context, id string) error {
	if b == nil || b.repository == nil {
		return errors.New("captcha database backend is not initialized")
	}
	return b.repository.DeleteCaptchaRecordByID(ctx, id)
}
