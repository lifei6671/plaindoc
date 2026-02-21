package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

type siteConfigPolicy struct {
	AllowRegistration *bool `json:"allowRegistration"`
}

// AuthRegistrationPolicyService 负责读取注册相关系统配置。
type AuthRegistrationPolicyService struct {
	systemConfigRepo repository.SystemConfigRepository
}

// NewAuthRegistrationPolicyService 创建注册策略服务。
func NewAuthRegistrationPolicyService(systemConfigRepo repository.SystemConfigRepository) *AuthRegistrationPolicyService {
	return &AuthRegistrationPolicyService{
		systemConfigRepo: systemConfigRepo,
	}
}

// AllowRegistration 判断是否允许新用户注册；缺省配置时默认允许。
func (s *AuthRegistrationPolicyService) AllowRegistration(ctx context.Context) (bool, error) {
	if s == nil || s.systemConfigRepo == nil {
		return true, nil
	}

	config, err := s.systemConfigRepo.GetByConfigKey(ctx, "site")
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return true, nil
		}
		return false, err
	}
	if config == nil {
		return true, nil
	}

	var payload siteConfigPolicy
	if err := json.Unmarshal([]byte(config.ConfigValueJSON), &payload); err != nil {
		return false, err
	}
	if payload.AllowRegistration == nil {
		return true, nil
	}
	return *payload.AllowRegistration, nil
}
