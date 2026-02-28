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
	defaultDocumentAttachmentDownloadLinkTTL = 24 * time.Hour
)

var (
	ErrDocumentAttachmentDownloadTokenInvalid = errors.New("document attachment download token invalid")
	ErrDocumentAttachmentDownloadTokenExpired = errors.New("document attachment download token expired")
)

// DocumentAttachmentLinkPurpose 表示访问链接用途。
type DocumentAttachmentLinkPurpose string

const (
	DocumentAttachmentLinkPurposeDownload DocumentAttachmentLinkPurpose = "download"
	DocumentAttachmentLinkPurposePreview  DocumentAttachmentLinkPurpose = "preview"
)

// DocumentAttachmentDownloadTokenClaims 定义附件访问链接载荷。
type DocumentAttachmentDownloadTokenClaims struct {
	AttachmentID  string                        `json:"attachmentId"`
	DocumentID    string                        `json:"documentId"`
	Purpose       DocumentAttachmentLinkPurpose `json:"purpose"`
	IssuedAtUnix  int64                         `json:"iat"`
	ExpiresAtUnix int64                         `json:"exp"`
}

// IssueDocumentAttachmentDownloadTokenInput 定义签发参数。
type IssueDocumentAttachmentDownloadTokenInput struct {
	AttachmentID string
	DocumentID   string
	Purpose      DocumentAttachmentLinkPurpose
	TTL          time.Duration
}

// DocumentAttachmentDownloadTokenService 提供附件访问链接签发与验签。
type DocumentAttachmentDownloadTokenService struct {
	signingSecret []byte
	defaultTTL    time.Duration
	nowFn         func() time.Time
}

// NewDocumentAttachmentDownloadTokenService 创建附件访问链接服务。
func NewDocumentAttachmentDownloadTokenService(
	secret string,
	defaultTTL time.Duration,
) *DocumentAttachmentDownloadTokenService {
	normalizedSecret := strings.TrimSpace(secret)
	if normalizedSecret == "" {
		normalizedSecret = "plaindoc-document-attachment-link-secret"
	}
	if defaultTTL <= 0 {
		defaultTTL = defaultDocumentAttachmentDownloadLinkTTL
	}
	return &DocumentAttachmentDownloadTokenService{
		signingSecret: []byte(normalizedSecret),
		defaultTTL:    defaultTTL,
		nowFn:         time.Now,
	}
}

// Issue 签发附件访问链接 token。
func (s *DocumentAttachmentDownloadTokenService) Issue(
	input IssueDocumentAttachmentDownloadTokenInput,
) (string, time.Time, error) {
	if s == nil || len(s.signingSecret) == 0 {
		return "", time.Time{}, errors.New("document attachment token service is not initialized")
	}

	attachmentID := strings.TrimSpace(input.AttachmentID)
	documentID := strings.TrimSpace(input.DocumentID)
	purpose := normalizeDocumentAttachmentLinkPurpose(input.Purpose)
	if attachmentID == "" || documentID == "" || purpose == "" {
		return "", time.Time{}, ErrDocumentAttachmentDownloadTokenInvalid
	}

	ttl := input.TTL
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	now := s.nowFn().UTC()
	expiresAt := now.Add(ttl)
	claims := DocumentAttachmentDownloadTokenClaims{
		AttachmentID:  attachmentID,
		DocumentID:    documentID,
		Purpose:       purpose,
		IssuedAtUnix:  now.Unix(),
		ExpiresAtUnix: expiresAt.Unix(),
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signatureEncoded := base64.RawURLEncoding.EncodeToString(
		computeDocumentAttachmentDownloadTokenSignature(s.signingSecret, payloadEncoded),
	)
	return payloadEncoded + "." + signatureEncoded, expiresAt, nil
}

// Parse 解析并校验附件访问链接 token。
func (s *DocumentAttachmentDownloadTokenService) Parse(
	token string,
) (DocumentAttachmentDownloadTokenClaims, error) {
	if s == nil || len(s.signingSecret) == 0 {
		return DocumentAttachmentDownloadTokenClaims{}, errors.New("document attachment token service is not initialized")
	}

	normalizedToken := strings.TrimSpace(token)
	parts := strings.Split(normalizedToken, ".")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return DocumentAttachmentDownloadTokenClaims{}, ErrDocumentAttachmentDownloadTokenInvalid
	}

	expectedSignature := computeDocumentAttachmentDownloadTokenSignature(s.signingSecret, parts[0])
	actualSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(actualSignature, expectedSignature) {
		return DocumentAttachmentDownloadTokenClaims{}, ErrDocumentAttachmentDownloadTokenInvalid
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return DocumentAttachmentDownloadTokenClaims{}, ErrDocumentAttachmentDownloadTokenInvalid
	}
	var claims DocumentAttachmentDownloadTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return DocumentAttachmentDownloadTokenClaims{}, ErrDocumentAttachmentDownloadTokenInvalid
	}

	claims.AttachmentID = strings.TrimSpace(claims.AttachmentID)
	claims.DocumentID = strings.TrimSpace(claims.DocumentID)
	claims.Purpose = normalizeDocumentAttachmentLinkPurpose(claims.Purpose)
	if claims.AttachmentID == "" || claims.DocumentID == "" || claims.Purpose == "" || claims.ExpiresAtUnix <= 0 {
		return DocumentAttachmentDownloadTokenClaims{}, ErrDocumentAttachmentDownloadTokenInvalid
	}

	nowUnix := s.nowFn().UTC().Unix()
	if nowUnix > claims.ExpiresAtUnix {
		return DocumentAttachmentDownloadTokenClaims{}, ErrDocumentAttachmentDownloadTokenExpired
	}
	return claims, nil
}

func computeDocumentAttachmentDownloadTokenSignature(secret []byte, payloadEncoded string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payloadEncoded))
	return mac.Sum(nil)
}

func normalizeDocumentAttachmentLinkPurpose(
	purpose DocumentAttachmentLinkPurpose,
) DocumentAttachmentLinkPurpose {
	switch DocumentAttachmentLinkPurpose(strings.ToLower(strings.TrimSpace(string(purpose)))) {
	case DocumentAttachmentLinkPurposeDownload:
		return DocumentAttachmentLinkPurposeDownload
	case DocumentAttachmentLinkPurposePreview:
		return DocumentAttachmentLinkPurposePreview
	default:
		return ""
	}
}
