package models

import "time"

// DocumentAttachmentBlob 对应 file_blobs 表。
// 该表表示“物理文件实体”，由多个文档附件记录引用。
type DocumentAttachmentBlob struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement"`
	BlobID          string     `gorm:"column:blob_id"`
	StorageProvider string     `gorm:"column:storage_provider"`
	ObjectKey       string     `gorm:"column:object_key"`
	ObjectURL       string     `gorm:"column:object_url"`
	MimeType        string     `gorm:"column:mime_type"`
	SizeBytes       int64      `gorm:"column:size_bytes"`
	ContentHashAlgo string     `gorm:"column:content_hash_algo"`
	ContentHash     string     `gorm:"column:content_hash"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at"`
}

func (DocumentAttachmentBlob) TableName() string {
	return "file_blobs"
}

var DocumentAttachmentBlobColumns = struct {
	ID              string
	BlobID          string
	StorageProvider string
	ObjectKey       string
	ObjectURL       string
	MimeType        string
	SizeBytes       string
	ContentHashAlgo string
	ContentHash     string
	CreatedAt       string
	UpdatedAt       string
	DeletedAt       string
}{
	ID:              "id",
	BlobID:          "blob_id",
	StorageProvider: "storage_provider",
	ObjectKey:       "object_key",
	ObjectURL:       "object_url",
	MimeType:        "mime_type",
	SizeBytes:       "size_bytes",
	ContentHashAlgo: "content_hash_algo",
	ContentHash:     "content_hash",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
	DeletedAt:       "deleted_at",
}
