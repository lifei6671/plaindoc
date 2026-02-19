package models

import "time"

type Role string

const (
	RoleOwner        Role = "owner"
	RoleCollaborator Role = "collaborator"
	RoleReader       Role = "reader"
)

type NodeType string

const (
	NodeTypeFolder NodeType = "folder"
	NodeTypeDoc    NodeType = "doc"
)

type RevisionSource string

const (
	RevisionSourceLocal  RevisionSource = "local"
	RevisionSourceRemote RevisionSource = "remote"
)

// User 对应 users 表。
type User struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ULID         string    `gorm:"column:ulid"`
	Email        string    `gorm:"column:email"`
	PasswordHash string    `gorm:"column:password_hash"`
	Name         string    `gorm:"column:name"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

// Space 对应 spaces 表。
type Space struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ULID      string    `gorm:"column:ulid"`
	Name      string    `gorm:"column:name"`
	OwnerULID string    `gorm:"column:owner_ulid"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// SpaceMember 对应 space_members 表。
type SpaceMember struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SpaceULID string    `gorm:"column:space_ulid"`
	UserULID  string    `gorm:"column:user_ulid"`
	Role      Role      `gorm:"column:role"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// Node 对应 nodes 表。
type Node struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ULID       string    `gorm:"column:ulid"`
	SpaceULID  string    `gorm:"column:space_ulid"`
	ParentULID *string   `gorm:"column:parent_ulid"`
	Type       NodeType  `gorm:"column:type"`
	Title      string    `gorm:"column:title"`
	Sort       int       `gorm:"column:sort"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

// Document 对应 documents 表。
type Document struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ULID          string    `gorm:"column:ulid"`
	NodeULID      string    `gorm:"column:node_ulid"`
	Title         string    `gorm:"column:title"`
	ContentMD     string    `gorm:"column:content_md"`
	Version       int       `gorm:"column:version"`
	UpdatedByULID *string   `gorm:"column:updated_by_ulid"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

// DocumentRevision 对应 document_revisions 表。
type DocumentRevision struct {
	ID           int64          `gorm:"column:id;primaryKey;autoIncrement"`
	ULID         string         `gorm:"column:ulid"`
	DocumentULID string         `gorm:"column:document_ulid"`
	Version      int            `gorm:"column:version"`
	ContentMD    string         `gorm:"column:content_md"`
	BaseVersion  int            `gorm:"column:base_version"`
	EditorULID   *string        `gorm:"column:editor_ulid"`
	Source       RevisionSource `gorm:"column:source"`
	CreatedAt    time.Time      `gorm:"column:created_at"`
}

// NodePermission 对应 node_permissions 表。
type NodePermission struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`
	NodeULID      string    `gorm:"column:node_ulid"`
	UserULID      string    `gorm:"column:user_ulid"`
	Role          Role      `gorm:"column:role"`
	GrantedByULID string    `gorm:"column:granted_by_ulid"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

// DocumentPermission 对应 document_permissions 表。
type DocumentPermission struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`
	DocumentULID  string    `gorm:"column:document_ulid"`
	UserULID      string    `gorm:"column:user_ulid"`
	Role          Role      `gorm:"column:role"`
	GrantedByULID string    `gorm:"column:granted_by_ulid"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (User) TableName() string {
	return "users"
}

func (Space) TableName() string {
	return "spaces"
}

func (SpaceMember) TableName() string {
	return "space_members"
}

func (Node) TableName() string {
	return "nodes"
}

func (Document) TableName() string {
	return "documents"
}

func (DocumentRevision) TableName() string {
	return "document_revisions"
}

func (NodePermission) TableName() string {
	return "node_permissions"
}

func (DocumentPermission) TableName() string {
	return "document_permissions"
}
