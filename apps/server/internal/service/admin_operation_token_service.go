package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
)

const (
	defaultAdminOperationTokenTTL = 2 * time.Minute
)

type adminOperationTokenRecord struct {
	ActorUserID string
	Operation   string
	TargetType  string
	TargetID    string
	ExpiresAt   time.Time
	Consumed    bool
}

// IssueAdminOperationTokenInput 定义后台高风险操作 token 的签发参数。
type IssueAdminOperationTokenInput struct {
	ActorUserID string
	Operation   string
	TargetType  string
	TargetID    string
}

// AdminOperationTokenIssueResult 定义后台高风险操作 token 的签发结果。
type AdminOperationTokenIssueResult struct {
	Token     string
	ExpiresAt time.Time
}

// ConsumeAdminOperationTokenInput 定义后台高风险操作 token 的消费参数。
type ConsumeAdminOperationTokenInput struct {
	ActorUserID string
	Token       string
	Operation   string
	TargetType  string
	TargetID    string
}

// AdminOperationTokenService 管理后台高风险操作的一次性防重放 token。
type AdminOperationTokenService struct {
	adminAccessService *AdminAccessService
	ttl                time.Duration

	mu     sync.Mutex
	tokens map[string]adminOperationTokenRecord
	nowFn  func() time.Time
}

// NewAdminOperationTokenService 创建后台高风险操作 token 服务。
func NewAdminOperationTokenService(
	adminAccessService *AdminAccessService,
	ttl time.Duration,
) *AdminOperationTokenService {
	if ttl <= 0 {
		ttl = defaultAdminOperationTokenTTL
	}

	return &AdminOperationTokenService{
		adminAccessService: adminAccessService,
		ttl:                ttl,
		tokens:             make(map[string]adminOperationTokenRecord),
		nowFn:              time.Now,
	}
}

// Issue 签发一次性操作 token。
func (s *AdminOperationTokenService) Issue(
	ctx context.Context,
	input IssueAdminOperationTokenInput,
) (result AdminOperationTokenIssueResult, err error) {
	defer func() {
		err = errcode.MapAdminOperationTokenError(err)
	}()

	if s == nil || s.adminAccessService == nil {
		return AdminOperationTokenIssueResult{}, errors.New("admin operation token service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return AdminOperationTokenIssueResult{}, errcode.ErrAdminForbidden
	}
	isAdmin, err := s.adminAccessService.IsAdmin(ctx, actorUserID)
	if err != nil {
		return AdminOperationTokenIssueResult{}, err
	}
	if !isAdmin {
		return AdminOperationTokenIssueResult{}, errcode.ErrAdminForbidden
	}

	operation := normalizeAdminOperationTokenOperation(input.Operation)
	if operation == "" {
		return AdminOperationTokenIssueResult{}, errcode.ErrAdminOperationTokenInvalidOperation
	}

	targetType := normalizeAdminOperationTokenTargetType(input.TargetType)
	targetID := strings.TrimSpace(input.TargetID)

	tokenValue, err := generateAdminOperationTokenValue()
	if err != nil {
		return AdminOperationTokenIssueResult{}, err
	}

	now := s.nowFn().UTC()
	record := adminOperationTokenRecord{
		ActorUserID: actorUserID,
		Operation:   operation,
		TargetType:  targetType,
		TargetID:    targetID,
		ExpiresAt:   now.Add(s.ttl),
		Consumed:    false,
	}

	tokenHash := hashAdminOperationToken(tokenValue)
	s.mu.Lock()
	s.cleanupExpiredTokensLocked(now)
	s.tokens[tokenHash] = record
	s.mu.Unlock()

	return AdminOperationTokenIssueResult{
		Token:     tokenValue,
		ExpiresAt: record.ExpiresAt,
	}, nil
}

// Consume 校验并消费一次性操作 token（单次可用，重复使用会失败）。
func (s *AdminOperationTokenService) Consume(
	ctx context.Context,
	input ConsumeAdminOperationTokenInput,
) (err error) {
	defer func() {
		err = errcode.MapAdminOperationTokenError(err)
	}()

	if s == nil || s.adminAccessService == nil {
		return errors.New("admin operation token service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return errcode.ErrAdminForbidden
	}
	isAdmin, err := s.adminAccessService.IsAdmin(ctx, actorUserID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return errcode.ErrAdminForbidden
	}

	tokenValue := strings.TrimSpace(input.Token)
	if tokenValue == "" {
		return errcode.ErrAdminOperationTokenRequired
	}
	operation := normalizeAdminOperationTokenOperation(input.Operation)
	if operation == "" {
		return errcode.ErrAdminOperationTokenInvalidOperation
	}

	targetType := normalizeAdminOperationTokenTargetType(input.TargetType)
	targetID := strings.TrimSpace(input.TargetID)

	tokenHash := hashAdminOperationToken(tokenValue)
	now := s.nowFn().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredTokensLocked(now)

	record, exists := s.tokens[tokenHash]
	if !exists {
		return errcode.ErrAdminOperationTokenInvalid
	}

	if record.ExpiresAt.Before(now) {
		delete(s.tokens, tokenHash)
		return errcode.ErrAdminOperationTokenExpired
	}

	if record.Consumed {
		return errcode.ErrAdminOperationTokenReplayed
	}

	if record.ActorUserID != actorUserID {
		return errcode.ErrAdminOperationTokenInvalid
	}
	if record.Operation != operation {
		return errcode.ErrAdminOperationTokenScopeMismatch
	}
	if !matchAdminOperationTarget(record.TargetType, record.TargetID, targetType, targetID) {
		return errcode.ErrAdminOperationTokenScopeMismatch
	}

	record.Consumed = true
	s.tokens[tokenHash] = record
	return nil
}

func (s *AdminOperationTokenService) cleanupExpiredTokensLocked(now time.Time) {
	for tokenHash, record := range s.tokens {
		if record.ExpiresAt.After(now) {
			continue
		}
		delete(s.tokens, tokenHash)
	}
}

func normalizeAdminOperationTokenOperation(rawOperation string) string {
	return strings.ToLower(strings.TrimSpace(rawOperation))
}

func normalizeAdminOperationTokenTargetType(rawTargetType string) string {
	return strings.ToLower(strings.TrimSpace(rawTargetType))
}

func matchAdminOperationTarget(
	tokenTargetType string,
	tokenTargetID string,
	requestTargetType string,
	requestTargetID string,
) bool {
	if tokenTargetType != requestTargetType {
		return false
	}
	if tokenTargetID != requestTargetID {
		return false
	}
	return true
}

func generateAdminOperationTokenValue() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashAdminOperationToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}
