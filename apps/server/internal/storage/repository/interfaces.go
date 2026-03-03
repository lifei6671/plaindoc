package repository

import (
	"context"
	"errors"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

var (
	// ErrInvalidSession 表示会话不存在、已吊销或刷新 token 与会话不匹配。
	ErrInvalidSession = errors.New("invalid session")
	// ErrWorkspaceMoveTargetParentNotFound 表示移动目标父节点不存在。
	ErrWorkspaceMoveTargetParentNotFound = errors.New("workspace move target parent not found")
	// ErrWorkspaceMoveTargetParentNotInSameSpace 表示目标父节点不在同一空间。
	ErrWorkspaceMoveTargetParentNotInSameSpace = errors.New("workspace move target parent not in same space")
	// ErrWorkspaceMoveCycleDetected 表示目标父节点在当前节点子树内，形成循环父子。
	ErrWorkspaceMoveCycleDetected = errors.New("workspace move cycle detected")
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

// UserIdentityRepository 外部身份绑定仓储接口。
type UserIdentityRepository interface {
	Upsert(ctx context.Context, params UpsertUserIdentityParams) (*models.UserIdentity, error)
	GetByProviderExternalID(ctx context.Context, providerID string, externalID string) (*models.UserIdentity, error)
	ListByUserID(ctx context.Context, userID string) ([]models.UserIdentity, error)
}

// UpsertUserIdentityParams 用户外部身份写入参数。
type UpsertUserIdentityParams struct {
	UserID       string
	ProviderType string
	ProviderID   string
	ExternalID   string
	LoginName    string
	LastLoginAt  *time.Time
	Now          time.Time
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

// CountPasswordResetTokensParams 密码重置令牌计数参数。
type CountPasswordResetTokensParams struct {
	UserID        string
	RequestIPHash string
	Since         time.Time
}

// InvalidatePasswordResetTokensParams 批量失效密码重置令牌参数。
type InvalidatePasswordResetTokensParams struct {
	UserID        string
	InvalidatedAt time.Time
}

// ConsumePasswordResetTokenParams 消费密码重置令牌参数。
type ConsumePasswordResetTokenParams struct {
	TokenID         string
	TokenSecretHash string
	ConsumedAt      time.Time
	Now             time.Time
}

// PasswordResetTokenRepository 密码重置令牌仓储接口。
type PasswordResetTokenRepository interface {
	Create(ctx context.Context, token *models.PasswordResetToken) error
	GetByTokenID(ctx context.Context, tokenID string) (*models.PasswordResetToken, error)
	CountRecent(ctx context.Context, params CountPasswordResetTokensParams) (int64, error)
	InvalidateActiveByUserID(ctx context.Context, params InvalidatePasswordResetTokensParams) (int64, error)
	Consume(ctx context.Context, params ConsumePasswordResetTokenParams) (*models.PasswordResetToken, error)
}

// AuthRiskStateRepository 认证风控状态仓储接口。
type AuthRiskStateRepository interface {
	GetByKey(ctx context.Context, scene string, subjectType string, subjectHash string) (*models.AuthRiskState, error)
	Create(ctx context.Context, state *models.AuthRiskState) error
	Update(ctx context.Context, state *models.AuthRiskState) error
}

// AuthCaptchaChallengeRepository 认证验证码会话仓储接口。
type AuthCaptchaChallengeRepository interface {
	GetByCaptchaID(ctx context.Context, captchaID string) (*models.AuthCaptchaChallenge, error)
	Create(ctx context.Context, challenge *models.AuthCaptchaChallenge) error
	Update(ctx context.Context, challenge *models.AuthCaptchaChallenge) error
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
	HardDelete(ctx context.Context, spaceID string) (bool, error)
	HasReaderAccess(ctx context.Context, spaceID string, userID string) (bool, error)
	HasWriterAccess(ctx context.Context, spaceID string, userID string) (bool, error)
}

// SearchVisibleDocumentRow 检索可见文档查询行。
type SearchVisibleDocumentRow struct {
	SpaceID         string
	DocumentID      string
	Title           string
	ContentMD       string
	VisibilityScope string
	MinRole         int
}

// SearchVisibleDocumentsParams 检索可见文档分页查询参数。
type SearchVisibleDocumentsParams struct {
	ActorUserID   string
	SpaceID       string
	ScopeSpaceIDs []string
	Terms         []string
	Limit         int
	Offset        int
}

// SearchVisibleDocumentIDsByCandidatesParams 候选文档可见性过滤参数。
type SearchVisibleDocumentIDsByCandidatesParams struct {
	ActorUserID          string
	SpaceID              string
	ScopeSpaceIDs        []string
	CandidateDocumentIDs []string
}

// SearchVisibilityRepository 检索可见性仓储接口。
type SearchVisibilityRepository interface {
	SearchVisibleDocuments(
		ctx context.Context,
		params SearchVisibleDocumentsParams,
	) ([]SearchVisibleDocumentRow, int64, error)
	FilterVisibleDocumentIDsByCandidates(
		ctx context.Context,
		params SearchVisibleDocumentIDsByCandidatesParams,
	) ([]string, error)
	ResolveUserRoleLevelsBySpaces(
		ctx context.Context,
		actorUserID string,
		spaceIDs []string,
	) (map[string]int, error)
	ResolveSearchScopeSpaceIDs(
		ctx context.Context,
		actorUserID string,
	) ([]string, error)
	ResolveUserRoleLevel(
		ctx context.Context,
		spaceID string,
		actorUserID string,
	) (int, error)
}

// SearchIndexSourceDocumentRecord 索引构建所需文档快照。
type SearchIndexSourceDocumentRecord struct {
	SpaceID         string
	DocumentID      string
	NodeID          string
	Title           string
	ContentMD       string
	SpaceVisibility string
	DocVisibility   string
	UpdatedAt       time.Time
}

// ListSearchIndexSourceDocumentsParams 索引源文档分页参数。
type ListSearchIndexSourceDocumentsParams struct {
	Limit  int
	Offset int
}

// ListSearchIndexSourceDocumentsBySpaceParams 按空间分页拉取索引源文档参数。
type ListSearchIndexSourceDocumentsBySpaceParams struct {
	SpaceID string
	Limit   int
	Offset  int
}

// SearchIndexSourceRepository 索引构建源数据仓储接口。
type SearchIndexSourceRepository interface {
	ListActiveDocuments(
		ctx context.Context,
		params ListSearchIndexSourceDocumentsParams,
	) ([]SearchIndexSourceDocumentRecord, error)
	GetActiveDocumentByDocumentID(
		ctx context.Context,
		documentID string,
	) (*SearchIndexSourceDocumentRecord, error)
	ListActiveDocumentsBySpaceID(
		ctx context.Context,
		params ListSearchIndexSourceDocumentsBySpaceParams,
	) ([]SearchIndexSourceDocumentRecord, error)
}

// SitemapPublicDocumentSourceRecord 表示 sitemap 公开文档源数据项。
type SitemapPublicDocumentSourceRecord struct {
	SpaceID           string
	DocumentID        string
	DocumentContentMD string
	SpaceUpdatedAt    time.Time
	DocumentUpdatedAt time.Time
}

// SitemapRepository sitemap 公开文档查询仓储接口。
type SitemapRepository interface {
	ListPublicDocuments(ctx context.Context) ([]SitemapPublicDocumentSourceRecord, error)
}

// HomeSearchDocumentMetadataRecord 首页检索文档元信息。
type HomeSearchDocumentMetadataRecord struct {
	SpaceID    string
	SpaceName  string
	DocumentID string
	Title      string
	UpdatedAt  time.Time
}

// HomeSearchRepository 首页检索数据仓储接口。
type HomeSearchRepository interface {
	ListActiveDocumentMetadataByDocumentIDs(
		ctx context.Context,
		documentIDs []string,
	) ([]HomeSearchDocumentMetadataRecord, error)
}

// ReaderPageDocumentRecord 阅读页文档详情聚合记录。
type ReaderPageDocumentRecord struct {
	DocumentID     string
	NodeID         string
	ThemeID        string
	Visibility     string
	Title          string
	ContentMD      string
	Version        int
	AuthorNickname string
	UpdatedAt      time.Time
	SpaceID        string
}

// ReaderPageTreeNodeRecord 阅读页目录树节点记录。
type ReaderPageTreeNodeRecord struct {
	NodeID             string
	DocumentID         *string
	ParentNodeID       *string
	Type               models.NodeType
	Title              string
	Sort               int
	DocumentVisibility *string
}

// ReaderPageRepository 阅读页数据仓储接口。
type ReaderPageRepository interface {
	ResolveDocumentID(
		ctx context.Context,
		spaceID string,
		rawDocumentID string,
	) (string, error)
	GetDocumentByDocumentID(
		ctx context.Context,
		documentID string,
	) (*ReaderPageDocumentRecord, error)
	ListSpaceDocumentIDs(
		ctx context.Context,
		spaceID string,
	) ([]string, error)
	ListTreeNodesBySpaceID(
		ctx context.Context,
		spaceID string,
	) ([]ReaderPageTreeNodeRecord, error)
}

// DeletedDocumentAttachmentCleanupCandidate 已删除文档附件清理候选。
type DeletedDocumentAttachmentCleanupCandidate struct {
	AttachmentID string
	BlobID       string
}

// DocumentAttachmentCleanupRepository 文档附件清理查询仓储接口。
type DocumentAttachmentCleanupRepository interface {
	ListDeletedDocumentAttachmentCandidates(
		ctx context.Context,
		batchSize int,
	) ([]DeletedDocumentAttachmentCleanupCandidate, error)
}

// DocumentImageAssetReferenceInput 文档图片引用输入项。
type DocumentImageAssetReferenceInput struct {
	StorageProvider string
	ObjectKey       string
	ObjectURL       string
}

// SyncDocumentImageAssetReferencesParams 文档图片引用同步参数。
type SyncDocumentImageAssetReferencesParams struct {
	DocumentID   string
	SpaceID      string
	ReferencedAt time.Time
	References   []DocumentImageAssetReferenceInput
}

// DocumentImageAssetCleanupCandidate 文档图片清理候选项。
type DocumentImageAssetCleanupCandidate struct {
	ID              int64
	StorageProvider string
	ObjectKey       string
}

// DocumentImageAssetLifecycleRepository 文档图片生命周期仓储接口。
type DocumentImageAssetLifecycleRepository interface {
	SyncDocumentReferences(
		ctx context.Context,
		params SyncDocumentImageAssetReferencesParams,
	) error
	ListPendingCleanupCandidates(
		ctx context.Context,
		cutoff time.Time,
		limit int,
	) ([]DocumentImageAssetCleanupCandidate, error)
	MarkDeletedDocumentReferencesPending(
		ctx context.Context,
		now time.Time,
	) error
	CountActiveReferencesByObject(
		ctx context.Context,
		storageProvider string,
		objectKey string,
	) (int64, error)
	MarkDeletedByID(
		ctx context.Context,
		id int64,
		now time.Time,
	) (int64, error)
	MarkDeletedByObject(
		ctx context.Context,
		storageProvider string,
		objectKey string,
		now time.Time,
	) (int64, error)
}

// DataRetentionRepository 数据保留清理仓储接口。
type DataRetentionRepository interface {
	DeleteAuditLogsBefore(ctx context.Context, cutoff time.Time, batchSize int) (int64, error)
	DeleteAuthCaptchaChallengesBefore(ctx context.Context, cutoff time.Time, batchSize int) (int64, error)
	DeleteAuthRiskStatesBefore(ctx context.Context, cutoff time.Time, batchSize int) (int64, error)
	DeleteUserSessionsBefore(ctx context.Context, cutoff time.Time, batchSize int) (int64, error)
}

// EnqueueSearchIndexJobParams 新增/合并检索索引任务参数。
type EnqueueSearchIndexJobParams struct {
	Provider    string
	JobType     string
	DedupeKey   string
	PayloadJSON string
	Priority    int
	NextRunAt   time.Time
}

// ClaimSearchIndexJobsParams 可执行任务拉取参数。
type ClaimSearchIndexJobsParams struct {
	Limit int
	Now   time.Time
}

// MarkSearchIndexJobRetryParams 任务重试回写参数。
type MarkSearchIndexJobRetryParams struct {
	JobID     string
	NextRunAt time.Time
	LastError string
}

// SearchIndexJobRepository 检索索引任务仓储接口。
type SearchIndexJobRepository interface {
	Enqueue(ctx context.Context, params EnqueueSearchIndexJobParams) error
	EnqueueInTx(ctx context.Context, tx *gorm.DB, params EnqueueSearchIndexJobParams) error
	ClaimRunnableJobs(ctx context.Context, params ClaimSearchIndexJobsParams) ([]models.SearchIndexJob, error)
	MarkSuccess(ctx context.Context, jobID string, finishedAt time.Time) error
	MarkRetry(ctx context.Context, params MarkSearchIndexJobRetryParams) error
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

// WorkspaceMoveNodeParams 工作区节点移动参数（支持同级重排与跨父级移动）。
type WorkspaceMoveNodeParams struct {
	NodeID             string
	TargetParentNodeID *string
	TargetIndex        int
	ActorUserID        string
	TouchSpace         string
	TouchedAt          time.Time
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
	MoveNode(ctx context.Context, params WorkspaceMoveNodeParams) error
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

// ListAdminDocumentAttachmentsParams 管理后台文档附件分页查询参数。
type ListAdminDocumentAttachmentsParams struct {
	ActorUserID      string
	RestrictToScopes bool
	Keyword          string
	SpaceID          string
	DocumentID       string
	Statuses         []models.EntityStatus
	StorageProviders []string
	Limit            int
	Offset           int
}

// AdminDocumentAttachmentListRecord 管理后台文档附件列表项。
type AdminDocumentAttachmentListRecord struct {
	Attachment      models.DocumentAttachment
	DocumentTitle   string
	DocumentStatus  models.EntityStatus
	SpaceName       string
	SpaceOwnerID    string
	SpaceOwnerName  string
	SpaceOwnerEmail string
	CreatedByName   string
	CreatedByEmail  string
}

// ListAdminDocumentImageAssetsParams 管理后台文档图片资源分页查询参数。
type ListAdminDocumentImageAssetsParams struct {
	ActorUserID      string
	RestrictToScopes bool
	Keyword          string
	SpaceID          string
	DocumentID       string
	Statuses         []string
	StorageProviders []string
	Limit            int
	Offset           int
}

// AdminDocumentImageAssetListRecord 管理后台文档图片资源列表项。
type AdminDocumentImageAssetListRecord struct {
	ImageAsset      models.DocumentImageAsset
	DocumentTitle   string
	DocumentStatus  models.EntityStatus
	SpaceName       string
	SpaceOwnerID    string
	SpaceOwnerName  string
	SpaceOwnerEmail string
}

// DocumentImageAssetReferenceRecord 表示同一图片对象的活跃引用记录。
type DocumentImageAssetReferenceRecord struct {
	ImageAssetID   string
	DocumentID     string
	DocumentTitle  string
	SpaceID        string
	SpaceName      string
	Status         string
	LastReferenced time.Time
}

// DocumentAttachmentReferenceRecord 表示同一物理文件的附件引用记录。
type DocumentAttachmentReferenceRecord struct {
	AttachmentID  string
	DocumentID    string
	DocumentTitle string
	SpaceID       string
	SpaceName     string
	FileName      string
	Status        models.EntityStatus
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
	HardDelete(ctx context.Context, documentID string) (bool, error)
	UpdateWithVersion(ctx context.Context, document *models.Document, baseVersion int) (bool, error)
}

// DocumentAttachmentRepository 文档附件仓储接口。
type DocumentAttachmentRepository interface {
	Create(ctx context.Context, attachment *models.DocumentAttachment) error
	ListByDocumentID(ctx context.Context, documentID string, includeDeleted bool) ([]models.DocumentAttachment, error)
	ListForAdmin(
		ctx context.Context,
		params ListAdminDocumentAttachmentsParams,
	) ([]AdminDocumentAttachmentListRecord, int64, error)
	FindBlobByHash(
		ctx context.Context,
		storageProvider string,
		contentHashAlgo string,
		contentHash string,
		sizeBytes int64,
	) (*models.DocumentAttachmentBlob, error)
	GetBlobByBlobID(ctx context.Context, blobID string) (*models.DocumentAttachmentBlob, error)
	ListOrphanBlobs(ctx context.Context, limit int) ([]models.DocumentAttachmentBlob, error)
	CreateBlob(ctx context.Context, blob *models.DocumentAttachmentBlob) error
	HardDeleteBlobIfUnreferenced(ctx context.Context, blobID string) (bool, error)
	CountActiveReferencesByBlobID(ctx context.Context, blobID string) (int64, error)
	ListActiveReferencesByBlobID(ctx context.Context, blobID string, limit int) ([]DocumentAttachmentReferenceRecord, error)
	GetByAttachmentID(ctx context.Context, attachmentID string) (*models.DocumentAttachment, error)
	SoftDelete(ctx context.Context, attachmentID string, deletedAt time.Time) (bool, error)
	HardDelete(ctx context.Context, attachmentID string) (bool, error)
}

// DocumentImageAssetRepository 文档图片资源仓储接口。
type DocumentImageAssetRepository interface {
	ListForAdmin(
		ctx context.Context,
		params ListAdminDocumentImageAssetsParams,
	) ([]AdminDocumentImageAssetListRecord, int64, error)
	GetByImageAssetID(ctx context.Context, imageAssetID string) (*models.DocumentImageAsset, error)
	SoftDelete(ctx context.Context, imageAssetID string, deletedAt time.Time) (bool, error)
	HardDelete(ctx context.Context, imageAssetID string) (bool, error)
	CountActiveReferencesByObject(
		ctx context.Context,
		storageProvider string,
		objectKey string,
	) (int64, error)
	ListActiveReferencesByObject(
		ctx context.Context,
		storageProvider string,
		objectKey string,
		limit int,
	) ([]DocumentImageAssetReferenceRecord, error)
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

// ListSearchAnalyzerDictEntriesParams 分词词典词条分页查询参数。
type ListSearchAnalyzerDictEntriesParams struct {
	Analyzer string
	Statuses []string
	Limit    int
	Offset   int
}

// SearchAnalyzerDictEntryRepository 分词词典词条仓储接口。
type SearchAnalyzerDictEntryRepository interface {
	List(ctx context.Context, params ListSearchAnalyzerDictEntriesParams) ([]models.SearchAnalyzerDictEntry, int64, error)
	ListActiveByAnalyzer(ctx context.Context, analyzer string) ([]models.SearchAnalyzerDictEntry, error)
	GetByID(ctx context.Context, id int64) (*models.SearchAnalyzerDictEntry, error)
	GetByAnalyzerAndTerm(ctx context.Context, analyzer string, term string) (*models.SearchAnalyzerDictEntry, error)
	Create(ctx context.Context, entry *models.SearchAnalyzerDictEntry) error
	UpdateByID(ctx context.Context, id int64, updates map[string]any) (bool, error)
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
