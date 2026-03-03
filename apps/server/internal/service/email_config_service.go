package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	// SystemConfigKeyEmail 邮件系统配置键。
	SystemConfigKeyEmail = "email"

	defaultEmailPasswordResetTokenTTLMinutes         = 30
	defaultEmailPasswordResetMinRequestIntervalSecs  = 60
	defaultEmailPasswordResetMaxRequestsPerHourEmail = 5
	defaultEmailPasswordResetMaxRequestsPerHourIP    = 20
)

// EmailSMTPConfig 邮件 SMTP 配置。
type EmailSMTPConfig struct {
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Username         string `json:"username"`
	PasswordCiphertext string `json:"passwordCiphertext"`
	Security         string `json:"security"`
	ConnectTimeoutMS int    `json:"connectTimeoutMs"`
	SendTimeoutMS    int    `json:"sendTimeoutMs"`
}

// EmailPasswordResetConfig 密码重置邮件策略配置。
type EmailPasswordResetConfig struct {
	TokenTTLMinutes           int `json:"tokenTTLMinutes"`
	MinRequestIntervalSeconds int `json:"minRequestIntervalSeconds"`
	MaxRequestsPerHourPerEmail int `json:"maxRequestsPerHourPerEmail"`
	MaxRequestsPerHourPerIP   int `json:"maxRequestsPerHourPerIP"`
}

// EmailConfig 邮件系统配置。
type EmailConfig struct {
	Enabled       bool                     `json:"enabled"`
	FromName      string                   `json:"fromName"`
	FromEmail     string                   `json:"fromEmail"`
	ReplyTo       string                   `json:"replyTo"`
	AppBaseURL    string                   `json:"appBaseUrl"`
	PasswordReset EmailPasswordResetConfig `json:"passwordReset"`
	SMTP          EmailSMTPConfig          `json:"smtp"`
}

// EmailConfigService 统一读取邮件配置。
type EmailConfigService struct {
	systemConfigRepo repository.SystemConfigRepository
}

// NewEmailConfigService 创建邮件配置服务。
func NewEmailConfigService(systemConfigRepo repository.SystemConfigRepository) *EmailConfigService {
	return &EmailConfigService{systemConfigRepo: systemConfigRepo}
}

// DefaultEmailConfig 返回默认邮件配置。
func DefaultEmailConfig() EmailConfig {
	return EmailConfig{
		Enabled:    false,
		FromName:   "PlainDoc",
		FromEmail:  "",
		ReplyTo:    "",
		AppBaseURL: "",
		PasswordReset: EmailPasswordResetConfig{
			TokenTTLMinutes:            defaultEmailPasswordResetTokenTTLMinutes,
			MinRequestIntervalSeconds:  defaultEmailPasswordResetMinRequestIntervalSecs,
			MaxRequestsPerHourPerEmail: defaultEmailPasswordResetMaxRequestsPerHourEmail,
			MaxRequestsPerHourPerIP:    defaultEmailPasswordResetMaxRequestsPerHourIP,
		},
		SMTP: EmailSMTPConfig{
			Host:               "",
			Port:               587,
			Username:           "",
			PasswordCiphertext: "",
			Security:           "starttls",
			ConnectTimeoutMS:   3000,
			SendTimeoutMS:      5000,
		},
	}
}

// NormalizeEmailConfig 将任意邮件配置归一化为可用结构。
func NormalizeEmailConfig(value map[string]any) EmailConfig {
	config := DefaultEmailConfig()
	if value == nil {
		return config
	}

	if enabled, ok := value["enabled"].(bool); ok {
		config.Enabled = enabled
	}
	if fromName := strings.TrimSpace(readString(value, "fromName")); fromName != "" {
		config.FromName = fromName
	}
	config.FromEmail = strings.TrimSpace(readString(value, "fromEmail"))
	config.ReplyTo = strings.TrimSpace(readString(value, "replyTo"))
	config.AppBaseURL = strings.TrimSpace(readString(value, "appBaseUrl"))

	if passwordReset, ok := readObject(value, "passwordReset"); ok {
		config.PasswordReset.TokenTTLMinutes = readInt(
			passwordReset,
			"tokenTTLMinutes",
			config.PasswordReset.TokenTTLMinutes,
		)
		config.PasswordReset.MinRequestIntervalSeconds = readInt(
			passwordReset,
			"minRequestIntervalSeconds",
			config.PasswordReset.MinRequestIntervalSeconds,
		)
		config.PasswordReset.MaxRequestsPerHourPerEmail = readInt(
			passwordReset,
			"maxRequestsPerHourPerEmail",
			config.PasswordReset.MaxRequestsPerHourPerEmail,
		)
		config.PasswordReset.MaxRequestsPerHourPerIP = readInt(
			passwordReset,
			"maxRequestsPerHourPerIP",
			config.PasswordReset.MaxRequestsPerHourPerIP,
		)
	}

	if smtp, ok := readObject(value, "smtp"); ok {
		config.SMTP.Host = strings.TrimSpace(readString(smtp, "host"))
		config.SMTP.Port = readInt(smtp, "port", config.SMTP.Port)
		config.SMTP.Username = strings.TrimSpace(readString(smtp, "username"))
		config.SMTP.PasswordCiphertext = readString(smtp, "passwordCiphertext")
		config.SMTP.Security = strings.ToLower(strings.TrimSpace(readString(smtp, "security")))
		config.SMTP.ConnectTimeoutMS = readInt(smtp, "connectTimeoutMs", config.SMTP.ConnectTimeoutMS)
		config.SMTP.SendTimeoutMS = readInt(smtp, "sendTimeoutMs", config.SMTP.SendTimeoutMS)
	}

	if config.SMTP.Port <= 0 {
		config.SMTP.Port = 587
	}
	switch config.SMTP.Security {
	case "plain", "starttls", "tls":
	default:
		config.SMTP.Security = "starttls"
	}
	if config.SMTP.ConnectTimeoutMS <= 0 {
		config.SMTP.ConnectTimeoutMS = 3000
	}
	if config.SMTP.SendTimeoutMS <= 0 {
		config.SMTP.SendTimeoutMS = 5000
	}

	if config.PasswordReset.TokenTTLMinutes <= 0 {
		config.PasswordReset.TokenTTLMinutes = defaultEmailPasswordResetTokenTTLMinutes
	}
	if config.PasswordReset.MinRequestIntervalSeconds < 0 {
		config.PasswordReset.MinRequestIntervalSeconds = defaultEmailPasswordResetMinRequestIntervalSecs
	}
	if config.PasswordReset.MaxRequestsPerHourPerEmail <= 0 {
		config.PasswordReset.MaxRequestsPerHourPerEmail = defaultEmailPasswordResetMaxRequestsPerHourEmail
	}
	if config.PasswordReset.MaxRequestsPerHourPerIP <= 0 {
		config.PasswordReset.MaxRequestsPerHourPerIP = defaultEmailPasswordResetMaxRequestsPerHourIP
	}

	return config
}

// GetConfig 返回当前生效邮件配置；未配置时回退默认值。
func (s *EmailConfigService) GetConfig(ctx context.Context) (EmailConfig, error) {
	defaultConfig := DefaultEmailConfig()
	if s == nil || s.systemConfigRepo == nil {
		return defaultConfig, nil
	}

	config, err := s.systemConfigRepo.GetByConfigKey(ctx, SystemConfigKeyEmail)
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
	return NormalizeEmailConfig(payload), nil
}
