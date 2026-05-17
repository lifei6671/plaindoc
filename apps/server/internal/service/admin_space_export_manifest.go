package service

const (
	AdminSpaceExportPackageType    = "plaindoc-space"
	AdminSpaceExportPackageVersion = 1
)

// AdminSpaceExportManifest 描述 zip 空间交换包的顶层元数据。
type AdminSpaceExportManifest struct {
	Version     int                             `json:"version"`
	PackageType string                          `json:"packageType"`
	ExportedAt  string                          `json:"exportedAt"`
	Format      AdminSpaceExportFormat          `json:"format"`
	Importable  bool                            `json:"importable"`
	Space       AdminSpaceExportManifestSpace   `json:"space"`
	Summary     AdminSpaceExportSummary         `json:"summary"`
	Documents   []AdminSpaceExportDocumentEntry `json:"documents"`
}

// AdminSpaceExportManifestSpace 记录源空间基础信息，导入时只作为新空间默认值。
type AdminSpaceExportManifestSpace struct {
	SpaceID     string                      `json:"spaceId"`
	Name        string                      `json:"name"`
	Description string                      `json:"description,omitempty"`
	CategoryID  string                      `json:"categoryId,omitempty"`
	Visibility  string                      `json:"visibility"`
	Cover       *AdminSpaceExportCoverEntry `json:"cover,omitempty"`
}

// AdminSpaceExportCoverEntry 记录空间封面在交换包中的文件引用和元数据。
type AdminSpaceExportCoverEntry struct {
	AssetID    string `json:"assetId,omitempty"`
	Path       string `json:"path"`
	FileName   string `json:"fileName,omitempty"`
	MimeType   string `json:"mimeType,omitempty"`
	SizeBytes  int64  `json:"sizeBytes,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Source     string `json:"source,omitempty"`
	Normalized bool   `json:"normalized,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
}

// AdminSpaceExportSummary 记录导出包内容统计。
type AdminSpaceExportSummary struct {
	FolderCount       int `json:"folderCount"`
	DocumentCount     int `json:"documentCount"`
	AttachmentCount   int `json:"attachmentCount"`
	OfficeSourceCount int `json:"officeSourceCount"`
	ImageCount        int `json:"imageCount,omitempty"`
	MaxDepth          int `json:"maxDepth,omitempty"`
}

// AdminSpaceExportManifestSummary 保留给导入预览响应使用的兼容别名。
type AdminSpaceExportManifestSummary = AdminSpaceExportSummary

// AdminSpaceExportDocumentEntry 记录单个文档在 zip 中的文件引用。
type AdminSpaceExportDocumentEntry struct {
	DocumentID        string                            `json:"documentId"`
	NodeID            string                            `json:"nodeId"`
	ParentNodeID      string                            `json:"parentNodeId,omitempty"`
	Title             string                            `json:"title"`
	Format            string                            `json:"format"`
	Sort              int                               `json:"sort"`
	Visibility        string                            `json:"visibility"`
	Path              string                            `json:"path"`
	ContentSHA256     string                            `json:"contentSha256,omitempty"`
	Attachments       []string                          `json:"attachments"`
	AttachmentEntries []AdminSpaceExportAttachmentEntry `json:"attachmentEntries,omitempty"`
	Source            *AdminSpaceExportSourceEntry      `json:"source"`
}

// AdminSpaceExportAttachmentEntry 记录附件在 zip 中的文件引用和原始元数据。
type AdminSpaceExportAttachmentEntry struct {
	AttachmentID string `json:"attachmentId,omitempty"`
	Path         string `json:"path"`
	FileName     string `json:"fileName,omitempty"`
	MimeType     string `json:"mimeType,omitempty"`
	SizeBytes    int64  `json:"sizeBytes,omitempty"`
	SHA256       string `json:"sha256,omitempty"`
}

// AdminSpaceExportSourceEntry 是 Office 源文件占位或引用。
type AdminSpaceExportSourceEntry struct {
	Path     string `json:"path"`
	Included bool   `json:"included"`
	SHA256   string `json:"sha256,omitempty"`
}

// AdminSpaceExportTree 保存原始目录树和节点关系。
type AdminSpaceExportTree struct {
	Version int                        `json:"version"`
	Root    []AdminSpaceExportTreeNode `json:"root"`
}

// AdminSpaceExportTreeNode 描述空间目录树节点。
type AdminSpaceExportTreeNode struct {
	NodeID       string                     `json:"nodeId"`
	DocumentID   string                     `json:"documentId,omitempty"`
	ParentNodeID *string                    `json:"parentNodeId"`
	Type         string                     `json:"type"`
	Title        string                     `json:"title"`
	Sort         int                        `json:"sort"`
	Format       string                     `json:"format,omitempty"`
	Children     []AdminSpaceExportTreeNode `json:"children,omitempty"`
}
