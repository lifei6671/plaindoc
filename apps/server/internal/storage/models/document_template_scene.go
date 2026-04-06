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

var DocumentTemplateSceneColumns = struct {
	ID              string
	SceneKey        string
	SceneName       string
	Description     string
	Sort            string
	IsBuiltin       string
	CreatedByUserID string
	UpdatedByUserID string
	CreatedAt       string
	UpdatedAt       string
}{
	ID:              "id",
	SceneKey:        "scene_key",
	SceneName:       "scene_name",
	Description:     "description",
	Sort:            "sort",
	IsBuiltin:       "is_builtin",
	CreatedByUserID: "created_by_user_id",
	UpdatedByUserID: "updated_by_user_id",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}
