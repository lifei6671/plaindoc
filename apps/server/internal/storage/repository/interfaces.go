package repository

import (
	"context"
	"errors"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

var (
	// ErrInvalidSession 表示会话不存在、已吊销或刷新 token 与会话不匹配。
	ErrInvalidSession = errors.New("invalid session")
)

// UserRepository 用户仓储最小接口，服务层可按需扩展。
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByUserID(ctx context.Context, userID string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	List(ctx context.Context, params ListUsersParams) ([]models.User, int64, error)
	UpdateStatus(ctx context.Context, params UpdateUserStatusParams) (bool, error)
	SoftDelete(ctx context.Context, userID string, deletedAt time.Time) (bool, error)
}

// ListUsersParams 管理后台用户分页查询参数。
type ListUsersParams struct {
	Keyword  string
	Statuses []models.EntityStatus
	Limit    int
	Offset   int
}

// UpdateUserStatusParams 管理后台用户状态更新参数。
type UpdateUserStatusParams struct {
	UserID       string
	Status       models.EntityStatus
	BannedReason string
	BannedAt     *time.Time
	UpdatedAt    time.Time
}

// RotateUserSessionParams 定义 refresh token 旋转所需参数。
type RotateUserSessionParams struct {
	UserID                  string
	CurrentSessionID        string
	CurrentRefreshTokenHash string
	NextSessionID           string
	NextRefreshTokenHash    string
	NextExpiresAt           time.Time
	Now                     time.Time
}

// UserSessionRepository 会话仓储最小接口。
type UserSessionRepository interface {
	Create(ctx context.Context, session *models.UserSession) error
	Rotate(ctx context.Context, params RotateUserSessionParams) error
	Revoke(ctx context.Context, userID string, sessionID string, revokedAt time.Time) error
	RevokeAllByUserID(ctx context.Context, userID string, revokedAt time.Time) error
}

// AdminRoleRepository 管理角色仓储接口。
type AdminRoleRepository interface {
	HasRole(ctx context.Context, userID string, role models.AdminRole) (bool, error)
	ListByUserID(ctx context.Context, userID string) ([]models.AdminRole, error)
}

// SpaceAdminScopeRepository 空间管理范围仓储接口。
type SpaceAdminScopeRepository interface {
	HasScope(ctx context.Context, userID string, spaceID string) (bool, error)
}

// SpaceRepository 空间仓储最小接口。
type SpaceRepository interface {
	Create(ctx context.Context, space *models.Space) error
	GetBySpaceID(ctx context.Context, spaceID string) (*models.Space, error)
	ListByUserID(ctx context.Context, userID string) ([]models.Space, error)
	ListForAdmin(ctx context.Context, params ListAdminSpacesParams) ([]AdminSpaceListRecord, int64, error)
	UpdateVisibility(ctx context.Context, spaceID string, visibility models.Visibility) (*models.Space, error)
	UpdateStatus(ctx context.Context, params UpdateSpaceStatusParams) (bool, error)
	UpdateMetadata(ctx context.Context, params UpdateSpaceMetadataParams) (bool, error)
	SoftDelete(ctx context.Context, spaceID string, deletedAt time.Time) (bool, error)
	HasReaderAccess(ctx context.Context, spaceID string, userID string) (bool, error)
}

// ListAdminSpacesParams 管理后台空间分页查询参数。
type ListAdminSpacesParams struct {
	ActorUserID      string
	RestrictToScopes bool
	Keyword          string
	Statuses         []models.EntityStatus
	Visibilities     []models.Visibility
	Limit            int
	Offset           int
}

// AdminSpaceListRecord 管理后台空间列表项。
type AdminSpaceListRecord struct {
	Space      models.Space
	OwnerName  string
	OwnerEmail string
}

// UpdateSpaceMetadataParams 管理后台空间元数据更新参数。
type UpdateSpaceMetadataParams struct {
	SpaceID    string
	Name       *string
	Visibility *models.Visibility
	UpdatedAt  time.Time
}

// UpdateSpaceStatusParams 管理后台空间状态更新参数。
type UpdateSpaceStatusParams struct {
	SpaceID      string
	Status       models.EntityStatus
	BannedReason string
	BannedAt     *time.Time
	UpdatedAt    time.Time
}

// DocumentAccessInfo 聚合文档及所属空间访问控制所需元数据。
type DocumentAccessInfo struct {
	Document          models.Document
	SpaceID           string
	SpaceName         string
	SpaceVisibility   models.Visibility
	SpaceStatus       models.EntityStatus
	SpaceBannedAt     *time.Time
	SpaceDeletedAt    *time.Time
	SpaceOwnerUserID  string
	SpaceBannedReason string
}

// ListAdminDocumentsParams 管理后台文档分页查询参数。
type ListAdminDocumentsParams struct {
	ActorUserID      string
	RestrictToScopes bool
	Keyword          string
	SpaceID          string
	Statuses         []models.EntityStatus
	Visibilities     []models.Visibility
	Limit            int
	Offset           int
}

// AdminDocumentListRecord 管理后台文档列表项。
type AdminDocumentListRecord struct {
	Document        models.Document
	SpaceID         string
	SpaceName       string
	SpaceOwnerID    string
	SpaceOwnerName  string
	SpaceOwnerEmail string
}

// UpdateDocumentStatusParams 管理后台文档状态更新参数。
type UpdateDocumentStatusParams struct {
	DocumentID   string
	Status       models.EntityStatus
	BannedReason string
	BannedAt     *time.Time
	UpdatedAt    time.Time
}

// NodeRepository 节点仓储最小接口。
type NodeRepository interface {
	Create(ctx context.Context, node *models.Node) error
	GetByNodeID(ctx context.Context, nodeID string) (*models.Node, error)
	ListBySpaceID(ctx context.Context, spaceID string) ([]models.Node, error)
	DeleteByNodeID(ctx context.Context, nodeID string) error
}

// DocumentRepository 文档仓储最小接口。
type DocumentRepository interface {
	Create(ctx context.Context, document *models.Document) error
	GetByDocumentID(ctx context.Context, documentID string) (*models.Document, error)
	GetAccessByDocumentID(ctx context.Context, documentID string) (*DocumentAccessInfo, error)
	ListForAdmin(ctx context.Context, params ListAdminDocumentsParams) ([]AdminDocumentListRecord, int64, error)
	UpdateTheme(ctx context.Context, documentID string, themeID string) (*models.Document, error)
	UpdateVisibility(ctx context.Context, documentID string, visibility models.Visibility) (*models.Document, error)
	UpdateStatus(ctx context.Context, params UpdateDocumentStatusParams) (bool, error)
	SoftDelete(ctx context.Context, documentID string, deletedAt time.Time) (bool, error)
	UpdateWithVersion(ctx context.Context, document *models.Document, baseVersion int) (bool, error)
}

// RevisionRepository 文档修订仓储最小接口。
type RevisionRepository interface {
	Create(ctx context.Context, revision *models.DocumentRevision) error
	ListByDocumentID(ctx context.Context, documentID string) ([]models.DocumentRevision, error)
}

// ThemeRepository 主题仓储最小接口。
type ThemeRepository interface {
	List(ctx context.Context, includeDisabled bool) ([]models.Theme, error)
	GetByThemeID(ctx context.Context, themeID string) (*models.Theme, error)
	Create(ctx context.Context, theme *models.Theme) error
	Update(ctx context.Context, params UpdateThemeParams) (bool, error)
	Delete(ctx context.Context, themeID string) (bool, error)
	CountDocumentReferences(ctx context.Context, themeID string) (int64, error)
}

// UpdateThemeParams 后台主题更新参数。
type UpdateThemeParams struct {
	ThemeID                string
	Name                   *string
	Description            *string
	VariablesJSON          *string
	SyntaxTheme            *string
	CodeBlockStyleJSON     *string
	CodeBlockCodeStyleJSON *string
	InlineCodeStyleJSON    *string
	CustomCSS              *string
	IsEnabled              *bool
	UpdatedAt              time.Time
}

// UpdateSystemConfigByVersionParams 系统配置按版本更新参数。
type UpdateSystemConfigByVersionParams struct {
	ConfigKey       string
	ConfigValueJSON string
	ExpectedVersion int
	NextVersion     int
	UpdatedByUserID *string
	UpdatedAt       time.Time
}

// SystemConfigRepository 系统配置仓储接口。
type SystemConfigRepository interface {
	List(ctx context.Context) ([]models.SystemConfig, error)
	GetByConfigKey(ctx context.Context, configKey string) (*models.SystemConfig, error)
	Create(ctx context.Context, config *models.SystemConfig) error
	UpdateByVersion(ctx context.Context, params UpdateSystemConfigByVersionParams) (bool, error)
}

// ListAuditLogsParams 后台审计日志分页查询参数。
type ListAuditLogsParams struct {
	ActorUserID     string
	Keyword         string
	Module          string
	Action          string
	TargetType      string
	TargetID        string
	RequestID       string
	CreatedAtFrom   *time.Time
	CreatedAtTo     *time.Time
	RestrictModules []string
	Limit           int
	Offset          int
}

// AdminAuditLogListRecord 后台审计日志列表项。
type AdminAuditLogListRecord struct {
	AuditLog   models.AuditLog
	ActorName  string
	ActorEmail string
}

// AuditLogRepository 审计日志仓储接口。
type AuditLogRepository interface {
	Create(ctx context.Context, auditLog *models.AuditLog) error
	List(ctx context.Context, params ListAuditLogsParams) ([]AdminAuditLogListRecord, int64, error)
}
