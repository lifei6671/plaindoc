package models

import "time"

const (
	// DefaultSpaceCategoryID 是系统内置“未分类”分类 ID（ULID 小写格式）。
	DefaultSpaceCategoryID = "01jmf4v2x7m7f1m6qv5kh0t2mn"
	// DefaultSpaceCategoryName 是系统内置“未分类”分类名称。
	DefaultSpaceCategoryName = "未分类"
)

// SpaceCategory 对应 space_categories 表。
type SpaceCategory struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	CategoryID string    `gorm:"column:category_id"`
	Name       string    `gorm:"column:name"`
	IsDefault  bool      `gorm:"column:is_default"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (SpaceCategory) TableName() string {
	return "space_categories"
}

var SpaceCategoryColumns = struct {
	ID         string
	CategoryID string
	Name       string
	IsDefault  string
	CreatedAt  string
	UpdatedAt  string
}{
	ID:         "id",
	CategoryID: "category_id",
	Name:       "name",
	IsDefault:  "is_default",
	CreatedAt:  "created_at",
	UpdatedAt:  "updated_at",
}
