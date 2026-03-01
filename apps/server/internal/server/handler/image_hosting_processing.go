package handler

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/chai2010/webp"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	_ "golang.org/x/image/webp"
)

type processedImageUpload struct {
	Content     []byte
	ContentType string
}

func processUploadedImageForStorage(
	originalContent []byte,
	detectedContentType string,
	config service.ImageHostingConfig,
) (processedImageUpload, error) {
	normalizedContentType := normalizeUploadedImageContentType(detectedContentType)
	if len(originalContent) == 0 {
		return processedImageUpload{}, errors.New("uploaded image content is empty")
	}
	if !strings.HasPrefix(normalizedContentType, "image/") {
		return processedImageUpload{}, errors.New("uploaded file is not image content")
	}

	imageProcessingConfig := config.ImageProcessing
	mode := normalizeImageProcessingMode(imageProcessingConfig.Mode)
	qualityPreset := normalizeImageQualityPreset(imageProcessingConfig.QualityPreset)
	maxWidth := normalizeImageMaxWidth(imageProcessingConfig.MaxWidth)
	maxHeight := normalizeImageMaxHeight(imageProcessingConfig.MaxHeight)
	skipAnimated := imageProcessingConfig.SkipAnimated

	if mode == service.ImageHostingImageProcessingModeSameFormat &&
		qualityPreset == service.ImageHostingImageQualityPresetOriginal {
		return processedImageUpload{
			Content:     originalContent,
			ContentType: normalizedContentType,
		}, nil
	}

	if skipAnimated && isAnimatedUploadedImage(originalContent, normalizedContentType) {
		return processedImageUpload{
			Content:     originalContent,
			ContentType: normalizedContentType,
		}, nil
	}

	// SVG 无法直接由 image 包解码，保持透传避免破坏可用性。
	if normalizedContentType == "image/svg+xml" {
		return processedImageUpload{
			Content:     originalContent,
			ContentType: normalizedContentType,
		}, nil
	}

	switch mode {
	case service.ImageHostingImageProcessingModeToWebP:
		decodedImage, decodeErr := decodeRasterImage(originalContent)
		if decodeErr != nil {
			return processedImageUpload{}, fmt.Errorf("decode image for webp conversion failed: %w", decodeErr)
		}
		if validateErr := validateImageDimensions(decodedImage, maxWidth, maxHeight); validateErr != nil {
			return processedImageUpload{}, validateErr
		}
		encodedWebP, encodeErr := encodeImageToWebP(decodedImage, qualityPreset)
		if encodeErr != nil {
			return processedImageUpload{}, encodeErr
		}
		return processedImageUpload{
			Content:     encodedWebP,
			ContentType: "image/webp",
		}, nil
	case service.ImageHostingImageProcessingModeSameFormat:
		return encodeImageSameFormat(originalContent, normalizedContentType, qualityPreset, maxWidth, maxHeight)
	default:
		return processedImageUpload{
			Content:     originalContent,
			ContentType: normalizedContentType,
		}, nil
	}
}

func normalizeImageProcessingMode(rawMode service.ImageHostingImageProcessingMode) service.ImageHostingImageProcessingMode {
	switch rawMode {
	case service.ImageHostingImageProcessingModeToWebP:
		return service.ImageHostingImageProcessingModeToWebP
	case service.ImageHostingImageProcessingModeSameFormat:
		return service.ImageHostingImageProcessingModeSameFormat
	default:
		return service.ImageHostingImageProcessingModeSameFormat
	}
}

func normalizeImageQualityPreset(rawPreset service.ImageHostingImageQualityPreset) service.ImageHostingImageQualityPreset {
	switch rawPreset {
	case service.ImageHostingImageQualityPresetOriginal:
		return service.ImageHostingImageQualityPresetOriginal
	case service.ImageHostingImageQualityPresetHigh:
		return service.ImageHostingImageQualityPresetHigh
	case service.ImageHostingImageQualityPresetStandard:
		return service.ImageHostingImageQualityPresetStandard
	case service.ImageHostingImageQualityPresetSaver:
		return service.ImageHostingImageQualityPresetSaver
	default:
		return service.ImageHostingImageQualityPresetStandard
	}
}

func normalizeImageMaxWidth(rawMaxWidth int) int {
	if rawMaxWidth <= 0 {
		return service.DefaultImageHostingConfig().ImageProcessing.MaxWidth
	}
	return rawMaxWidth
}

func normalizeImageMaxHeight(rawMaxHeight int) int {
	if rawMaxHeight <= 0 {
		return service.DefaultImageHostingConfig().ImageProcessing.MaxHeight
	}
	return rawMaxHeight
}

func normalizeUploadedImageContentType(rawContentType string) string {
	trimmed := strings.TrimSpace(strings.ToLower(rawContentType))
	if trimmed == "" {
		return ""
	}
	if separatorIndex := strings.Index(trimmed, ";"); separatorIndex > 0 {
		trimmed = strings.TrimSpace(trimmed[:separatorIndex])
	}
	return trimmed
}

func decodeRasterImage(content []byte) (image.Image, error) {
	decodedImage, _, decodeErr := image.Decode(bytes.NewReader(content))
	if decodeErr != nil {
		return nil, decodeErr
	}
	return decodedImage, nil
}

func validateImageDimensions(imageValue image.Image, maxWidth int, maxHeight int) error {
	if imageValue == nil {
		return errors.New("image is nil")
	}
	bounds := imageValue.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return errors.New("image bounds is invalid")
	}
	if width > maxWidth {
		return fmt.Errorf("image width %d exceeds configured max width %d", width, maxWidth)
	}
	if height > maxHeight {
		return fmt.Errorf("image height %d exceeds configured max height %d", height, maxHeight)
	}
	pixels := int64(width) * int64(height)
	maxArea := int64(maxWidth) * int64(maxHeight)
	if pixels > maxArea {
		return fmt.Errorf("image area %d exceeds configured max area %d", pixels, maxArea)
	}
	return nil
}

func encodeImageToWebP(
	imageValue image.Image,
	qualityPreset service.ImageHostingImageQualityPreset,
) ([]byte, error) {
	if imageValue == nil {
		return nil, errors.New("image is nil")
	}
	options := &webp.Options{
		Lossless: false,
		Quality:  float32(resolveImageQualityValue(qualityPreset)),
	}
	if qualityPreset == service.ImageHostingImageQualityPresetOriginal {
		options.Lossless = true
		options.Quality = 100
	}
	var output bytes.Buffer
	if err := webp.Encode(&output, imageValue, options); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func encodeImageSameFormat(
	originalContent []byte,
	contentType string,
	qualityPreset service.ImageHostingImageQualityPreset,
	maxWidth int,
	maxHeight int,
) (processedImageUpload, error) {
	if qualityPreset == service.ImageHostingImageQualityPresetOriginal {
		return processedImageUpload{
			Content:     originalContent,
			ContentType: contentType,
		}, nil
	}

	switch contentType {
	case "image/jpeg", "image/jpg":
		decodedImage, decodeErr := decodeRasterImage(originalContent)
		if decodeErr != nil {
			return processedImageUpload{}, decodeErr
		}
		if validateErr := validateImageDimensions(decodedImage, maxWidth, maxHeight); validateErr != nil {
			return processedImageUpload{}, validateErr
		}
		var output bytes.Buffer
		if err := jpeg.Encode(&output, decodedImage, &jpeg.Options{Quality: resolveImageQualityValue(qualityPreset)}); err != nil {
			return processedImageUpload{}, err
		}
		return processedImageUpload{
			Content:     output.Bytes(),
			ContentType: "image/jpeg",
		}, nil
	case "image/png":
		decodedImage, decodeErr := decodeRasterImage(originalContent)
		if decodeErr != nil {
			return processedImageUpload{}, decodeErr
		}
		if validateErr := validateImageDimensions(decodedImage, maxWidth, maxHeight); validateErr != nil {
			return processedImageUpload{}, validateErr
		}
		var output bytes.Buffer
		encoder := png.Encoder{
			CompressionLevel: resolvePNGCompressionLevel(qualityPreset),
		}
		if err := encoder.Encode(&output, decodedImage); err != nil {
			return processedImageUpload{}, err
		}
		return processedImageUpload{
			Content:     output.Bytes(),
			ContentType: "image/png",
		}, nil
	case "image/webp":
		decodedImage, decodeErr := decodeRasterImage(originalContent)
		if decodeErr != nil {
			return processedImageUpload{}, decodeErr
		}
		if validateErr := validateImageDimensions(decodedImage, maxWidth, maxHeight); validateErr != nil {
			return processedImageUpload{}, validateErr
		}
		encodedWebP, encodeErr := encodeImageToWebP(decodedImage, qualityPreset)
		if encodeErr != nil {
			return processedImageUpload{}, encodeErr
		}
		return processedImageUpload{
			Content:     encodedWebP,
			ContentType: "image/webp",
		}, nil
	default:
		return processedImageUpload{
			Content:     originalContent,
			ContentType: contentType,
		}, nil
	}
}

func resolveImageQualityValue(qualityPreset service.ImageHostingImageQualityPreset) int {
	switch qualityPreset {
	case service.ImageHostingImageQualityPresetHigh:
		return 92
	case service.ImageHostingImageQualityPresetSaver:
		return 70
	case service.ImageHostingImageQualityPresetStandard:
		return 82
	default:
		return 82
	}
}

func resolvePNGCompressionLevel(
	qualityPreset service.ImageHostingImageQualityPreset,
) png.CompressionLevel {
	switch qualityPreset {
	case service.ImageHostingImageQualityPresetHigh:
		return png.BestSpeed
	case service.ImageHostingImageQualityPresetSaver:
		return png.BestCompression
	case service.ImageHostingImageQualityPresetStandard:
		return png.DefaultCompression
	default:
		return png.DefaultCompression
	}
}

func isAnimatedUploadedImage(content []byte, contentType string) bool {
	switch contentType {
	case "image/gif":
		decodedGIF, decodeErr := gif.DecodeAll(bytes.NewReader(content))
		if decodeErr != nil {
			return false
		}
		return len(decodedGIF.Image) > 1
	case "image/webp":
		return bytes.Contains(content, []byte("ANIM"))
	default:
		return false
	}
}
