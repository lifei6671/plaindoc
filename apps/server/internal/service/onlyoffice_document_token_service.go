package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	defaultOnlyOfficeDocumentTokenTTL = 24 * time.Hour
)

var (
	ErrOnlyOfficeDocumentTokenInvalid = errors.New("onlyoffice document token invalid")
	ErrOnlyOfficeDocumentTokenExpired = errors.New("onlyoffice document token expired")
)

// OnlyOfficeDocumentTokenPurpose 表示 ONLYOFFICE 文档令牌用途。
type OnlyOfficeDocumentTokenPurpose string

const (
	OnlyOfficeDocumentTokenPurposeSource   OnlyOfficeDocumentTokenPurpose = "source"
	OnlyOfficeDocumentTokenPurposeCallback OnlyOfficeDocumentTokenPurpose = "callback"
)

// OnlyOfficeDocumentTokenClaims 定义 ONLYOFFICE 会话令牌载荷。
type OnlyOfficeDocumentTokenClaims struct {
	DocumentID     string                         `json:"documentId"`
	ContentVersion int                            `json:"contentVersion"`
	ActorUserID    string                         `json:"actorUserId,omitempty"`
	Purpose        OnlyOfficeDocumentTokenPurpose `json:"purpose"`
	IssuedAtUnix   int64                          `json:"iat"`
	ExpiresAtUnix  int64                          `json:"exp"`
}

// IssueOnlyOfficeDocumentTokenInput 定义 ONLYOFFICE 会话令牌签发参数。
type IssueOnlyOfficeDocumentTokenInput struct {
	DocumentID     string
	ContentVersion int
	ActorUserID    string
	Purpose        OnlyOfficeDocumentTokenPurpose
	TTL            time.Duration
}

// OnlyOfficeDocumentTokenService 提供 ONLYOFFICE 拉源/回调签名令牌能力。
type OnlyOfficeDocumentTokenService struct {
	signingSecret []byte
	defaultTTL    time.Duration
	nowFn         func() time.Time
}

// NewOnlyOfficeDocumentTokenService 创建 ONLYOFFICE 文档令牌服务。
func NewOnlyOfficeDocumentTokenService(
	secret string,
	defaultTTL time.Duration,
) *OnlyOfficeDocumentTokenService {
	normalizedSecret := strings.TrimSpace(secret)
	if normalizedSecret == "" {
		normalizedSecret = "plaindoc-onlyoffice-document-link-secret"
	}
	if defaultTTL <= 0 {
		defaultTTL = defaultOnlyOfficeDocumentTokenTTL
	}
	return &OnlyOfficeDocumentTokenService{
		signingSecret: []byte(normalizedSecret),
		defaultTTL:    defaultTTL,
		nowFn:         time.Now,
	}
}

// Issue 签发 ONLYOFFICE 文档令牌。
func (s *OnlyOfficeDocumentTokenService) Issue(
	input IssueOnlyOfficeDocumentTokenInput,
) (string, time.Time, error) {
	if s == nil || len(s.signingSecret) == 0 {
		return "", time.Time{}, errors.New("onlyoffice document token service is not initialized")
	}

	documentID := strings.TrimSpace(input.DocumentID)
	actorUserID := strings.TrimSpace(input.ActorUserID)
	purpose := normalizeOnlyOfficeDocumentTokenPurpose(input.Purpose)
	if documentID == "" || input.ContentVersion <= 0 || purpose == "" {
		return "", time.Time{}, ErrOnlyOfficeDocumentTokenInvalid
	}

	ttl := input.TTL
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	now := s.nowFn().UTC()
	expiresAt := now.Add(ttl)
	claims := OnlyOfficeDocumentTokenClaims{
		DocumentID:     documentID,
		ContentVersion: input.ContentVersion,
		ActorUserID:    actorUserID,
		Purpose:        purpose,
		IssuedAtUnix:   now.Unix(),
		ExpiresAtUnix:  expiresAt.Unix(),
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signatureEncoded := base64.RawURLEncoding.EncodeToString(
		computeOnlyOfficeDocumentTokenSignature(s.signingSecret, payloadEncoded),
	)
	return payloadEncoded + "." + signatureEncoded, expiresAt, nil
}

// Parse 解析并校验 ONLYOFFICE 文档令牌。
func (s *OnlyOfficeDocumentTokenService) Parse(
	token string,
) (OnlyOfficeDocumentTokenClaims, error) {
	if s == nil || len(s.signingSecret) == 0 {
		return OnlyOfficeDocumentTokenClaims{}, errors.New("onlyoffice document token service is not initialized")
	}

	normalizedToken := strings.TrimSpace(token)
	parts := strings.Split(normalizedToken, ".")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return OnlyOfficeDocumentTokenClaims{}, ErrOnlyOfficeDocumentTokenInvalid
	}

	expectedSignature := computeOnlyOfficeDocumentTokenSignature(s.signingSecret, parts[0])
	actualSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(actualSignature, expectedSignature) {
		return OnlyOfficeDocumentTokenClaims{}, ErrOnlyOfficeDocumentTokenInvalid
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return OnlyOfficeDocumentTokenClaims{}, ErrOnlyOfficeDocumentTokenInvalid
	}
	var claims OnlyOfficeDocumentTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return OnlyOfficeDocumentTokenClaims{}, ErrOnlyOfficeDocumentTokenInvalid
	}

	claims.DocumentID = strings.TrimSpace(claims.DocumentID)
	claims.ActorUserID = strings.TrimSpace(claims.ActorUserID)
	claims.Purpose = normalizeOnlyOfficeDocumentTokenPurpose(claims.Purpose)
	if claims.DocumentID == "" || claims.ContentVersion <= 0 || claims.Purpose == "" || claims.ExpiresAtUnix <= 0 {
		return OnlyOfficeDocumentTokenClaims{}, ErrOnlyOfficeDocumentTokenInvalid
	}

	nowUnix := s.nowFn().UTC().Unix()
	if nowUnix > claims.ExpiresAtUnix {
		return OnlyOfficeDocumentTokenClaims{}, ErrOnlyOfficeDocumentTokenExpired
	}
	return claims, nil
}

func computeOnlyOfficeDocumentTokenSignature(secret []byte, payloadEncoded string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payloadEncoded))
	return mac.Sum(nil)
}

func normalizeOnlyOfficeDocumentTokenPurpose(
	purpose OnlyOfficeDocumentTokenPurpose,
) OnlyOfficeDocumentTokenPurpose {
	switch OnlyOfficeDocumentTokenPurpose(strings.ToLower(strings.TrimSpace(string(purpose)))) {
	case OnlyOfficeDocumentTokenPurposeSource:
		return OnlyOfficeDocumentTokenPurposeSource
	case OnlyOfficeDocumentTokenPurposeCallback:
		return OnlyOfficeDocumentTokenPurposeCallback
	default:
		return ""
	}
}
