package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	defaultDataRetentionEnabled                 = true
	defaultDataRetentionScheduleMinutes         = 60
	defaultDataRetentionBatchSize               = 500
	defaultAuditLogRetentionDays                = 180
	defaultAuthCaptchaRetentionHours            = 72
	defaultAuthRiskStateRetentionDays           = 30
	defaultUserSessionRetentionDays             = 30
	defaultDocumentRevisionRetentionCount       = 30
	defaultDataRetentionRetryIntervalWhenFailed = 10 * time.Minute
)

const (
	DataRetentionCleanupTableAuditLogs             = "audit_logs"
	DataRetentionCleanupTableAuthCaptchaChallenges = "auth_captcha_challenges"
	DataRetentionCleanupTableAuthRiskStates        = "auth_risk_states"
	DataRetentionCleanupTableUserSessions          = "user_sessions"
	DataRetentionCleanupTableDocumentAttachments   = "document_attachments"
	DataRetentionCleanupTableDocumentImageAssets   = "document_image_assets"
	DataRetentionCleanupTableDocumentRevisions     = "document_revisions"
)

var defaultDataRetentionCleanupTables = []string{
	DataRetentionCleanupTableAuditLogs,
	DataRetentionCleanupTableAuthCaptchaChallenges,
	DataRetentionCleanupTableAuthRiskStates,
	DataRetentionCleanupTableUserSessions,
	DataRetentionCleanupTableDocumentAttachments,
	DataRetentionCleanupTableDocumentImageAssets,
	DataRetentionCleanupTableDocumentRevisions,
}

// DataRetentionPolicy 描述数据清理策略。
type DataRetentionPolicy struct {
	Enabled                        bool
	ScheduleMinutes                int
	CleanupBatchSize               int
	CleanupTables                  []string
	AuditLogRetentionDays          int
	AuthCaptchaRetentionHours      int
	AuthRiskStateRetentionDays     int
	UserSessionRetentionDays       int
	DocumentRevisionRetentionCount int
}

// DataRetentionCleanupResult 描述一次清理执行结果。
type DataRetentionCleanupResult struct {
	Policy                       DataRetentionPolicy
	StartedAt                    time.Time
	FinishedAt                   time.Time
	DeletedAuditLogs             int64
	DeletedAuthCaptchaChallenges int64
	DeletedAuthRiskStates        int64
	DeletedUserSessions          int64
	DeletedDocumentAttachments   int64
	DeletedAttachmentBlobs       int64
	DeletedDocumentImageAssets   int64
	DeletedDocumentRevisions     int64
}

type dataRetentionPolicyPayload struct {
	Enabled                        *bool    `json:"enabled"`
	ScheduleMinutes                *int     `json:"scheduleMinutes"`
	CleanupBatchSize               *int     `json:"cleanupBatchSize"`
	CleanupTables                  []string `json:"cleanupTables"`
	AuditLogRetentionDays          *int     `json:"auditLogRetentionDays"`
	AuthCaptchaRetentionHours      *int     `json:"authCaptchaRetentionHours"`
	AuthRiskStateRetentionDays     *int     `json:"authRiskStateRetentionDays"`
	UserSessionRetentionDays       *int     `json:"userSessionRetentionDays"`
	DocumentRevisionRetentionCount *int     `json:"documentRevisionRetentionCount"`
}

// DataRetentionCleanupService 负责按策略清理持续增长的审计和临时数据。
type DataRetentionCleanupService struct {
	dataRetentionRepo                repository.DataRetentionRepository
	systemConfigRepo                 repository.SystemConfigRepository
	documentAttachmentCleanupService *DocumentAttachmentCleanupService
	documentImageAssetService        *DocumentImageAssetService
}

// NewDataRetentionCleanupService 创建数据清理服务。
func NewDataRetentionCleanupService(
	db *gorm.DB,
	systemConfigRepo repository.SystemConfigRepository,
) *DataRetentionCleanupService {
	imageHostingService := NewImageHostingService(systemConfigRepo)
	documentAttachmentRepo := repository.NewGormDocumentAttachmentRepository(db)
	return &DataRetentionCleanupService{
		dataRetentionRepo:                repository.NewGormDataRetentionRepository(db),
		systemConfigRepo:                 systemConfigRepo,
		documentAttachmentCleanupService: NewDocumentAttachmentCleanupService(db, documentAttachmentRepo, imageHostingService),
		documentImageAssetService:        NewDocumentImageAssetService(db, imageHostingService),
	}
}

// RunOnce 按当前策略执行一次数据清理。
func (s *DataRetentionCleanupService) RunOnce(
	ctx context.Context,
) (DataRetentionCleanupResult, error) {
	return s.runOnce(ctx, false)
}

// RunOnceForced 按当前策略执行一次数据清理（忽略 enabled 开关）。
func (s *DataRetentionCleanupService) RunOnceForced(
	ctx context.Context,
) (DataRetentionCleanupResult, error) {
	return s.runOnce(ctx, true)
}

func (s *DataRetentionCleanupService) runOnce(
	ctx context.Context,
	force bool,
) (DataRetentionCleanupResult, error) {
	startedAt := time.Now().UTC()
	result := DataRetentionCleanupResult{
		Policy:    s.ResolvePolicy(ctx),
		StartedAt: startedAt,
	}
	defer func() {
		result.FinishedAt = time.Now().UTC()
	}()

	if s == nil || s.dataRetentionRepo == nil {
		return result, errors.New("data retention cleanup service repository is nil")
	}
	if !result.Policy.Enabled && !force {
		return result, nil
	}

	now := time.Now().UTC()
	auditLogCutoff := now.AddDate(0, 0, -result.Policy.AuditLogRetentionDays)
	authCaptchaCutoff := now.Add(-time.Duration(result.Policy.AuthCaptchaRetentionHours) * time.Hour)
	authRiskStateCutoff := now.AddDate(0, 0, -result.Policy.AuthRiskStateRetentionDays)
	userSessionCutoff := now.AddDate(0, 0, -result.Policy.UserSessionRetentionDays)

	var err error
	if hasDataRetentionCleanupTable(result.Policy.CleanupTables, DataRetentionCleanupTableAuditLogs) {
		result.DeletedAuditLogs, err = s.cleanupAuditLogs(ctx, auditLogCutoff, result.Policy.CleanupBatchSize)
		if err != nil {
			return result, err
		}
	}
	if hasDataRetentionCleanupTable(result.Policy.CleanupTables, DataRetentionCleanupTableAuthCaptchaChallenges) {
		result.DeletedAuthCaptchaChallenges, err = s.cleanupAuthCaptchaChallenges(ctx, authCaptchaCutoff, result.Policy.CleanupBatchSize)
		if err != nil {
			return result, err
		}
	}
	if hasDataRetentionCleanupTable(result.Policy.CleanupTables, DataRetentionCleanupTableAuthRiskStates) {
		result.DeletedAuthRiskStates, err = s.cleanupAuthRiskStates(ctx, authRiskStateCutoff, result.Policy.CleanupBatchSize)
		if err != nil {
			return result, err
		}
	}
	if hasDataRetentionCleanupTable(result.Policy.CleanupTables, DataRetentionCleanupTableUserSessions) {
		result.DeletedUserSessions, err = s.cleanupUserSessions(ctx, userSessionCutoff, result.Policy.CleanupBatchSize)
		if err != nil {
			return result, err
		}
	}
	if hasDataRetentionCleanupTable(result.Policy.CleanupTables, DataRetentionCleanupTableDocumentRevisions) {
		var deletedRevisionBlobs int64
		result.DeletedDocumentRevisions, deletedRevisionBlobs, err = s.cleanupDocumentRevisions(
			ctx,
			result.Policy.DocumentRevisionRetentionCount,
			result.Policy.CleanupBatchSize,
		)
		if err != nil {
			return result, err
		}
		result.DeletedAttachmentBlobs += deletedRevisionBlobs
	}
	if hasDataRetentionCleanupTable(result.Policy.CleanupTables, DataRetentionCleanupTableDocumentAttachments) {
		var deletedAttachmentBlobs int64
		result.DeletedDocumentAttachments, deletedAttachmentBlobs, err = s.cleanupDocumentAttachments(ctx, result.Policy.CleanupBatchSize)
		if err != nil {
			return result, err
		}
		result.DeletedAttachmentBlobs += deletedAttachmentBlobs
	}
	if hasDataRetentionCleanupTable(result.Policy.CleanupTables, DataRetentionCleanupTableDocumentImageAssets) {
		result.DeletedDocumentImageAssets, err = s.cleanupDocumentImageAssets(ctx, result.Policy.CleanupBatchSize)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

// ResolvePolicy 返回当前生效的数据清理策略；当配置缺失或非法时回退默认值。
func (s *DataRetentionCleanupService) ResolvePolicy(ctx context.Context) DataRetentionPolicy {
	policy := defaultDataRetentionPolicy()
	if s == nil || s.systemConfigRepo == nil {
		return policy
	}

	config, err := s.systemConfigRepo.GetByConfigKey(ctx, SystemConfigKeyDataRetention)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return policy
		}
		return policy
	}
	if config == nil || strings.TrimSpace(config.ConfigValueJSON) == "" {
		return policy
	}

	var payload dataRetentionPolicyPayload
	if err := json.Unmarshal([]byte(config.ConfigValueJSON), &payload); err != nil {
		return policy
	}
	patchDataRetentionPolicy(&policy, payload)
	normalizeDataRetentionPolicy(&policy)
	return policy
}

// ResolveNextRunInterval 返回下一次清理的执行间隔。
func (s *DataRetentionCleanupService) ResolveNextRunInterval(ctx context.Context) time.Duration {
	policy := s.ResolvePolicy(ctx)
	return time.Duration(policy.ScheduleMinutes) * time.Minute
}

// ResolveCleanupRetryInterval 返回清理失败后的重试间隔。
func ResolveCleanupRetryInterval() time.Duration {
	return defaultDataRetentionRetryIntervalWhenFailed
}

func defaultDataRetentionPolicy() DataRetentionPolicy {
	return DataRetentionPolicy{
		Enabled:                        defaultDataRetentionEnabled,
		ScheduleMinutes:                defaultDataRetentionScheduleMinutes,
		CleanupBatchSize:               defaultDataRetentionBatchSize,
		CleanupTables:                  append([]string(nil), defaultDataRetentionCleanupTables...),
		AuditLogRetentionDays:          defaultAuditLogRetentionDays,
		AuthCaptchaRetentionHours:      defaultAuthCaptchaRetentionHours,
		AuthRiskStateRetentionDays:     defaultAuthRiskStateRetentionDays,
		UserSessionRetentionDays:       defaultUserSessionRetentionDays,
		DocumentRevisionRetentionCount: defaultDocumentRevisionRetentionCount,
	}
}

func patchDataRetentionPolicy(policy *DataRetentionPolicy, patch dataRetentionPolicyPayload) {
	if policy == nil {
		return
	}
	if patch.Enabled != nil {
		policy.Enabled = *patch.Enabled
	}
	if patch.ScheduleMinutes != nil {
		policy.ScheduleMinutes = *patch.ScheduleMinutes
	}
	if patch.CleanupBatchSize != nil {
		policy.CleanupBatchSize = *patch.CleanupBatchSize
	}
	if patch.CleanupTables != nil {
		policy.CleanupTables = normalizeDataRetentionCleanupTables(patch.CleanupTables)
	}
	if patch.AuditLogRetentionDays != nil {
		policy.AuditLogRetentionDays = *patch.AuditLogRetentionDays
	}
	if patch.AuthCaptchaRetentionHours != nil {
		policy.AuthCaptchaRetentionHours = *patch.AuthCaptchaRetentionHours
	}
	if patch.AuthRiskStateRetentionDays != nil {
		policy.AuthRiskStateRetentionDays = *patch.AuthRiskStateRetentionDays
	}
	if patch.UserSessionRetentionDays != nil {
		policy.UserSessionRetentionDays = *patch.UserSessionRetentionDays
	}
	if patch.DocumentRevisionRetentionCount != nil {
		policy.DocumentRevisionRetentionCount = *patch.DocumentRevisionRetentionCount
	}
}

func normalizeDataRetentionPolicy(policy *DataRetentionPolicy) {
	if policy == nil {
		return
	}
	policy.ScheduleMinutes = clampRetentionInt(
		policy.ScheduleMinutes,
		minDataRetentionScheduleMinutes,
		maxDataRetentionScheduleMinutes,
	)
	policy.CleanupBatchSize = clampRetentionInt(
		policy.CleanupBatchSize,
		minDataRetentionBatchSize,
		maxDataRetentionBatchSize,
	)
	policy.CleanupTables = normalizeDataRetentionCleanupTables(policy.CleanupTables)
	if len(policy.CleanupTables) == 0 {
		policy.CleanupTables = append([]string(nil), defaultDataRetentionCleanupTables...)
	}
	policy.AuditLogRetentionDays = clampRetentionInt(
		policy.AuditLogRetentionDays,
		minAuditLogRetentionDays,
		maxAuditLogRetentionDays,
	)
	policy.AuthCaptchaRetentionHours = clampRetentionInt(
		policy.AuthCaptchaRetentionHours,
		minAuthCaptchaRetentionHours,
		maxAuthCaptchaRetentionHours,
	)
	policy.AuthRiskStateRetentionDays = clampRetentionInt(
		policy.AuthRiskStateRetentionDays,
		minAuthRiskStateRetentionDays,
		maxAuthRiskStateRetentionDays,
	)
	policy.UserSessionRetentionDays = clampRetentionInt(
		policy.UserSessionRetentionDays,
		minUserSessionRetentionDays,
		maxUserSessionRetentionDays,
	)
	policy.DocumentRevisionRetentionCount = clampRetentionInt(
		policy.DocumentRevisionRetentionCount,
		minDocumentRevisionRetentionCount,
		maxDocumentRevisionRetentionCount,
	)
}

func clampRetentionInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func normalizeDataRetentionCleanupTables(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(input))
	for _, item := range input {
		normalized := strings.TrimSpace(strings.ToLower(item))
		if normalized == "" || !isSupportedDataRetentionCleanupTable(normalized) {
			continue
		}
		if slices.Contains(result, normalized) {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func isSupportedDataRetentionCleanupTable(table string) bool {
	switch strings.TrimSpace(strings.ToLower(table)) {
	case DataRetentionCleanupTableAuditLogs,
		DataRetentionCleanupTableAuthCaptchaChallenges,
		DataRetentionCleanupTableAuthRiskStates,
		DataRetentionCleanupTableUserSessions,
		DataRetentionCleanupTableDocumentAttachments,
		DataRetentionCleanupTableDocumentImageAssets,
		DataRetentionCleanupTableDocumentRevisions:
		return true
	default:
		return false
	}
}

func hasDataRetentionCleanupTable(selectedTables []string, table string) bool {
	normalizedTable := strings.TrimSpace(strings.ToLower(table))
	if normalizedTable == "" {
		return false
	}
	for _, item := range selectedTables {
		if strings.TrimSpace(strings.ToLower(item)) == normalizedTable {
			return true
		}
	}
	return false
}

func (s *DataRetentionCleanupService) cleanupAuditLogs(ctx context.Context, cutoff time.Time, batchSize int) (int64, error) {
	if s == nil || s.dataRetentionRepo == nil {
		return 0, errors.New("data retention cleanup repository is nil")
	}
	deleted, err := s.dataRetentionRepo.DeleteAuditLogsBefore(ctx, cutoff, batchSize)
	if err != nil {
		return deleted, fmt.Errorf("cleanup audit_logs failed: %w", err)
	}
	return deleted, nil
}

func (s *DataRetentionCleanupService) cleanupAuthCaptchaChallenges(ctx context.Context, cutoff time.Time, batchSize int) (int64, error) {
	if s == nil || s.dataRetentionRepo == nil {
		return 0, errors.New("data retention cleanup repository is nil")
	}
	deleted, err := s.dataRetentionRepo.DeleteAuthCaptchaChallengesBefore(ctx, cutoff, batchSize)
	if err != nil {
		return deleted, fmt.Errorf("cleanup auth_captcha_challenges failed: %w", err)
	}
	return deleted, nil
}

func (s *DataRetentionCleanupService) cleanupAuthRiskStates(ctx context.Context, cutoff time.Time, batchSize int) (int64, error) {
	if s == nil || s.dataRetentionRepo == nil {
		return 0, errors.New("data retention cleanup repository is nil")
	}
	deleted, err := s.dataRetentionRepo.DeleteAuthRiskStatesBefore(ctx, cutoff, batchSize)
	if err != nil {
		return deleted, fmt.Errorf("cleanup auth_risk_states failed: %w", err)
	}
	return deleted, nil
}

func (s *DataRetentionCleanupService) cleanupUserSessions(ctx context.Context, cutoff time.Time, batchSize int) (int64, error) {
	if s == nil || s.dataRetentionRepo == nil {
		return 0, errors.New("data retention cleanup repository is nil")
	}
	deleted, err := s.dataRetentionRepo.DeleteUserSessionsBefore(ctx, cutoff, batchSize)
	if err != nil {
		return deleted, fmt.Errorf("cleanup user_sessions failed: %w", err)
	}
	return deleted, nil
}

func (s *DataRetentionCleanupService) cleanupDocumentImageAssets(
	ctx context.Context,
	batchSize int,
) (int64, error) {
	if s == nil || s.documentImageAssetService == nil {
		return 0, nil
	}
	deleted, err := s.documentImageAssetService.CleanupPendingDocumentImageAssets(ctx, batchSize)
	if err != nil {
		return deleted, fmt.Errorf("cleanup document_image_assets failed: %w", err)
	}
	return deleted, nil
}

func (s *DataRetentionCleanupService) cleanupDocumentAttachments(
	ctx context.Context,
	batchSize int,
) (int64, int64, error) {
	if s == nil || s.documentAttachmentCleanupService == nil {
		return 0, 0, nil
	}
	result, err := s.documentAttachmentCleanupService.CleanupDeletedDocumentAttachments(ctx, batchSize)
	if err != nil {
		return result.DeletedAttachments, result.DeletedBlobs, fmt.Errorf("cleanup document_attachments failed: %w", err)
	}
	return result.DeletedAttachments, result.DeletedBlobs, nil
}

func (s *DataRetentionCleanupService) cleanupDocumentRevisions(
	ctx context.Context,
	keepCount int,
	batchSize int,
) (int64, int64, error) {
	if s == nil || s.dataRetentionRepo == nil {
		return 0, 0, errors.New("data retention cleanup repository is nil")
	}
	deleteResult, err := s.dataRetentionRepo.DeleteDocumentRevisionsExceedingKeepCount(ctx, keepCount, batchSize)
	if err != nil {
		return deleteResult.DeletedRows, 0, fmt.Errorf("cleanup document_revisions failed: %w", err)
	}
	if s.documentAttachmentCleanupService == nil {
		return deleteResult.DeletedRows, 0, nil
	}
	var deletedBlobs int64
	for _, blobID := range deleteResult.ReleasedBlobIDs {
		deletedBlob, cleanupErr := s.documentAttachmentCleanupService.cleanupBlobIfUnreferenced(ctx, blobID)
		if cleanupErr != nil {
			return deleteResult.DeletedRows, deletedBlobs, fmt.Errorf(
				"cleanup released revision blob %s failed: %w",
				blobID,
				cleanupErr,
			)
		}
		if deletedBlob {
			deletedBlobs++
		}
	}
	return deleteResult.DeletedRows, deletedBlobs, nil
}
