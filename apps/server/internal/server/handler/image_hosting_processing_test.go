package handler

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/rand"
	"strings"
	"testing"

	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

func TestProcessUploadedImageForStorage_ToWebP(t *testing.T) {
	t.Parallel()

	content := encodeJPEG(t, buildSolidRGBA(64, 64, color.RGBA{R: 180, G: 20, B: 50, A: 255}), 90)
	config := service.DefaultImageHostingConfig()
	config.ImageProcessing.Mode = service.ImageHostingImageProcessingModeToWebP
	config.ImageProcessing.QualityPreset = service.ImageHostingImageQualityPresetStandard

	processed, err := processUploadedImageForStorage(content, "image/jpeg", config)
	if err != nil {
		t.Fatalf("process uploaded image failed: %v", err)
	}
	if processed.ContentType != "image/webp" {
		t.Fatalf("expected webp content type, got %s", processed.ContentType)
	}
	if len(processed.Content) == 0 {
		t.Fatalf("expected non-empty output")
	}
}

func TestProcessUploadedImageForStorage_SameFormatOriginalPassthrough(t *testing.T) {
	t.Parallel()

	content := encodePNG(t, buildSolidRGBA(24, 24, color.RGBA{R: 16, G: 128, B: 200, A: 255}))
	config := service.DefaultImageHostingConfig()
	config.ImageProcessing.Mode = service.ImageHostingImageProcessingModeSameFormat
	config.ImageProcessing.QualityPreset = service.ImageHostingImageQualityPresetOriginal

	processed, err := processUploadedImageForStorage(content, "image/png", config)
	if err != nil {
		t.Fatalf("process uploaded image failed: %v", err)
	}
	if processed.ContentType != "image/png" {
		t.Fatalf("expected image/png content type, got %s", processed.ContentType)
	}
	if !bytes.Equal(processed.Content, content) {
		t.Fatalf("expected passthrough content in original quality mode")
	}
}

func TestProcessUploadedImageForStorage_SameFormatSaverJPEG(t *testing.T) {
	t.Parallel()

	noiseImage := buildNoiseRGBA(512, 512, 20260301)
	content := encodeJPEG(t, noiseImage, 100)
	config := service.DefaultImageHostingConfig()
	config.ImageProcessing.Mode = service.ImageHostingImageProcessingModeSameFormat
	config.ImageProcessing.QualityPreset = service.ImageHostingImageQualityPresetSaver

	processed, err := processUploadedImageForStorage(content, "image/jpeg", config)
	if err != nil {
		t.Fatalf("process uploaded image failed: %v", err)
	}
	if processed.ContentType != "image/jpeg" {
		t.Fatalf("expected image/jpeg content type, got %s", processed.ContentType)
	}
	if len(processed.Content) >= len(content) {
		t.Fatalf("expected compressed jpeg to be smaller, original=%d processed=%d", len(content), len(processed.Content))
	}
}

func TestProcessUploadedImageForStorage_SkipAnimatedGIF(t *testing.T) {
	t.Parallel()

	content := encodeAnimatedGIF(t)
	config := service.DefaultImageHostingConfig()
	config.ImageProcessing.Mode = service.ImageHostingImageProcessingModeToWebP
	config.ImageProcessing.SkipAnimated = true

	processed, err := processUploadedImageForStorage(content, "image/gif", config)
	if err != nil {
		t.Fatalf("process uploaded image failed: %v", err)
	}
	if processed.ContentType != "image/gif" {
		t.Fatalf("expected image/gif content type, got %s", processed.ContentType)
	}
	if !bytes.Equal(processed.Content, content) {
		t.Fatalf("expected animated gif passthrough when skipAnimated=true")
	}
}

func TestProcessUploadedImageForStorage_MaxDimensionsLimit(t *testing.T) {
	t.Parallel()

	content := encodeJPEG(t, buildSolidRGBA(2048, 2048, color.RGBA{R: 30, G: 60, B: 90, A: 255}), 90)
	config := service.DefaultImageHostingConfig()
	config.ImageProcessing.Mode = service.ImageHostingImageProcessingModeToWebP
	config.ImageProcessing.MaxWidth = 1200
	config.ImageProcessing.MaxHeight = 1200

	_, err := processUploadedImageForStorage(content, "image/jpeg", config)
	if err == nil {
		t.Fatalf("expected max dimensions validation error")
	}
	if !strings.Contains(err.Error(), "max width") {
		t.Fatalf("expected max width error, got %v", err)
	}
}

func buildSolidRGBA(width, height int, value color.RGBA) *image.RGBA {
	imageValue := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			imageValue.SetRGBA(x, y, value)
		}
	}
	return imageValue
}

func buildNoiseRGBA(width, height int, seed int64) *image.RGBA {
	random := rand.New(rand.NewSource(seed))
	imageValue := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			imageValue.SetRGBA(x, y, color.RGBA{
				R: uint8(random.Intn(256)),
				G: uint8(random.Intn(256)),
				B: uint8(random.Intn(256)),
				A: 255,
			})
		}
	}
	return imageValue
}

func encodeJPEG(t *testing.T, imageValue image.Image, quality int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, imageValue, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatalf("encode jpeg failed: %v", err)
	}
	return buffer.Bytes()
}

func encodePNG(t *testing.T, imageValue image.Image) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, imageValue); err != nil {
		t.Fatalf("encode png failed: %v", err)
	}
	return buffer.Bytes()
}

func encodeAnimatedGIF(t *testing.T) []byte {
	t.Helper()

	palette := []color.Color{
		color.RGBA{0, 0, 0, 255},
		color.RGBA{255, 255, 255, 255},
		color.RGBA{255, 0, 0, 255},
		color.RGBA{0, 0, 255, 255},
	}
	frame1 := image.NewPaletted(image.Rect(0, 0, 12, 12), palette)
	frame2 := image.NewPaletted(image.Rect(0, 0, 12, 12), palette)
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			if (x+y)%2 == 0 {
				frame1.SetColorIndex(x, y, 2)
				frame2.SetColorIndex(x, y, 3)
			} else {
				frame1.SetColorIndex(x, y, 1)
				frame2.SetColorIndex(x, y, 0)
			}
		}
	}
	var buffer bytes.Buffer
	if err := gif.EncodeAll(&buffer, &gif.GIF{
		Image: []*image.Paletted{frame1, frame2},
		Delay: []int{8, 8},
	}); err != nil {
		t.Fatalf("encode animated gif failed: %v", err)
	}
	return buffer.Bytes()
}
