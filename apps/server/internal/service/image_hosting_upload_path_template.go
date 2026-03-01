package service

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

const (
	// DefaultImageHostingUploadPathTemplate 统一默认图片对象路径模板。
	DefaultImageHostingUploadPathTemplate = "images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}"
	maxImageHostingUploadPathTemplateLen  = 256
)

var (
	imageHostingUploadPathVariablePattern = regexp.MustCompile(`\{([a-zA-Z][a-zA-Z0-9]*)\}`)
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
		if _, ok := imageHostingUploadPathVariables[variable]; !ok {
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
