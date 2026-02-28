package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/mojocn/base64Captcha"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

type authRiskScene string

const (
	authRiskSceneLogin    authRiskScene = "login"
	authRiskSceneRegister authRiskScene = "register"

	authRiskSubjectIP           = "ip"
	authRiskSubjectIdentifier   = "identifier"
	authRiskSubjectIPIdentifier = "ip_identifier"

	AuthRiskErrorTypeCaptchaRequired = "captcha_required"
	AuthRiskErrorTypeCaptchaInvalid  = "captcha_invalid"
	AuthRiskErrorTypeTemporarilyLock = "temporarily_locked"

	authRiskHashSecretFallback = "plaindoc-auth-risk-secret"
)

// AuthRiskCheckInput 描述认证风控预检输入。
type AuthRiskCheckInput struct {
	Scene         string
	ClientIP      string
	Identifier    string
	CaptchaID     string
	CaptchaAnswer string
}

// AuthRiskRecordInput 描述认证结果回写输入。
type AuthRiskRecordInput struct {
	Scene      string
	ClientIP   string
	Identifier string
	Success    bool
}

// AuthCaptchaChallenge 供前端展示的验证码挑战信息。
type AuthCaptchaChallenge struct {
	CaptchaID           string `json:"captchaId"`
	CaptchaImageDataURL string `json:"captchaImageDataUrl"`
	Level               int    `json:"level"`
	ExpiresInSeconds    int    `json:"expiresInSeconds"`
}

// AuthRiskResult 描述风控反馈数据。
type AuthRiskResult struct {
	Challenge         *AuthCaptchaChallenge
	LockedUntil       *time.Time
	RetryAfterSeconds int
}

// AuthRiskError 是认证风控业务错误，供 handler 映射为统一响应。
type AuthRiskError struct {
	Type    string
	Message string
	Result  AuthRiskResult
}

func (e *AuthRiskError) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Message)
}

type authRiskSubject struct {
	SubjectType string
	SubjectHash string
}

// AuthRiskControlService 负责认证入口的验证码与封禁策略执行。
type AuthRiskControlService struct {
	policyService *AuthRiskPolicyService
	stateRepo     repository.AuthRiskStateRepository
	captchaRepo   repository.AuthCaptchaChallengeRepository
	hashSecret    []byte
	now           func() time.Time
}

// NewAuthRiskControlService 创建认证风控服务。
func NewAuthRiskControlService(
	policyService *AuthRiskPolicyService,
	stateRepo repository.AuthRiskStateRepository,
	captchaRepo repository.AuthCaptchaChallengeRepository,
	hashSecret string,
) *AuthRiskControlService {
	normalizedHashSecret := strings.TrimSpace(hashSecret)
	if normalizedHashSecret == "" {
		normalizedHashSecret = authRiskHashSecretFallback
	}

	return &AuthRiskControlService{
		policyService: policyService,
		stateRepo:     stateRepo,
		captchaRepo:   captchaRepo,
		hashSecret:    []byte(normalizedHashSecret),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// Check 在登录/注册前执行风控预检。
func (s *AuthRiskControlService) Check(ctx context.Context, input AuthRiskCheckInput) error {
	policy, now, scene, subjects, states, err := s.prepareSceneStates(ctx, input.Scene, input.ClientIP, input.Identifier)
	if err != nil {
		return err
	}
	if !policy.Enabled {
		return nil
	}

	if lockUntil, retryAfter := detectStateLock(states, now); lockUntil != nil {
		return &AuthRiskError{
			Type:    AuthRiskErrorTypeTemporarilyLock,
			Message: "操作过于频繁，请稍后再试",
			Result: AuthRiskResult{
				LockedUntil:       lockUntil,
				RetryAfterSeconds: retryAfter,
			},
		}
	}

	requiredLevel, dominantState := resolveRequiredCaptchaLevel(scene, states, policy)
	if requiredLevel <= 0 || dominantState == nil {
		return nil
	}

	captchaID := strings.TrimSpace(input.CaptchaID)
	captchaAnswer := strings.TrimSpace(input.CaptchaAnswer)
	if captchaID == "" || captchaAnswer == "" {
		challenge, challengeErr := s.issueChallenge(ctx, scene, dominantState.SubjectHash, input.ClientIP, requiredLevel, policy)
		if challengeErr != nil {
			return challengeErr
		}
		return &AuthRiskError{
			Type:    AuthRiskErrorTypeCaptchaRequired,
			Message: "需要验证码校验",
			Result: AuthRiskResult{
				Challenge: challenge,
			},
		}
	}

	valid, validateErr := s.validateChallenge(
		ctx,
		scene,
		subjects,
		input.ClientIP,
		captchaID,
		captchaAnswer,
		now,
	)
	if validateErr != nil {
		return validateErr
	}
	if valid {
		return nil
	}

	if err := s.recordCaptchaFailure(ctx, scene, states, now, policy); err != nil {
		return err
	}

	if lockUntil, retryAfter := detectStateLock(states, now); lockUntil != nil {
		return &AuthRiskError{
			Type:    AuthRiskErrorTypeTemporarilyLock,
			Message: "操作过于频繁，请稍后再试",
			Result: AuthRiskResult{
				LockedUntil:       lockUntil,
				RetryAfterSeconds: retryAfter,
			},
		}
	}

	nextLevel, dominantAfterFailure := resolveRequiredCaptchaLevel(scene, states, policy)
	if nextLevel <= 0 {
		nextLevel = requiredLevel
	}
	dominantSubjectHash := dominantState.SubjectHash
	if dominantAfterFailure != nil && strings.TrimSpace(dominantAfterFailure.SubjectHash) != "" {
		dominantSubjectHash = dominantAfterFailure.SubjectHash
	}
	challenge, challengeErr := s.issueChallenge(ctx, scene, dominantSubjectHash, input.ClientIP, nextLevel, policy)
	if challengeErr != nil {
		return challengeErr
	}
	return &AuthRiskError{
		Type:    AuthRiskErrorTypeCaptchaInvalid,
		Message: "验证码错误或已过期",
		Result: AuthRiskResult{
			Challenge: challenge,
		},
	}
}

// RecordResult 在登录/注册结束后回写风控计数。
func (s *AuthRiskControlService) RecordResult(ctx context.Context, input AuthRiskRecordInput) error {
	policy, now, scene, _, states, err := s.prepareSceneStates(ctx, input.Scene, input.ClientIP, input.Identifier)
	if err != nil {
		return err
	}
	if !policy.Enabled {
		return nil
	}

	for _, state := range states {
		if state == nil {
			continue
		}
		switch scene {
		case authRiskSceneRegister:
			state.AttemptCount += 1
			if !input.Success {
				state.FailedCount += 1
			}
		case authRiskSceneLogin:
			if input.Success {
				resetAuthRiskStateCounter(state, now)
			} else {
				state.AttemptCount += 1
				state.FailedCount += 1
			}
		default:
			continue
		}

		if shouldLockState(scene, state, policy) {
			lockUntil := now.Add(time.Duration(policy.LockSeconds) * time.Second)
			state.LockUntil = &lockUntil
		}
		state.UpdatedAt = now
		if err := s.stateRepo.Update(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

func (s *AuthRiskControlService) prepareSceneStates(
	ctx context.Context,
	rawScene string,
	clientIP string,
	identifier string,
) (AuthRiskPolicy, time.Time, authRiskScene, []authRiskSubject, []*models.AuthRiskState, error) {
	policy := defaultAuthRiskPolicy()
	if s != nil && s.policyService != nil {
		policy = s.policyService.Resolve(ctx)
	}
	now := time.Now().UTC()
	if s != nil && s.now != nil {
		now = s.now().UTC()
	}

	scene := normalizeAuthRiskScene(rawScene)
	if scene == "" {
		return policy, now, "", nil, nil, fmt.Errorf("unsupported auth risk scene %q", strings.TrimSpace(rawScene))
	}

	if s == nil || s.stateRepo == nil || s.captchaRepo == nil {
		return policy, now, scene, nil, nil, nil
	}

	subjects := s.resolveSubjects(scene, clientIP, identifier)
	states, err := s.loadOrCreateStates(ctx, scene, subjects, now, policy)
	if err != nil {
		return policy, now, scene, nil, nil, err
	}
	return policy, now, scene, subjects, states, nil
}

func (s *AuthRiskControlService) loadOrCreateStates(
	ctx context.Context,
	scene authRiskScene,
	subjects []authRiskSubject,
	now time.Time,
	policy AuthRiskPolicy,
) ([]*models.AuthRiskState, error) {
	states := make([]*models.AuthRiskState, 0, len(subjects))
	for _, subject := range subjects {
		if strings.TrimSpace(subject.SubjectType) == "" || strings.TrimSpace(subject.SubjectHash) == "" {
			continue
		}

		state, err := s.stateRepo.GetByKey(ctx, string(scene), subject.SubjectType, subject.SubjectHash)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
			created, createErr := s.createRiskState(ctx, scene, subject, now)
			if createErr != nil {
				return nil, createErr
			}
			state = created
		}

		if normalizeAuthRiskStateWindow(state, now, policy.WindowSeconds) {
			state.UpdatedAt = now
			if err := s.stateRepo.Update(ctx, state); err != nil {
				return nil, err
			}
		}

		states = append(states, state)
	}
	return states, nil
}

func (s *AuthRiskControlService) createRiskState(
	ctx context.Context,
	scene authRiskScene,
	subject authRiskSubject,
	now time.Time,
) (*models.AuthRiskState, error) {
	state := &models.AuthRiskState{
		Scene:            string(scene),
		SubjectType:      subject.SubjectType,
		SubjectHash:      subject.SubjectHash,
		WindowStartedAt:  now,
		AttemptCount:     0,
		FailedCount:      0,
		CaptchaFailCount: 0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.stateRepo.Create(ctx, state); err != nil {
		if !isAuthRiskUniqueConstraintError(err) {
			return nil, err
		}
		loaded, loadErr := s.stateRepo.GetByKey(ctx, string(scene), subject.SubjectType, subject.SubjectHash)
		if loadErr != nil {
			return nil, loadErr
		}
		return loaded, nil
	}
	return state, nil
}

func (s *AuthRiskControlService) resolveSubjects(
	scene authRiskScene,
	clientIP string,
	identifier string,
) []authRiskSubject {
	normalizedIP := strings.TrimSpace(clientIP)
	normalizedIdentifier := normalizeAuthRiskIdentifier(identifier)
	subjects := make([]authRiskSubject, 0, 3)
	exists := map[string]struct{}{}

	appendSubject := func(subjectType string, rawValue string) {
		normalizedValue := strings.TrimSpace(rawValue)
		if normalizedValue == "" {
			return
		}
		subjectHash := s.hashSubjectValue(string(scene), subjectType, normalizedValue)
		if subjectHash == "" {
			return
		}
		key := subjectType + ":" + subjectHash
		if _, duplicated := exists[key]; duplicated {
			return
		}
		exists[key] = struct{}{}
		subjects = append(subjects, authRiskSubject{
			SubjectType: subjectType,
			SubjectHash: subjectHash,
		})
	}

	switch scene {
	case authRiskSceneLogin:
		// 登录场景保留 IP 维度，用于拦截跨账号撞库。
		appendSubject(authRiskSubjectIP, normalizedIP)
		appendSubject(authRiskSubjectIdentifier, normalizedIdentifier)
		if normalizedIP != "" && normalizedIdentifier != "" {
			appendSubject(authRiskSubjectIPIdentifier, normalizedIP+"|"+normalizedIdentifier)
		}
	case authRiskSceneRegister:
		// 注册场景优先账号维度，避免不同新账号在同一出口 IP 下互相污染。
		appendSubject(authRiskSubjectIdentifier, normalizedIdentifier)
		if normalizedIP != "" && normalizedIdentifier != "" {
			appendSubject(authRiskSubjectIPIdentifier, normalizedIP+"|"+normalizedIdentifier)
		}
		if len(subjects) == 0 {
			appendSubject(authRiskSubjectIP, normalizedIP)
		}
	default:
		appendSubject(authRiskSubjectIP, normalizedIP)
		appendSubject(authRiskSubjectIdentifier, normalizedIdentifier)
		if normalizedIP != "" && normalizedIdentifier != "" {
			appendSubject(authRiskSubjectIPIdentifier, normalizedIP+"|"+normalizedIdentifier)
		}
	}

	return subjects
}

func (s *AuthRiskControlService) validateChallenge(
	ctx context.Context,
	scene authRiskScene,
	subjects []authRiskSubject,
	clientIP string,
	captchaID string,
	captchaAnswer string,
	now time.Time,
) (bool, error) {
	challenge, err := s.captchaRepo.GetByCaptchaID(ctx, captchaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if challenge == nil {
		return false, nil
	}

	if !strings.EqualFold(strings.TrimSpace(challenge.Scene), string(scene)) {
		_ = s.bumpChallengeFailedCount(ctx, challenge, now)
		return false, nil
	}
	if !isSubjectHashAllowed(challenge.SubjectHash, subjects) {
		_ = s.bumpChallengeFailedCount(ctx, challenge, now)
		return false, nil
	}
	if !isIssuedIPMatch(s.hashSubjectValue(string(scene), authRiskSubjectIP, strings.TrimSpace(clientIP)), challenge.IssuedIPHash) {
		_ = s.bumpChallengeFailedCount(ctx, challenge, now)
		return false, nil
	}
	if challenge.ConsumedAt != nil {
		return false, nil
	}
	if !challenge.ExpiresAt.After(now) {
		return false, nil
	}

	expectedHash := s.hashCaptchaAnswer(captchaAnswer, challenge.AnswerSalt)
	if subtle.ConstantTimeCompare([]byte(expectedHash), []byte(challenge.AnswerHash)) != 1 {
		if err := s.bumpChallengeFailedCount(ctx, challenge, now); err != nil {
			return false, err
		}
		return false, nil
	}

	challenge.ConsumedAt = &now
	challenge.UpdatedAt = now
	if err := s.captchaRepo.Update(ctx, challenge); err != nil {
		return false, err
	}
	return true, nil
}

func (s *AuthRiskControlService) bumpChallengeFailedCount(
	ctx context.Context,
	challenge *models.AuthCaptchaChallenge,
	now time.Time,
) error {
	if challenge == nil {
		return nil
	}
	challenge.FailedVerifyCount += 1
	challenge.UpdatedAt = now
	return s.captchaRepo.Update(ctx, challenge)
}

func (s *AuthRiskControlService) issueChallenge(
	ctx context.Context,
	scene authRiskScene,
	subjectHash string,
	clientIP string,
	level int,
	policy AuthRiskPolicy,
) (*AuthCaptchaChallenge, error) {
	if level <= 0 {
		level = 1
	}

	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}

	captchaCode, captchaImageDataURL, captchaLength, err := generateDigitCaptchaChallenge(level)
	if err != nil {
		return nil, err
	}
	captchaID := strings.ToLower(ulid.Make().String())
	answerSalt, err := randomHexString(12)
	if err != nil {
		return nil, err
	}

	challenge := &models.AuthCaptchaChallenge{
		CaptchaID:   captchaID,
		Scene:       string(scene),
		SubjectHash: strings.TrimSpace(subjectHash),
		// 约定：Level 存验证码字符数量（如 4/5/6），而非风险等级序号（1/2/3）。
		Level:             captchaLength,
		AnswerHash:        s.hashCaptchaAnswer(captchaCode, answerSalt),
		AnswerSalt:        answerSalt,
		IssuedIPHash:      s.hashSubjectValue(string(scene), authRiskSubjectIP, strings.TrimSpace(clientIP)),
		ExpiresAt:         now.Add(time.Duration(policy.CaptchaTTLSeconds) * time.Second),
		FailedVerifyCount: 0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.captchaRepo.Create(ctx, challenge); err != nil {
		return nil, err
	}

	return &AuthCaptchaChallenge{
		CaptchaID:           captchaID,
		CaptchaImageDataURL: captchaImageDataURL,
		Level:               captchaLength,
		ExpiresInSeconds:    policy.CaptchaTTLSeconds,
	}, nil
}

func (s *AuthRiskControlService) recordCaptchaFailure(
	ctx context.Context,
	scene authRiskScene,
	states []*models.AuthRiskState,
	now time.Time,
	policy AuthRiskPolicy,
) error {
	for _, state := range states {
		if state == nil {
			continue
		}
		state.AttemptCount += 1
		state.CaptchaFailCount += 1
		if shouldLockState(scene, state, policy) {
			lockUntil := now.Add(time.Duration(policy.LockSeconds) * time.Second)
			state.LockUntil = &lockUntil
		}
		state.UpdatedAt = now
		if err := s.stateRepo.Update(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

func (s *AuthRiskControlService) hashSubjectValue(scene string, subjectType string, value string) string {
	normalizedScene := strings.TrimSpace(strings.ToLower(scene))
	normalizedType := strings.TrimSpace(strings.ToLower(subjectType))
	normalizedValue := strings.TrimSpace(strings.ToLower(value))
	if normalizedScene == "" || normalizedType == "" || normalizedValue == "" {
		return ""
	}

	mac := hmac.New(sha256.New, s.hashSecret)
	_, _ = mac.Write([]byte(normalizedScene))
	_, _ = mac.Write([]byte{':'})
	_, _ = mac.Write([]byte(normalizedType))
	_, _ = mac.Write([]byte{':'})
	_, _ = mac.Write([]byte(normalizedValue))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *AuthRiskControlService) hashCaptchaAnswer(answer string, salt string) string {
	normalizedAnswer := strings.ToUpper(strings.TrimSpace(answer))
	mac := hmac.New(sha256.New, s.hashSecret)
	_, _ = mac.Write([]byte(strings.TrimSpace(salt)))
	_, _ = mac.Write([]byte{':'})
	_, _ = mac.Write([]byte(normalizedAnswer))
	return hex.EncodeToString(mac.Sum(nil))
}

func normalizeAuthRiskScene(raw string) authRiskScene {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(authRiskSceneLogin):
		return authRiskSceneLogin
	case string(authRiskSceneRegister):
		return authRiskSceneRegister
	default:
		return ""
	}
}

func normalizeAuthRiskIdentifier(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func resetAuthRiskStateCounter(state *models.AuthRiskState, now time.Time) {
	if state == nil {
		return
	}
	state.AttemptCount = 0
	state.FailedCount = 0
	state.CaptchaFailCount = 0
	state.LockUntil = nil
	state.WindowStartedAt = now
}

func normalizeAuthRiskStateWindow(state *models.AuthRiskState, now time.Time, windowSeconds int) bool {
	if state == nil {
		return false
	}

	changed := false
	if state.WindowStartedAt.IsZero() {
		state.WindowStartedAt = now
		changed = true
	}

	if state.LockUntil != nil && !state.LockUntil.After(now) {
		resetAuthRiskStateCounter(state, now)
		changed = true
	}
	if state.LockUntil != nil && state.LockUntil.After(now) {
		return changed
	}

	windowDuration := time.Duration(windowSeconds) * time.Second
	if now.Sub(state.WindowStartedAt) >= windowDuration {
		resetAuthRiskStateCounter(state, now)
		changed = true
	}
	return changed
}

func resolveRequiredCaptchaLevel(
	scene authRiskScene,
	states []*models.AuthRiskState,
	policy AuthRiskPolicy,
) (int, *models.AuthRiskState) {
	level := 0
	var dominant *models.AuthRiskState
	for _, state := range states {
		if state == nil {
			continue
		}
		current := requiredCaptchaLevel(scene, state, policy)
		if current > level {
			level = current
			dominant = state
		}
	}
	return level, dominant
}

func requiredCaptchaLevel(scene authRiskScene, state *models.AuthRiskState, policy AuthRiskPolicy) int {
	if state == nil {
		return 0
	}
	switch scene {
	case authRiskSceneLogin:
		score := state.FailedCount + state.CaptchaFailCount
		return scoreToCaptchaLevel(score, policy.LoginThresholds)
	case authRiskSceneRegister:
		score := state.AttemptCount + state.CaptchaFailCount
		return scoreToCaptchaLevel(score, policy.RegisterThresholds)
	default:
		return 0
	}
}

func shouldLockState(scene authRiskScene, state *models.AuthRiskState, policy AuthRiskPolicy) bool {
	if state == nil {
		return false
	}
	switch scene {
	case authRiskSceneLogin:
		score := state.FailedCount + state.CaptchaFailCount
		return score >= policy.LoginThresholds.Lock
	case authRiskSceneRegister:
		score := state.AttemptCount + state.CaptchaFailCount
		return score >= policy.RegisterThresholds.Lock
	default:
		return false
	}
}

func scoreToCaptchaLevel(score int, thresholds AuthRiskThresholds) int {
	switch {
	case score >= thresholds.CaptchaLevel3:
		return 3
	case score >= thresholds.CaptchaLevel2:
		return 2
	case score >= thresholds.CaptchaLevel1:
		return 1
	default:
		return 0
	}
}

func detectStateLock(states []*models.AuthRiskState, now time.Time) (*time.Time, int) {
	var lockUntil *time.Time
	for _, state := range states {
		if state == nil || state.LockUntil == nil {
			continue
		}
		if !state.LockUntil.After(now) {
			continue
		}
		if lockUntil == nil || state.LockUntil.After(*lockUntil) {
			value := state.LockUntil.UTC()
			lockUntil = &value
		}
	}
	if lockUntil == nil {
		return nil, 0
	}
	retryAfter := int(lockUntil.Sub(now).Seconds())
	if retryAfter < 0 {
		retryAfter = 0
	}
	return lockUntil, retryAfter
}

func isSubjectHashAllowed(target string, subjects []authRiskSubject) bool {
	normalizedTarget := strings.TrimSpace(target)
	if normalizedTarget == "" {
		return false
	}
	for _, subject := range subjects {
		if strings.TrimSpace(subject.SubjectHash) == normalizedTarget {
			return true
		}
	}
	return false
}

func isIssuedIPMatch(currentHash string, issuedHash string) bool {
	normalizedIssuedHash := strings.TrimSpace(issuedHash)
	if normalizedIssuedHash == "" {
		return true
	}
	return strings.TrimSpace(currentHash) == normalizedIssuedHash
}

func isAuthRiskUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate entry") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "unique failed")
}

func randomHexString(size int) (string, error) {
	if size <= 0 {
		return "", fmt.Errorf("size must be greater than 0")
	}
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func generateDigitCaptchaChallenge(riskLevel int) (answer string, imageDataURL string, length int, err error) {
	length, maxSkew, dotCount := resolveDigitCaptchaConfig(riskLevel)
	width := 42*length + 20
	height := 60

	driver := base64Captcha.NewDriverDigit(height, width, length, maxSkew, dotCount)
	_, question, captchaAnswer := driver.GenerateIdQuestionAnswer()
	item, drawErr := driver.DrawCaptcha(question)
	if drawErr != nil {
		return "", "", 0, drawErr
	}

	encoded := strings.TrimSpace(item.EncodeB64string())
	if encoded == "" {
		return "", "", 0, fmt.Errorf("captcha image is empty")
	}

	if !strings.HasPrefix(encoded, "data:image/") {
		encoded = "data:image/png;base64," + encoded
	}
	return captchaAnswer, encoded, length, nil
}

func resolveDigitCaptchaConfig(riskLevel int) (length int, maxSkew float64, dotCount int) {
	switch riskLevel {
	case 1:
		return 6, 0.7, 120
	case 2:
		return 8, 0.8, 140
	default:
		return 10, 0.9, 160
	}
}
