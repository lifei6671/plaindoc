package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lifei6671/plaindoc/apps/server/internal/config"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	authTokenTypeAccess  = "access"
	authTokenTypeRefresh = "refresh"
)

var (
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrUserBanned          = errors.New("user banned")
	ErrUserDeleted         = errors.New("user deleted")
)

type authTokenClaims struct {
	TokenType string `json:"tokenType"`
	jwt.RegisteredClaims
}

type parsedToken struct {
	UserID    string
	SessionID string
}

type AuthUser struct {
	ID    string
	Email string
	Name  string
}

type AuthSession struct {
	User         AuthUser
	Token        string
	RefreshToken string
}

// AuthService 负责认证业务流程编排，屏蔽 handler 对底层存储细节的直接依赖。
type AuthService struct {
	userRepo        repository.UserRepository
	userSessionRepo repository.UserSessionRepository
	jwtSecret       []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// NewAuthService 创建认证服务。
func NewAuthService(
	userRepo repository.UserRepository,
	userSessionRepo repository.UserSessionRepository,
	jwtConfig config.JWTConfig,
) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		userSessionRepo: userSessionRepo,
		jwtSecret:       []byte(jwtConfig.Secret),
		accessTokenTTL:  jwtConfig.AccessTokenTTL,
		refreshTokenTTL: jwtConfig.RefreshTokenTTL,
	}
}

func (s *AuthService) Register(ctx context.Context, email string, password string, name string) (AuthSession, error) {
	if s == nil || s.userRepo == nil || s.userSessionRepo == nil {
		return AuthSession{}, errors.New("auth service dependencies are nil")
	}

	normalizedEmail := normalizeEmail(email)

	_, err := s.userRepo.GetByEmail(ctx, normalizedEmail)
	if err == nil {
		return AuthSession{}, ErrEmailAlreadyExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return AuthSession{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthSession{}, err
	}

	user := &models.User{
		UserID:       strings.ToLower(ulid.Make().String()),
		Email:        normalizedEmail,
		PasswordHash: string(passwordHash),
		Name:         strings.TrimSpace(name),
		Status:       models.EntityStatusActive,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		if isUniqueConstraintError(err) {
			return AuthSession{}, ErrEmailAlreadyExists
		}
		return AuthSession{}, err
	}

	accessToken, refreshToken, err := s.issueSessionTokens(ctx, user.UserID)
	if err != nil {
		return AuthSession{}, err
	}

	return AuthSession{
		User: AuthUser{
			ID:    user.UserID,
			Email: user.Email,
			Name:  user.Name,
		},
		Token:        accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, email string, password string) (AuthSession, error) {
	if s == nil || s.userRepo == nil || s.userSessionRepo == nil {
		return AuthSession{}, errors.New("auth service dependencies are nil")
	}

	normalizedEmail := normalizeEmail(email)
	user, err := s.userRepo.GetByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AuthSession{}, ErrInvalidCredentials
		}
		return AuthSession{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return AuthSession{}, ErrInvalidCredentials
	}
	if err := ensureAuthUserActive(user); err != nil {
		return AuthSession{}, err
	}

	accessToken, refreshToken, err := s.issueSessionTokens(ctx, user.UserID)
	if err != nil {
		return AuthSession{}, err
	}

	return AuthSession{
		User: AuthUser{
			ID:    user.UserID,
			Email: user.Email,
			Name:  user.Name,
		},
		Token:        accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (AuthSession, error) {
	if s == nil || s.userRepo == nil || s.userSessionRepo == nil {
		return AuthSession{}, errors.New("auth service dependencies are nil")
	}

	token, err := s.parseToken(refreshToken, authTokenTypeRefresh)
	if err != nil {
		return AuthSession{}, ErrInvalidRefreshToken
	}

	user, err := s.userRepo.GetByUserID(ctx, token.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AuthSession{}, ErrUnauthorized
		}
		return AuthSession{}, err
	}
	if err := ensureAuthUserActive(user); err != nil {
		if errors.Is(err, ErrUserBanned) || errors.Is(err, ErrUserDeleted) {
			return AuthSession{}, ErrUnauthorized
		}
		return AuthSession{}, err
	}

	accessToken, nextRefreshToken, err := s.rotateSessionTokens(ctx, token, refreshToken)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidSession) {
			return AuthSession{}, ErrInvalidRefreshToken
		}
		return AuthSession{}, err
	}

	return AuthSession{
		User: AuthUser{
			ID:    user.UserID,
			Email: user.Email,
			Name:  user.Name,
		},
		Token:        accessToken,
		RefreshToken: nextRefreshToken,
	}, nil
}

func (s *AuthService) Me(ctx context.Context, accessToken string) (AuthSession, error) {
	if s == nil || s.userRepo == nil {
		return AuthSession{}, errors.New("auth service dependencies are nil")
	}

	token, err := s.parseToken(accessToken, authTokenTypeAccess)
	if err != nil {
		return AuthSession{}, ErrUnauthorized
	}

	user, err := s.userRepo.GetByUserID(ctx, token.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AuthSession{}, ErrUnauthorized
		}
		return AuthSession{}, err
	}
	if err := ensureAuthUserActive(user); err != nil {
		if errors.Is(err, ErrUserBanned) || errors.Is(err, ErrUserDeleted) {
			return AuthSession{}, ErrUnauthorized
		}
		return AuthSession{}, err
	}

	return AuthSession{
		User: AuthUser{
			ID:    user.UserID,
			Email: user.Email,
			Name:  user.Name,
		},
		Token: accessToken,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, accessToken string) error {
	if s == nil || s.userSessionRepo == nil {
		return errors.New("auth service dependencies are nil")
	}

	token, err := s.parseToken(accessToken, authTokenTypeAccess)
	if err != nil {
		return nil
	}

	now := time.Now().UTC()
	return s.userSessionRepo.Revoke(ctx, token.UserID, token.SessionID, now)
}

func (s *AuthService) issueSessionTokens(ctx context.Context, userID string) (string, string, error) {
	now := time.Now().UTC()
	sessionID := strings.ToLower(ulid.Make().String())

	accessToken, _, err := s.issueToken(userID, sessionID, authTokenTypeAccess, s.accessTokenTTL)
	if err != nil {
		return "", "", err
	}

	refreshToken, refreshExpiresAt, err := s.issueToken(userID, sessionID, authTokenTypeRefresh, s.refreshTokenTTL)
	if err != nil {
		return "", "", err
	}

	session := &models.UserSession{
		SessionID:        sessionID,
		UserID:           userID,
		RefreshTokenHash: hashToken(refreshToken),
		ExpiresAt:        refreshExpiresAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.userSessionRepo.Create(ctx, session); err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (s *AuthService) rotateSessionTokens(
	ctx context.Context,
	currentToken parsedToken,
	rawRefreshToken string,
) (string, string, error) {
	now := time.Now().UTC()
	nextSessionID := strings.ToLower(ulid.Make().String())

	nextAccessToken, _, err := s.issueToken(currentToken.UserID, nextSessionID, authTokenTypeAccess, s.accessTokenTTL)
	if err != nil {
		return "", "", err
	}
	nextRefreshToken, nextExpiresAt, err := s.issueToken(currentToken.UserID, nextSessionID, authTokenTypeRefresh, s.refreshTokenTTL)
	if err != nil {
		return "", "", err
	}

	if err := s.userSessionRepo.Rotate(ctx, repository.RotateUserSessionParams{
		UserID:                  currentToken.UserID,
		CurrentSessionID:        currentToken.SessionID,
		CurrentRefreshTokenHash: hashToken(rawRefreshToken),
		NextSessionID:           nextSessionID,
		NextRefreshTokenHash:    hashToken(nextRefreshToken),
		NextExpiresAt:           nextExpiresAt,
		Now:                     now,
	}); err != nil {
		return "", "", err
	}

	return nextAccessToken, nextRefreshToken, nil
}

func (s *AuthService) issueToken(
	userID string,
	sessionID string,
	tokenType string,
	ttl time.Duration,
) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)
	claims := authTokenClaims{
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        sessionID,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}
	return tokenString, expiresAt, nil
}

func (s *AuthService) parseToken(rawToken string, expectedTokenType string) (parsedToken, error) {
	claims := &authTokenClaims{}
	parsedTokenValue, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return parsedToken{}, err
	}
	if !parsedTokenValue.Valid {
		return parsedToken{}, errors.New("token is invalid")
	}
	if claims.Subject == "" {
		return parsedToken{}, errors.New("token subject is empty")
	}
	if claims.ID == "" {
		return parsedToken{}, errors.New("token session id is empty")
	}
	if claims.TokenType != expectedTokenType {
		return parsedToken{}, errors.New("token type mismatch")
	}
	return parsedToken{
		UserID:    claims.Subject,
		SessionID: claims.ID,
	}, nil
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func hashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") ||
		strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "unique failed")
}

func ensureAuthUserActive(user *models.User) error {
	if user == nil {
		return ErrUnauthorized
	}
	switch EnsureEntityActive(user.Status, user.BannedAt, user.DeletedAt) {
	case nil:
		return nil
	case ErrEntityBanned:
		return ErrUserBanned
	case ErrEntityDeleted:
		return ErrUserDeleted
	default:
		return ErrUnauthorized
	}
}
