package service

import "github.com/lifei6671/plaindoc/apps/server/internal/storage/models"

// ReaderTreeNodeViewModel 表示阅读页目录树节点。
type ReaderTreeNodeViewModel struct {
	ID                 string                    `json:"id"`
	DocumentID         *string                   `json:"documentId,omitempty"`
	DocumentIdentifier *string                   `json:"documentIdentifier,omitempty"`
	DocumentRouteKey   *string                   `json:"documentRouteKey,omitempty"`
	DocumentFormat     *models.DocumentFormat    `json:"documentFormat,omitempty"`
	ParentID           *string                   `json:"parentId,omitempty"`
	Type               models.NodeType           `json:"type"`
	Title              string                    `json:"title"`
	Sort               int                       `json:"sort"`
	Visibility         *models.Visibility        `json:"visibility,omitempty"`
	Children           []ReaderTreeNodeViewModel `json:"children"`
}

// ReaderDocumentViewModel 表示阅读页正文数据。
type ReaderDocumentViewModel struct {
	ID             string                `json:"id"`
	NodeID         string                `json:"nodeId"`
	Identifier     string                `json:"identifier,omitempty"`
	RouteKey       string                `json:"routeKey"`
	ThemeID        string                `json:"themeId"`
	Format         models.DocumentFormat `json:"format"`
	Visibility     models.Visibility     `json:"visibility"`
	Title          string                `json:"title"`
	ContentMD      string                `json:"contentMd"`
	Version        int                   `json:"version"`
	SourceBlobID   *string               `json:"sourceBlobId,omitempty"`
	SourceFileName *string               `json:"sourceFileName,omitempty"`
	SourceMimeType *string               `json:"sourceMimeType,omitempty"`
	ContentVersion int                   `json:"contentVersion"`
	AuthorNickname string                `json:"authorNickname"`
	UpdatedAt      string                `json:"updatedAt"`
}

// ReaderDocumentAttachmentViewModel 表示阅读页附件元数据。
type ReaderDocumentAttachmentViewModel struct {
	AttachmentID     string `json:"attachmentId"`
	DocumentID       string `json:"documentId"`
	FileName         string `json:"fileName"`
	MimeType         string `json:"mimeType"`
	SizeBytes        int64  `json:"sizeBytes"`
	PreviewKind      string `json:"previewKind"`
	PreviewSupported bool   `json:"previewSupported"`
}

// ReaderSpaceViewModel 表示阅读页空间元信息。
type ReaderSpaceViewModel struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

// ReaderPageViewModel 表示阅读页 SSR 所需完整输入。
type ReaderPageViewModel struct {
	Space       ReaderSpaceViewModel                `json:"space"`
	Document    ReaderDocumentViewModel             `json:"document"`
	Attachments []ReaderDocumentAttachmentViewModel `json:"attachments"`
	Tree        []ReaderTreeNodeViewModel           `json:"tree"`
	ActiveDocID string                              `json:"activeDocId"`
}
