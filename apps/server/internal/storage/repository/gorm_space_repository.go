package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormSpaceRepository struct {
	db *gorm.DB
}

// NewGormSpaceRepository 创建基于 GORM 的空间仓储实现。
func NewGormSpaceRepository(db *gorm.DB) SpaceRepository {
	return &gormSpaceRepository{db: db}
}

func (r *gormSpaceRepository) Create(ctx context.Context, space *models.Space) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("space repository db is nil")
	}
	if space != nil {
		if !models.IsValidVisibility(space.Visibility) {
			space.Visibility = models.VisibilityMember
		}
		if !models.IsValidEntityStatus(space.Status) {
			space.Status = models.EntityStatusActive
		}
	}
	return r.db.WithContext(ctx).Create(space).Error
}

func (r *gormSpaceRepository) GetBySpaceID(ctx context.Context, spaceID string) (*models.Space, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space repository db is nil")
	}

	var space models.Space
	if err := r.db.WithContext(ctx).
		Select(
			"id",
			"space_id",
			"name",
			"owner_user_id",
			"visibility",
			"status",
			"banned_reason",
			"banned_at",
			"deleted_at",
		).
		Where("space_id = ?", spaceID).
		Take(&space).Error; err != nil {
		return nil, err
	}
	if !models.IsValidVisibility(space.Visibility) {
		space.Visibility = models.VisibilityMember
	}
	if !models.IsValidEntityStatus(space.Status) {
		space.Status = models.EntityStatusActive
	}
	return &space, nil
}

func (r *gormSpaceRepository) ListByUserID(ctx context.Context, userID string) ([]models.Space, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space repository db is nil")
	}

	var spaces []models.Space
	if err := r.db.WithContext(ctx).
		Table("spaces AS s").
		Select(
			"s.id",
			"s.space_id",
			"s.name",
			"s.owner_user_id",
			"s.visibility",
			"s.status",
			"s.banned_reason",
			"s.banned_at",
			"s.deleted_at",
		).
		Joins("LEFT JOIN space_members AS sm ON sm.space_id = s.space_id AND sm.user_id = ?", userID).
		Where("s.owner_user_id = ? OR sm.id IS NOT NULL", userID).
		Order("s.id DESC").
		Find(&spaces).Error; err != nil {
		return nil, err
	}

	for i := range spaces {
		if !models.IsValidVisibility(spaces[i].Visibility) {
			spaces[i].Visibility = models.VisibilityMember
		}
		if !models.IsValidEntityStatus(spaces[i].Status) {
			spaces[i].Status = models.EntityStatusActive
		}
	}

	return spaces, nil
}

func (r *gormSpaceRepository) ListForAdmin(
	ctx context.Context,
	params ListAdminSpacesParams,
) ([]AdminSpaceListRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("space repository db is nil")
	}

	baseQuery := r.db.WithContext(ctx).
		Table("spaces AS s").
		Joins("JOIN users AS u ON u.user_id = s.owner_user_id")

	if params.RestrictToScopes {
		baseQuery = baseQuery.Joins(
			"JOIN space_admin_scopes AS sas ON sas.space_id = s.space_id AND sas.user_id = ?",
			strings.TrimSpace(params.ActorUserID),
		)
	}

	keyword := strings.ToLower(strings.TrimSpace(params.Keyword))
	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		baseQuery = baseQuery.Where(
			"LOWER(s.space_id) LIKE ? OR LOWER(s.name) LIKE ? OR LOWER(u.user_id) LIKE ? OR LOWER(u.email) LIKE ? OR LOWER(u.name) LIKE ?",
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
		)
	}

	statuses := normalizeSpaceStatuses(params.Statuses)
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where("s.status IN ?", statuses)
	}

	visibilities := normalizeSpaceVisibilities(params.Visibilities)
	if len(visibilities) > 0 {
		baseQuery = baseQuery.Where("s.visibility IN ?", visibilities)
	}

	var total int64
	if err := baseQuery.Session(&gorm.Session{}).
		Count(&total).Error; err != nil {
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

	type adminSpaceListRow struct {
		ID           int64               `gorm:"column:id"`
		SpaceID      string              `gorm:"column:space_id"`
		Name         string              `gorm:"column:name"`
		OwnerUserID  string              `gorm:"column:owner_user_id"`
		Visibility   models.Visibility   `gorm:"column:visibility"`
		Status       models.EntityStatus `gorm:"column:status"`
		BannedReason string              `gorm:"column:banned_reason"`
		BannedAt     *time.Time          `gorm:"column:banned_at"`
		DeletedAt    *time.Time          `gorm:"column:deleted_at"`
		CreatedAtRaw string              `gorm:"column:created_at"`
		UpdatedAtRaw string              `gorm:"column:updated_at"`
		OwnerName    string              `gorm:"column:owner_name"`
		OwnerEmail   string              `gorm:"column:owner_email"`
	}

	var rows []adminSpaceListRow
	if err := baseQuery.Session(&gorm.Session{}).
		Select(
			"s.id",
			"s.space_id",
			"s.name",
			"s.owner_user_id",
			"s.visibility",
			"s.status",
			"s.banned_reason",
			"s.banned_at",
			"s.deleted_at",
			"s.created_at",
			"s.updated_at",
			"u.name AS owner_name",
			"u.email AS owner_email",
		).
		Order("s.created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	result := make([]AdminSpaceListRecord, 0, len(rows))
	for _, row := range rows {
		space := models.Space{
			ID:           row.ID,
			SpaceID:      row.SpaceID,
			Name:         row.Name,
			OwnerUserID:  row.OwnerUserID,
			Visibility:   row.Visibility,
			Status:       row.Status,
			BannedReason: row.BannedReason,
			BannedAt:     row.BannedAt,
			DeletedAt:    row.DeletedAt,
			CreatedAt:    parseSpaceRecordTime(row.CreatedAtRaw),
			UpdatedAt:    parseSpaceRecordTime(row.UpdatedAtRaw),
		}
		if !models.IsValidVisibility(space.Visibility) {
			space.Visibility = models.VisibilityMember
		}
		if !models.IsValidEntityStatus(space.Status) {
			space.Status = models.EntityStatusActive
		}
		result = append(result, AdminSpaceListRecord{
			Space:      space,
			OwnerName:  row.OwnerName,
			OwnerEmail: row.OwnerEmail,
		})
	}

	return result, total, nil
}

func (r *gormSpaceRepository) UpdateVisibility(
	ctx context.Context,
	spaceID string,
	visibility models.Visibility,
) (*models.Space, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space repository db is nil")
	}

	updateTx := r.db.WithContext(ctx).
		Model(&models.Space{}).
		Where("space_id = ?", spaceID).
		Updates(map[string]any{
			"visibility": visibility,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if updateTx.Error != nil {
		return nil, updateTx.Error
	}
	if updateTx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return r.GetBySpaceID(ctx, spaceID)
}

func (r *gormSpaceRepository) UpdateStatus(ctx context.Context, params UpdateSpaceStatusParams) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	if strings.TrimSpace(params.SpaceID) == "" {
		return false, nil
	}
	if !models.IsValidEntityStatus(params.Status) {
		return false, nil
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	updateValues := map[string]any{
		"status":        params.Status,
		"updated_at":    updatedAt,
		"banned_reason": "",
		"banned_at":     nil,
	}
	if params.Status == models.EntityStatusBanned {
		updateValues["banned_reason"] = strings.TrimSpace(params.BannedReason)
		updateValues["banned_at"] = params.BannedAt
	}

	tx := r.db.WithContext(ctx).
		Model(&models.Space{}).
		Where("space_id = ? AND status <> ?", params.SpaceID, models.EntityStatusDeleted).
		Updates(updateValues)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *gormSpaceRepository) UpdateMetadata(ctx context.Context, params UpdateSpaceMetadataParams) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	if strings.TrimSpace(params.SpaceID) == "" {
		return false, nil
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	updateValues := map[string]any{
		"updated_at": updatedAt,
	}
	if params.Name != nil {
		updateValues["name"] = strings.TrimSpace(*params.Name)
	}
	if params.Visibility != nil {
		if !models.IsValidVisibility(*params.Visibility) {
			return false, nil
		}
		updateValues["visibility"] = *params.Visibility
	}

	tx := r.db.WithContext(ctx).
		Model(&models.Space{}).
		Where("space_id = ? AND status <> ?", params.SpaceID, models.EntityStatusDeleted).
		Updates(updateValues)
	if tx.Error != nil {
		return false, tx.Error
	}

	return tx.RowsAffected > 0, nil
}

func (r *gormSpaceRepository) SoftDelete(ctx context.Context, spaceID string, deletedAt time.Time) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	if strings.TrimSpace(spaceID) == "" {
		return false, nil
	}
	if deletedAt.IsZero() {
		deletedAt = time.Now().UTC()
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		spaceTx := tx.Model(&models.Space{}).
			Where("space_id = ? AND status <> ?", spaceID, models.EntityStatusDeleted).
			Updates(map[string]any{
				"status":        models.EntityStatusDeleted,
				"deleted_at":    deletedAt,
				"banned_reason": "",
				"banned_at":     nil,
				"updated_at":    deletedAt,
			})
		if spaceTx.Error != nil {
			return spaceTx.Error
		}
		if spaceTx.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		documentsQuery := tx.Table("nodes").
			Select("node_id").
			Where("space_id = ?", spaceID)

		documentTx := tx.Model(&models.Document{}).
			Where("node_id IN (?) AND status <> ?", documentsQuery, models.EntityStatusDeleted).
			Updates(map[string]any{
				"status":        models.EntityStatusDeleted,
				"deleted_at":    deletedAt,
				"banned_reason": "",
				"banned_at":     nil,
				"updated_at":    deletedAt,
			})
		if documentTx.Error != nil {
			return documentTx.Error
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *gormSpaceRepository) HasReaderAccess(ctx context.Context, spaceID string, userID string) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	if userID == "" {
		return false, nil
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Table("spaces AS s").
		Joins("LEFT JOIN space_members AS sm ON sm.space_id = s.space_id AND sm.user_id = ?", userID).
		Where(
			"s.space_id = ? AND s.status = ? AND s.deleted_at IS NULL AND (s.owner_user_id = ? OR (sm.id IS NOT NULL AND sm.role IN ?))",
			spaceID,
			models.EntityStatusActive,
			userID,
			[]models.Role{models.RoleOwner, models.RoleCollaborator, models.RoleReader},
		).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func normalizeSpaceStatuses(input []models.EntityStatus) []models.EntityStatus {
	if len(input) == 0 {
		return nil
	}
	statuses := make([]models.EntityStatus, 0, len(input))
	seen := make(map[models.EntityStatus]struct{}, len(input))
	for _, status := range input {
		if !models.IsValidEntityStatus(status) {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		statuses = append(statuses, status)
	}
	return statuses
}

func normalizeSpaceVisibilities(input []models.Visibility) []models.Visibility {
	if len(input) == 0 {
		return nil
	}
	visibilities := make([]models.Visibility, 0, len(input))
	seen := make(map[models.Visibility]struct{}, len(input))
	for _, visibility := range input {
		if !models.IsValidVisibility(visibility) {
			continue
		}
		if _, ok := seen[visibility]; ok {
			continue
		}
		seen[visibility] = struct{}{}
		visibilities = append(visibilities, visibility)
	}
	return visibilities
}

func parseSpaceRecordTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
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
	if parsedAt, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC); err == nil {
		return parsedAt.UTC()
	}
	return time.Time{}
}
