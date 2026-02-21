package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

type ImageHostingProvider string

const (
	ImageHostingProviderLocal        ImageHostingProvider = "local"
	ImageHostingProviderCloudflareR2 ImageHostingProvider = "cloudflare-r2"
	ImageHostingProviderAliyunOSS    ImageHostingProvider = "aliyun-oss"
)

// CloudflareR2ImageHostingConfig Cloudflare R2 图床配置。
type CloudflareR2ImageHostingConfig struct {
	AccountID       string `json:"accountId"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	PublicBaseURL   string `json:"publicBaseUrl"`
}

// AliyunOSSImageHostingConfig 阿里云 OSS 图床配置。
type AliyunOSSImageHostingConfig struct {
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret"`
	PublicBaseURL   string `json:"publicBaseUrl"`
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
			AccountID:       "",
			Bucket:          "",
			AccessKeyID:     "",
			SecretAccessKey: "",
			PublicBaseURL:   "",
		},
		AliyunOSS: AliyunOSSImageHostingConfig{
			Region:          "",
			Bucket:          "",
			Endpoint:        "",
			AccessKeyID:     "",
			AccessKeySecret: "",
			PublicBaseURL:   "",
		},
		Local: LocalImageHostingConfig{
			UploadEndpoint: "/api/uploads/images",
			PublicBaseURL:  "/api/uploads/local",
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
	}

	if aliyunOSS, ok := readObject(value, "aliyunOss"); ok {
		config.AliyunOSS.Region = readString(aliyunOSS, "region")
		config.AliyunOSS.Bucket = readString(aliyunOSS, "bucket")
		config.AliyunOSS.Endpoint = readString(aliyunOSS, "endpoint")
		config.AliyunOSS.AccessKeyID = readString(aliyunOSS, "accessKeyId")
		config.AliyunOSS.AccessKeySecret = readString(aliyunOSS, "accessKeySecret")
		config.AliyunOSS.PublicBaseURL = readString(aliyunOSS, "publicBaseUrl")
	}

	if local, ok := readObject(value, "local"); ok {
		if uploadEndpoint := readString(local, "uploadEndpoint"); uploadEndpoint != "" {
			config.Local.UploadEndpoint = uploadEndpoint
		}
		if publicBaseURL := readString(local, "publicBaseUrl"); publicBaseURL != "" {
			config.Local.PublicBaseURL = publicBaseURL
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
