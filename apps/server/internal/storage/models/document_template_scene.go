package models

import "time"

// DocumentTemplateScene 对应 document_template_scenes 表。
type DocumentTemplateScene struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SceneKey        string    `gorm:"column:scene_key"`
	SceneName       string    `gorm:"column:scene_name"`
	Description     string    `gorm:"column:description"`
	Sort            int       `gorm:"column:sort"`
	IsBuiltin       bool      `gorm:"column:is_builtin"`
	CreatedByUserID *string   `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string   `gorm:"column:updated_by_user_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (DocumentTemplateScene) TableName() string {
	return "document_template_scenes"
}
