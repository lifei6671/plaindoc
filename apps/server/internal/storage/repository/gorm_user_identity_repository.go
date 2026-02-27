package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormUserIdentityRepository struct {
	db *gorm.DB
}

type userIdentityRow struct {
	ID             int64   `gorm:"column:id"`
	UserID         string  `gorm:"column:user_id"`
	ProviderType   string  `gorm:"column:provider_type"`
	ProviderID     string  `gorm:"column:provider_id"`
	ExternalID     string  `gorm:"column:external_id"`
	LoginName      string  `gorm:"column:login_name"`
	LastLoginAtRaw *string `gorm:"column:last_login_at"`
	CreatedAtRaw   string  `gorm:"column:created_at"`
	UpdatedAtRaw   string  `gorm:"column:updated_at"`
}

// NewGormUserIdentityRepository 创建基于 GORM 的用户外部身份仓储实现。
func NewGormUserIdentityRepository(db *gorm.DB) UserIdentityRepository {
	return &gormUserIdentityRepository{db: db}
}

func (r *gormUserIdentityRepository) Upsert(
	ctx context.Context,
	params UpsertUserIdentityParams,
) (*models.UserIdentity, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("user identity repository db is nil")
	}

	userID := strings.TrimSpace(params.UserID)
	providerType := strings.TrimSpace(params.ProviderType)
	providerID := strings.TrimSpace(params.ProviderID)
	externalID := strings.TrimSpace(params.ExternalID)
	if userID == "" || providerType == "" || providerID == "" || externalID == "" {
		return nil, fmt.Errorf("user id/provider type/provider id/external id must not be empty")
	}

	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	loginName := strings.TrimSpace(params.LoginName)
	lastLoginAt := formatNullableRecordTime(params.LastLoginAt)
	updateValues := map[string]any{
		"user_id":       userID,
		"provider_type": providerType,
		"login_name":    loginName,
		"last_login_at": lastLoginAt,
		"updated_at":    now,
	}
	insertValues := map[string]any{
		"user_id":       userID,
		"provider_type": providerType,
		"provider_id":   providerID,
		"external_id":   externalID,
		"login_name":    loginName,
		"last_login_at": lastLoginAt,
		"created_at":    now,
		"updated_at":    now,
	}

	tx := r.db.WithContext(ctx).
		Table("user_identities").
		Where("provider_id = ? AND external_id = ?", providerID, externalID).
		Updates(updateValues)
	if tx.Error != nil {
		return nil, tx.Error
	}

	if tx.RowsAffected == 0 {
		if err := r.db.WithContext(ctx).
			Table("user_identities").
			Create(insertValues).Error; err != nil {
			if !isUserIdentityUniqueConstraintError(err) {
				return nil, err
			}
			retryTx := r.db.WithContext(ctx).
				Table("user_identities").
				Where("provider_id = ? AND external_id = ?", providerID, externalID).
				Updates(updateValues)
			if retryTx.Error != nil {
				return nil, retryTx.Error
			}
		}
	}

	return r.GetByProviderExternalID(ctx, providerID, externalID)
}

func (r *gormUserIdentityRepository) GetByProviderExternalID(
	ctx context.Context,
	providerID string,
	externalID string,
) (*models.UserIdentity, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("user identity repository db is nil")
	}

	normalizedProviderID := strings.TrimSpace(providerID)
	normalizedExternalID := strings.TrimSpace(externalID)
	if normalizedProviderID == "" || normalizedExternalID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row userIdentityRow
	if err := r.db.WithContext(ctx).
		Table("user_identities").
		Select(
			"id, user_id, provider_type, provider_id, external_id, login_name, last_login_at, created_at, updated_at",
		).
		Where("provider_id = ? AND external_id = ?", normalizedProviderID, normalizedExternalID).
		Take(&row).Error; err != nil {
		return nil, err
	}
	identity := mapUserIdentityRow(row)
	return &identity, nil
}

func (r *gormUserIdentityRepository) ListByUserID(
	ctx context.Context,
	userID string,
) ([]models.UserIdentity, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("user identity repository db is nil")
	}

	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return []models.UserIdentity{}, nil
	}

	var rows []userIdentityRow
	if err := r.db.WithContext(ctx).
		Table("user_identities").
		Select(
			"id, user_id, provider_type, provider_id, external_id, login_name, last_login_at, created_at, updated_at",
		).
		Where("user_id = ?", normalizedUserID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	identities := make([]models.UserIdentity, 0, len(rows))
	for _, row := range rows {
		identities = append(identities, mapUserIdentityRow(row))
	}
	return identities, nil
}

func mapUserIdentityRow(row userIdentityRow) models.UserIdentity {
	return models.UserIdentity{
		ID:           row.ID,
		UserID:       row.UserID,
		ProviderType: row.ProviderType,
		ProviderID:   row.ProviderID,
		ExternalID:   row.ExternalID,
		LoginName:    row.LoginName,
		LastLoginAt:  parseNullableRecordTime(row.LastLoginAtRaw),
		CreatedAt:    parseRecordTime(row.CreatedAtRaw),
		UpdatedAt:    parseRecordTime(row.UpdatedAtRaw),
	}
}

func parseNullableRecordTime(raw *string) *time.Time {
	if raw == nil {
		return nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil
	}
	parsedAt := parseRecordTime(value)
	if parsedAt.IsZero() {
		return nil
	}
	return &parsedAt
}

func formatNullableRecordTime(raw *time.Time) any {
	if raw == nil {
		return nil
	}
	return raw.UTC().Format(time.RFC3339Nano)
}

func isUserIdentityUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate entry") ||
		strings.Contains(message, "duplicate key")
}
