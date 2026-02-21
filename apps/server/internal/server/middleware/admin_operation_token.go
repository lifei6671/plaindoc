package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

const (
	// AdminOperationTokenHeader 是后台一次性操作 token 的请求头名称。
	AdminOperationTokenHeader = "X-Admin-Operation-Token"
)

// AdminOperationTokenBinding 描述高风险路由与一次性 token 的绑定关系。
type AdminOperationTokenBinding struct {
	Operation     string
	TargetType    string
	TargetIDParam string
}

// RequireAdminOperationToken 校验并消费后台高风险操作的一次性 token。
func RequireAdminOperationToken(
	adminOperationTokenService *service.AdminOperationTokenService,
	binding AdminOperationTokenBinding,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminOperationTokenService == nil {
			response.InternalError(c)
			return
		}

		actorUserID, err := AdminActorUserID(c)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
			return
		}

		token := strings.TrimSpace(c.GetHeader(AdminOperationTokenHeader))
		targetID := ""
		if binding.TargetIDParam != "" {
			targetID = strings.TrimSpace(c.Param(binding.TargetIDParam))
		}

		err = adminOperationTokenService.Consume(c.Request.Context(), service.ConsumeAdminOperationTokenInput{
			ActorUserID: actorUserID,
			Token:       token,
			Operation:   binding.Operation,
			TargetType:  binding.TargetType,
			TargetID:    targetID,
		})
		if err != nil {
			switch {
			case errors.Is(err, service.ErrAdminForbidden):
				response.Error(c, http.StatusForbidden, "FORBIDDEN", "admin role is required")
			case errors.Is(err, service.ErrAdminOperationTokenRequired):
				response.Error(c, http.StatusBadRequest, "OPERATION_TOKEN_REQUIRED", "operation token is required")
			case errors.Is(err, service.ErrAdminOperationTokenReplayed):
				response.Error(c, http.StatusConflict, "OPERATION_TOKEN_REPLAYED", "operation token already used")
			case errors.Is(err, service.ErrAdminOperationTokenExpired):
				response.Error(c, http.StatusConflict, "OPERATION_TOKEN_EXPIRED", "operation token is expired")
			case errors.Is(err, service.ErrAdminOperationTokenScopeMismatch):
				response.Error(c, http.StatusConflict, "OPERATION_TOKEN_SCOPE_MISMATCH", "operation token scope mismatch")
			case errors.Is(err, service.ErrAdminOperationTokenInvalid):
				response.Error(c, http.StatusConflict, "OPERATION_TOKEN_INVALID", "operation token is invalid")
			default:
				response.InternalError(c)
			}
			return
		}

		c.Next()
	}
}
