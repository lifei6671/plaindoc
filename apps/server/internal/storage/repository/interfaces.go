package repository

import (
	"context"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

// UserRepository 用户仓储最小接口，服务层可按需扩展。
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByULID(ctx context.Context, ulid string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
}

// SpaceRepository 空间仓储最小接口。
type SpaceRepository interface {
	Create(ctx context.Context, space *models.Space) error
	GetByULID(ctx context.Context, ulid string) (*models.Space, error)
	ListByUserULID(ctx context.Context, userULID string) ([]models.Space, error)
}

// NodeRepository 节点仓储最小接口。
type NodeRepository interface {
	Create(ctx context.Context, node *models.Node) error
	GetByULID(ctx context.Context, ulid string) (*models.Node, error)
	ListBySpaceULID(ctx context.Context, spaceULID string) ([]models.Node, error)
	DeleteByULID(ctx context.Context, ulid string) error
}

// DocumentRepository 文档仓储最小接口。
type DocumentRepository interface {
	Create(ctx context.Context, document *models.Document) error
	GetByULID(ctx context.Context, ulid string) (*models.Document, error)
	UpdateWithVersion(ctx context.Context, document *models.Document, baseVersion int) (bool, error)
}

// RevisionRepository 文档修订仓储最小接口。
type RevisionRepository interface {
	Create(ctx context.Context, revision *models.DocumentRevision) error
	ListByDocumentULID(ctx context.Context, documentULID string) ([]models.DocumentRevision, error)
}
