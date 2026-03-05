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
	defaultDocumentShareAccessTokenTTL = 30 * 24 * time.Hour
)

var (
	ErrDocumentShareAccessTokenInvalid = errors.New("document share access token invalid")
	ErrDocumentShareAccessTokenExpired = errors.New("document share access token expired")
)

// DocumentShareAccessTokenClaims 定义分享免密令牌载荷。
type DocumentShareAccessTokenClaims struct {
	ShareID        string `json:"shareId"`
	AccessVersion  int    `json:"accessVersion"`
	IssuedAtUnix   int64  `json:"iat"`
	ExpiresAtUnix  int64  `json:"exp"`
}

// IssueDocumentShareAccessTokenInput 定义签发参数。
type IssueDocumentShareAccessTokenInput struct {
	ShareID       string
	AccessVersion int
	TTL           time.Duration
}

// DocumentShareAccessTokenService 提供分享免密令牌签发与验签。
type DocumentShareAccessTokenService struct {
	signingSecret []byte
	defaultTTL    time.Duration
	nowFn         func() time.Time
}

// NewDocumentShareAccessTokenService 创建分享免密令牌服务。
func NewDocumentShareAccessTokenService(secret string, defaultTTL time.Duration) *DocumentShareAccessTokenService {
	normalizedSecret := strings.TrimSpace(secret)
	if normalizedSecret == "" {
		normalizedSecret = "plaindoc-document-share-access-secret"
	}
	if defaultTTL <= 0 {
		defaultTTL = defaultDocumentShareAccessTokenTTL
	}
	return &DocumentShareAccessTokenService{
		signingSecret: []byte(normalizedSecret),
		defaultTTL:    defaultTTL,
		nowFn:         time.Now,
	}
}

// Issue 签发分享免密令牌。
func (s *DocumentShareAccessTokenService) Issue(
	input IssueDocumentShareAccessTokenInput,
) (string, time.Time, error) {
	if s == nil || len(s.signingSecret) == 0 {
		return "", time.Time{}, errors.New("document share access token service is not initialized")
	}
	shareID := strings.TrimSpace(input.ShareID)
	if shareID == "" || input.AccessVersion <= 0 {
		return "", time.Time{}, ErrDocumentShareAccessTokenInvalid
	}
	ttl := input.TTL
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	now := s.nowFn().UTC()
	expiresAt := now.Add(ttl)
	claims := DocumentShareAccessTokenClaims{
		ShareID:       shareID,
		AccessVersion: input.AccessVersion,
		IssuedAtUnix:  now.Unix(),
		ExpiresAtUnix: expiresAt.Unix(),
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := computeDocumentShareAccessTokenSignature(s.signingSecret, payloadEncoded)
	signatureEncoded := base64.RawURLEncoding.EncodeToString(signature)
	return payloadEncoded + "." + signatureEncoded, expiresAt, nil
}

// Parse 解析并校验分享免密令牌。
func (s *DocumentShareAccessTokenService) Parse(token string) (DocumentShareAccessTokenClaims, error) {
	if s == nil || len(s.signingSecret) == 0 {
		return DocumentShareAccessTokenClaims{}, errors.New("document share access token service is not initialized")
	}
	normalizedToken := strings.TrimSpace(token)
	parts := strings.Split(normalizedToken, ".")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return DocumentShareAccessTokenClaims{}, ErrDocumentShareAccessTokenInvalid
	}

	expectedSignature := computeDocumentShareAccessTokenSignature(s.signingSecret, parts[0])
	actualSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(actualSignature, expectedSignature) {
		return DocumentShareAccessTokenClaims{}, ErrDocumentShareAccessTokenInvalid
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return DocumentShareAccessTokenClaims{}, ErrDocumentShareAccessTokenInvalid
	}
	var claims DocumentShareAccessTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return DocumentShareAccessTokenClaims{}, ErrDocumentShareAccessTokenInvalid
	}

	claims.ShareID = strings.TrimSpace(claims.ShareID)
	if claims.ShareID == "" || claims.AccessVersion <= 0 || claims.ExpiresAtUnix <= 0 {
		return DocumentShareAccessTokenClaims{}, ErrDocumentShareAccessTokenInvalid
	}
	if s.nowFn().UTC().Unix() > claims.ExpiresAtUnix {
		return DocumentShareAccessTokenClaims{}, ErrDocumentShareAccessTokenExpired
	}
	return claims, nil
}

func computeDocumentShareAccessTokenSignature(secret []byte, payloadEncoded string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payloadEncoded))
	return mac.Sum(nil)
}
