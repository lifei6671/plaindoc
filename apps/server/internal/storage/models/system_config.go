package models

import "time"

// SystemConfig 对应 system_configs 表。
type SystemConfig struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ConfigKey       string    `gorm:"column:config_key"`
	ConfigValueJSON string    `gorm:"column:config_value_json"`
	Version         int       `gorm:"column:version"`
	UpdatedByUserID *string   `gorm:"column:updated_by_user_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (SystemConfig) TableName() string {
	return "system_configs"
}

var SystemConfigColumns = struct {
	ID              string
	ConfigKey       string
	ConfigValueJSON string
	Version         string
	UpdatedByUserID string
	CreatedAt       string
	UpdatedAt       string
}{
	ID:              "id",
	ConfigKey:       "config_key",
	ConfigValueJSON: "config_value_json",
	Version:         "version",
	UpdatedByUserID: "updated_by_user_id",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}
