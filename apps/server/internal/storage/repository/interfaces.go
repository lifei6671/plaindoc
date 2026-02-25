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
	UpdateProfile(ctx context.Context, params UpdateUserProfileParams) (bool, error)
	UpdatePassword(ctx context.Context, userID string, passwordHash string, updatedAt time.Time) (bool, error)
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

// UpdateUserProfileParams 用户资料更新参数。
type UpdateUserProfileParams struct {
	UserID    string
	Name      *string
	AvatarURL *string
	UpdatedAt time.Time
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
	ListByUserIDs(ctx context.Context, userIDs []string) (map[string][]models.AdminRole, error)
	ReplaceByUserID(ctx context.Context, userID string, roles []models.AdminRole) error
}

// SpaceAdminScopeRepository 空间管理范围仓储接口。
type SpaceAdminScopeRepository interface {
	HasScope(ctx context.Context, userID string, spaceID string) (bool, error)
	UpsertScope(ctx context.Context, userID string, spaceID string) error
	DeleteScope(ctx context.Context, userID string, spaceID string) error
	ListByUserID(ctx context.Context, userID string) ([]string, error)
}

// SpaceCategoryRepository 空间分类仓储接口。
type SpaceCategoryRepository interface {
	List(ctx context.Context) ([]models.SpaceCategory, error)
	GetByCategoryID(ctx context.Context, categoryID string) (*models.SpaceCategory, error)
	GetByName(ctx context.Context, name string) (*models.SpaceCategory, error)
	GetDefault(ctx context.Context) (*models.SpaceCategory, error)
	Create(ctx context.Context, category *models.SpaceCategory) error
	RenameAndSyncSpaces(ctx context.Context, categoryID string, name string, updatedAt time.Time) (bool, error)
	DeleteAndReassignSpaces(
		ctx context.Context,
		categoryID string,
		fallbackCategoryID string,
		fallbackCategoryName string,
		updatedAt time.Time,
	) (int64, bool, error)
}

// SpaceRepository 空间仓储最小接口。
type SpaceRepository interface {
	Create(ctx context.Context, space *models.Space) error
	GetBySpaceID(ctx context.Context, spaceID string) (*models.Space, error)
	GetCoverAssetByAssetID(ctx context.Context, assetID string) (*models.SpaceCoverAsset, error)
	ListByUserID(ctx context.Context, userID string) ([]models.Space, error)
	ListVisibleForHomepage(
		ctx context.Context,
		params ListVisibleHomepageSpacesParams,
	) ([]HomepageVisibleSpaceRecord, int64, error)
	ListForAdmin(ctx context.Context, params ListAdminSpacesParams) ([]AdminSpaceListRecord, int64, error)
	ListMembers(ctx context.Context, spaceID string) ([]SpaceMemberListRecord, error)
	UpsertMember(ctx context.Context, params UpsertSpaceMemberParams) error
	UpdateMemberRole(ctx context.Context, params UpdateSpaceMemberRoleParams) (bool, error)
	DeleteMember(ctx context.Context, spaceID string, userID string) (bool, error)
	CreateCoverAsset(ctx context.Context, asset *models.SpaceCoverAsset) error
	UpdateVisibility(ctx context.Context, spaceID string, visibility models.Visibility) (*models.Space, error)
	UpdateStatus(ctx context.Context, params UpdateSpaceStatusParams) (bool, error)
	UpdateMetadata(ctx context.Context, params UpdateSpaceMetadataParams) (bool, error)
	IsMember(ctx context.Context, spaceID string, userID string) (bool, error)
	TransferOwnership(ctx context.Context, spaceID string, fromUserID string, toUserID string, updatedAt time.Time) (bool, error)
	SoftDelete(ctx context.Context, spaceID string, deletedAt time.Time) (bool, error)
	HasReaderAccess(ctx context.Context, spaceID string, userID string) (bool, error)
	HasWriterAccess(ctx context.Context, spaceID string, userID string) (bool, error)
}

// WorkspaceSpaceListRecord 编辑器工作区空间列表项。
type WorkspaceSpaceListRecord struct {
	SpaceID      string
	Name         string
	CreatedAtRaw string
	UpdatedAtRaw string
}

// WorkspaceSpacePermissionSnapshot 工作区空间权限判断快照。
type WorkspaceSpacePermissionSnapshot struct {
	SpaceID            string
	OwnerUserID        string
	Visibility         models.Visibility
	Status             models.EntityStatus
	DeletedAt          *time.Time
	IsPlatformAdmin    bool
	HasSpaceAdminScope bool
	MemberRole         *models.Role
}

// WorkspaceTreeNodeRecord 工作区目录树节点记录。
type WorkspaceTreeNodeRecord struct {
	NodeID             string
	DocumentID         *string
	SpaceID            string
	ParentNodeID       *string
	Type               models.NodeType
	Title              string
	Sort               int
	DocumentVisibility *string
}

// WorkspaceNodeRecord 工作区节点记录。
type WorkspaceNodeRecord struct {
	NodeID       string
	SpaceID      string
	ParentNodeID *string
	Type         models.NodeType
	Title        string
	Sort         int
}

// WorkspaceDocumentRecord 工作区文档记录。
type WorkspaceDocumentRecord struct {
	DocumentID   string
	NodeID       string
	ThemeID      string
	Title        string
	ContentMD    string
	Version      int
	SpaceID      string
	UpdatedAtRaw string
}

// WorkspaceRevisionRecord 工作区文档修订记录。
type WorkspaceRevisionRecord struct {
	DocumentRevisionID string
	DocumentID         string
	Version            int
	ContentMD          string
	BaseVersion        int
	Source             models.RevisionSource
	CreatedAtRaw       string
}

// WorkspaceCreateNodeParams 工作区创建节点参数。
type WorkspaceCreateNodeParams struct {
	Node       *models.Node
	Document   *models.Document
	Revision   *models.DocumentRevision
	TouchSpace string
	TouchedAt  time.Time
}

// WorkspaceUpdateNodeParams 工作区更新节点参数。
type WorkspaceUpdateNodeParams struct {
	NodeID        string
	UpdateValues  map[string]any
	DocumentTitle *string
	ActorUserID   string
	TouchSpace    string
	TouchedAt     time.Time
}

// WorkspaceSaveDocumentParams 工作区保存文档参数。
type WorkspaceSaveDocumentParams struct {
	DocumentID  string
	BaseVersion int
	NextVersion int
	ContentMD   string
	ActorUserID string
	NodeID      string
	SpaceID     string
	TouchedAt   time.Time
	Revision    *models.DocumentRevision
}

// WorkspaceRepository 编辑器工作区仓储接口。
type WorkspaceRepository interface {
	ListSpacesByActor(ctx context.Context, actorUserID string) ([]WorkspaceSpaceListRecord, error)
	GetDefaultCategory(ctx context.Context) (*models.SpaceCategory, error)
	CreateSpace(ctx context.Context, space *models.Space) error
	GetSpacePermissionSnapshot(
		ctx context.Context,
		spaceID string,
		actorUserID string,
	) (*WorkspaceSpacePermissionSnapshot, error)
	ListTreeNodesBySpaceID(ctx context.Context, spaceID string) ([]WorkspaceTreeNodeRecord, error)
	GetNodeByNodeID(ctx context.Context, nodeID string) (*WorkspaceNodeRecord, error)
	GetMaxNodeSort(ctx context.Context, spaceID string, parentNodeID *string) (int, error)
	CreateNode(ctx context.Context, params WorkspaceCreateNodeParams) error
	UpdateNode(ctx context.Context, params WorkspaceUpdateNodeParams) error
	DeleteNode(ctx context.Context, nodeID string, touchSpace string, touchedAt time.Time) (bool, error)
	GetDocumentByDocumentID(ctx context.Context, documentID string) (*WorkspaceDocumentRecord, error)
	SaveDocument(ctx context.Context, params WorkspaceSaveDocumentParams) (bool, error)
	ListRevisionsByDocumentID(ctx context.Context, documentID string) ([]WorkspaceRevisionRecord, error)
}

// ListVisibleHomepageSpacesParams 首页/分类页可见空间查询参数。
type ListVisibleHomepageSpacesParams struct {
	ViewerUserID string
	CategoryID   string
	Limit        int
	Offset       int
}

// HomepageVisibleSpaceRecord 首页/分类页可见空间列表项。
type HomepageVisibleSpaceRecord struct {
	Space          models.Space
	OwnerName      string
	OwnerAvatarURL string
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
	Space         models.Space
	CategoryID    string
	CategoryName  string
	CategoryIsDef bool
	OwnerName     string
	OwnerEmail    string
}

// SpaceMemberListRecord 空间成员列表项。
type SpaceMemberListRecord struct {
	UserID    string
	Email     string
	Name      string
	Role      models.Role
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UpdateSpaceMetadataParams 管理后台空间元数据更新参数（包含描述与封面字段的可选更新）。
type UpdateSpaceMetadataParams struct {
	SpaceID      string
	Name         *string
	Description  *string
	CategoryID   *string
	Category     *string
	Visibility   *models.Visibility
	CoverAssetID *string
	CoverKey     *string
	CoverURL     *string
	CoverWidth   *int
	CoverHeight  *int
	CoverSource  *string
	UpdatedAt    time.Time
}

// UpsertSpaceMemberParams 空间成员新增/更新参数。
type UpsertSpaceMemberParams struct {
	SpaceID   string
	UserID    string
	Role      models.Role
	UpdatedAt time.Time
}

// UpdateSpaceMemberRoleParams 空间成员角色更新参数。
type UpdateSpaceMemberRoleParams struct {
	SpaceID   string
	UserID    string
	Role      models.Role
	UpdatedAt time.Time
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
