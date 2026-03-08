package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// Document 对应 documents 表。
type Document struct {
	ID              int64                `gorm:"column:id;primaryKey;autoIncrement"`
	DocumentID      string               `gorm:"column:document_id"`
	NodeID          string               `gorm:"column:node_id"`
	ThemeID         string               `gorm:"column:theme_id"`
	Visibility      Visibility           `gorm:"column:visibility"`
	Status          EntityStatus         `gorm:"column:status"`
	BannedReason    string               `gorm:"column:banned_reason"`
	BannedAt        *time.Time           `gorm:"column:banned_at"`
	DeletedAt       *time.Time           `gorm:"column:deleted_at"`
	Title           string               `gorm:"column:title"`
	Format          DocumentFormat       `gorm:"column:format;default:markdown"`
	ContentMD       string               `gorm:"column:content_md"`
	RenderStatus    DocumentRenderStatus `gorm:"column:render_status;default:idle"`
	RenderError     string               `gorm:"column:render_error"`
	RenderedAt      *time.Time           `gorm:"column:rendered_at"`
	Version         int                  `gorm:"column:version"`
	SourceBlobID    *string              `gorm:"column:source_blob_id"`
	SourceFileName  *string              `gorm:"column:source_file_name"`
	SourceMimeType  *string              `gorm:"column:source_mime_type"`
	ContentVersion  int                  `gorm:"column:content_version;default:1"`
	CreatedByUserID *string              `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string              `gorm:"column:updated_by_user_id"`
	CreatedAt       time.Time            `gorm:"column:created_at"`
	UpdatedAt       time.Time            `gorm:"column:updated_at"`
}

func (Document) TableName() string {
	return "documents"
}

func (d *Document) BeforeCreate(_ *gorm.DB) error {
	if d == nil {
		return nil
	}

	d.Format = NormalizeDocumentFormat(d.Format)
	d.RenderStatus = NormalizeDocumentRenderStatus(d.RenderStatus)
	if d.ContentVersion <= 0 {
		switch {
		case d.Version > 0:
			d.ContentVersion = d.Version
		default:
			d.ContentVersion = 1
		}
	}

	d.SourceBlobID = trimDocumentOptionalString(d.SourceBlobID)
	d.SourceFileName = trimDocumentOptionalString(d.SourceFileName)
	d.SourceMimeType = trimDocumentOptionalString(d.SourceMimeType)
	if d.Format == DocumentFormatMarkdown {
		d.SourceBlobID = nil
		d.SourceFileName = nil
		d.SourceMimeType = nil
	}
	return nil
}

func trimDocumentOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
