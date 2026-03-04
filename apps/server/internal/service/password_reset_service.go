package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	minPasswordResetPasswordLength = 6

	passwordResetSourceSelfService    = "self_service"
	passwordResetSourceAdminInitiated = "admin_initiated"
)

var (
	ErrPasswordResetEmailDisabled   = errors.New("password reset email is disabled")
	ErrPasswordResetEmailSendFailed = errors.New("password reset email send failed")
	ErrPasswordResetRateLimited     = errors.New("password reset request rate limited")

	ErrPasswordResetUserNotFound      = errors.New("password reset user not found")
	ErrPasswordResetUserNotSupported  = errors.New("password reset user not supported")
	ErrPasswordResetTokenInvalid      = errors.New("password reset token invalid")
	ErrPasswordResetTokenExpired      = errors.New("password reset token expired")
	ErrPasswordResetTokenConsumed     = errors.New("password reset token consumed")
	ErrPasswordResetPasswordTooShort  = errors.New("password reset password too short")
	ErrPasswordResetConfirmMismatch   = errors.New("password reset password confirm mismatch")
	ErrPasswordResetPasswordUnchanged = errors.New("password reset password unchanged")
)

// RequestPasswordResetByEmailInput 自助找回密码参数。
type RequestPasswordResetByEmailInput struct {
	Email    string
	ClientIP string
}

// RequestPasswordResetByAdminInput 管理员触发重置密码参数。
type RequestPasswordResetByAdminInput struct {
	TargetUserID      string
	RequestedByUserID string
	ClientIP          string
}

// VerifyPasswordResetTokenResult 密码重置令牌校验结果。
type VerifyPasswordResetTokenResult struct {
	UserID    string
	ExpiresAt time.Time
}

// ConfirmPasswordResetInput 提交重置密码参数。
type ConfirmPasswordResetInput struct {
	Token           string
	NewPassword     string
	ConfirmPassword string
}

// PasswordResetService 处理密码重置申请、校验与确认。
type PasswordResetService struct {
	userRepo           repository.UserRepository
	userSessionRepo    repository.UserSessionRepository
	tokenRepo          repository.PasswordResetTokenRepository
	emailConfigService *EmailConfigService
	mailSender         MailSender
	secret             []byte
	now                func() time.Time
}

// NewPasswordResetService 创建密码重置服务。
func NewPasswordResetService(
	userRepo repository.UserRepository,
	userSessionRepo repository.UserSessionRepository,
	tokenRepo repository.PasswordResetTokenRepository,
	emailConfigService *EmailConfigService,
	mailSender MailSender,
	secret string,
) *PasswordResetService {
	if mailSender == nil {
		mailSender = NewSMTPMailSender()
	}
	return &PasswordResetService{
		userRepo:           userRepo,
		userSessionRepo:    userSessionRepo,
		tokenRepo:          tokenRepo,
		emailConfigService: emailConfigService,
		mailSender:         mailSender,
		secret:             []byte(secret),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// RequestByEmail 自助发起密码重置邮件。
func (s *PasswordResetService) RequestByEmail(ctx context.Context, input RequestPasswordResetByEmailInput) error {
	if s == nil || s.userRepo == nil || s.tokenRepo == nil || s.emailConfigService == nil || s.mailSender == nil {
		return errors.New("password reset service dependencies are nil")
	}

	cfg, err := s.emailConfigService.GetConfig(ctx)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return ErrPasswordResetEmailDisabled
	}

	email := normalizeEmail(input.Email)
	if email == "" || !strings.Contains(email, "@") {
		return nil
	}
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if !isPasswordResettableUser(user) {
		return nil
	}

	if err := s.checkRateLimit(ctx, cfg, user.UserID, input.ClientIP); err != nil {
		return err
	}
	return s.issueAndSendResetEmail(ctx, cfg, issuePasswordResetEmailInput{
		UserID:      user.UserID,
		UserEmail:   user.Email,
		UserName:    user.Name,
		Source:      passwordResetSourceSelfService,
		RequestIP:   input.ClientIP,
		RequestedBy: nil,
	})
}

// IsEmailEnabled 返回当前是否开启邮件能力（密码找回入口展示开关）。
func (s *PasswordResetService) IsEmailEnabled(ctx context.Context) (bool, error) {
	if s == nil || s.emailConfigService == nil {
		return false, nil
	}
	cfg, err := s.emailConfigService.GetConfig(ctx)
	if err != nil {
		return false, err
	}
	return cfg.Enabled, nil
}

// RequestByAdmin 管理员触发密码重置邮件发送。
func (s *PasswordResetService) RequestByAdmin(
	ctx context.Context,
	input RequestPasswordResetByAdminInput,
) (string, error) {
	if s == nil || s.userRepo == nil || s.tokenRepo == nil || s.emailConfigService == nil || s.mailSender == nil {
		return "", errors.New("password reset service dependencies are nil")
	}

	cfg, err := s.emailConfigService.GetConfig(ctx)
	if err != nil {
		return "", err
	}
	if !cfg.Enabled {
		return "", ErrPasswordResetEmailDisabled
	}

	targetUserID := strings.TrimSpace(input.TargetUserID)
	if targetUserID == "" {
		return "", ErrPasswordResetUserNotFound
	}

	user, err := s.userRepo.GetByUserID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrPasswordResetUserNotFound
		}
		return "", err
	}
	if !isPasswordResettableUser(user) {
		return "", ErrPasswordResetUserNotSupported
	}

	if err := s.checkRateLimit(ctx, cfg, user.UserID, input.ClientIP); err != nil {
		return "", err
	}

	requestedBy := strings.TrimSpace(input.RequestedByUserID)
	requestedByPtr := (*string)(nil)
	if requestedBy != "" {
		requestedByPtr = &requestedBy
	}
	if err := s.issueAndSendResetEmail(ctx, cfg, issuePasswordResetEmailInput{
		UserID:      user.UserID,
		UserEmail:   user.Email,
		UserName:    user.Name,
		Source:      passwordResetSourceAdminInitiated,
		RequestIP:   input.ClientIP,
		RequestedBy: requestedByPtr,
	}); err != nil {
		return "", err
	}
	return user.Email, nil
}

// VerifyToken 校验重置令牌是否有效。
func (s *PasswordResetService) VerifyToken(
	ctx context.Context,
	rawToken string,
) (VerifyPasswordResetTokenResult, error) {
	if s == nil || s.tokenRepo == nil {
		return VerifyPasswordResetTokenResult{}, errors.New("password reset service dependencies are nil")
	}

	tokenID, secretPart, err := parsePasswordResetRawToken(rawToken)
	if err != nil {
		return VerifyPasswordResetTokenResult{}, ErrPasswordResetTokenInvalid
	}
	token, err := s.tokenRepo.GetByTokenID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return VerifyPasswordResetTokenResult{}, ErrPasswordResetTokenInvalid
		}
		return VerifyPasswordResetTokenResult{}, err
	}
	if s.hashToken(tokenID+"."+secretPart) != strings.TrimSpace(token.TokenSecretHash) {
		return VerifyPasswordResetTokenResult{}, ErrPasswordResetTokenInvalid
	}
	now := s.now().UTC()
	if token.ConsumedAt != nil {
		return VerifyPasswordResetTokenResult{}, ErrPasswordResetTokenConsumed
	}
	if token.InvalidatedAt != nil {
		return VerifyPasswordResetTokenResult{}, ErrPasswordResetTokenInvalid
	}
	if !token.ExpiresAt.After(now) {
		return VerifyPasswordResetTokenResult{}, ErrPasswordResetTokenExpired
	}
	return VerifyPasswordResetTokenResult{
		UserID:    token.UserID,
		ExpiresAt: token.ExpiresAt,
	}, nil
}

// Confirm 使用重置令牌提交新密码。
func (s *PasswordResetService) Confirm(
	ctx context.Context,
	input ConfirmPasswordResetInput,
) error {
	if s == nil || s.userRepo == nil || s.userSessionRepo == nil || s.tokenRepo == nil {
		return errors.New("password reset service dependencies are nil")
	}
	if input.NewPassword != input.ConfirmPassword {
		return ErrPasswordResetConfirmMismatch
	}
	if len(input.NewPassword) < minPasswordResetPasswordLength {
		return ErrPasswordResetPasswordTooShort
	}

	tokenID, secretPart, err := parsePasswordResetRawToken(input.Token)
	if err != nil {
		return ErrPasswordResetTokenInvalid
	}
	now := s.now().UTC()
	tokenSecretHash := s.hashToken(tokenID + "." + secretPart)
	consumedToken, err := s.tokenRepo.Consume(ctx, repository.ConsumePasswordResetTokenParams{
		TokenID:         tokenID,
		TokenSecretHash: tokenSecretHash,
		ConsumedAt:      now,
		Now:             now,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.resolveTokenConsumeError(ctx, tokenID, tokenSecretHash, now)
		}
		return err
	}

	user, err := s.userRepo.GetByUserID(ctx, consumedToken.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPasswordResetUserNotFound
		}
		return err
	}
	if !isPasswordResettableUser(user) {
		return ErrPasswordResetUserNotSupported
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.NewPassword)) == nil {
		return ErrPasswordResetPasswordUnchanged
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	updated, err := s.userRepo.UpdatePassword(ctx, user.UserID, string(passwordHash), now)
	if err != nil {
		return err
	}
	if !updated {
		return ErrPasswordResetUserNotFound
	}
	if err := s.userSessionRepo.RevokeAllByUserID(ctx, user.UserID, now); err != nil {
		return err
	}
	return nil
}

func (s *PasswordResetService) resolveTokenConsumeError(
	ctx context.Context,
	tokenID string,
	tokenSecretHash string,
	now time.Time,
) error {
	token, err := s.tokenRepo.GetByTokenID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPasswordResetTokenInvalid
		}
		return err
	}
	if strings.TrimSpace(token.TokenSecretHash) != tokenSecretHash {
		return ErrPasswordResetTokenInvalid
	}
	if token.ConsumedAt != nil {
		return ErrPasswordResetTokenConsumed
	}
	if token.InvalidatedAt != nil {
		return ErrPasswordResetTokenInvalid
	}
	if !token.ExpiresAt.After(now) {
		return ErrPasswordResetTokenExpired
	}
	return ErrPasswordResetTokenInvalid
}

type issuePasswordResetEmailInput struct {
	UserID      string
	UserEmail   string
	UserName    string
	Source      string
	RequestIP   string
	RequestedBy *string
}

func (s *PasswordResetService) issueAndSendResetEmail(
	ctx context.Context,
	cfg EmailConfig,
	input issuePasswordResetEmailInput,
) error {
	now := s.now().UTC()
	_, _ = s.tokenRepo.InvalidateActiveByUserID(ctx, repository.InvalidatePasswordResetTokensParams{
		UserID:        input.UserID,
		InvalidatedAt: now,
	})

	tokenID := strings.ToLower(ulid.Make().String())
	secretPart, err := generatePasswordResetTokenSecret()
	if err != nil {
		return err
	}
	rawToken := tokenID + "." + secretPart
	tokenSecretHash := s.hashToken(rawToken)
	requestIPHash := s.hashIP(strings.TrimSpace(input.RequestIP))
	expiresAt := now.Add(time.Duration(cfg.PasswordReset.TokenTTLMinutes) * time.Minute)

	record := &models.PasswordResetToken{
		TokenID:           tokenID,
		TokenSecretHash:   tokenSecretHash,
		UserID:            strings.TrimSpace(input.UserID),
		Source:            strings.TrimSpace(input.Source),
		RequestedByUserID: input.RequestedBy,
		RequestIPHash:     requestIPHash,
		ExpiresAt:         expiresAt,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.tokenRepo.Create(ctx, record); err != nil {
		return err
	}

	resetURL := composePasswordResetURL(cfg.AppBaseURL, rawToken)
	subject := "PlainDoc 密码重置"
	body := buildPasswordResetMailText(input.UserName, resetURL, cfg.PasswordReset.TokenTTLMinutes)
	if err := s.mailSender.Send(ctx, cfg, MailMessage{
		To:       []string{input.UserEmail},
		Subject:  subject,
		TextBody: body,
	}); err != nil {
		return err
	}
	return nil
}

func (s *PasswordResetService) checkRateLimit(
	ctx context.Context,
	cfg EmailConfig,
	userID string,
	clientIP string,
) error {
	now := s.now().UTC()
	minIntervalSeconds := cfg.PasswordReset.MinRequestIntervalSeconds
	if minIntervalSeconds > 0 {
		since := now.Add(-time.Duration(minIntervalSeconds) * time.Second)
		recentCount, err := s.tokenRepo.CountRecent(ctx, repository.CountPasswordResetTokensParams{
			UserID: strings.TrimSpace(userID),
			Since:  since,
		})
		if err != nil {
			return err
		}
		if recentCount > 0 {
			return ErrPasswordResetRateLimited
		}
	}

	emailHourLimit := cfg.PasswordReset.MaxRequestsPerHourPerEmail
	if emailHourLimit > 0 {
		hourCount, err := s.tokenRepo.CountRecent(ctx, repository.CountPasswordResetTokensParams{
			UserID: strings.TrimSpace(userID),
			Since:  now.Add(-time.Hour),
		})
		if err != nil {
			return err
		}
		if hourCount >= int64(emailHourLimit) {
			return ErrPasswordResetRateLimited
		}
	}

	requestIPHash := s.hashIP(strings.TrimSpace(clientIP))
	if requestIPHash != "" && cfg.PasswordReset.MaxRequestsPerHourPerIP > 0 {
		ipHourCount, err := s.tokenRepo.CountRecent(ctx, repository.CountPasswordResetTokensParams{
			RequestIPHash: requestIPHash,
			Since:         now.Add(-time.Hour),
		})
		if err != nil {
			return err
		}
		if ipHourCount >= int64(cfg.PasswordReset.MaxRequestsPerHourPerIP) {
			return ErrPasswordResetRateLimited
		}
	}
	return nil
}

func (s *PasswordResetService) hashToken(rawToken string) string {
	if strings.TrimSpace(rawToken) == "" {
		return ""
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(rawToken))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *PasswordResetService) hashIP(rawIP string) string {
	normalizedIP := strings.TrimSpace(rawIP)
	if normalizedIP == "" {
		return ""
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte("ip:" + normalizedIP))
	return hex.EncodeToString(mac.Sum(nil))
}

func parsePasswordResetRawToken(rawToken string) (string, string, error) {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return "", "", errors.New("password reset token is empty")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", "", errors.New("password reset token format is invalid")
	}
	tokenID := strings.TrimSpace(parts[0])
	secretPart := strings.TrimSpace(parts[1])
	if len(tokenID) != 26 || tokenID == "" || secretPart == "" {
		return "", "", errors.New("password reset token format is invalid")
	}
	return tokenID, secretPart, nil
}

func generatePasswordResetTokenSecret() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func composePasswordResetURL(appBaseURL string, rawToken string) string {
	base := strings.TrimSpace(appBaseURL)
	if base == "" {
		base = ""
	}
	base = strings.TrimRight(base, "/")
	encodedToken := url.QueryEscape(strings.TrimSpace(rawToken))
	if base == "" {
		return "/reset-password#token=" + encodedToken
	}
	return base + "/reset-password#token=" + encodedToken
}

func buildPasswordResetMailText(userName string, resetURL string, tokenTTLMinutes int) string {
	displayName := strings.TrimSpace(userName)
	if displayName == "" {
		displayName = "用户"
	}
	if tokenTTLMinutes <= 0 {
		tokenTTLMinutes = defaultEmailPasswordResetTokenTTLMinutes
	}
	return fmt.Sprintf(
		"%s，您好：\n\n我们收到了您的密码重置申请。\n请在 %d 分钟内点击以下链接完成重置：\n%s\n\n如果这不是您的操作，请忽略这封邮件。\n\nPlainDoc",
		displayName,
		tokenTTLMinutes,
		resetURL,
	)
}

func isPasswordResettableUser(user *models.User) bool {
	if user == nil {
		return false
	}
	if strings.TrimSpace(user.Email) == "" || !strings.Contains(user.Email, "@") {
		return false
	}
	if normalizeEntityStatus(user.Status) != models.EntityStatusActive {
		return false
	}
	if strings.TrimSpace(user.PasswordHash) == LDAPUserPasswordPlaceholder {
		return false
	}
	return true
}
