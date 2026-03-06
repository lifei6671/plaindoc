package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type onlyOfficeHandler struct {
	authService             *service.AuthService
	onlyOfficeConfigService *service.OnlyOfficeConfigService
}

type onlyOfficeClientConfigResponse struct {
	Enabled bool `json:"enabled"`
}

// NewOnlyOfficeHandler 创建 ONLYOFFICE 运行时配置处理器。
func NewOnlyOfficeHandler(
	authService *service.AuthService,
	onlyOfficeConfigService *service.OnlyOfficeConfigService,
) *onlyOfficeHandler {
	return &onlyOfficeHandler{
		authService:             authService,
		onlyOfficeConfigService: onlyOfficeConfigService,
	}
}

// GetConfig 返回当前生效的 ONLYOFFICE 前端运行时配置。
func (h *onlyOfficeHandler) GetConfig(c *gin.Context) {
	if h == nil || h.authService == nil || h.onlyOfficeConfigService == nil {
		response.InternalError(c)
		return
	}
	if _, ok := h.requireAuthenticatedUser(c); !ok {
		return
	}

	config, err := h.onlyOfficeConfigService.GetConfig(c.Request.Context())
	if err != nil {
		setRequestErrmsg(c, err, "读取 ONLYOFFICE 配置失败")
		response.InternalError(c)
		return
	}

	response.JSON(c, http.StatusOK, onlyOfficeClientConfigResponse{
		Enabled: config.Enabled,
	})
}

func (h *onlyOfficeHandler) requireAuthenticatedUser(c *gin.Context) (string, bool) {
	accessToken, ok := bearerTokenFromRequest(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "需要登录")
		return "", false
	}

	session, err := h.authService.Me(c.Request.Context(), accessToken)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "访问令牌无效")
		return "", false
	}

	return session.User.ID, true
}
