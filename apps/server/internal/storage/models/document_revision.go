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

var DocumentRevisionColumns = struct {
	ID                 string
	DocumentRevisionID string
	DocumentID         string
	Version            string
	ContentMD          string
	BaseVersion        string
	EditorUserID       string
	Source             string
	CreatedAt          string
}{
	ID:                 "id",
	DocumentRevisionID: "document_revision_id",
	DocumentID:         "document_id",
	Version:            "version",
	ContentMD:          "content_md",
	BaseVersion:        "base_version",
	EditorUserID:       "editor_user_id",
	Source:             "source",
	CreatedAt:          "created_at",
}
