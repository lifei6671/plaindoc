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

type gormSystemConfigRepository struct {
	db *gorm.DB
}

type systemConfigRow = systemConfigRowDB

// NewGormSystemConfigRepository 创建基于 GORM 的系统配置仓储实现。
func NewGormSystemConfigRepository(db *gorm.DB) SystemConfigRepository {
	return &gormSystemConfigRepository{db: db}
}

func (r *gormSystemConfigRepository) List(ctx context.Context) ([]models.SystemConfig, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("system config repository db is nil")
	}

	var rows []systemConfigRow
	if err := r.db.WithContext(ctx).
		Model(&models.SystemConfig{}).
		Select(
			models.SystemConfigColumns.ID,
			models.SystemConfigColumns.ConfigKey,
			models.SystemConfigColumns.ConfigValueJSON,
			models.SystemConfigColumns.Version,
			models.SystemConfigColumns.UpdatedByUserID,
			models.SystemConfigColumns.CreatedAt+" AS created_at_raw",
			models.SystemConfigColumns.UpdatedAt+" AS updated_at_raw",
		).
		Order(clause.OrderByColumn{
			Column: clause.Column{Name: models.SystemConfigColumns.ConfigKey},
		}).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]models.SystemConfig, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapSystemConfigRow(row))
	}
	return items, nil
}

func (r *gormSystemConfigRepository) GetByConfigKey(
	ctx context.Context,
	configKey string,
) (*models.SystemConfig, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("system config repository db is nil")
	}

	var row systemConfigRow
	if err := r.db.WithContext(ctx).
		Model(&models.SystemConfig{}).
		Select(
			models.SystemConfigColumns.ID,
			models.SystemConfigColumns.ConfigKey,
			models.SystemConfigColumns.ConfigValueJSON,
			models.SystemConfigColumns.Version,
			models.SystemConfigColumns.UpdatedByUserID,
			models.SystemConfigColumns.CreatedAt+" AS created_at_raw",
			models.SystemConfigColumns.UpdatedAt+" AS updated_at_raw",
		).
		Where(models.SystemConfigColumns.ConfigKey+" = ?", strings.TrimSpace(configKey)).
		Take(&row).Error; err != nil {
		return nil, err
	}

	item := mapSystemConfigRow(row)
	return &item, nil
}

func (r *gormSystemConfigRepository) Create(ctx context.Context, config *models.SystemConfig) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("system config repository db is nil")
	}
	if config == nil {
		return fmt.Errorf("system config is nil")
	}
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *gormSystemConfigRepository) UpdateByVersion(
	ctx context.Context,
	params UpdateSystemConfigByVersionParams,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("system config repository db is nil")
	}

	configKey := strings.TrimSpace(params.ConfigKey)
	if configKey == "" || params.ExpectedVersion <= 0 || params.NextVersion <= 0 {
		return false, nil
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	tx := r.db.WithContext(ctx).
		Model(&models.SystemConfig{}).
		Where(models.SystemConfigColumns.ConfigKey+" = ?", configKey).
		Where(models.SystemConfigColumns.Version+" = ?", params.ExpectedVersion).
		Updates(map[string]any{
			models.SystemConfigColumns.ConfigValueJSON: params.ConfigValueJSON,
			models.SystemConfigColumns.Version:         params.NextVersion,
			models.SystemConfigColumns.UpdatedByUserID: params.UpdatedByUserID,
			models.SystemConfigColumns.UpdatedAt:       updatedAt,
		})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func mapSystemConfigRow(row systemConfigRow) models.SystemConfig {
	return models.SystemConfig{
		ID:              row.ID,
		ConfigKey:       row.ConfigKey,
		ConfigValueJSON: row.ConfigValueJSON,
		Version:         row.Version,
		UpdatedByUserID: row.UpdatedByUserID,
		CreatedAt:       recordtime.Parse(row.CreatedAtRaw),
		UpdatedAt:       recordtime.Parse(row.UpdatedAtRaw),
	}
}
