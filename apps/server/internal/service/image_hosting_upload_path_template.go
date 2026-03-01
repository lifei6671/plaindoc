package service

import (
	"crypto/rand"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const (
	// DefaultImageHostingUploadPathTemplate 统一默认图片对象路径模板。
	DefaultImageHostingUploadPathTemplate = "images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}"
	maxImageHostingUploadPathTemplateLen  = 256
)

var (
	imageHostingUploadPathVariablePattern = regexp.MustCompile(`\{([^{}]+)\}`)
	imageHostingUploadPathRandPattern     = regexp.MustCompile(`(?i)^rand:(4|5|6|7|8|9|10)$`)
)

var imageHostingUploadPathVariables = map[string]struct{}{
	"spaceId":    {},
	"docId":      {},
	"yyyy":       {},
	"mm":         {},
	"dd":         {},
	"hh":         {},
	"assetId":    {},
	"origName":   {},
	"ext":        {},
	"uploaderId": {},
}

const imageHostingUploadPathRandomCharset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// NormalizeImageHostingUploadPathTemplate 将模板归一化为非空可用值。
func NormalizeImageHostingUploadPathTemplate(rawValue string) string {
	normalized := strings.TrimSpace(rawValue)
	if normalized == "" {
		return DefaultImageHostingUploadPathTemplate
	}
	return normalized
}

// ValidateImageHostingUploadPathTemplate 校验上传路径模板约束。
func ValidateImageHostingUploadPathTemplate(rawValue string) error {
	template := strings.TrimSpace(rawValue)
	if template == "" {
		return fmt.Errorf("uploadPathTemplate must not be empty")
	}
	if len(template) > maxImageHostingUploadPathTemplateLen {
		return fmt.Errorf("uploadPathTemplate must be at most %d chars", maxImageHostingUploadPathTemplateLen)
	}
	if strings.Contains(template, `\`) {
		return fmt.Errorf("uploadPathTemplate must not contain backslash")
	}
	if strings.Contains(template, "..") {
		return fmt.Errorf("uploadPathTemplate must not contain ..")
	}
	if strings.HasPrefix(template, "/") {
		return fmt.Errorf("uploadPathTemplate must be relative path")
	}
	if strings.Contains(template, "//") {
		return fmt.Errorf("uploadPathTemplate must not contain empty path segment")
	}
	if !strings.HasPrefix(template, "images/") {
		return fmt.Errorf("uploadPathTemplate must start with images/")
	}
	if !strings.Contains(template, "{assetId}") {
		return fmt.Errorf("uploadPathTemplate must contain {assetId}")
	}

	matchedPlaceholders := imageHostingUploadPathVariablePattern.FindAllStringSubmatch(template, -1)
	for _, item := range matchedPlaceholders {
		if len(item) != 2 {
			continue
		}
		variable := strings.TrimSpace(item[1])
		if !isSupportedImageHostingUploadPathVariable(variable) {
			return fmt.Errorf("uploadPathTemplate contains unsupported variable {%s}", variable)
		}
	}

	placeholderStripped := imageHostingUploadPathVariablePattern.ReplaceAllString(template, "x")
	if strings.Contains(placeholderStripped, "{") || strings.Contains(placeholderStripped, "}") {
		return fmt.Errorf("uploadPathTemplate contains invalid placeholder braces")
	}

	cleaned := path.Clean(template)
	if cleaned == "." || cleaned == "/" || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("uploadPathTemplate is invalid path")
	}
	return nil
}

// RenderImageHostingUploadPathTemplate 按模板与变量渲染最终对象 key 片段。
// 支持固定变量与动态随机变量 {Rand:N}（N 仅允许 4-10）。
func RenderImageHostingUploadPathTemplate(
	rawTemplate string,
	variables map[string]string,
) (string, error) {
	template := NormalizeImageHostingUploadPathTemplate(rawTemplate)
	var renderErr error
	rendered := imageHostingUploadPathVariablePattern.ReplaceAllStringFunc(template, func(placeholder string) string {
		if renderErr != nil {
			return placeholder
		}
		matched := imageHostingUploadPathVariablePattern.FindStringSubmatch(placeholder)
		if len(matched) != 2 {
			renderErr = fmt.Errorf("invalid placeholder %s", placeholder)
			return placeholder
		}
		variable := strings.TrimSpace(matched[1])
		if value, ok := variables[variable]; ok {
			return value
		}
		randLength, ok := parseImageHostingUploadPathRandLength(variable)
		if !ok {
			renderErr = fmt.Errorf("unsupported variable {%s}", variable)
			return placeholder
		}
		randomValue, err := generateImageHostingUploadPathRandString(randLength)
		if err != nil {
			renderErr = err
			return placeholder
		}
		return randomValue
	})
	if renderErr != nil {
		return "", renderErr
	}
	if strings.Contains(rendered, "{") || strings.Contains(rendered, "}") {
		return "", fmt.Errorf("unresolved placeholders in upload path template")
	}
	return rendered, nil
}

func isSupportedImageHostingUploadPathVariable(variable string) bool {
	if _, ok := imageHostingUploadPathVariables[variable]; ok {
		return true
	}
	_, ok := parseImageHostingUploadPathRandLength(variable)
	return ok
}

func parseImageHostingUploadPathRandLength(variable string) (int, bool) {
	matched := imageHostingUploadPathRandPattern.FindStringSubmatch(strings.TrimSpace(variable))
	if len(matched) != 2 {
		return 0, false
	}
	length, err := strconv.Atoi(strings.TrimSpace(matched[1]))
	if err != nil || length < 4 || length > 10 {
		return 0, false
	}
	return length, true
}

func generateImageHostingUploadPathRandString(length int) (string, error) {
	if length < 4 || length > 10 {
		return "", fmt.Errorf("rand length must be between 4 and 10")
	}
	randomBytes := make([]byte, length)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	charsetLength := byte(len(imageHostingUploadPathRandomCharset))
	output := make([]byte, length)
	for index, value := range randomBytes {
		output[index] = imageHostingUploadPathRandomCharset[value%charsetLength]
	}
	return string(output), nil
}
