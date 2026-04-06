package models

import "time"

// Node 对应 nodes 表。
type Node struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	NodeID          string    `gorm:"column:node_id"`
	SpaceID         string    `gorm:"column:space_id"`
	ParentNodeID    *string   `gorm:"column:parent_node_id"`
	ReaderSlug      *string   `gorm:"column:reader_slug"`
	Type            NodeType  `gorm:"column:type"`
	Title           string    `gorm:"column:title"`
	Sort            int       `gorm:"column:sort"`
	CreatedByUserID *string   `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string   `gorm:"column:updated_by_user_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (Node) TableName() string {
	return "nodes"
}

var NodeColumns = struct {
	ID              string
	NodeID          string
	SpaceID         string
	ParentNodeID    string
	ReaderSlug      string
	Type            string
	Title           string
	Sort            string
	CreatedByUserID string
	UpdatedByUserID string
	CreatedAt       string
	UpdatedAt       string
}{
	ID:              "id",
	NodeID:          "node_id",
	SpaceID:         "space_id",
	ParentNodeID:    "parent_node_id",
	ReaderSlug:      "reader_slug",
	Type:            "type",
	Title:           "title",
	Sort:            "sort",
	CreatedByUserID: "created_by_user_id",
	UpdatedByUserID: "updated_by_user_id",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}
