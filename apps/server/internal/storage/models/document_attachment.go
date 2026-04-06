package models

import "time"

// DocumentAttachment 对应 document_attachments 表。
type DocumentAttachment struct {
	ID              int64        `gorm:"column:id;primaryKey;autoIncrement"`
	AttachmentID    string       `gorm:"column:attachment_id"`
	BlobID          string       `gorm:"column:blob_id"`
	DocumentID      string       `gorm:"column:document_id"`
	SpaceID         string       `gorm:"column:space_id"`
	StorageProvider string       `gorm:"column:storage_provider"`
	FileName        string       `gorm:"column:file_name"`
	ObjectKey       string       `gorm:"column:object_key"`
	ObjectURL       string       `gorm:"column:object_url"`
	MimeType        string       `gorm:"column:mime_type"`
	SizeBytes       int64        `gorm:"column:size_bytes"`
	ContentHashAlgo string       `gorm:"column:content_hash_algo"`
	ContentHash     string       `gorm:"column:content_hash"`
	PreviewKind     string       `gorm:"column:preview_kind"`
	Status          EntityStatus `gorm:"column:status"`
	DeletedAt       *time.Time   `gorm:"column:deleted_at"`
	CreatedByUserID *string      `gorm:"column:created_by_user_id"`
	CreatedAt       time.Time    `gorm:"column:created_at"`
	UpdatedAt       time.Time    `gorm:"column:updated_at"`
}

func (DocumentAttachment) TableName() string {
	return "document_attachments"
}

var DocumentAttachmentColumns = struct {
	ID              string
	AttachmentID    string
	BlobID          string
	DocumentID      string
	SpaceID         string
	StorageProvider string
	FileName        string
	ObjectKey       string
	ObjectURL       string
	MimeType        string
	SizeBytes       string
	ContentHashAlgo string
	ContentHash     string
	PreviewKind     string
	Status          string
	DeletedAt       string
	CreatedByUserID string
	CreatedAt       string
	UpdatedAt       string
}{
	ID:              "id",
	AttachmentID:    "attachment_id",
	BlobID:          "blob_id",
	DocumentID:      "document_id",
	SpaceID:         "space_id",
	StorageProvider: "storage_provider",
	FileName:        "file_name",
	ObjectKey:       "object_key",
	ObjectURL:       "object_url",
	MimeType:        "mime_type",
	SizeBytes:       "size_bytes",
	ContentHashAlgo: "content_hash_algo",
	ContentHash:     "content_hash",
	PreviewKind:     "preview_kind",
	Status:          "status",
	DeletedAt:       "deleted_at",
	CreatedByUserID: "created_by_user_id",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}
