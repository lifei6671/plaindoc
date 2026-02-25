package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	HomepageAnonymousCacheConfigKey = "homepage.ssr.anonymous_cache"
	homepageDefaultPageSize         = 20
	homepageMaxPageSize             = 200
	homepageDefaultMaxAgeSeconds    = 60
	homepageDefaultSMaxAgeSeconds   = 300
	homepageDefaultSWRSeconds       = 60
)

// HomepageQueryInput 首页/分类页查询参数。
type HomepageQueryInput struct {
	ViewerUserID string
	CategoryID   string
	Page         int
	PageSize     int
}

// HomepageCategoryRecord 首页/分类页分类导航项。
type HomepageCategoryRecord struct {
	CategoryID string
	Name       string
	IsDefault  bool
}

// HomepageSpaceRecord 首页/分类页空间卡片项。
type HomepageSpaceRecord struct {
	SpaceID     string
	Name        string
	Description string
	CategoryID  string
	CoverURL    string
	Visibility  models.Visibility
	PublishedAt time.Time
	UpdatedAt   time.Time
	OwnerUserID string
	OwnerName   string
	OwnerAvatar string
}

// HomepagePagination 首页/分类页分页信息。
type HomepagePagination struct {
	Page     int
	PageSize int
	Total    int64
}

// HomepagePageRecord 首页/分类页页面数据。
type HomepagePageRecord struct {
	Categories       []HomepageCategoryRecord
	Spaces           []HomepageSpaceRecord
	ActiveCategoryID string
	ActiveCategory   string
	Pagination       HomepagePagination
	CacheControl     string
}

type homepageAnonymousCacheConfig struct {
	MaxAgeSeconds               int `json:"maxAgeSeconds"`
	SMaxAgeSeconds              int `json:"sMaxAgeSeconds"`
	StaleWhileRevalidateSeconds int `json:"staleWhileRevalidateSeconds"`
}

// HomeService 封装首页/分类页查询与缓存策略。
type HomeService struct {
	spaceRepo         repository.SpaceRepository
	spaceCategoryRepo repository.SpaceCategoryRepository
	systemConfigRepo  repository.SystemConfigRepository
}

// NewHomeService 创建首页服务。
func NewHomeService(
	spaceRepo repository.SpaceRepository,
	spaceCategoryRepo repository.SpaceCategoryRepository,
	systemConfigRepo repository.SystemConfigRepository,
) *HomeService {
	return &HomeService{
		spaceRepo:         spaceRepo,
		spaceCategoryRepo: spaceCategoryRepo,
		systemConfigRepo:  systemConfigRepo,
	}
}

// GetPage 查询首页/分类页数据（含可见性过滤、分类导航与缓存策略）。
func (s *HomeService) GetPage(ctx context.Context, input HomepageQueryInput) (HomepagePageRecord, error) {
	if s == nil || s.spaceRepo == nil || s.spaceCategoryRepo == nil {
		return HomepagePageRecord{}, errors.New("home service dependencies are nil")
	}

	page, pageSize, offset := normalizeHomepagePagination(input.Page, input.PageSize)
	viewerUserID := strings.TrimSpace(input.ViewerUserID)
	categoryID := strings.TrimSpace(input.CategoryID)

	categories, err := s.spaceCategoryRepo.List(ctx)
	if err != nil {
		return HomepagePageRecord{}, err
	}

	records, total, err := s.spaceRepo.ListVisibleForHomepage(ctx, repository.ListVisibleHomepageSpacesParams{
		ViewerUserID: viewerUserID,
		CategoryID:   categoryID,
		Limit:        pageSize,
		Offset:       offset,
	})
	if err != nil {
		return HomepagePageRecord{}, err
	}

	result := HomepagePageRecord{
		Categories:       make([]HomepageCategoryRecord, 0, len(categories)),
		Spaces:           make([]HomepageSpaceRecord, 0, len(records)),
		ActiveCategoryID: categoryID,
		Pagination: HomepagePagination{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
		CacheControl: "private, no-store, max-age=0",
	}

	categoryNameByID := make(map[string]string, len(categories))
	for _, item := range categories {
		normalizedCategoryID := strings.TrimSpace(item.CategoryID)
		normalizedCategoryName := strings.TrimSpace(item.Name)
		if normalizedCategoryID != "" && normalizedCategoryName != "" {
			categoryNameByID[normalizedCategoryID] = normalizedCategoryName
		}
		result.Categories = append(result.Categories, HomepageCategoryRecord{
			CategoryID: normalizedCategoryID,
			Name:       normalizedCategoryName,
			IsDefault:  item.IsDefault,
		})
	}

	result.ActiveCategory = strings.TrimSpace(categoryNameByID[categoryID])
	if result.ActiveCategory == "" && categoryID == models.DefaultSpaceCategoryID {
		result.ActiveCategory = models.DefaultSpaceCategoryName
	}

	for _, item := range records {
		space := item.Space
		ownerName := strings.TrimSpace(item.OwnerName)
		if ownerName == "" {
			ownerName = strings.TrimSpace(space.OwnerUserID)
		}
		if ownerName == "" {
			ownerName = "未知用户"
		}
		result.Spaces = append(result.Spaces, HomepageSpaceRecord{
			SpaceID:     strings.TrimSpace(space.SpaceID),
			Name:        strings.TrimSpace(space.Name),
			Description: strings.TrimSpace(space.Description),
			CategoryID:  strings.TrimSpace(space.CategoryID),
			CoverURL:    strings.TrimSpace(space.CoverURL),
			Visibility:  space.Visibility,
			PublishedAt: space.CreatedAt,
			UpdatedAt:   space.UpdatedAt,
			OwnerUserID: strings.TrimSpace(space.OwnerUserID),
			OwnerName:   ownerName,
			OwnerAvatar: strings.TrimSpace(item.OwnerAvatarURL),
		})
	}

	if viewerUserID == "" {
		result.CacheControl = s.resolveAnonymousCacheControl(ctx)
	}

	return result, nil
}

func normalizeHomepagePagination(rawPage int, rawPageSize int) (int, int, int) {
	page := rawPage
	if page <= 0 {
		page = 1
	}

	pageSize := rawPageSize
	if pageSize <= 0 {
		pageSize = homepageDefaultPageSize
	}
	if pageSize > homepageMaxPageSize {
		pageSize = homepageMaxPageSize
	}

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	return page, pageSize, offset
}

func (s *HomeService) resolveAnonymousCacheControl(ctx context.Context) string {
	config := homepageAnonymousCacheConfig{
		MaxAgeSeconds:               homepageDefaultMaxAgeSeconds,
		SMaxAgeSeconds:              homepageDefaultSMaxAgeSeconds,
		StaleWhileRevalidateSeconds: homepageDefaultSWRSeconds,
	}

	if s != nil && s.systemConfigRepo != nil {
		item, err := s.systemConfigRepo.GetByConfigKey(ctx, HomepageAnonymousCacheConfigKey)
		if err == nil && item != nil {
			_ = decodeHomepageAnonymousCacheConfig(item.ConfigValueJSON, &config)
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			// 关键分支：读取配置失败时回退默认值，避免影响页面可用性。
		}
	}

	return fmt.Sprintf(
		"public, max-age=%d, s-maxage=%d, stale-while-revalidate=%d",
		config.MaxAgeSeconds,
		config.SMaxAgeSeconds,
		config.StaleWhileRevalidateSeconds,
	)
}

func decodeHomepageAnonymousCacheConfig(
	rawJSON string,
	target *homepageAnonymousCacheConfig,
) error {
	if target == nil {
		return errors.New("homepage anonymous cache target is nil")
	}

	trimmed := strings.TrimSpace(rawJSON)
	if trimmed == "" {
		return nil
	}

	var payload homepageAnonymousCacheConfig
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return err
	}

	target.MaxAgeSeconds = normalizeHomepageCacheTTL(payload.MaxAgeSeconds, homepageDefaultMaxAgeSeconds)
	target.SMaxAgeSeconds = normalizeHomepageCacheTTL(payload.SMaxAgeSeconds, homepageDefaultSMaxAgeSeconds)
	target.StaleWhileRevalidateSeconds = normalizeHomepageCacheTTL(
		payload.StaleWhileRevalidateSeconds,
		homepageDefaultSWRSeconds,
	)
	return nil
}

func normalizeHomepageCacheTTL(value int, fallback int) int {
	if value < 0 {
		return fallback
	}
	if value > 86_400 {
		return 86_400
	}
	return value
}
