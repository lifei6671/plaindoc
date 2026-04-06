package models

import "time"

// DocumentFileRevision 对应 document_file_revisions 表。
type DocumentFileRevision struct {
	ID                     int64          `gorm:"column:id;primaryKey;autoIncrement"`
	DocumentFileRevisionID string         `gorm:"column:document_file_revision_id"`
	DocumentID             string         `gorm:"column:document_id"`
	BlobID                 string         `gorm:"column:blob_id"`
	FileName               string         `gorm:"column:file_name"`
	MimeType               string         `gorm:"column:mime_type"`
	Version                int            `gorm:"column:version"`
	BaseVersion            int            `gorm:"column:base_version"`
	EditorUserID           *string        `gorm:"column:editor_user_id"`
	Source                 RevisionSource `gorm:"column:source"`
	CreatedAt              time.Time      `gorm:"column:created_at"`
}

func (DocumentFileRevision) TableName() string {
	return "document_file_revisions"
}

var DocumentFileRevisionColumns = struct {
	ID                     string
	DocumentFileRevisionID string
	DocumentID             string
	BlobID                 string
	FileName               string
	MimeType               string
	Version                string
	BaseVersion            string
	EditorUserID           string
	Source                 string
	CreatedAt              string
}{
	ID:                     "id",
	DocumentFileRevisionID: "document_file_revision_id",
	DocumentID:             "document_id",
	BlobID:                 "blob_id",
	FileName:               "file_name",
	MimeType:               "mime_type",
	Version:                "version",
	BaseVersion:            "base_version",
	EditorUserID:           "editor_user_id",
	Source:                 "source",
	CreatedAt:              "created_at",
}
