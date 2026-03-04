package models

import "time"

// DocumentTemplate 对应 document_templates 表。
type DocumentTemplate struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	TemplateID      string    `gorm:"column:template_id"`
	SceneKey        string    `gorm:"column:scene_key"`
	SceneName       string    `gorm:"column:scene_name"`
	Name            string    `gorm:"column:name"`
	Description     string    `gorm:"column:description"`
	DefaultTitle    string    `gorm:"column:default_title"`
	ContentMD       string    `gorm:"column:content_md"`
	Sort            int       `gorm:"column:sort"`
	IsBuiltin       bool      `gorm:"column:is_builtin"`
	IsEnabled       bool      `gorm:"column:is_enabled"`
	CreatedByUserID *string   `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string   `gorm:"column:updated_by_user_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (DocumentTemplate) TableName() string {
	return "document_templates"
}
