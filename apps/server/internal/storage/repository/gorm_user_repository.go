package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormUserRepository struct {
	db *gorm.DB
}

type userListRow = userListRowDB

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
		Model(&models.User{}).
		Select(
			models.UserColumns.ID,
			models.UserColumns.UserID,
			models.UserColumns.Email,
			models.UserColumns.PasswordHash,
			models.UserColumns.Name,
			models.UserColumns.AvatarURL,
			models.UserColumns.Status,
			models.UserColumns.BannedReason,
			models.UserColumns.BannedAt,
			models.UserColumns.DeletedAt,
		).
		Where(models.UserColumns.UserID+" = ?", userID).
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
		Model(&models.User{}).
		Select(
			models.UserColumns.ID,
			models.UserColumns.UserID,
			models.UserColumns.Email,
			models.UserColumns.PasswordHash,
			models.UserColumns.Name,
			models.UserColumns.AvatarURL,
			models.UserColumns.Status,
			models.UserColumns.BannedReason,
			models.UserColumns.BannedAt,
			models.UserColumns.DeletedAt,
		).
		Where(models.UserColumns.Email+" = ?", email).
		Take(&user).Error; err != nil {
		return nil, err
	}
	if !models.IsValidEntityStatus(user.Status) {
		user.Status = models.EntityStatusActive
	}
	return &user, nil
}

func (r *gormUserRepository) List(
	ctx context.Context,
	params ListUsersParams,
) ([]models.User, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("user repository db is nil")
	}

	baseQuery := r.db.WithContext(ctx).Model(&models.User{})

	keyword := strings.ToLower(strings.TrimSpace(params.Keyword))
	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		baseQuery = baseQuery.Where(
			"LOWER("+models.UserColumns.UserID+") LIKE ? OR "+
				"LOWER("+models.UserColumns.Email+") LIKE ? OR "+
				"LOWER("+models.UserColumns.Name+") LIKE ?",
			likeKeyword,
			likeKeyword,
			likeKeyword,
		)
	}

	statuses := normalizeStatuses(params.Statuses)
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where(models.UserColumns.Status+" IN ?", statuses)
	}

	var total int64
	if err := baseQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	var rows []userListRow
	if err := baseQuery.Session(&gorm.Session{}).
		Select(
			models.UserColumns.ID,
			models.UserColumns.UserID,
			models.UserColumns.Email,
			models.UserColumns.PasswordHash,
			models.UserColumns.Name,
			models.UserColumns.AvatarURL,
			models.UserColumns.Status,
			models.UserColumns.BannedReason,
			models.UserColumns.BannedAt,
			models.UserColumns.DeletedAt,
			models.UserColumns.CreatedAt+" AS created_at_raw",
			models.UserColumns.UpdatedAt+" AS updated_at_raw",
		).
		Order(models.UserColumns.CreatedAt + " DESC").
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	users := make([]models.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, models.User{
			ID:           row.ID,
			UserID:       row.UserID,
			Email:        row.Email,
			PasswordHash: row.PasswordHash,
			Name:         row.Name,
			AvatarURL:    row.AvatarURL,
			Status:       row.Status,
			BannedReason: row.BannedReason,
			BannedAt:     row.BannedAt,
			DeletedAt:    row.DeletedAt,
			CreatedAt:    recordtime.Parse(row.CreatedAtRaw),
			UpdatedAt:    recordtime.Parse(row.UpdatedAtRaw),
		})
	}

	for idx := range users {
		if !models.IsValidEntityStatus(users[idx].Status) {
			users[idx].Status = models.EntityStatusActive
		}
	}

	return users, total, nil
}

func (r *gormUserRepository) UpdateStatus(ctx context.Context, params UpdateUserStatusParams) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("user repository db is nil")
	}
	if params.UserID == "" {
		return false, nil
	}
	if !models.IsValidEntityStatus(params.Status) {
		return false, nil
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	updateValues := map[string]any{
		models.UserColumns.Status:       params.Status,
		models.UserColumns.UpdatedAt:    updatedAt,
		models.UserColumns.BannedReason: "",
		models.UserColumns.BannedAt:     nil,
	}
	if params.Status == models.EntityStatusBanned {
		updateValues[models.UserColumns.BannedReason] = strings.TrimSpace(params.BannedReason)
		updateValues[models.UserColumns.BannedAt] = params.BannedAt
	}

	tx := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where(models.UserColumns.UserID+" = ?", params.UserID).
		Updates(updateValues)
	if tx.Error != nil {
		return false, tx.Error
	}

	return tx.RowsAffected > 0, nil
}

func (r *gormUserRepository) UpdateProfile(ctx context.Context, params UpdateUserProfileParams) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("user repository db is nil")
	}
	userID := strings.TrimSpace(params.UserID)
	if userID == "" {
		return false, nil
	}

	updateValues := map[string]any{}
	if params.Name != nil {
		updateValues[models.UserColumns.Name] = strings.TrimSpace(*params.Name)
	}
	if params.AvatarURL != nil {
		updateValues[models.UserColumns.AvatarURL] = strings.TrimSpace(*params.AvatarURL)
	}
	if len(updateValues) == 0 {
		return false, nil
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	updateValues[models.UserColumns.UpdatedAt] = updatedAt

	tx := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where(models.UserColumns.UserID+" = ?", userID).
		Updates(updateValues)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *gormUserRepository) UpdatePassword(
	ctx context.Context,
	userID string,
	passwordHash string,
	updatedAt time.Time,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("user repository db is nil")
	}
	targetUserID := strings.TrimSpace(userID)
	if targetUserID == "" {
		return false, nil
	}
	targetPasswordHash := strings.TrimSpace(passwordHash)
	if targetPasswordHash == "" {
		return false, nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	tx := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where(models.UserColumns.UserID+" = ?", targetUserID).
		Updates(map[string]any{
			models.UserColumns.PasswordHash: targetPasswordHash,
			models.UserColumns.UpdatedAt:    updatedAt,
		})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *gormUserRepository) SoftDelete(ctx context.Context, userID string, deletedAt time.Time) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("user repository db is nil")
	}
	if strings.TrimSpace(userID) == "" {
		return false, nil
	}
	if deletedAt.IsZero() {
		deletedAt = time.Now().UTC()
	}

	tx := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where(models.UserColumns.UserID+" = ?", userID).
		Where(models.UserColumns.Status+" <> ?", models.EntityStatusDeleted).
		Updates(map[string]any{
			models.UserColumns.Status:       models.EntityStatusDeleted,
			models.UserColumns.DeletedAt:    deletedAt,
			models.UserColumns.BannedReason: "",
			models.UserColumns.BannedAt:     nil,
			models.UserColumns.UpdatedAt:    deletedAt,
		})
	if tx.Error != nil {
		return false, tx.Error
	}

	return tx.RowsAffected > 0, nil
}

func normalizeStatuses(input []models.EntityStatus) []models.EntityStatus {
	if len(input) == 0 {
		return nil
	}
	statuses := make([]models.EntityStatus, 0, len(input))
	exists := make(map[models.EntityStatus]struct{}, len(input))
	for _, status := range input {
		if !models.IsValidEntityStatus(status) {
			continue
		}
		if _, ok := exists[status]; ok {
			continue
		}
		exists[status] = struct{}{}
		statuses = append(statuses, status)
	}
	return statuses
}

func parseRecordTime(raw string) time.Time {
	return recordtime.Parse(raw)
}
