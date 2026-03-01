package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

type ImageHostingProvider string

const (
	ImageHostingProviderLocal        ImageHostingProvider = "local"
	ImageHostingProviderCloudflareR2 ImageHostingProvider = "cloudflare-r2"
	ImageHostingProviderAliyunOSS    ImageHostingProvider = "aliyun-oss"
)

type ImageHostingDownloadStrategy string

const (
	ImageHostingDownloadStrategyPublic ImageHostingDownloadStrategy = "public"
	ImageHostingDownloadStrategySigned ImageHostingDownloadStrategy = "signed"
)

const (
	defaultImageHostingSignedURLTTLSeconds = 24 * 60 * 60
	minImageHostingSignedURLTTLSeconds     = 60
	maxImageHostingSignedURLTTLSeconds     = 7 * 24 * 60 * 60
)

// CloudflareR2ImageHostingConfig Cloudflare R2 图床配置。
type CloudflareR2ImageHostingConfig struct {
	AccountID           string                       `json:"accountId"`
	Bucket              string                       `json:"bucket"`
	AccessKeyID         string                       `json:"accessKeyId"`
	SecretAccessKey     string                       `json:"secretAccessKey"`
	PublicBaseURL       string                       `json:"publicBaseUrl"`
	DownloadStrategy    ImageHostingDownloadStrategy `json:"downloadStrategy"`
	SignedURLTTLSeconds int                          `json:"signedUrlTtlSeconds"`
}

// AliyunOSSImageHostingConfig 阿里云 OSS 图床配置。
type AliyunOSSImageHostingConfig struct {
	Region              string                       `json:"region"`
	Bucket              string                       `json:"bucket"`
	Endpoint            string                       `json:"endpoint"`
	AccessKeyID         string                       `json:"accessKeyId"`
	AccessKeySecret     string                       `json:"accessKeySecret"`
	PublicBaseURL       string                       `json:"publicBaseUrl"`
	DownloadStrategy    ImageHostingDownloadStrategy `json:"downloadStrategy"`
	SignedURLTTLSeconds int                          `json:"signedUrlTtlSeconds"`
}

// LocalImageHostingConfig 本地图片存储配置。
type LocalImageHostingConfig struct {
	UploadEndpoint string `json:"uploadEndpoint"`
	PublicBaseURL  string `json:"publicBaseUrl"`
}

// ImageHostingConfig 图床系统配置。
type ImageHostingConfig struct {
	DefaultProvider ImageHostingProvider           `json:"defaultProvider"`
	CloudflareR2    CloudflareR2ImageHostingConfig `json:"cloudflareR2"`
	AliyunOSS       AliyunOSSImageHostingConfig    `json:"aliyunOss"`
	Local           LocalImageHostingConfig        `json:"local"`
}

// ImageHostingService 统一读取图床系统配置。
type ImageHostingService struct {
	systemConfigRepo repository.SystemConfigRepository
}

// NewImageHostingService 创建图床配置服务。
func NewImageHostingService(systemConfigRepo repository.SystemConfigRepository) *ImageHostingService {
	return &ImageHostingService{
		systemConfigRepo: systemConfigRepo,
	}
}

// DefaultImageHostingConfig 返回图床默认配置。
func DefaultImageHostingConfig() ImageHostingConfig {
	return ImageHostingConfig{
		DefaultProvider: ImageHostingProviderLocal,
		CloudflareR2: CloudflareR2ImageHostingConfig{
			AccountID:           "",
			Bucket:              "",
			AccessKeyID:         "",
			SecretAccessKey:     "",
			PublicBaseURL:       "",
			DownloadStrategy:    ImageHostingDownloadStrategyPublic,
			SignedURLTTLSeconds: defaultImageHostingSignedURLTTLSeconds,
		},
		AliyunOSS: AliyunOSSImageHostingConfig{
			Region:              "",
			Bucket:              "",
			Endpoint:            "",
			AccessKeyID:         "",
			AccessKeySecret:     "",
			PublicBaseURL:       "",
			DownloadStrategy:    ImageHostingDownloadStrategyPublic,
			SignedURLTTLSeconds: defaultImageHostingSignedURLTTLSeconds,
		},
		Local: LocalImageHostingConfig{
			UploadEndpoint: "/api/uploads/images",
			PublicBaseURL:  "/uploads",
		},
	}
}

// NormalizeImageHostingConfig 将任意图床配置归一为可用结构。
func NormalizeImageHostingConfig(value map[string]any) ImageHostingConfig {
	config := DefaultImageHostingConfig()
	if value == nil {
		return config
	}

	defaultProvider := normalizeImageHostingProvider(readString(value, "defaultProvider"))
	if defaultProvider == "" {
		defaultProvider = normalizeImageHostingProvider(readString(value, "activeProvider"))
	}
	if defaultProvider != "" {
		config.DefaultProvider = defaultProvider
	}

	if cloudflareR2, ok := readObject(value, "cloudflareR2"); ok {
		config.CloudflareR2.AccountID = readString(cloudflareR2, "accountId")
		config.CloudflareR2.Bucket = readString(cloudflareR2, "bucket")
		config.CloudflareR2.AccessKeyID = readString(cloudflareR2, "accessKeyId")
		config.CloudflareR2.SecretAccessKey = readString(cloudflareR2, "secretAccessKey")
		config.CloudflareR2.PublicBaseURL = readString(cloudflareR2, "publicBaseUrl")
		if strategy := normalizeImageHostingDownloadStrategy(readString(cloudflareR2, "downloadStrategy")); strategy != "" {
			config.CloudflareR2.DownloadStrategy = strategy
		}
		config.CloudflareR2.SignedURLTTLSeconds = normalizeImageHostingSignedURLTTLSeconds(
			readInt(cloudflareR2, "signedUrlTtlSeconds", config.CloudflareR2.SignedURLTTLSeconds),
		)
	}

	if aliyunOSS, ok := readObject(value, "aliyunOss"); ok {
		config.AliyunOSS.Region = readString(aliyunOSS, "region")
		config.AliyunOSS.Bucket = readString(aliyunOSS, "bucket")
		config.AliyunOSS.Endpoint = readString(aliyunOSS, "endpoint")
		config.AliyunOSS.AccessKeyID = readString(aliyunOSS, "accessKeyId")
		config.AliyunOSS.AccessKeySecret = readString(aliyunOSS, "accessKeySecret")
		config.AliyunOSS.PublicBaseURL = readString(aliyunOSS, "publicBaseUrl")
		if strategy := normalizeImageHostingDownloadStrategy(readString(aliyunOSS, "downloadStrategy")); strategy != "" {
			config.AliyunOSS.DownloadStrategy = strategy
		}
		config.AliyunOSS.SignedURLTTLSeconds = normalizeImageHostingSignedURLTTLSeconds(
			readInt(aliyunOSS, "signedUrlTtlSeconds", config.AliyunOSS.SignedURLTTLSeconds),
		)
	}

	if local, ok := readObject(value, "local"); ok {
		if uploadEndpoint := readString(local, "uploadEndpoint"); uploadEndpoint != "" {
			config.Local.UploadEndpoint = uploadEndpoint
		}
		if publicBaseURL := readString(local, "publicBaseUrl"); publicBaseURL != "" {
			config.Local.PublicBaseURL = normalizeLocalPublicBaseURL(publicBaseURL)
		}
	}

	return config
}

// ToMap 将图床配置序列化为 map 结构，便于统一输出为 JSON。
func (c ImageHostingConfig) ToMap() map[string]any {
	raw, err := json.Marshal(c)
	if err != nil {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{}
	}
	if payload == nil {
		return map[string]any{}
	}
	return payload
}

// GetConfig 返回当前生效图床配置；未配置时回退默认值。
func (s *ImageHostingService) GetConfig(ctx context.Context) (ImageHostingConfig, error) {
	defaultConfig := DefaultImageHostingConfig()
	if s == nil || s.systemConfigRepo == nil {
		return defaultConfig, nil
	}

	config, err := s.systemConfigRepo.GetByConfigKey(ctx, "image-hosting")
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
	return NormalizeImageHostingConfig(payload), nil
}

func normalizeImageHostingProvider(rawValue string) ImageHostingProvider {
	switch strings.ToLower(strings.TrimSpace(rawValue)) {
	case string(ImageHostingProviderLocal):
		return ImageHostingProviderLocal
	case string(ImageHostingProviderCloudflareR2):
		return ImageHostingProviderCloudflareR2
	case string(ImageHostingProviderAliyunOSS):
		return ImageHostingProviderAliyunOSS
	default:
		return ""
	}
}

func normalizeImageHostingDownloadStrategy(rawValue string) ImageHostingDownloadStrategy {
	switch strings.ToLower(strings.TrimSpace(rawValue)) {
	case string(ImageHostingDownloadStrategyPublic):
		return ImageHostingDownloadStrategyPublic
	case string(ImageHostingDownloadStrategySigned):
		return ImageHostingDownloadStrategySigned
	default:
		return ""
	}
}

func normalizeImageHostingSignedURLTTLSeconds(rawValue int) int {
	if rawValue < minImageHostingSignedURLTTLSeconds || rawValue > maxImageHostingSignedURLTTLSeconds {
		return defaultImageHostingSignedURLTTLSeconds
	}
	return rawValue
}

func (c ImageHostingConfig) DownloadStrategy(provider ImageHostingProvider) ImageHostingDownloadStrategy {
	switch provider {
	case ImageHostingProviderCloudflareR2:
		if c.CloudflareR2.DownloadStrategy != "" {
			return c.CloudflareR2.DownloadStrategy
		}
	case ImageHostingProviderAliyunOSS:
		if c.AliyunOSS.DownloadStrategy != "" {
			return c.AliyunOSS.DownloadStrategy
		}
	}
	return ImageHostingDownloadStrategyPublic
}

func (c ImageHostingConfig) SignedURLTTL(provider ImageHostingProvider) time.Duration {
	ttlSeconds := defaultImageHostingSignedURLTTLSeconds
	switch provider {
	case ImageHostingProviderCloudflareR2:
		ttlSeconds = normalizeImageHostingSignedURLTTLSeconds(c.CloudflareR2.SignedURLTTLSeconds)
	case ImageHostingProviderAliyunOSS:
		ttlSeconds = normalizeImageHostingSignedURLTTLSeconds(c.AliyunOSS.SignedURLTTLSeconds)
	}
	return time.Duration(ttlSeconds) * time.Second
}

func normalizeLocalPublicBaseURL(rawValue string) string {
	trimmed := strings.TrimSpace(rawValue)
	normalized := strings.TrimRight(trimmed, "/")
	if normalized == "/api/uploads/local" || normalized == "/uploads/local" {
		return "/uploads"
	}
	if strings.HasPrefix(trimmed, "/api/uploads/") {
		return strings.TrimPrefix(trimmed, "/api")
	}
	return trimmed
}

func readString(payload map[string]any, key string) string {
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

func readObject(payload map[string]any, key string) (map[string]any, bool) {
	rawValue, ok := payload[key]
	if !ok {
		return nil, false
	}
	value, ok := rawValue.(map[string]any)
	if !ok || value == nil {
		return nil, false
	}
	return value, true
}

func readInt(payload map[string]any, key string, fallback int) int {
	rawValue, ok := payload[key]
	if !ok {
		return fallback
	}
	switch value := rawValue.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil {
			return parsed
		}
	}
	return fallback
}
