package handler

import (
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

func TestBuildImageObjectKey_UsesImagesPrefix(t *testing.T) {
	objectKey, err := buildImageObjectKey(
		"diagram.png",
		"image/png",
		"space-a",
		"doc-a",
		"user-a",
		time.Date(2026, time.March, 25, 10, 0, 0, 0, time.UTC),
		"images/{spaceId}/{docId}/{assetId}.{ext}",
	)
	if err != nil {
		t.Fatalf("build image object key failed: %v", err)
	}
	if !strings.HasPrefix(objectKey, "images/") {
		t.Fatalf("expected image object key start with images/, got %q", objectKey)
	}
}

func TestBuildDocumentAttachmentObjectKey_UsesAttachmentsPrefix(t *testing.T) {
	objectKey, err := buildDocumentAttachmentObjectKey(
		"manual.pdf",
		"application/pdf",
		"space-a",
		"doc-a",
		"user-a",
		time.Date(2026, time.March, 25, 10, 0, 0, 0, time.UTC),
		"attachments/{spaceId}/{docId}/{assetId}.{ext}",
	)
	if err != nil {
		t.Fatalf("build attachment object key failed: %v", err)
	}
	if !strings.HasPrefix(objectKey, "attachments/") {
		t.Fatalf("expected attachment object key start with attachments/, got %q", objectKey)
	}
}

func TestBuildDocumentAttachmentObjectKey_AcceptsDefaultAttachmentTemplate(t *testing.T) {
	objectKey, err := buildDocumentAttachmentObjectKey(
		"manual.pdf",
		"application/pdf",
		"space-a",
		"doc-a",
		"user-a",
		time.Date(2026, time.March, 25, 10, 0, 0, 0, time.UTC),
		service.DefaultImageHostingConfig().AttachmentUploadPathTemplate(service.ImageHostingProviderLocal),
	)
	if err != nil {
		t.Fatalf("build attachment object key with default attachment template failed: %v", err)
	}
	if !strings.HasPrefix(objectKey, "attachments/") {
		t.Fatalf("expected attachment object key start with attachments/, got %q", objectKey)
	}
}
