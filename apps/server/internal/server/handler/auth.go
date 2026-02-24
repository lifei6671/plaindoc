package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/config"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type authHandler struct {
	authService                   *service.AuthService
	authRegistrationPolicyService *service.AuthRegistrationPolicyService
	accessTokenTTL                time.Duration
	refreshTokenTTL               time.Duration
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type authUserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

type authSessionResponse struct {
	User         authUserResponse `json:"user"`
	Token        string           `json:"token,omitempty"`
	RefreshToken string           `json:"refreshToken,omitempty"`
}

const (
	accessTokenCookieName  = "accessToken"
	refreshTokenCookieName = "refreshToken"
)

// NewAuthHandler 创建认证处理器，负责注册、登录、会话校验和 token 刷新。
func NewAuthHandler(
	authService *service.AuthService,
	authRegistrationPolicyService *service.AuthRegistrationPolicyService,
	jwtConfig config.JWTConfig,
) *authHandler {
	return &authHandler{
		authService:                   authService,
		authRegistrationPolicyService: authRegistrationPolicyService,
		accessTokenTTL:                jwtConfig.AccessTokenTTL,
		refreshTokenTTL:               jwtConfig.RefreshTokenTTL,
	}
}

// Register 创建账号并返回会话 token。
func (h *authHandler) Register(c *gin.Context) {
	if h == nil || h.authService == nil {
		response.InternalError(c)
		return
	}
	if h.authRegistrationPolicyService != nil {
		allowRegistration, err := h.authRegistrationPolicyService.AllowRegistration(c.Request.Context())
		if err != nil {
			response.InternalError(c)
			return
		}
		if !allowRegistration {
			response.AuthErrRegistrationDisabled.Write(c)
			return
		}
	}

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AuthErrRequestBody.Write(c)
		return
	}

	email := normalizeEmail(req.Email)
	name := strings.TrimSpace(req.Name)
	password := req.Password
	if email == "" || !strings.Contains(email, "@") {
		response.AuthErrEmail.Write(c)
		return
	}
	if len(password) < 6 {
		response.AuthErrPasswordLeast6Characters.Write(c)
		return
	}
	if name == "" {
		response.AuthErrNameRequired.Write(c)
		return
	}

	session, err := h.authService.Register(c.Request.Context(), email, password, name)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailAlreadyExists):
			response.AuthErrEmailAlreadyExists.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}
	h.writeSessionCookies(c, session.Token, session.RefreshToken)

	response.JSON(c, http.StatusCreated, authSessionResponse{
		User: authUserResponse{
			ID:        session.User.ID,
			Email:     session.User.Email,
			Name:      session.User.Name,
			AvatarURL: session.User.AvatarURL,
		},
		Token:        session.Token,
		RefreshToken: session.RefreshToken,
	})
}

// Login 校验账号密码并返回会话 token。
func (h *authHandler) Login(c *gin.Context) {
	if h == nil || h.authService == nil {
		response.InternalError(c)
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AuthErrRequestBody.Write(c)
		return
	}

	email := normalizeEmail(req.Email)
	if email == "" || req.Password == "" {
		response.AuthErrEmailPasswordRequired.Write(c)
		return
	}

	session, err := h.authService.Login(c.Request.Context(), email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			response.AuthErrEmailPassword.Write(c)
		case errors.Is(err, service.ErrUserBanned):
			response.AuthErrUserHasBeenBanned.Write(c)
		case errors.Is(err, service.ErrUserDeleted):
			response.AuthErrUserHasBeenDeleted.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}
	h.writeSessionCookies(c, session.Token, session.RefreshToken)

	response.JSON(c, http.StatusOK, authSessionResponse{
		User: authUserResponse{
			ID:        session.User.ID,
			Email:     session.User.Email,
			Name:      session.User.Name,
			AvatarURL: session.User.AvatarURL,
		},
		Token:        session.Token,
		RefreshToken: session.RefreshToken,
	})
}

// Refresh 使用 refresh token 换发新 token。
func (h *authHandler) Refresh(c *gin.Context) {
	if h == nil || h.authService == nil {
		response.InternalError(c)
		return
	}

	refreshToken := strings.TrimSpace(c.GetHeader("X-Refresh-Token"))
	if refreshToken == "" {
		var req refreshRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			refreshToken = strings.TrimSpace(req.RefreshToken)
		}
	}
	if refreshToken == "" {
		tokenFromHeader, ok := bearerTokenFromRequest(c)
		if ok {
			refreshToken = tokenFromHeader
		}
	}
	if refreshToken == "" {
		tokenFromCookie, ok := refreshTokenFromCookie(c)
		if ok {
			refreshToken = tokenFromCookie
		}
	}
	if refreshToken == "" {
		response.AuthErrRefreshTokenRequired.Write(c)
		return
	}

	session, err := h.authService.Refresh(c.Request.Context(), refreshToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRefreshToken):
			response.AuthErrRefreshToken.Write(c)
		case errors.Is(err, service.ErrUnauthorized):
			response.AuthErrUserNotFound.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}
	h.writeSessionCookies(c, session.Token, session.RefreshToken)

	response.JSON(c, http.StatusOK, authSessionResponse{
		User: authUserResponse{
			ID:        session.User.ID,
			Email:     session.User.Email,
			Name:      session.User.Name,
			AvatarURL: session.User.AvatarURL,
		},
		Token:        session.Token,
		RefreshToken: session.RefreshToken,
	})
}

// Me 返回当前登录用户信息。
func (h *authHandler) Me(c *gin.Context) {
	if h == nil || h.authService == nil {
		response.InternalError(c)
		return
	}

	accessToken, ok := bearerTokenFromRequest(c)
	if !ok {
		response.AuthErrAuthorizationTokenRequired.Write(c)
		return
	}

	session, err := h.authService.Me(c.Request.Context(), accessToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUnauthorized):
			response.AuthErrAccessToken.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	response.JSON(c, http.StatusOK, authSessionResponse{
		User: authUserResponse{
			ID:        session.User.ID,
			Email:     session.User.Email,
			Name:      session.User.Name,
			AvatarURL: session.User.AvatarURL,
		},
		Token: session.Token,
	})
}

// Logout 退出当前会话：有 token 时尽力吊销会话，无 token 时返回统一成功体。
func (h *authHandler) Logout(c *gin.Context) {
	if h == nil || h.authService == nil {
		clearSessionCookies(c)
		response.JSON(c, http.StatusOK, struct{}{})
		return
	}

	accessToken, ok := bearerTokenFromRequest(c)
	if ok {
		_ = h.authService.Logout(c.Request.Context(), accessToken)
	}
	clearSessionCookies(c)
	response.JSON(c, http.StatusOK, struct{}{})
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func bearerTokenFromRequest(c *gin.Context) (string, bool) {
	rawAuthorization := strings.TrimSpace(c.GetHeader("Authorization"))
	if rawAuthorization != "" {
		parts := strings.SplitN(rawAuthorization, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token := strings.TrimSpace(parts[1])
			if token != "" {
				return token, true
			}
		}
	}

	tokenFromCookie, err := c.Cookie(accessTokenCookieName)
	if err != nil {
		return "", false
	}
	token := strings.TrimSpace(tokenFromCookie)
	if token == "" {
		return "", false
	}
	return token, true
}

func refreshTokenFromCookie(c *gin.Context) (string, bool) {
	tokenFromCookie, err := c.Cookie(refreshTokenCookieName)
	if err != nil {
		return "", false
	}
	token := strings.TrimSpace(tokenFromCookie)
	if token == "" {
		return "", false
	}
	return token, true
}

func (h *authHandler) writeSessionCookies(c *gin.Context, accessToken string, refreshToken string) {
	if c == nil {
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	secure := requestUsesHTTPS(c)
	accessTokenMaxAge := int(h.accessTokenTTL.Seconds())
	refreshTokenMaxAge := int(h.refreshTokenTTL.Seconds())
	if accessTokenMaxAge < 0 {
		accessTokenMaxAge = 0
	}
	if refreshTokenMaxAge < 0 {
		refreshTokenMaxAge = 0
	}
	c.SetCookie(accessTokenCookieName, accessToken, accessTokenMaxAge, "/", "", secure, true)
	c.SetCookie(refreshTokenCookieName, refreshToken, refreshTokenMaxAge, "/", "", secure, true)
}

func clearSessionCookies(c *gin.Context) {
	if c == nil {
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	secure := requestUsesHTTPS(c)
	c.SetCookie(accessTokenCookieName, "", -1, "/", "", secure, true)
	c.SetCookie(refreshTokenCookieName, "", -1, "/", "", secure, true)
}

func requestUsesHTTPS(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if c.Request.TLS != nil {
		return true
	}
	rawForwardedProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if rawForwardedProto == "" {
		return false
	}
	for _, item := range strings.Split(rawForwardedProto, ",") {
		if strings.EqualFold(strings.TrimSpace(item), "https") {
			return true
		}
	}
	return false
}
