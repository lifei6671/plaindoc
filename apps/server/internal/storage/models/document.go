package models

import "time"

// Document 对应 documents 表。
type Document struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	DocumentID      string    `gorm:"column:document_id"`
	NodeID          string    `gorm:"column:node_id"`
	ThemeID         string    `gorm:"column:theme_id"`
	Title           string    `gorm:"column:title"`
	ContentMD       string    `gorm:"column:content_md"`
	Version         int       `gorm:"column:version"`
	UpdatedByUserID *string   `gorm:"column:updated_by_user_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (Document) TableName() string {
	return "documents"
}
