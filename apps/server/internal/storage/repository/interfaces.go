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
	UpdateVisibility(ctx context.Context, spaceID string, visibility models.Visibility) (*models.Space, error)
	HasReaderAccess(ctx context.Context, spaceID string, userID string) (bool, error)
}

// DocumentAccessInfo 聚合文档及所属空间访问控制所需元数据。
type DocumentAccessInfo struct {
	Document          models.Document
	SpaceID           string
	SpaceVisibility   models.Visibility
	SpaceStatus       models.EntityStatus
	SpaceBannedAt     *time.Time
	SpaceDeletedAt    *time.Time
	SpaceOwnerUserID  string
	SpaceBannedReason string
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
	UpdateTheme(ctx context.Context, documentID string, themeID string) (*models.Document, error)
	UpdateVisibility(ctx context.Context, documentID string, visibility models.Visibility) (*models.Document, error)
	UpdateWithVersion(ctx context.Context, document *models.Document, baseVersion int) (bool, error)
}

// RevisionRepository 文档修订仓储最小接口。
type RevisionRepository interface {
	Create(ctx context.Context, revision *models.DocumentRevision) error
	ListByDocumentID(ctx context.Context, documentID string) ([]models.DocumentRevision, error)
}

// ThemeRepository 主题仓储最小接口。
type ThemeRepository interface {
	List(ctx context.Context) ([]models.Theme, error)
	GetByThemeID(ctx context.Context, themeID string) (*models.Theme, error)
}
