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
	DocumentRouteKey  string
	SpaceUpdatedAt    time.Time
	DocumentUpdatedAt time.Time
}

// SitemapService 负责生成 sitemap 所需的公开数据集合。
type SitemapService struct {
	sitemapRepo      repository.SitemapRepository
	systemConfigRepo repository.SystemConfigRepository
}

// NewSitemapService 创建 sitemap 服务。
func NewSitemapService(
	db *gorm.DB,
	systemConfigRepo repository.SystemConfigRepository,
) *SitemapService {
	return &SitemapService{
		sitemapRepo:      repository.NewGormSitemapRepository(db),
		systemConfigRepo: systemConfigRepo,
	}
}

// ListPublicDocuments 返回「空间与文档均为完全公开」且文档内容非空的记录。
func (s *SitemapService) ListPublicDocuments(
	ctx context.Context,
) ([]SitemapPublicDocumentRecord, error) {
	if s == nil || s.sitemapRepo == nil {
		return nil, errors.New("sitemap service repository is nil")
	}

	config, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	updatedAfter := time.Time{}
	if config.GenerationMode == SitemapGenerationModeUpdatedWithinDays {
		updatedAfter = time.Now().UTC().AddDate(0, 0, -config.MaxUpdatedWithinDays)
	}

	rows, err := s.sitemapRepo.ListPublicDocuments(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]SitemapPublicDocumentRecord, 0, len(rows))
	for _, row := range rows {
		spaceID := strings.TrimSpace(row.SpaceID)
		documentID := strings.TrimSpace(row.DocumentID)
		if spaceID == "" || documentID == "" {
			continue
		}
		if models.NormalizeDocumentFormat(row.DocumentFormat) != models.DocumentFormatMarkdown {
			continue
		}
		// 过滤空白内容文档，避免无内容页面进入 sitemap。
		if strings.TrimSpace(row.DocumentContentMD) == "" {
			continue
		}
		documentUpdatedAt := row.DocumentUpdatedAt
		if !updatedAfter.IsZero() {
			if documentUpdatedAt.IsZero() || documentUpdatedAt.Before(updatedAfter) {
				continue
			}
		}
		result = append(result, SitemapPublicDocumentRecord{
			SpaceID:           spaceID,
			DocumentID:        documentID,
			DocumentRouteKey:  strings.TrimSpace(row.DocumentRouteKey),
			SpaceUpdatedAt:    row.SpaceUpdatedAt,
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
