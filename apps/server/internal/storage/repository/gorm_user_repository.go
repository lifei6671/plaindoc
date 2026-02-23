package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

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
		Select("id, user_id, email, password_hash, name, avatar_url, status, banned_reason, banned_at, deleted_at").
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
		Select("id, user_id, email, password_hash, name, avatar_url, status, banned_reason, banned_at, deleted_at").
		Where("email = ?", email).
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
			"LOWER(user_id) LIKE ? OR LOWER(email) LIKE ? OR LOWER(name) LIKE ?",
			likeKeyword,
			likeKeyword,
			likeKeyword,
		)
	}

	statuses := normalizeStatuses(params.Statuses)
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where("status IN ?", statuses)
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

	type userListRow struct {
		ID           int64               `gorm:"column:id"`
		UserID       string              `gorm:"column:user_id"`
		Email        string              `gorm:"column:email"`
		PasswordHash string              `gorm:"column:password_hash"`
		Name         string              `gorm:"column:name"`
		AvatarURL    string              `gorm:"column:avatar_url"`
		Status       models.EntityStatus `gorm:"column:status"`
		BannedReason string              `gorm:"column:banned_reason"`
		BannedAt     *time.Time          `gorm:"column:banned_at"`
		DeletedAt    *time.Time          `gorm:"column:deleted_at"`
		CreatedAtRaw string              `gorm:"column:created_at"`
		UpdatedAtRaw string              `gorm:"column:updated_at"`
	}

	var rows []userListRow
	if err := baseQuery.Session(&gorm.Session{}).
		Select("id, user_id, email, password_hash, name, avatar_url, status, banned_reason, banned_at, deleted_at, created_at, updated_at").
		Order("created_at DESC").
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
			CreatedAt:    parseRecordTime(row.CreatedAtRaw),
			UpdatedAt:    parseRecordTime(row.UpdatedAtRaw),
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
		"status":        params.Status,
		"updated_at":    updatedAt,
		"banned_reason": "",
		"banned_at":     nil,
	}
	if params.Status == models.EntityStatusBanned {
		updateValues["banned_reason"] = strings.TrimSpace(params.BannedReason)
		updateValues["banned_at"] = params.BannedAt
	}

	tx := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("user_id = ?", params.UserID).
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
		updateValues["name"] = strings.TrimSpace(*params.Name)
	}
	if params.AvatarURL != nil {
		updateValues["avatar_url"] = strings.TrimSpace(*params.AvatarURL)
	}
	if len(updateValues) == 0 {
		return false, nil
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	updateValues["updated_at"] = updatedAt

	tx := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("user_id = ?", userID).
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
		Where("user_id = ?", targetUserID).
		Updates(map[string]any{
			"password_hash": targetPasswordHash,
			"updated_at":    updatedAt,
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
		Where("user_id = ? AND status <> ?", userID, models.EntityStatusDeleted).
		Updates(map[string]any{
			"status":        models.EntityStatusDeleted,
			"deleted_at":    deletedAt,
			"banned_reason": "",
			"banned_at":     nil,
			"updated_at":    deletedAt,
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
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05.999-07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsedAt, err := time.Parse(layout, value); err == nil {
			return parsedAt.UTC()
		}
	}
	// 兼容数据库返回的 "YYYY-MM-DD HH:MM:SS+00:00" 等格式，统一转为 RFC3339 再解析。
	normalized := strings.Replace(value, " ", "T", 1)
	timePart := normalized
	if index := strings.IndexByte(normalized, 'T'); index >= 0 && index < len(normalized)-1 {
		timePart = normalized[index+1:]
	}
	if !strings.ContainsAny(timePart, "Zz+-") {
		normalized += "Z"
	}
	if parsedAt, err := time.Parse(time.RFC3339Nano, normalized); err == nil {
		return parsedAt.UTC()
	}
	if parsedAt, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC); err == nil {
		return parsedAt.UTC()
	}
	return time.Time{}
}
