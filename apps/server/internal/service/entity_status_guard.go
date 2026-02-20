package service

import (
	"errors"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

var (
	ErrEntityBanned  = errors.New("entity banned")
	ErrEntityDeleted = errors.New("entity deleted")
)

// EnsureEntityActive 统一判定实体是否可访问，避免在 handler/service 重复写状态判断。
func EnsureEntityActive(status models.EntityStatus, bannedAt *time.Time, deletedAt *time.Time) error {
	normalizedStatus := normalizeEntityStatus(status)
	if normalizedStatus == models.EntityStatusDeleted || deletedAt != nil {
		return ErrEntityDeleted
	}
	if normalizedStatus == models.EntityStatusBanned || bannedAt != nil {
		return ErrEntityBanned
	}
	return nil
}

func normalizeEntityStatus(status models.EntityStatus) models.EntityStatus {
	if models.IsValidEntityStatus(status) {
		return status
	}
	return models.EntityStatusActive
}
