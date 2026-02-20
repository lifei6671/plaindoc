package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormSystemConfigRepository struct {
	db *gorm.DB
}

type systemConfigRow struct {
	ID              int64   `gorm:"column:id"`
	ConfigKey       string  `gorm:"column:config_key"`
	ConfigValueJSON string  `gorm:"column:config_value_json"`
	Version         int     `gorm:"column:version"`
	UpdatedByUserID *string `gorm:"column:updated_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at"`
	UpdatedAtRaw    string  `gorm:"column:updated_at"`
}

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
		Table("system_configs").
		Select(
			"id",
			"config_key",
			"config_value_json",
			"version",
			"updated_by_user_id",
			"created_at",
			"updated_at",
		).
		Order("config_key ASC").
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
		Table("system_configs").
		Select(
			"id",
			"config_key",
			"config_value_json",
			"version",
			"updated_by_user_id",
			"created_at",
			"updated_at",
		).
		Where("config_key = ?", strings.TrimSpace(configKey)).
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
		Where("config_key = ? AND version = ?", configKey, params.ExpectedVersion).
		Updates(map[string]any{
			"config_value_json":  params.ConfigValueJSON,
			"version":            params.NextVersion,
			"updated_by_user_id": params.UpdatedByUserID,
			"updated_at":         updatedAt,
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
		CreatedAt:       parseSystemConfigRecordTime(row.CreatedAtRaw),
		UpdatedAt:       parseSystemConfigRecordTime(row.UpdatedAtRaw),
	}
}

func parseSystemConfigRecordTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
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
	if parsedAt, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC); err == nil {
		return parsedAt.UTC()
	}
	return time.Time{}
}
