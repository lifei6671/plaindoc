package repository

import (
	"context"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

// UserRepository 用户仓储最小接口，服务层可按需扩展。
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByUserID(ctx context.Context, userID string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
}

// SpaceRepository 空间仓储最小接口。
type SpaceRepository interface {
	Create(ctx context.Context, space *models.Space) error
	GetBySpaceID(ctx context.Context, spaceID string) (*models.Space, error)
	ListByUserID(ctx context.Context, userID string) ([]models.Space, error)
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
	UpdateWithVersion(ctx context.Context, document *models.Document, baseVersion int) (bool, error)
}

// RevisionRepository 文档修订仓储最小接口。
type RevisionRepository interface {
	Create(ctx context.Context, revision *models.DocumentRevision) error
	ListByDocumentID(ctx context.Context, documentID string) ([]models.DocumentRevision, error)
}
