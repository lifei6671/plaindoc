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
