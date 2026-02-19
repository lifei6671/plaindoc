package models

import "time"

// DocumentRevision 对应 document_revisions 表。
type DocumentRevision struct {
	ID                 int64          `gorm:"column:id;primaryKey;autoIncrement"`
	DocumentRevisionID string         `gorm:"column:document_revision_id"`
	DocumentID         string         `gorm:"column:document_id"`
	Version            int            `gorm:"column:version"`
	ContentMD          string         `gorm:"column:content_md"`
	BaseVersion        int            `gorm:"column:base_version"`
	EditorUserID       *string        `gorm:"column:editor_user_id"`
	Source             RevisionSource `gorm:"column:source"`
	CreatedAt          time.Time      `gorm:"column:created_at"`
}

func (DocumentRevision) TableName() string {
	return "document_revisions"
}
