package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

type SitemapGenerationMode string

const (
	// SitemapConfigKey 为 sitemap 生成策略系统配置键。
	SitemapConfigKey = "sitemap"

	// SitemapGenerationModeAllPublic 表示输出全部公开文档。
	SitemapGenerationModeAllPublic SitemapGenerationMode = "all_public"
	// SitemapGenerationModeUpdatedWithinDays 表示仅输出最近 N 天更新文档。
	SitemapGenerationModeUpdatedWithinDays SitemapGenerationMode = "updated_within_days"

	sitemapMinMaxUpdatedWithinDays     = 1
	sitemapMaxMaxUpdatedWithinDays     = 3650
	sitemapDefaultMaxUpdatedWithinDays = 180
)

// SitemapConfig 为 sitemap 生成规则配置。
type SitemapConfig struct {
	GenerationMode       SitemapGenerationMode `json:"generationMode"`
	MaxUpdatedWithinDays int                   `json:"maxUpdatedWithinDays"`
}

// SitemapPublicDocumentRecord 表示可进入 sitemap 的公开文档项。
type SitemapPublicDocumentRecord struct {
	SpaceID           string
	DocumentID        string
	SpaceUpdatedAt    time.Time
	DocumentUpdatedAt time.Time
}

type sitemapPublicDocumentRow struct {
	SpaceID              string `gorm:"column:space_id"`
	SpaceUpdatedAtRaw    string `gorm:"column:space_updated_at"`
	DocumentID           string `gorm:"column:document_id"`
	DocumentContentMD    string `gorm:"column:document_content_md"`
	DocumentUpdatedAtRaw string `gorm:"column:document_updated_at"`
}

// SitemapService 负责生成 sitemap 所需的公开数据集合。
type SitemapService struct {
	db               *gorm.DB
	systemConfigRepo repository.SystemConfigRepository
}

// NewSitemapService 创建 sitemap 服务。
func NewSitemapService(
	db *gorm.DB,
	systemConfigRepo repository.SystemConfigRepository,
) *SitemapService {
	return &SitemapService{
		db:               db,
		systemConfigRepo: systemConfigRepo,
	}
}

// ListPublicDocuments 返回「空间与文档均为完全公开」且文档内容非空的记录。
func (s *SitemapService) ListPublicDocuments(
	ctx context.Context,
) ([]SitemapPublicDocumentRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sitemap service db is nil")
	}

	config, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	updatedAfter := time.Time{}
	if config.GenerationMode == SitemapGenerationModeUpdatedWithinDays {
		updatedAfter = time.Now().UTC().AddDate(0, 0, -config.MaxUpdatedWithinDays)
	}

	var rows []sitemapPublicDocumentRow
	if err := s.db.WithContext(ctx).
		Table("documents AS d").
		Select(
			"s.space_id AS space_id",
			"s.updated_at AS space_updated_at",
			"d.document_id AS document_id",
			"d.content_md AS document_content_md",
			"d.updated_at AS document_updated_at",
		).
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Where("n.type = ?", models.NodeTypeDoc).
		Where("s.visibility = ?", models.VisibilityPublic).
		Where("s.status = ?", models.EntityStatusActive).
		Where("d.visibility = ?", models.VisibilityPublic).
		Where("d.status = ?", models.EntityStatusActive).
		Order("s.space_id ASC, d.document_id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]SitemapPublicDocumentRecord, 0, len(rows))
	for _, row := range rows {
		spaceID := strings.TrimSpace(row.SpaceID)
		documentID := strings.TrimSpace(row.DocumentID)
		if spaceID == "" || documentID == "" {
			continue
		}
		// 过滤空白内容文档，避免无内容页面进入 sitemap。
		if strings.TrimSpace(row.DocumentContentMD) == "" {
			continue
		}
		documentUpdatedAt := parseSitemapRecordTime(row.DocumentUpdatedAtRaw)
		if !updatedAfter.IsZero() {
			if documentUpdatedAt.IsZero() || documentUpdatedAt.Before(updatedAfter) {
				continue
			}
		}
		result = append(result, SitemapPublicDocumentRecord{
			SpaceID:           spaceID,
			DocumentID:        documentID,
			SpaceUpdatedAt:    parseSitemapRecordTime(row.SpaceUpdatedAtRaw),
			DocumentUpdatedAt: documentUpdatedAt,
		})
	}
	return result, nil
}

// DefaultSitemapConfig 返回 sitemap 默认生成规则。
func DefaultSitemapConfig() SitemapConfig {
	return SitemapConfig{
		GenerationMode:       SitemapGenerationModeAllPublic,
		MaxUpdatedWithinDays: sitemapDefaultMaxUpdatedWithinDays,
	}
}

// NormalizeSitemapConfig 将任意配置归一为可用结构。
func NormalizeSitemapConfig(value map[string]any) SitemapConfig {
	config := DefaultSitemapConfig()
	if value == nil {
		return config
	}

	switch normalizeSitemapGenerationMode(readSitemapConfigString(value, "generationMode")) {
	case SitemapGenerationModeAllPublic:
		config.GenerationMode = SitemapGenerationModeAllPublic
	case SitemapGenerationModeUpdatedWithinDays:
		config.GenerationMode = SitemapGenerationModeUpdatedWithinDays
	}

	if maxDays, ok := readSitemapConfigInt(value, "maxUpdatedWithinDays"); ok {
		config.MaxUpdatedWithinDays = normalizeSitemapMaxUpdatedWithinDays(maxDays)
	}
	return config
}

// GetConfig 返回当前生效 sitemap 配置；未配置时回退默认值。
func (s *SitemapService) GetConfig(ctx context.Context) (SitemapConfig, error) {
	defaultConfig := DefaultSitemapConfig()
	if s == nil || s.systemConfigRepo == nil {
		return defaultConfig, nil
	}

	config, err := s.systemConfigRepo.GetByConfigKey(ctx, SitemapConfigKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultConfig, nil
		}
		return defaultConfig, err
	}
	if config == nil || strings.TrimSpace(config.ConfigValueJSON) == "" {
		return defaultConfig, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(config.ConfigValueJSON), &payload); err != nil {
		return defaultConfig, err
	}
	return NormalizeSitemapConfig(payload), nil
}

func normalizeSitemapGenerationMode(rawValue string) SitemapGenerationMode {
	switch strings.ToLower(strings.TrimSpace(rawValue)) {
	case string(SitemapGenerationModeAllPublic):
		return SitemapGenerationModeAllPublic
	case string(SitemapGenerationModeUpdatedWithinDays):
		return SitemapGenerationModeUpdatedWithinDays
	default:
		return ""
	}
}

func normalizeSitemapMaxUpdatedWithinDays(value int) int {
	switch {
	case value < sitemapMinMaxUpdatedWithinDays:
		return sitemapMinMaxUpdatedWithinDays
	case value > sitemapMaxMaxUpdatedWithinDays:
		return sitemapMaxMaxUpdatedWithinDays
	default:
		return value
	}
}

func readSitemapConfigString(payload map[string]any, key string) string {
	rawValue, ok := payload[key]
	if !ok {
		return ""
	}
	value, ok := rawValue.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func readSitemapConfigInt(payload map[string]any, key string) (int, bool) {
	rawValue, ok := payload[key]
	if !ok {
		return 0, false
	}

	switch value := rawValue.(type) {
	case int:
		return value, true
	case int8:
		return int(value), true
	case int16:
		return int(value), true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case uint:
		return int(value), true
	case uint8:
		return int(value), true
	case uint16:
		return int(value), true
	case uint32:
		return int(value), true
	case uint64:
		return int(value), true
	case float32:
		number := float64(value)
		if math.IsNaN(number) || math.IsInf(number, 0) || number != math.Trunc(number) {
			return 0, false
		}
		return int(number), true
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
			return 0, false
		}
		return int(value), true
	default:
		return 0, false
	}
}

func parseSitemapRecordTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsedAt, err := time.Parse(layout, value); err == nil {
			return parsedAt.UTC()
		}
	}

	normalized := strings.Replace(value, " ", "T", 1)
	timePart := normalized
	if index := strings.IndexByte(normalized, 'T'); index >= 0 && index < len(normalized)-1 {
		timePart = normalized[index+1:]
	}
	if !strings.ContainsAny(timePart, "Zz+-") {
		normalized += "Z"
	}
	if parsedAt, err := time.Parse(time.RFC3339Nano, normalized); err == nil {
		return parsedAt.UTC()
	}
	return time.Time{}
}
