package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormUserIdentityRepository struct {
	db *gorm.DB
}

type userIdentityRow = userIdentityRowDB

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
	now = now.UTC()

	loginName := strings.TrimSpace(params.LoginName)
	lastLoginAt := normalizeNullableTime(params.LastLoginAt)
	updateValues := map[string]any{
		models.UserIdentityColumns.UserID:       userID,
		models.UserIdentityColumns.ProviderType: providerType,
		models.UserIdentityColumns.LoginName:    loginName,
		models.UserIdentityColumns.LastLoginAt:  lastLoginAt,
		models.UserIdentityColumns.UpdatedAt:    now,
	}
	insertRecord := &models.UserIdentity{
		UserID:       userID,
		ProviderType: providerType,
		ProviderID:   providerID,
		ExternalID:   externalID,
		LoginName:    loginName,
		LastLoginAt:  lastLoginAt,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	tx := r.db.WithContext(ctx).
		Model(&models.UserIdentity{}).
		Where(models.UserIdentityColumns.ProviderID+" = ?", providerID).
		Where(models.UserIdentityColumns.ExternalID+" = ?", externalID).
		Updates(updateValues)
	if tx.Error != nil {
		return nil, tx.Error
	}

	if tx.RowsAffected == 0 {
		if err := r.db.WithContext(ctx).
			Create(insertRecord).Error; err != nil {
			if !isUserIdentityUniqueConstraintError(err) {
				return nil, err
			}
			retryTx := r.db.WithContext(ctx).
				Model(&models.UserIdentity{}).
				Where(models.UserIdentityColumns.ProviderID+" = ?", providerID).
				Where(models.UserIdentityColumns.ExternalID+" = ?", externalID).
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
		Model(&models.UserIdentity{}).
		Select(
			models.UserIdentityColumns.ID,
			models.UserIdentityColumns.UserID,
			models.UserIdentityColumns.ProviderType,
			models.UserIdentityColumns.ProviderID,
			models.UserIdentityColumns.ExternalID,
			models.UserIdentityColumns.LoginName,
			models.UserIdentityColumns.LastLoginAt+" AS last_login_at_raw",
			models.UserIdentityColumns.CreatedAt+" AS created_at_raw",
			models.UserIdentityColumns.UpdatedAt+" AS updated_at_raw",
		).
		Where(models.UserIdentityColumns.ProviderID+" = ?", normalizedProviderID).
		Where(models.UserIdentityColumns.ExternalID+" = ?", normalizedExternalID).
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
		Model(&models.UserIdentity{}).
		Select(
			models.UserIdentityColumns.ID,
			models.UserIdentityColumns.UserID,
			models.UserIdentityColumns.ProviderType,
			models.UserIdentityColumns.ProviderID,
			models.UserIdentityColumns.ExternalID,
			models.UserIdentityColumns.LoginName,
			models.UserIdentityColumns.LastLoginAt+" AS last_login_at_raw",
			models.UserIdentityColumns.CreatedAt+" AS created_at_raw",
			models.UserIdentityColumns.UpdatedAt+" AS updated_at_raw",
		).
		Where(models.UserIdentityColumns.UserID+" = ?", normalizedUserID).
		Order(clause.OrderByColumn{
			Column: clause.Column{Name: models.UserIdentityColumns.CreatedAt},
		}).
		Order(clause.OrderByColumn{
			Column: clause.Column{Name: models.UserIdentityColumns.ID},
		}).
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
		UserID:       strings.TrimSpace(row.UserID),
		ProviderType: strings.TrimSpace(row.ProviderType),
		ProviderID:   strings.TrimSpace(row.ProviderID),
		ExternalID:   strings.TrimSpace(row.ExternalID),
		LoginName:    strings.TrimSpace(row.LoginName),
		LastLoginAt:  recordtime.ParseNullable(row.LastLoginAtRaw),
		CreatedAt:    recordtime.Parse(row.CreatedAtRaw),
		UpdatedAt:    recordtime.Parse(row.UpdatedAtRaw),
	}
}

func normalizeNullableTime(raw *time.Time) *time.Time {
	if raw == nil {
		return nil
	}
	normalized := raw.UTC()
	return &normalized
}

func parseNullableRecordTime(raw *string) *time.Time {
	return recordtime.ParseNullable(raw)
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
