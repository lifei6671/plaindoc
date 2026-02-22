package service

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chai2010/webp"
	"github.com/fogleman/gg"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
)

const (
	adminSpaceCoverStorageRootDir         = "uploads/local"
	adminSpaceCoverObjectPrefix           = "space-covers"
	adminSpaceCoverPublicPrefix           = "/api/uploads/local"
	adminSpaceCoverMaxUploadSizeBytes     = 10 << 20
	adminSpaceCoverMaxImageDimension      = 12000
	adminSpaceCoverMaxPixelCount          = 40_000_000
	adminSpaceCoverTargetRatioNumerator   = 5
	adminSpaceCoverTargetRatioDenominator = 8
	adminSpaceCoverMaxWidth               = 1600
	adminSpaceCoverMaxHeight              = 2560
	adminSpaceCoverDefaultQuality         = 82.0
	adminSpaceCoverSafeMargin             = 128
	adminSpaceCoverMaxTitleLines          = 3
)

var (
	spaceTitleTokenRegexp = regexp.MustCompile(`[A-Za-z0-9._-]+|\s+|.`)
	adminSpaceFontLoader  adminSpaceFontRegistry
)

type processAdminSpaceUserUploadCoverInput struct {
	FileName        string
	FileContentType string
	FileBytes       []byte
	Quality         float64
}

type processAdminSpaceCoverResult struct {
	WebPBytes  []byte
	Width      int
	Height     int
	Normalized bool
}

type renderAdminSpaceSystemCoverInput struct {
	SpaceName string
	Quality   float64
}

type adminSpaceFontRegistry struct {
	once     sync.Once
	fontData []byte
	fontPath string
	err      error
}

func processAdminSpaceUserUploadCover(
	input processAdminSpaceUserUploadCoverInput,
) (processAdminSpaceCoverResult, error) {
	fileBytes := input.FileBytes
	if len(fileBytes) == 0 {
		return processAdminSpaceCoverResult{}, ErrAdminSpaceCoverFileRequired
	}
	if len(fileBytes) > adminSpaceCoverMaxUploadSizeBytes {
		return processAdminSpaceCoverResult{}, ErrAdminSpaceCoverImageTooLarge
	}

	contentType := detectAdminSpaceCoverContentType(fileBytes, input.FileContentType)
	if !strings.HasPrefix(contentType, "image/") {
		return processAdminSpaceCoverResult{}, ErrAdminSpaceCoverImageInvalid
	}

	decodedImage, _, err := image.Decode(bytes.NewReader(fileBytes))
	if err != nil {
		return processAdminSpaceCoverResult{}, ErrAdminSpaceCoverImageInvalid
	}

	sourceBounds := decodedImage.Bounds()
	sourceWidth := sourceBounds.Dx()
	sourceHeight := sourceBounds.Dy()
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return processAdminSpaceCoverResult{}, ErrAdminSpaceCoverImageInvalid
	}
	if sourceWidth > adminSpaceCoverMaxImageDimension || sourceHeight > adminSpaceCoverMaxImageDimension {
		return processAdminSpaceCoverResult{}, ErrAdminSpaceCoverImageTooManyPixels
	}
	if int64(sourceWidth)*int64(sourceHeight) > adminSpaceCoverMaxPixelCount {
		return processAdminSpaceCoverResult{}, ErrAdminSpaceCoverImageTooManyPixels
	}

	cropRect := computeAdminSpaceCoverCropRect(sourceBounds)
	outputWidth, outputHeight := computeAdminSpaceCoverOutputSize(cropRect.Dx(), cropRect.Dy())
	if outputWidth <= 0 || outputHeight <= 0 {
		return processAdminSpaceCoverResult{}, ErrAdminSpaceCoverImageInvalid
	}

	resized := image.NewRGBA(image.Rect(0, 0, outputWidth, outputHeight))
	xdraw.CatmullRom.Scale(
		resized,
		resized.Bounds(),
		decodedImage,
		cropRect,
		xdraw.Over,
		nil,
	)

	webpBytes, err := encodeAdminSpaceCoverWebP(resized, input.Quality)
	if err != nil {
		return processAdminSpaceCoverResult{}, err
	}

	normalized := contentType != "image/webp" ||
		cropRect.Dx() != sourceWidth ||
		cropRect.Dy() != sourceHeight ||
		outputWidth != sourceWidth ||
		outputHeight != sourceHeight

	return processAdminSpaceCoverResult{
		WebPBytes:  webpBytes,
		Width:      outputWidth,
		Height:     outputHeight,
		Normalized: normalized,
	}, nil
}

func renderAdminSpaceSystemCover(
	input renderAdminSpaceSystemCoverInput,
) (processAdminSpaceCoverResult, error) {
	title := normalizeAdminSpaceCoverTitle(input.SpaceName)
	if title == "" {
		return processAdminSpaceCoverResult{}, ErrAdminSpaceCoverSpaceNameRequired
	}

	canvas := image.NewRGBA(image.Rect(0, 0, adminSpaceCoverMaxWidth, adminSpaceCoverMaxHeight))
	ctx := gg.NewContextForRGBA(canvas)
	drawAdminSpaceCoverBackground(ctx)

	if err := drawAdminSpaceCoverTitle(canvas, title); err != nil {
		return processAdminSpaceCoverResult{}, err
	}

	webpBytes, err := encodeAdminSpaceCoverWebP(canvas, input.Quality)
	if err != nil {
		return processAdminSpaceCoverResult{}, err
	}

	return processAdminSpaceCoverResult{
		WebPBytes:  webpBytes,
		Width:      adminSpaceCoverMaxWidth,
		Height:     adminSpaceCoverMaxHeight,
		Normalized: true,
	}, nil
}

func drawAdminSpaceCoverBackground(ctx *gg.Context) {
	if ctx == nil {
		return
	}
	width := float64(ctx.Width())
	height := float64(ctx.Height())

	ctx.SetColor(color.RGBA{R: 245, G: 247, B: 251, A: 255})
	ctx.Clear()

	ctx.DrawRoundedRectangle(72, 88, width-144, height-176, 54)
	ctx.SetColor(color.RGBA{R: 225, G: 231, B: 241, A: 255})
	ctx.Fill()

	ctx.DrawRoundedRectangle(92, 112, width-184, height-224, 42)
	ctx.SetColor(color.RGBA{R: 252, G: 253, B: 255, A: 255})
	ctx.Fill()

	ctx.DrawRoundedRectangle(128, 188, width-256, 26, 13)
	ctx.SetColor(color.RGBA{R: 74, G: 110, B: 170, A: 255})
	ctx.Fill()
}

func drawAdminSpaceCoverTitle(dst *image.RGBA, title string) error {
	if dst == nil {
		return ErrAdminSpaceCoverImageInvalid
	}

	contentWidth := dst.Bounds().Dx() - adminSpaceCoverSafeMargin*2
	if contentWidth <= 0 {
		return ErrAdminSpaceCoverImageInvalid
	}

	face, lines, lineHeight, err := chooseAdminSpaceCoverTitleLayout(title, contentWidth)
	if err != nil {
		return err
	}

	textColor := image.NewUniform(color.RGBA{R: 25, G: 45, B: 84, A: 255})
	drawer := &font.Drawer{
		Dst:  dst,
		Src:  textColor,
		Face: face,
	}

	totalHeight := lineHeight * len(lines)
	startY := (dst.Bounds().Dy()-totalHeight)/2 + lineHeight
	if startY < adminSpaceCoverSafeMargin+lineHeight/2 {
		startY = adminSpaceCoverSafeMargin + lineHeight/2
	}
	startX := adminSpaceCoverSafeMargin

	for index, line := range lines {
		y := startY + index*lineHeight
		drawer.Dot = fixed.Point26_6{
			X: fixed.I(startX),
			Y: fixed.I(y),
		}
		drawer.DrawString(line)
	}

	return nil
}

func chooseAdminSpaceCoverTitleLayout(
	title string,
	maxWidth int,
) (font.Face, []string, int, error) {
	sizes := []float64{154, 146, 138, 130, 122, 114, 106, 98, 92, 86, 80, 74, 68, 62}
	for _, size := range sizes {
		face, err := loadAdminSpaceCoverFontFace(size)
		if err != nil {
			if errors.Is(err, ErrAdminSpaceFontUnavailable) {
				return nil, nil, 0, err
			}
			continue
		}

		lines, truncated := wrapAdminSpaceCoverTitle(title, face, maxWidth, adminSpaceCoverMaxTitleLines)
		lineHeight := computeAdminSpaceCoverLineHeight(face, size)
		if len(lines) == 0 {
			continue
		}

		maxBlockHeight := int(math.Round(float64(adminSpaceCoverMaxHeight) * 0.58))
		if lineHeight*len(lines) > maxBlockHeight {
			continue
		}
		if truncated && size > 86 {
			continue
		}
		return face, lines, lineHeight, nil
	}

	face, err := loadAdminSpaceCoverFontFace(60)
	if err != nil {
		return nil, nil, 0, err
	}
	lines, _ := wrapAdminSpaceCoverTitle(title, face, maxWidth, adminSpaceCoverMaxTitleLines)
	if len(lines) == 0 {
		return nil, nil, 0, ErrAdminSpaceCoverImageInvalid
	}
	return face, lines, computeAdminSpaceCoverLineHeight(face, 60), nil
}

func wrapAdminSpaceCoverTitle(
	title string,
	face font.Face,
	maxWidth int,
	maxLines int,
) ([]string, bool) {
	if face == nil || maxWidth <= 0 || maxLines <= 0 {
		return nil, false
	}

	tokens := tokenizeAdminSpaceCoverTitle(title)
	if len(tokens) == 0 {
		return []string{"未命名空间"}, false
	}

	lines := make([]string, 0, maxLines)
	current := ""
	truncated := false

	appendLine := func(value string) {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			return
		}
		lines = append(lines, normalized)
	}

	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if strings.TrimSpace(token) == "" {
			if current == "" || strings.HasSuffix(current, " ") {
				continue
			}
			token = " "
		}

		candidate := current + token
		if measureAdminSpaceCoverTextWidth(face, candidate) <= maxWidth {
			current = candidate
			continue
		}

		if strings.TrimSpace(current) == "" {
			current = fitAdminSpaceCoverToken(face, token, maxWidth)
		}

		if len(lines) == maxLines-1 {
			remaining := strings.TrimSpace(current + token + strings.Join(tokens[index+1:], ""))
			appendLine(trimAdminSpaceCoverTextWithEllipsis(face, remaining, maxWidth))
			truncated = true
			return lines, truncated
		}

		appendLine(current)
		current = strings.TrimLeft(token, " ")
	}

	if strings.TrimSpace(current) != "" {
		if len(lines) >= maxLines {
			lines[maxLines-1] = trimAdminSpaceCoverTextWithEllipsis(face, lines[maxLines-1], maxWidth)
			return lines[:maxLines], true
		}
		appendLine(current)
	}

	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = trimAdminSpaceCoverTextWithEllipsis(face, lines[maxLines-1], maxWidth)
		truncated = true
	}
	return lines, truncated
}

func tokenizeAdminSpaceCoverTitle(title string) []string {
	normalized := normalizeAdminSpaceCoverTitle(title)
	if normalized == "" {
		return nil
	}
	matches := spaceTitleTokenRegexp.FindAllString(normalized, -1)
	if len(matches) == 0 {
		return []string{normalized}
	}
	return matches
}

func normalizeAdminSpaceCoverTitle(raw string) string {
	title := strings.TrimSpace(raw)
	if title == "" {
		return ""
	}
	parts := strings.Fields(strings.ReplaceAll(title, "\n", " "))
	if len(parts) == 0 {
		return ""
	}
	normalized := strings.Join(parts, " ")
	if normalized == "" {
		return "未命名空间"
	}
	return normalized
}

func fitAdminSpaceCoverToken(face font.Face, token string, maxWidth int) string {
	if strings.TrimSpace(token) == "" {
		return ""
	}
	if measureAdminSpaceCoverTextWidth(face, token) <= maxWidth {
		return token
	}
	runes := []rune(strings.TrimSpace(token))
	if len(runes) == 0 {
		return ""
	}
	for size := len(runes); size > 0; size-- {
		candidate := string(runes[:size])
		if measureAdminSpaceCoverTextWidth(face, candidate) <= maxWidth {
			return candidate
		}
	}
	return string(runes[0])
}

func trimAdminSpaceCoverTextWithEllipsis(face font.Face, text string, maxWidth int) string {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return "…"
	}
	if measureAdminSpaceCoverTextWidth(face, normalized) <= maxWidth {
		return normalized
	}

	const ellipsis = "…"
	runes := []rune(normalized)
	for size := len(runes) - 1; size >= 0; size-- {
		candidate := strings.TrimSpace(string(runes[:size])) + ellipsis
		if measureAdminSpaceCoverTextWidth(face, candidate) <= maxWidth {
			return candidate
		}
	}
	return ellipsis
}

func measureAdminSpaceCoverTextWidth(face font.Face, text string) int {
	if face == nil {
		return 0
	}
	return font.MeasureString(face, text).Ceil()
}

func computeAdminSpaceCoverLineHeight(face font.Face, fontSize float64) int {
	if face == nil {
		return int(math.Round(fontSize * 1.3))
	}
	metrics := face.Metrics()
	base := metrics.Ascent.Ceil() + metrics.Descent.Ceil()
	if base <= 0 {
		base = int(math.Round(fontSize * 1.2))
	}
	gap := int(math.Round(float64(base) * 0.28))
	if gap < 12 {
		gap = 12
	}
	return base + gap
}

func loadAdminSpaceCoverFontFace(size float64) (font.Face, error) {
	fontData, err := adminSpaceFontLoader.loadFontData()
	if err != nil {
		return nil, err
	}
	parsed, err := opentype.Parse(fontData)
	if err != nil {
		return nil, ErrAdminSpaceFontUnavailable
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, ErrAdminSpaceFontUnavailable
	}
	return face, nil
}

func (r *adminSpaceFontRegistry) loadFontData() ([]byte, error) {
	r.once.Do(func() {
		candidates := adminSpaceCoverFontCandidates()
		for _, candidate := range candidates {
			if strings.TrimSpace(candidate) == "" {
				continue
			}
			content, err := os.ReadFile(candidate)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				continue
			}
			if len(content) == 0 {
				continue
			}
			parsed, parseErr := opentype.Parse(content)
			if parseErr != nil || parsed == nil {
				continue
			}
			r.fontData = content
			r.fontPath = candidate
			return
		}
		r.err = ErrAdminSpaceFontUnavailable
	})

	if len(r.fontData) == 0 {
		if r.err != nil {
			return nil, r.err
		}
		return nil, ErrAdminSpaceFontUnavailable
	}
	return r.fontData, nil
}

func adminSpaceCoverFontCandidates() []string {
	custom := strings.TrimSpace(os.Getenv("PLAINDOC_SPACE_COVER_FONT_PATH"))
	candidates := []string{
		custom,
		"/usr/share/fonts/truetype/noto/NotoSansSC-Regular.ttf",
		"/usr/share/fonts/opentype/noto/NotoSansSC-Regular.otf",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.otf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	}
	return candidates
}

func computeAdminSpaceCoverCropRect(bounds image.Rectangle) image.Rectangle {
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return image.Rect(0, 0, 1, 1)
	}

	targetRatio := float64(adminSpaceCoverTargetRatioNumerator) / float64(adminSpaceCoverTargetRatioDenominator)
	currentRatio := float64(width) / float64(height)

	if math.Abs(currentRatio-targetRatio) < 0.0001 {
		return bounds
	}

	if currentRatio > targetRatio {
		nextWidth := int(math.Round(float64(height) * targetRatio))
		if nextWidth <= 0 {
			nextWidth = 1
		}
		offsetX := (width - nextWidth) / 2
		return image.Rect(bounds.Min.X+offsetX, bounds.Min.Y, bounds.Min.X+offsetX+nextWidth, bounds.Max.Y)
	}

	nextHeight := int(math.Round(float64(width) / targetRatio))
	if nextHeight <= 0 {
		nextHeight = 1
	}
	offsetY := (height - nextHeight) / 2
	return image.Rect(bounds.Min.X, bounds.Min.Y+offsetY, bounds.Max.X, bounds.Min.Y+offsetY+nextHeight)
}

func computeAdminSpaceCoverOutputSize(sourceWidth int, sourceHeight int) (int, int) {
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return 0, 0
	}
	scale := math.Min(
		1,
		math.Min(
			float64(adminSpaceCoverMaxWidth)/float64(sourceWidth),
			float64(adminSpaceCoverMaxHeight)/float64(sourceHeight),
		),
	)
	width := int(math.Round(float64(sourceWidth) * scale))
	height := int(math.Round(float64(sourceHeight) * scale))
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	return width, height
}

func detectAdminSpaceCoverContentType(fileBytes []byte, fallbackContentType string) string {
	detected := strings.TrimSpace(strings.ToLower(http.DetectContentType(firstAdminSpaceBytes(fileBytes, 512))))
	if strings.HasPrefix(detected, "image/") {
		return detected
	}

	candidate := strings.TrimSpace(strings.ToLower(fallbackContentType))
	if strings.HasPrefix(candidate, "image/") {
		return candidate
	}
	extensions, err := mime.ExtensionsByType(candidate)
	if err == nil && len(extensions) > 0 {
		return candidate
	}
	return detected
}

func firstAdminSpaceBytes(input []byte, limit int) []byte {
	if len(input) <= limit {
		return input
	}
	return input[:limit]
}

func encodeAdminSpaceCoverWebP(imageValue image.Image, quality float64) ([]byte, error) {
	if imageValue == nil {
		return nil, ErrAdminSpaceCoverImageInvalid
	}
	var output bytes.Buffer
	if err := webp.Encode(&output, imageValue, &webp.Options{
		Lossless: false,
		Quality:  normalizeAdminSpaceCoverQuality(quality),
	}); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func normalizeAdminSpaceCoverQuality(raw float64) float32 {
	quality := raw
	if quality <= 0 {
		quality = adminSpaceCoverDefaultQuality
	}
	if quality <= 1 {
		quality *= 100
	}
	if quality < 10 {
		quality = 10
	}
	if quality > 100 {
		quality = 100
	}
	return float32(quality)
}

func buildAdminSpaceCoverObjectKey(now time.Time) (string, error) {
	randomSuffix, err := randomAdminSpaceCoverHex(4)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"%s/%04d/%02d/%02d/%d-%s.webp",
		adminSpaceCoverObjectPrefix,
		now.Year(),
		int(now.Month()),
		now.Day(),
		now.UnixMilli(),
		randomSuffix,
	), nil
}

func saveAdminSpaceCoverObject(objectKey string, content []byte) error {
	if strings.TrimSpace(objectKey) == "" {
		return errors.New("space cover object key is required")
	}
	if len(content) == 0 {
		return errors.New("space cover content is empty")
	}

	targetPath := filepath.Join(adminSpaceCoverStorageRootDir, filepath.FromSlash(objectKey))
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, content, 0o644)
}

func resolveAdminSpaceCoverPublicURL(objectKey string) string {
	return strings.TrimRight(adminSpaceCoverPublicPrefix, "/") + "/" + strings.TrimLeft(objectKey, "/")
}

func randomAdminSpaceCoverHex(lengthBytes int) (string, error) {
	if lengthBytes <= 0 {
		return "", errors.New("random length must be positive")
	}
	buffer := make([]byte, lengthBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
