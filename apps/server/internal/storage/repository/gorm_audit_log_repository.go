package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormAuditLogRepository struct {
	db *gorm.DB
}

type auditLogListRow = auditLogListRowDB

// NewGormAuditLogRepository 创建基于 GORM 的审计日志仓储实现。
func NewGormAuditLogRepository(db *gorm.DB) AuditLogRepository {
	return &gormAuditLogRepository{db: db}
}

func (r *gormAuditLogRepository) Create(ctx context.Context, auditLog *models.AuditLog) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("audit log repository db is nil")
	}
	if auditLog == nil {
		return fmt.Errorf("audit log is nil")
	}

	auditLog.Module = strings.ToLower(strings.TrimSpace(auditLog.Module))
	auditLog.Action = strings.ToLower(strings.TrimSpace(auditLog.Action))
	auditLog.TargetType = strings.ToLower(strings.TrimSpace(auditLog.TargetType))
	auditLog.TargetID = strings.TrimSpace(auditLog.TargetID)
	auditLog.Summary = strings.TrimSpace(auditLog.Summary)
	auditLog.RequestID = strings.TrimSpace(auditLog.RequestID)
	if strings.TrimSpace(auditLog.DetailJSON) == "" {
		auditLog.DetailJSON = "{}"
	}
	if auditLog.CreatedAt.IsZero() {
		auditLog.CreatedAt = time.Now().UTC()
	}

	if auditLog.ActorUserID != nil {
		actorUserID := strings.TrimSpace(*auditLog.ActorUserID)
		if actorUserID == "" {
			auditLog.ActorUserID = nil
		} else {
			auditLog.ActorUserID = &actorUserID
		}
	}

	return r.db.WithContext(ctx).Create(auditLog).Error
}

func (r *gormAuditLogRepository) List(
	ctx context.Context,
	params ListAuditLogsParams,
) ([]AdminAuditLogListRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("audit log repository db is nil")
	}

	auditLogTableName := (models.AuditLog{}).TableName()
	userTableName := (models.User{}).TableName()
	auditAlias := "al"
	userAlias := "u"

	baseQuery := r.db.WithContext(ctx).
		Table(auditLogTableName + " AS " + auditAlias).
		Joins(
			"LEFT JOIN " + userTableName + " AS " + userAlias +
				" ON " + qualifiedColumn(userAlias, models.UserColumns.UserID) +
				" = " + qualifiedColumn(auditAlias, models.AuditLogColumns.ActorUserID),
		)

	if actorUserID := strings.TrimSpace(params.ActorUserID); actorUserID != "" {
		baseQuery = baseQuery.Where(qualifiedColumn(auditAlias, models.AuditLogColumns.ActorUserID)+" = ?", actorUserID)
	}
	if module := strings.ToLower(strings.TrimSpace(params.Module)); module != "" {
		baseQuery = baseQuery.Where(qualifiedColumn(auditAlias, models.AuditLogColumns.Module)+" = ?", module)
	}
	if action := strings.ToLower(strings.TrimSpace(params.Action)); action != "" {
		baseQuery = baseQuery.Where(qualifiedColumn(auditAlias, models.AuditLogColumns.Action)+" = ?", action)
	}
	if targetType := strings.ToLower(strings.TrimSpace(params.TargetType)); targetType != "" {
		baseQuery = baseQuery.Where(qualifiedColumn(auditAlias, models.AuditLogColumns.TargetType)+" = ?", targetType)
	}
	if targetID := strings.TrimSpace(params.TargetID); targetID != "" {
		baseQuery = baseQuery.Where(qualifiedColumn(auditAlias, models.AuditLogColumns.TargetID)+" = ?", targetID)
	}
	if requestID := strings.TrimSpace(params.RequestID); requestID != "" {
		baseQuery = baseQuery.Where(qualifiedColumn(auditAlias, models.AuditLogColumns.RequestID)+" = ?", requestID)
	}
	if params.CreatedAtFrom != nil {
		baseQuery = baseQuery.Where(qualifiedColumn(auditAlias, models.AuditLogColumns.CreatedAt)+" >= ?", params.CreatedAtFrom.UTC())
	}
	if params.CreatedAtTo != nil {
		baseQuery = baseQuery.Where(qualifiedColumn(auditAlias, models.AuditLogColumns.CreatedAt)+" <= ?", params.CreatedAtTo.UTC())
	}

	restrictModules := normalizeAuditModules(params.RestrictModules)
	if len(restrictModules) > 0 {
		baseQuery = baseQuery.Where(qualifiedColumn(auditAlias, models.AuditLogColumns.Module)+" IN ?", restrictModules)
	}

	if keyword := strings.ToLower(strings.TrimSpace(params.Keyword)); keyword != "" {
		likeKeyword := "%" + keyword + "%"
		baseQuery = baseQuery.Where(
			`LOWER(`+qualifiedColumn(auditAlias, models.AuditLogColumns.ActorUserID)+`) LIKE ? OR LOWER(`+qualifiedColumn(auditAlias, models.AuditLogColumns.Module)+`) LIKE ? OR LOWER(`+qualifiedColumn(auditAlias, models.AuditLogColumns.Action)+`) LIKE ? OR LOWER(`+qualifiedColumn(auditAlias, models.AuditLogColumns.TargetType)+`) LIKE ? OR LOWER(`+qualifiedColumn(auditAlias, models.AuditLogColumns.TargetID)+`) LIKE ? OR LOWER(`+qualifiedColumn(auditAlias, models.AuditLogColumns.Summary)+`) LIKE ? OR LOWER(`+qualifiedColumn(auditAlias, models.AuditLogColumns.RequestID)+`) LIKE ? OR LOWER(COALESCE(`+qualifiedColumn(userAlias, models.UserColumns.Name)+`, '')) LIKE ? OR LOWER(COALESCE(`+qualifiedColumn(userAlias, models.UserColumns.Email)+`, '')) LIKE ?`,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
		)
	}

	var total int64
	if err := baseQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	var rows []auditLogListRow
	if err := baseQuery.Session(&gorm.Session{}).
		Select(
			qualifiedColumn(auditAlias, models.AuditLogColumns.ID),
			qualifiedColumn(auditAlias, models.AuditLogColumns.ActorUserID),
			qualifiedColumn(auditAlias, models.AuditLogColumns.Module),
			qualifiedColumn(auditAlias, models.AuditLogColumns.Action),
			qualifiedColumn(auditAlias, models.AuditLogColumns.TargetType),
			qualifiedColumn(auditAlias, models.AuditLogColumns.TargetID),
			qualifiedColumn(auditAlias, models.AuditLogColumns.Summary),
			qualifiedColumn(auditAlias, models.AuditLogColumns.DetailJSON),
			qualifiedColumn(auditAlias, models.AuditLogColumns.RequestID),
			qualifiedColumn(auditAlias, models.AuditLogColumns.CreatedAt)+" AS created_at_raw",
			qualifiedColumn(userAlias, models.UserColumns.Name)+" AS actor_name",
			qualifiedColumn(userAlias, models.UserColumns.Email)+" AS actor_email",
		).
		Order(clause.OrderByColumn{
			Column: clause.Column{Table: auditAlias, Name: models.AuditLogColumns.CreatedAt},
			Desc:   true,
		}).
		Order(clause.OrderByColumn{
			Column: clause.Column{Table: auditAlias, Name: models.AuditLogColumns.ID},
			Desc:   true,
		}).
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	records := make([]AdminAuditLogListRecord, 0, len(rows))
	for _, row := range rows {
		auditLog := models.AuditLog{
			ID:          row.ID,
			ActorUserID: row.ActorUserID,
			Module:      row.Module,
			Action:      row.Action,
			TargetType:  row.TargetType,
			TargetID:    row.TargetID,
			Summary:     row.Summary,
			DetailJSON:  row.DetailJSON,
			RequestID:   row.RequestID,
			CreatedAt:   recordtime.Parse(row.CreatedAtRaw),
		}

		record := AdminAuditLogListRecord{AuditLog: auditLog}
		if row.ActorName != nil {
			record.ActorName = *row.ActorName
		}
		if row.ActorEmail != nil {
			record.ActorEmail = *row.ActorEmail
		}
		records = append(records, record)
	}

	return records, total, nil
}

func normalizeAuditModules(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	result := make([]string, 0, len(input))
	exists := make(map[string]struct{}, len(input))
	for _, item := range input {
		module := strings.ToLower(strings.TrimSpace(item))
		if module == "" {
			continue
		}
		if _, ok := exists[module]; ok {
			continue
		}
		exists[module] = struct{}{}
		result = append(result, module)
	}
	return result
}
