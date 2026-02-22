package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type adminOperationTokenHandler struct {
	adminOperationTokenService *service.AdminOperationTokenService
}

type issueAdminOperationTokenRequest struct {
	Operation  string `json:"operation"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
}

type issueAdminOperationTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// NewAdminOperationTokenHandler 创建后台高风险操作 token 处理器。
func NewAdminOperationTokenHandler(
	adminOperationTokenService *service.AdminOperationTokenService,
) *adminOperationTokenHandler {
	return &adminOperationTokenHandler{
		adminOperationTokenService: adminOperationTokenService,
	}
}

// Issue 生成一次性后台操作 token，用于高风险写接口防重放。
func (h *adminOperationTokenHandler) Issue(c *gin.Context) {
	if h == nil || h.adminOperationTokenService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	var req issueAdminOperationTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	payload, err := h.adminOperationTokenService.Issue(c.Request.Context(), service.IssueAdminOperationTokenInput{
		ActorUserID: actorUserID,
		Operation:   req.Operation,
		TargetType:  strings.TrimSpace(req.TargetType),
		TargetID:    strings.TrimSpace(req.TargetID),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "admin role is required")
		case errors.Is(err, service.ErrAdminOperationTokenInvalidOperation):
			response.Error(c, http.StatusBadRequest, "INVALID_OPERATION", "operation is required")
		default:
			response.InternalError(c)
		}
		return
	}

	response.JSON(c, http.StatusOK, issueAdminOperationTokenResponse{
		Token:     payload.Token,
		ExpiresAt: payload.ExpiresAt,
	})
}
