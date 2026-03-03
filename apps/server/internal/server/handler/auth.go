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
	authLoginOrchestrator         *service.AuthLoginOrchestrator
	authRegistrationPolicyService *service.AuthRegistrationPolicyService
	authRiskControlService        *service.AuthRiskControlService
	passwordResetService          *service.PasswordResetService
	accessTokenTTL                time.Duration
	refreshTokenTTL               time.Duration
}

type registerRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	Name          string `json:"name"`
	CaptchaID     string `json:"captchaId"`
	CaptchaAnswer string `json:"captchaAnswer"`
}

type loginRequest struct {
	Email         string `json:"email"`
	Identifier    string `json:"identifier"`
	Provider      string `json:"provider"`
	Password      string `json:"password"`
	CaptchaID     string `json:"captchaId"`
	CaptchaAnswer string `json:"captchaAnswer"`
}

type refreshCaptchaRequest struct {
	Scene      string `json:"scene"`
	Identifier string `json:"identifier"`
	Email      string `json:"email"`
	CaptchaID  string `json:"captchaId"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type passwordResetRequestRequest struct {
	Email string `json:"email"`
}

type passwordResetVerifyRequest struct {
	Token string `json:"token"`
}

type passwordResetConfirmRequest struct {
	Token           string `json:"token"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

type authLoginProviderOptionResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Priority int    `json:"priority"`
}

type authLoginOptionsResponse struct {
	LoginMode         string                            `json:"loginMode"`
	DefaultProviderID string                            `json:"defaultProviderId"`
	AllowUserRegister bool                              `json:"allowUserRegister"`
	Providers         []authLoginProviderOptionResponse `json:"providers"`
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

var (
	authRegisterErrorMappings = []response.ErrorTemplateMapping{
		{
			Target:   service.ErrEmailAlreadyExists,
			Template: response.AuthErrEmailAlreadyExists,
		},
	}
	authLoginErrorMappings = []response.ErrorTemplateMapping{
		{
			Target:   service.ErrInvalidCredentials,
			Template: response.AuthErrEmailPassword,
		},
		{
			Target:   service.ErrAuthProviderUnavailable,
			Template: response.AuthErrEmailPassword,
		},
		{
			Target:   service.ErrAuthProviderFailure,
			Template: response.AuthErrEmailPassword,
		},
		{
			Target:   service.ErrUserBanned,
			Template: response.AuthErrUserHasBeenBanned,
		},
		{
			Target:   service.ErrUserDeleted,
			Template: response.AuthErrUserHasBeenDeleted,
		},
	}
	authRefreshErrorMappings = []response.ErrorTemplateMapping{
		{
			Target:   service.ErrInvalidRefreshToken,
			Template: response.AuthErrRefreshToken,
		},
		{
			Target:   service.ErrUnauthorized,
			Template: response.AuthErrUserNotFound,
		},
	}
	authMeErrorMappings = []response.ErrorTemplateMapping{
		{
			Target:   service.ErrUnauthorized,
			Template: response.AuthErrAccessToken,
		},
	}
	authPasswordResetRequestErrorMappings = []response.ErrorTemplateMapping{
		{
			Target:   service.ErrPasswordResetEmailDisabled,
			Template: response.AuthErrPasswordResetUnavailable,
		},
		{
			Target:   service.ErrPasswordResetRateLimited,
			Template: response.AuthErrPasswordResetRateLimited,
		},
		{
			Target:   service.ErrPasswordResetEmailSendFailed,
			Template: response.AuthErrPasswordResetSendFailed,
		},
	}
	authPasswordResetVerifyErrorMappings = []response.ErrorTemplateMapping{
		{
			Target:   service.ErrPasswordResetTokenInvalid,
			Template: response.AuthErrPasswordResetTokenInvalid,
		},
		{
			Target:   service.ErrPasswordResetTokenExpired,
			Template: response.AuthErrPasswordResetTokenExpired,
		},
		{
			Target:   service.ErrPasswordResetTokenConsumed,
			Template: response.AuthErrPasswordResetTokenConsumed,
		},
	}
	authPasswordResetConfirmErrorMappings = []response.ErrorTemplateMapping{
		{
			Target:   service.ErrPasswordResetTokenInvalid,
			Template: response.AuthErrPasswordResetTokenInvalid,
		},
		{
			Target:   service.ErrPasswordResetTokenExpired,
			Template: response.AuthErrPasswordResetTokenExpired,
		},
		{
			Target:   service.ErrPasswordResetTokenConsumed,
			Template: response.AuthErrPasswordResetTokenConsumed,
		},
		{
			Target:   service.ErrPasswordResetPasswordTooShort,
			Template: response.AuthErrPasswordLeast6Characters,
		},
		{
			Target:   service.ErrPasswordResetConfirmMismatch,
			Template: response.AuthErrPasswordConfirmMismatch,
		},
		{
			Target:   service.ErrPasswordResetPasswordUnchanged,
			Template: response.AuthErrPasswordUnchanged,
		},
		{
			Target:   service.ErrPasswordResetUserNotSupported,
			Template: response.AuthErrPasswordResetUnsupported,
		},
	}
)

// NewAuthHandler 创建认证处理器，负责注册、登录、会话校验和 token 刷新。
func NewAuthHandler(
	authService *service.AuthService,
	authRegistrationPolicyService *service.AuthRegistrationPolicyService,
	authLoginOrchestrator *service.AuthLoginOrchestrator,
	authRiskControlService *service.AuthRiskControlService,
	passwordResetService *service.PasswordResetService,
	jwtConfig config.JWTConfig,
) *authHandler {
	if authLoginOrchestrator == nil {
		authLoginOrchestrator = service.NewAuthLoginOrchestrator(
			service.AuthProviderLocalID,
			service.NewLocalAuthLoginProvider(authService),
		)
	}
	return &authHandler{
		authService:                   authService,
		authLoginOrchestrator:         authLoginOrchestrator,
		authRegistrationPolicyService: authRegistrationPolicyService,
		authRiskControlService:        authRiskControlService,
		passwordResetService:          passwordResetService,
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
			setRequestErrmsg(c, err, "检查注册策略失败")
			response.InternalError(c)
			return
		}
		if !allowRegistration {
			setRequestErrmsg(c, nil, "网站未开启注册")
			response.AuthErrRegistrationDisabled.Write(c)
			return
		}
	}

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.AuthErrRequestBody.Write(c)
		return
	}

	email := normalizeEmail(req.Email)
	name := strings.TrimSpace(req.Name)
	password := req.Password
	if email == "" || !strings.Contains(email, "@") {
		setRequestErrmsg(c, nil, "邮箱格式不正确")
		response.AuthErrEmail.Write(c)
		return
	}
	if len(password) < 6 {
		setRequestErrmsg(c, nil, "密码长度至少 6 位")
		response.AuthErrPasswordLeast6Characters.Write(c)
		return
	}
	if name == "" {
		setRequestErrmsg(c, nil, "名字不能为空")
		response.AuthErrNameRequired.Write(c)
		return
	}
	if err := h.checkAuthRisk(
		c,
		service.AuthRiskCheckInput{
			Scene:         "register",
			ClientIP:      strings.TrimSpace(c.ClientIP()),
			Identifier:    email,
			CaptchaID:     strings.TrimSpace(req.CaptchaID),
			CaptchaAnswer: strings.TrimSpace(req.CaptchaAnswer),
		},
	); err != nil {
		setRequestErrmsg(c, err, "检查认证风险失败")
		return
	}

	session, err := h.authService.Register(c.Request.Context(), email, password, name)
	if err != nil {
		setRequestErrmsg(c, err, "用户注册失败")
		h.recordAuthRisk(c, service.AuthRiskRecordInput{
			Scene:      "register",
			ClientIP:   strings.TrimSpace(c.ClientIP()),
			Identifier: email,
			Success:    false,
		})
		if !response.WriteMappedError(c, err, authRegisterErrorMappings...) {
			response.InternalError(c)
		}
		return
	}
	h.recordAuthRisk(c, service.AuthRiskRecordInput{
		Scene:      "register",
		ClientIP:   strings.TrimSpace(c.ClientIP()),
		Identifier: email,
		Success:    true,
	})
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

// Options 返回登录页所需的认证策略选项（不包含敏感配置）。
func (h *authHandler) Options(c *gin.Context) {
	if h == nil || h.authRegistrationPolicyService == nil {
		response.JSON(c, http.StatusOK, authLoginOptionsResponse{
			LoginMode:         "local_only",
			DefaultProviderID: service.AuthProviderLocalID,
			AllowUserRegister: true,
			Providers:         []authLoginProviderOptionResponse{},
		})
		return
	}

	options, err := h.authRegistrationPolicyService.ResolveLoginOptions(c.Request.Context())
	if err != nil {
		setRequestErrmsg(c, err, "获取登录选项失败")
		response.InternalError(c)
		return
	}

	providers := make([]authLoginProviderOptionResponse, 0, len(options.Providers))
	for _, provider := range options.Providers {
		providers = append(providers, authLoginProviderOptionResponse{
			ID:       provider.ID,
			Name:     provider.Name,
			Type:     provider.Type,
			Priority: provider.Priority,
		})
	}

	response.JSON(c, http.StatusOK, authLoginOptionsResponse{
		LoginMode:         options.LoginMode,
		DefaultProviderID: options.DefaultProviderID,
		AllowUserRegister: options.AllowUserRegister,
		Providers:         providers,
	})
}

// Login 校验账号密码并返回会话 token。
func (h *authHandler) Login(c *gin.Context) {
	if h == nil || h.authService == nil || h.authLoginOrchestrator == nil {
		response.InternalError(c)
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.AuthErrRequestBody.Write(c)
		return
	}

	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	if identifier == "" || req.Password == "" {
		setRequestErrmsg(c, nil, "邮箱/用户名和密码不能为空")
		response.AuthErrEmailPasswordRequired.Write(c)
		return
	}
	if err := h.checkAuthRisk(
		c,
		service.AuthRiskCheckInput{
			Scene:         "login",
			ClientIP:      strings.TrimSpace(c.ClientIP()),
			Identifier:    identifier,
			CaptchaID:     strings.TrimSpace(req.CaptchaID),
			CaptchaAnswer: strings.TrimSpace(req.CaptchaAnswer),
		},
	); err != nil {
		return
	}

	session, err := h.authLoginOrchestrator.Login(c.Request.Context(), service.AuthProviderLoginInput{
		Provider:   strings.TrimSpace(req.Provider),
		Identifier: identifier,
		Password:   req.Password,
	})
	if err != nil {
		setRequestErrmsg(c, err, "用户登录失败")
		h.recordAuthRisk(c, service.AuthRiskRecordInput{
			Scene:      "login",
			ClientIP:   strings.TrimSpace(c.ClientIP()),
			Identifier: identifier,
			Success:    false,
		})
		if !response.WriteMappedError(c, err, authLoginErrorMappings...) {
			response.InternalError(c)
		}
		return
	}
	h.recordAuthRisk(c, service.AuthRiskRecordInput{
		Scene:      "login",
		ClientIP:   strings.TrimSpace(c.ClientIP()),
		Identifier: identifier,
		Success:    true,
	})
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

// RefreshCaptcha 刷新登录/注册验证码验证图片。
func (h *authHandler) RefreshCaptcha(c *gin.Context) {
	if h == nil || h.authRiskControlService == nil {
		response.InternalError(c)
		return
	}

	var req refreshCaptchaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.AuthErrRequestBody.Write(c)
		return
	}

	scene := strings.ToLower(strings.TrimSpace(req.Scene))
	if scene != "login" && scene != "register" {
		setRequestErrmsg(c, nil, "场景参数不合法")
		response.AuthErrRequestBody.Write(c)
		return
	}

	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	if scene == "register" {
		identifier = normalizeEmail(identifier)
	}
	if identifier == "" {
		response.AuthErrRequestBody.Write(c)
		return
	}

	challenge, err := h.authRiskControlService.RefreshChallenge(c.Request.Context(), service.AuthRiskRefreshInput{
		Scene:      scene,
		ClientIP:   strings.TrimSpace(c.ClientIP()),
		Identifier: identifier,
		CaptchaID:  strings.TrimSpace(req.CaptchaID),
	})
	if err != nil {
		setRequestErrmsg(c, err, "刷新验证码失败")
		var riskErr *service.AuthRiskError
		if errors.As(err, &riskErr) {
			h.writeAuthRiskError(c, riskErr)
			return
		}
		response.InternalError(c)
		return
	}

	response.JSON(c, http.StatusOK, challenge)
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
		setRequestErrmsg(c, err, "刷新 token 失败")
		if !response.WriteMappedError(c, err, authRefreshErrorMappings...) {
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
		setRequestErrmsg(c, err, "获取会话信息失败")
		if !response.WriteMappedError(c, err, authMeErrorMappings...) {
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

// RequestPasswordReset 接收邮箱并发送密码重置邮件（防枚举：不存在邮箱同样返回成功）。
func (h *authHandler) RequestPasswordReset(c *gin.Context) {
	if h == nil || h.passwordResetService == nil {
		response.AuthErrPasswordResetUnavailable.Write(c)
		return
	}

	var req passwordResetRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.AuthErrRequestBody.Write(c)
		return
	}

	if err := h.passwordResetService.RequestByEmail(c.Request.Context(), service.RequestPasswordResetByEmailInput{
		Email:    req.Email,
		ClientIP: strings.TrimSpace(c.ClientIP()),
	}); err != nil {
		setRequestErrmsg(c, err, "发送密码重置邮件失败")
		if !response.WriteMappedError(c, err, authPasswordResetRequestErrorMappings...) {
			response.InternalError(c)
		}
		return
	}
	response.JSON(c, http.StatusOK, struct{}{})
}

// VerifyPasswordResetToken 校验密码重置令牌有效性。
func (h *authHandler) VerifyPasswordResetToken(c *gin.Context) {
	if h == nil || h.passwordResetService == nil {
		response.AuthErrPasswordResetUnavailable.Write(c)
		return
	}

	var req passwordResetVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.AuthErrRequestBody.Write(c)
		return
	}

	result, err := h.passwordResetService.VerifyToken(c.Request.Context(), strings.TrimSpace(req.Token))
	if err != nil {
		setRequestErrmsg(c, err, "校验密码重置令牌失败")
		if !response.WriteMappedError(c, err, authPasswordResetVerifyErrorMappings...) {
			response.InternalError(c)
		}
		return
	}
	response.JSON(c, http.StatusOK, map[string]any{
		"valid":     true,
		"expiresAt": result.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// ConfirmPasswordReset 使用重置令牌更新密码。
func (h *authHandler) ConfirmPasswordReset(c *gin.Context) {
	if h == nil || h.passwordResetService == nil {
		response.AuthErrPasswordResetUnavailable.Write(c)
		return
	}

	var req passwordResetConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.AuthErrRequestBody.Write(c)
		return
	}

	if err := h.passwordResetService.Confirm(c.Request.Context(), service.ConfirmPasswordResetInput{
		Token:           strings.TrimSpace(req.Token),
		NewPassword:     req.NewPassword,
		ConfirmPassword: req.ConfirmPassword,
	}); err != nil {
		setRequestErrmsg(c, err, "重置密码失败")
		if !response.WriteMappedError(c, err, authPasswordResetConfirmErrorMappings...) {
			response.InternalError(c)
		}
		return
	}
	clearSessionCookies(c)
	response.JSON(c, http.StatusOK, struct{}{})
}

func (h *authHandler) checkAuthRisk(c *gin.Context, input service.AuthRiskCheckInput) error {
	if h == nil || h.authRiskControlService == nil {
		return nil
	}
	if c == nil || c.Request == nil {
		return nil
	}

	if err := h.authRiskControlService.Check(c.Request.Context(), input); err != nil {
		setRequestErrmsg(c, err, "检查风控信息失败")
		var riskErr *service.AuthRiskError
		if errors.As(err, &riskErr) {
			h.writeAuthRiskError(c, riskErr)
			return err
		}
		response.InternalError(c)
		return err
	}
	return nil
}

func (h *authHandler) recordAuthRisk(c *gin.Context, input service.AuthRiskRecordInput) {
	if h == nil || h.authRiskControlService == nil {
		return
	}
	if c == nil || c.Request == nil {
		return
	}
	_ = h.authRiskControlService.RecordResult(c.Request.Context(), input)
}

func (h *authHandler) writeAuthRiskError(c *gin.Context, riskErr *service.AuthRiskError) {
	if c == nil {
		return
	}
	if riskErr == nil {
		response.InternalError(c)
		return
	}

	code := response.CodeInternalError
	message := strings.TrimSpace(riskErr.Message)
	if message == "" {
		message = "认证风控策略触发"
	}
	payload := map[string]any{}

	switch strings.TrimSpace(riskErr.Type) {
	case service.AuthRiskErrorTypeCaptchaRequired:
		code = response.CodeCaptchaRequired
		if message == "认证风控策略触发" {
			message = "需要验证码校验"
		}
	case service.AuthRiskErrorTypeCaptchaInvalid:
		code = response.CodeCaptchaInvalid
		if message == "认证风控策略触发" {
			message = "验证码错误或已过期"
		}
	case service.AuthRiskErrorTypeTemporarilyLock:
		code = response.CodeAuthTemporarilyLocked
		if message == "认证风控策略触发" {
			message = "操作过于频繁，请稍后再试"
		}
	default:
		code = response.CodeInternalError
	}

	if riskErr.Result.Challenge != nil {
		payload["captchaId"] = riskErr.Result.Challenge.CaptchaID
		payload["captchaImageDataUrl"] = riskErr.Result.Challenge.CaptchaImageDataURL
		payload["level"] = riskErr.Result.Challenge.Level
		payload["expiresInSeconds"] = riskErr.Result.Challenge.ExpiresInSeconds
	}
	if riskErr.Result.LockedUntil != nil {
		payload["lockedUntil"] = riskErr.Result.LockedUntil.UTC().Format(time.RFC3339)
		payload["retryAfterSeconds"] = riskErr.Result.RetryAfterSeconds
	}

	c.JSON(http.StatusOK, response.JsonResult[map[string]any]{
		Code:      response.ResolveErrorCode(code),
		Message:   message,
		RequestID: response.RequestIDFromContext(c),
		Data:      payload,
	})
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
