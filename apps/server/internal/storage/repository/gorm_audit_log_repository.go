package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormAuditLogRepository struct {
	db *gorm.DB
}

type auditLogListRow struct {
	ID          int64   `gorm:"column:id"`
	ActorUserID *string `gorm:"column:actor_user_id"`
	Module      string  `gorm:"column:module"`
	Action      string  `gorm:"column:action"`
	TargetType  string  `gorm:"column:target_type"`
	TargetID    string  `gorm:"column:target_id"`
	Summary     string  `gorm:"column:summary"`
	DetailJSON  string  `gorm:"column:detail_json"`
	RequestID   string  `gorm:"column:request_id"`
	CreatedAt   string  `gorm:"column:created_at"`
	ActorName   *string `gorm:"column:actor_name"`
	ActorEmail  *string `gorm:"column:actor_email"`
}

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

	baseQuery := r.db.WithContext(ctx).
		Table("audit_logs AS al").
		Joins("LEFT JOIN users AS u ON u.user_id = al.actor_user_id")

	if actorUserID := strings.TrimSpace(params.ActorUserID); actorUserID != "" {
		baseQuery = baseQuery.Where("al.actor_user_id = ?", actorUserID)
	}
	if module := strings.ToLower(strings.TrimSpace(params.Module)); module != "" {
		baseQuery = baseQuery.Where("al.module = ?", module)
	}
	if action := strings.ToLower(strings.TrimSpace(params.Action)); action != "" {
		baseQuery = baseQuery.Where("al.action = ?", action)
	}
	if targetType := strings.ToLower(strings.TrimSpace(params.TargetType)); targetType != "" {
		baseQuery = baseQuery.Where("al.target_type = ?", targetType)
	}
	if targetID := strings.TrimSpace(params.TargetID); targetID != "" {
		baseQuery = baseQuery.Where("al.target_id = ?", targetID)
	}
	if requestID := strings.TrimSpace(params.RequestID); requestID != "" {
		baseQuery = baseQuery.Where("al.request_id = ?", requestID)
	}
	if params.CreatedAtFrom != nil {
		baseQuery = baseQuery.Where("al.created_at >= ?", params.CreatedAtFrom.UTC())
	}
	if params.CreatedAtTo != nil {
		baseQuery = baseQuery.Where("al.created_at <= ?", params.CreatedAtTo.UTC())
	}

	restrictModules := normalizeAuditModules(params.RestrictModules)
	if len(restrictModules) > 0 {
		baseQuery = baseQuery.Where("al.module IN ?", restrictModules)
	}

	if keyword := strings.ToLower(strings.TrimSpace(params.Keyword)); keyword != "" {
		likeKeyword := "%" + keyword + "%"
		baseQuery = baseQuery.Where(
			`LOWER(al.actor_user_id) LIKE ? OR LOWER(al.module) LIKE ? OR LOWER(al.action) LIKE ? OR LOWER(al.target_type) LIKE ? OR LOWER(al.target_id) LIKE ? OR LOWER(al.summary) LIKE ? OR LOWER(al.request_id) LIKE ? OR LOWER(COALESCE(u.name, '')) LIKE ? OR LOWER(COALESCE(u.email, '')) LIKE ?`,
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
			"al.id",
			"al.actor_user_id",
			"al.module",
			"al.action",
			"al.target_type",
			"al.target_id",
			"al.summary",
			"al.detail_json",
			"al.request_id",
			"al.created_at",
			"u.name AS actor_name",
			"u.email AS actor_email",
		).
		Order("al.created_at DESC").
		Order("al.id DESC").
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
			CreatedAt:   parseAuditLogRecordTime(row.CreatedAt),
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

func parseAuditLogRecordTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05.999-07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsedAt, err := time.Parse(layout, value); err == nil {
			return parsedAt.UTC()
		}
	}
	// 兼容数据库返回的 "YYYY-MM-DD HH:MM:SS+00:00" 等格式，统一转为 RFC3339 再解析。
	normalized := strings.Replace(value, " ", "T", 1)
	timePart := normalized
	if index := strings.IndexByte(normalized, 'T'); index >= 0 && index < len(normalized)-1 {
		timePart = normalized[index+1:]
	}
	if !strings.ContainsAny(timePart, "Zz+-") {
		normalized += "Z"
	}
	if parsedAt, err := time.Parse(time.RFC3339Nano, normalized); err == nil {
		return parsedAt.UTC()
	}
	if parsedAt, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC); err == nil {
		return parsedAt.UTC()
	}
	return time.Time{}
}
