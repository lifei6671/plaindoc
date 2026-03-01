package service

import "testing"

func TestNormalizeImageHostingConfig_ImageProcessing(t *testing.T) {
	t.Parallel()

	config := NormalizeImageHostingConfig(map[string]any{
		"defaultProvider": "local",
		"cloudflareR2":    map[string]any{},
		"aliyunOss":       map[string]any{},
		"local":           map[string]any{},
		"imageProcessing": map[string]any{
			"mode":          "to_webp",
			"qualityPreset": "saver",
			"maxWidth":      4096,
			"maxHeight":     3072,
			"skipAnimated":  false,
		},
	})

	if config.ImageProcessing.Mode != ImageHostingImageProcessingModeToWebP {
		t.Fatalf("expected to_webp mode, got %s", config.ImageProcessing.Mode)
	}
	if config.ImageProcessing.QualityPreset != ImageHostingImageQualityPresetSaver {
		t.Fatalf("expected saver preset, got %s", config.ImageProcessing.QualityPreset)
	}
	if config.ImageProcessing.MaxWidth != 4096 {
		t.Fatalf("expected max width 4096, got %d", config.ImageProcessing.MaxWidth)
	}
	if config.ImageProcessing.MaxHeight != 3072 {
		t.Fatalf("expected max height 3072, got %d", config.ImageProcessing.MaxHeight)
	}
	if config.ImageProcessing.SkipAnimated {
		t.Fatalf("expected skipAnimated=false")
	}
}

func TestNormalizeImageHostingConfig_ImageProcessingFallback(t *testing.T) {
	t.Parallel()

	config := NormalizeImageHostingConfig(map[string]any{
		"defaultProvider": "local",
		"cloudflareR2":    map[string]any{},
		"aliyunOss":       map[string]any{},
		"local":           map[string]any{},
		"imageProcessing": map[string]any{
			"mode":          "invalid",
			"qualityPreset": "invalid",
			"maxWidth":      120,
			"maxHeight":     120,
			"skipAnimated":  "no",
		},
	})

	if config.ImageProcessing.Mode != DefaultImageHostingConfig().ImageProcessing.Mode {
		t.Fatalf("expected default mode fallback, got %s", config.ImageProcessing.Mode)
	}
	if config.ImageProcessing.QualityPreset != DefaultImageHostingConfig().ImageProcessing.QualityPreset {
		t.Fatalf("expected default quality fallback, got %s", config.ImageProcessing.QualityPreset)
	}
	if config.ImageProcessing.MaxWidth != defaultImageHostingImageMaxWidth {
		t.Fatalf("expected default max width %d, got %d", defaultImageHostingImageMaxWidth, config.ImageProcessing.MaxWidth)
	}
	if config.ImageProcessing.MaxHeight != defaultImageHostingImageMaxHeight {
		t.Fatalf("expected default max height %d, got %d", defaultImageHostingImageMaxHeight, config.ImageProcessing.MaxHeight)
	}
	if config.ImageProcessing.SkipAnimated != DefaultImageHostingConfig().ImageProcessing.SkipAnimated {
		t.Fatalf("expected default skipAnimated fallback")
	}
}

func TestNormalizeImageHostingConfig_ImageProcessingLegacyMaxPixelsFallback(t *testing.T) {
	t.Parallel()

	config := NormalizeImageHostingConfig(map[string]any{
		"defaultProvider": "local",
		"cloudflareR2":    map[string]any{},
		"aliyunOss":       map[string]any{},
		"local":           map[string]any{},
		"imageProcessing": map[string]any{
			"mode":          "same_format",
			"qualityPreset": "standard",
			"maxPixels":     4_000_000,
			"skipAnimated":  true,
		},
	})

	if config.ImageProcessing.MaxWidth <= 0 || config.ImageProcessing.MaxHeight <= 0 {
		t.Fatalf("expected legacy maxPixels to derive positive dimensions")
	}
}

func TestValidateImageHostingConfig_ImageProcessing(t *testing.T) {
	t.Parallel()

	validPayload := DefaultImageHostingConfig().ToMap()
	if err := validateImageHostingConfig(validPayload); err != nil {
		t.Fatalf("expected valid payload, got %v", err)
	}

	invalidDimensionsPayload := DefaultImageHostingConfig().ToMap()
	invalidDimensionsPayload["imageProcessing"] = map[string]any{
		"mode":          "same_format",
		"qualityPreset": "standard",
		"maxWidth":      120,
		"maxHeight":     120,
		"skipAnimated":  true,
	}
	if err := validateImageHostingConfig(invalidDimensionsPayload); err == nil {
		t.Fatalf("expected max width/height validation error")
	}

	invalidUnknownKeyPayload := DefaultImageHostingConfig().ToMap()
	invalidUnknownKeyPayload["imageProcessing"] = map[string]any{
		"mode":          "same_format",
		"qualityPreset": "standard",
		"maxWidth":      4096,
		"maxHeight":     3072,
		"skipAnimated":  true,
		"unsupported":   true,
	}
	if err := validateImageHostingConfig(invalidUnknownKeyPayload); err == nil {
		t.Fatalf("expected unknown key validation error")
	}
}
