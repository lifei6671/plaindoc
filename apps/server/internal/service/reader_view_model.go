package service

import "github.com/lifei6671/plaindoc/apps/server/internal/storage/models"

// ReaderTreeNodeViewModel 表示阅读页目录树节点。
type ReaderTreeNodeViewModel struct {
	ID         string                    `json:"id"`
	DocumentID *string                   `json:"documentId,omitempty"`
	ParentID   *string                   `json:"parentId,omitempty"`
	Type       models.NodeType           `json:"type"`
	Title      string                    `json:"title"`
	Sort       int                       `json:"sort"`
	Visibility *models.Visibility        `json:"visibility,omitempty"`
	Children   []ReaderTreeNodeViewModel `json:"children"`
}

// ReaderDocumentViewModel 表示阅读页正文数据。
type ReaderDocumentViewModel struct {
	ID         string            `json:"id"`
	NodeID     string            `json:"nodeId"`
	ThemeID    string            `json:"themeId"`
	Visibility models.Visibility `json:"visibility"`
	Title      string            `json:"title"`
	ContentMD  string            `json:"contentMd"`
	Version    int               `json:"version"`
	UpdatedAt  string            `json:"updatedAt"`
}

// ReaderSpaceViewModel 表示阅读页空间元信息。
type ReaderSpaceViewModel struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

// ReaderPageViewModel 表示阅读页 SSR 所需完整输入。
type ReaderPageViewModel struct {
	Space       ReaderSpaceViewModel      `json:"space"`
	Document    ReaderDocumentViewModel   `json:"document"`
	Tree        []ReaderTreeNodeViewModel `json:"tree"`
	ActiveDocID string                    `json:"activeDocId"`
}
