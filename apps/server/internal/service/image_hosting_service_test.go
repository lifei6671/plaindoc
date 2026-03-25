package service

import "testing"

func TestDefaultImageHostingConfig_UsesSeparateImageAndAttachmentTemplates(t *testing.T) {
	config := DefaultImageHostingConfig()

	if config.Local.ImageUploadPathTemplate != DefaultImageHostingImageUploadPathTemplate {
		t.Fatalf("expected local image template %q, got %q", DefaultImageHostingImageUploadPathTemplate, config.Local.ImageUploadPathTemplate)
	}
	if config.Local.AttachmentUploadPathTemplate != DefaultImageHostingAttachmentUploadPathTemplate {
		t.Fatalf("expected local attachment template %q, got %q", DefaultImageHostingAttachmentUploadPathTemplate, config.Local.AttachmentUploadPathTemplate)
	}
	if config.CloudflareR2.AttachmentUploadPathTemplate != DefaultImageHostingAttachmentUploadPathTemplate {
		t.Fatalf("expected cloudflare attachment template %q, got %q", DefaultImageHostingAttachmentUploadPathTemplate, config.CloudflareR2.AttachmentUploadPathTemplate)
	}
	if config.AliyunOSS.AttachmentUploadPathTemplate != DefaultImageHostingAttachmentUploadPathTemplate {
		t.Fatalf("expected aliyun attachment template %q, got %q", DefaultImageHostingAttachmentUploadPathTemplate, config.AliyunOSS.AttachmentUploadPathTemplate)
	}
}

func TestNormalizeImageHostingConfig_LegacyUploadPathTemplateMigratesForAttachments(t *testing.T) {
	config := NormalizeImageHostingConfig(map[string]any{
		"defaultProvider": "local",
		"local": map[string]any{
			"uploadEndpoint":     "/api/uploads/images",
			"publicBaseUrl":      "/uploads",
			"uploadPathTemplate": "images/custom/{spaceId}/{assetId}.{ext}",
		},
	})

	if config.Local.ImageUploadPathTemplate != "images/custom/{spaceId}/{assetId}.{ext}" {
		t.Fatalf("expected local image template fallback from legacy field, got %q", config.Local.ImageUploadPathTemplate)
	}
	if config.Local.AttachmentUploadPathTemplate != "attachments/custom/{spaceId}/{assetId}.{ext}" {
		t.Fatalf("expected local attachment template migrate from legacy field, got %q", config.Local.AttachmentUploadPathTemplate)
	}
}

func TestNormalizeImageHostingConfig_PrefersNewSeparateTemplates(t *testing.T) {
	config := NormalizeImageHostingConfig(map[string]any{
		"defaultProvider": "cloudflare-r2",
		"cloudflareR2": map[string]any{
			"accountId":                    "acc",
			"bucket":                       "bucket",
			"accessKeyId":                  "key",
			"secretAccessKey":              "secret",
			"publicBaseUrl":                "https://img.example.com",
			"uploadPathTemplate":           "images/legacy/{assetId}.{ext}",
			"imageUploadPathTemplate":      "images/new/{assetId}.{ext}",
			"attachmentUploadPathTemplate": "attachments/new/{assetId}.{ext}",
		},
	})

	if config.CloudflareR2.ImageUploadPathTemplate != "images/new/{assetId}.{ext}" {
		t.Fatalf("expected cloudflare image template use new field, got %q", config.CloudflareR2.ImageUploadPathTemplate)
	}
	if config.CloudflareR2.AttachmentUploadPathTemplate != "attachments/new/{assetId}.{ext}" {
		t.Fatalf("expected cloudflare attachment template use new field, got %q", config.CloudflareR2.AttachmentUploadPathTemplate)
	}
}
