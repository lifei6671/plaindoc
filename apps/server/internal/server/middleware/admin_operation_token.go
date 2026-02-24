package middleware

import (
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
			response.MiddlewareAdminOperationTokenErrAdminActorMissing.Write(c)
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
			response.FromError(c, err)
			return
		}

		c.Next()
	}
}
