package service

import (
	"context"
	"strings"
)

type adminAuditContextKey string

const (
	adminAuditActorUserIDContextKey adminAuditContextKey = "admin_audit_actor_user_id"
	adminAuditRequestIDContextKey   adminAuditContextKey = "admin_audit_request_id"
)

// WithAdminAuditMeta 将后台审计需要的 actor/request_id 注入上下文。
func WithAdminAuditMeta(ctx context.Context, actorUserID string, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	normalizedActorUserID := strings.TrimSpace(actorUserID)
	normalizedRequestID := strings.TrimSpace(requestID)
	ctx = context.WithValue(ctx, adminAuditActorUserIDContextKey, normalizedActorUserID)
	ctx = context.WithValue(ctx, adminAuditRequestIDContextKey, normalizedRequestID)
	return ctx
}

func adminAuditActorUserIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(adminAuditActorUserIDContextKey).(string)
	return strings.TrimSpace(value)
}

func adminAuditRequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(adminAuditRequestIDContextKey).(string)
	return strings.TrimSpace(value)
}
