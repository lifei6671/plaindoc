package repository

import (
	"context"
	"fmt"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormUserRepository struct {
	db *gorm.DB
}

// NewGormUserRepository 创建基于 GORM 的用户仓储实现。
func NewGormUserRepository(db *gorm.DB) UserRepository {
	return &gormUserRepository{db: db}
}

func (r *gormUserRepository) Create(ctx context.Context, user *models.User) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("user repository db is nil")
	}
	if user != nil {
		if !models.IsValidEntityStatus(user.Status) {
			user.Status = models.EntityStatusActive
		}
	}
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *gormUserRepository) GetByUserID(ctx context.Context, userID string) (*models.User, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("user repository db is nil")
	}
	var user models.User
	if err := r.db.WithContext(ctx).
		Select("id", "user_id", "email", "password_hash", "name", "status", "banned_reason", "banned_at", "deleted_at").
		Where("user_id = ?", userID).
		Take(&user).Error; err != nil {
		return nil, err
	}
	if !models.IsValidEntityStatus(user.Status) {
		user.Status = models.EntityStatusActive
	}
	return &user, nil
}

func (r *gormUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("user repository db is nil")
	}
	var user models.User
	if err := r.db.WithContext(ctx).
		Select("id", "user_id", "email", "password_hash", "name", "status", "banned_reason", "banned_at", "deleted_at").
		Where("email = ?", email).
		Take(&user).Error; err != nil {
		return nil, err
	}
	if !models.IsValidEntityStatus(user.Status) {
		user.Status = models.EntityStatusActive
	}
	return &user, nil
}
