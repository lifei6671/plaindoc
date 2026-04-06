package repository

import (
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

type auditLogListRowDB struct {
	ID           int64   `gorm:"column:id"`
	ActorUserID  *string `gorm:"column:actor_user_id"`
	Module       string  `gorm:"column:module"`
	Action       string  `gorm:"column:action"`
	TargetType   string  `gorm:"column:target_type"`
	TargetID     string  `gorm:"column:target_id"`
	Summary      string  `gorm:"column:summary"`
	DetailJSON   string  `gorm:"column:detail_json"`
	RequestID    string  `gorm:"column:request_id"`
	CreatedAtRaw string  `gorm:"column:created_at_raw"`
	ActorName    *string `gorm:"column:actor_name"`
	ActorEmail   *string `gorm:"column:actor_email"`
}

type captchaStoreRowDB struct {
	CaptchaID    string `gorm:"column:captcha_id"`
	AnswerHash   string `gorm:"column:answer_hash"`
	ExpiresAtRaw string `gorm:"column:expires_at_raw"`
	CreatedAtRaw string `gorm:"column:created_at_raw"`
	UpdatedAtRaw string `gorm:"column:updated_at_raw"`
}

type authCaptchaChallengeRowDB struct {
	ID                int64   `gorm:"column:id"`
	CaptchaID         string  `gorm:"column:captcha_id"`
	Scene             string  `gorm:"column:scene"`
	SubjectHash       string  `gorm:"column:subject_hash"`
	Level             int     `gorm:"column:level"`
	AnswerHash        string  `gorm:"column:answer_hash"`
	AnswerSalt        string  `gorm:"column:answer_salt"`
	IssuedIPHash      string  `gorm:"column:issued_ip_hash"`
	ExpiresAtRaw      string  `gorm:"column:expires_at_raw"`
	ConsumedAtRaw     *string `gorm:"column:consumed_at_raw"`
	FailedVerifyCount int     `gorm:"column:failed_verify_count"`
	CreatedAtRaw      string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw      string  `gorm:"column:updated_at_raw"`
}

type authRiskStateRowDB struct {
	ID                 int64   `gorm:"column:id"`
	Scene              string  `gorm:"column:scene"`
	SubjectType        string  `gorm:"column:subject_type"`
	SubjectHash        string  `gorm:"column:subject_hash"`
	WindowStartedAtRaw string  `gorm:"column:window_started_at_raw"`
	AttemptCount       int     `gorm:"column:attempt_count"`
	FailedCount        int     `gorm:"column:failed_count"`
	CaptchaFailedCount int     `gorm:"column:captcha_failed_count"`
	LockUntilRaw       *string `gorm:"column:lock_until_raw"`
	CreatedAtRaw       string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw       string  `gorm:"column:updated_at_raw"`
}

type deletedDocumentAttachmentCleanupCandidateRowDB struct {
	AttachmentID string `gorm:"column:attachment_id"`
	BlobID       string `gorm:"column:blob_id"`
}

type workspaceNodeDeleteSnapshotDB struct {
	NodeID       string          `gorm:"column:node_id"`
	SpaceID      string          `gorm:"column:space_id"`
	ParentNodeID *string         `gorm:"column:parent_node_id"`
	Type         models.NodeType `gorm:"column:type"`
}

type documentAttachmentRowDB struct {
	ID              int64   `gorm:"column:id"`
	AttachmentID    string  `gorm:"column:attachment_id"`
	BlobID          string  `gorm:"column:blob_id"`
	DocumentID      string  `gorm:"column:document_id"`
	SpaceID         string  `gorm:"column:space_id"`
	StorageProvider string  `gorm:"column:storage_provider"`
	FileName        string  `gorm:"column:file_name"`
	ObjectKey       string  `gorm:"column:object_key"`
	ObjectURL       string  `gorm:"column:object_url"`
	MimeType        string  `gorm:"column:mime_type"`
	SizeBytes       int64   `gorm:"column:size_bytes"`
	ContentHashAlgo string  `gorm:"column:content_hash_algo"`
	ContentHash     string  `gorm:"column:content_hash"`
	PreviewKind     string  `gorm:"column:preview_kind"`
	Status          string  `gorm:"column:status"`
	DeletedAtRaw    *string `gorm:"column:deleted_at_raw"`
	CreatedByUserID *string `gorm:"column:created_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw    string  `gorm:"column:updated_at_raw"`
}

type documentAttachmentBlobRowDB struct {
	ID              int64   `gorm:"column:id"`
	BlobID          string  `gorm:"column:blob_id"`
	StorageProvider string  `gorm:"column:storage_provider"`
	ObjectKey       string  `gorm:"column:object_key"`
	ObjectURL       string  `gorm:"column:object_url"`
	MimeType        string  `gorm:"column:mime_type"`
	SizeBytes       int64   `gorm:"column:size_bytes"`
	ContentHashAlgo string  `gorm:"column:content_hash_algo"`
	ContentHash     string  `gorm:"column:content_hash"`
	DeletedAtRaw    *string `gorm:"column:deleted_at_raw"`
	CreatedAtRaw    string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw    string  `gorm:"column:updated_at_raw"`
}

type adminDocumentAttachmentListRowDB struct {
	ID              int64               `gorm:"column:id"`
	AttachmentID    string              `gorm:"column:attachment_id"`
	BlobID          string              `gorm:"column:blob_id"`
	DocumentID      string              `gorm:"column:document_id"`
	ReaderSlug      *string             `gorm:"column:reader_slug"`
	SpaceID         string              `gorm:"column:space_id"`
	StorageProvider string              `gorm:"column:storage_provider"`
	FileName        string              `gorm:"column:file_name"`
	ObjectKey       string              `gorm:"column:object_key"`
	ObjectURL       string              `gorm:"column:object_url"`
	MimeType        string              `gorm:"column:mime_type"`
	SizeBytes       int64               `gorm:"column:size_bytes"`
	ContentHashAlgo string              `gorm:"column:content_hash_algo"`
	ContentHash     string              `gorm:"column:content_hash"`
	PreviewKind     string              `gorm:"column:preview_kind"`
	Status          models.EntityStatus `gorm:"column:status"`
	DeletedAtRaw    *string             `gorm:"column:deleted_at_raw"`
	CreatedByUserID *string             `gorm:"column:created_by_user_id"`
	CreatedAtRaw    string              `gorm:"column:created_at_raw"`
	UpdatedAtRaw    string              `gorm:"column:updated_at_raw"`
	DocumentTitle   string              `gorm:"column:document_title"`
	DocumentStatus  models.EntityStatus `gorm:"column:document_status"`
	SpaceName       string              `gorm:"column:space_name"`
	SpaceOwnerID    string              `gorm:"column:space_owner_id"`
	SpaceOwnerName  string              `gorm:"column:space_owner_name"`
	SpaceOwnerMail  string              `gorm:"column:space_owner_mail"`
	CreatedByName   string              `gorm:"column:created_by_name"`
	CreatedByEmail  string              `gorm:"column:created_by_email"`
}

type documentAttachmentReferenceRowDB struct {
	AttachmentID  string              `gorm:"column:attachment_id"`
	DocumentID    string              `gorm:"column:document_id"`
	DocumentTitle string              `gorm:"column:document_title"`
	SpaceID       string              `gorm:"column:space_id"`
	SpaceName     string              `gorm:"column:space_name"`
	FileName      string              `gorm:"column:file_name"`
	Status        models.EntityStatus `gorm:"column:status"`
}

type documentImageAssetRowDB struct {
	ID                  int64   `gorm:"column:id"`
	ImageAssetID        string  `gorm:"column:image_asset_id"`
	DocumentID          string  `gorm:"column:document_id"`
	SpaceID             string  `gorm:"column:space_id"`
	BlobID              *string `gorm:"column:blob_id"`
	StorageProvider     string  `gorm:"column:storage_provider"`
	ObjectKey           string  `gorm:"column:object_key"`
	ObjectURL           string  `gorm:"column:object_url"`
	Status              string  `gorm:"column:status"`
	PendingCleanupAtRaw *string `gorm:"column:pending_cleanup_at_raw"`
	DeletedAtRaw        *string `gorm:"column:deleted_at_raw"`
	LastReferencedAtRaw string  `gorm:"column:last_referenced_at_raw"`
	CreatedAtRaw        string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw        string  `gorm:"column:updated_at_raw"`
}

type adminDocumentImageAssetListRowDB struct {
	ID                  int64               `gorm:"column:id"`
	ImageAssetID        string              `gorm:"column:image_asset_id"`
	DocumentID          string              `gorm:"column:document_id"`
	ReaderSlug          *string             `gorm:"column:reader_slug"`
	SpaceID             string              `gorm:"column:space_id"`
	BlobID              *string             `gorm:"column:blob_id"`
	StorageProvider     string              `gorm:"column:storage_provider"`
	ObjectKey           string              `gorm:"column:object_key"`
	ObjectURL           string              `gorm:"column:object_url"`
	Status              string              `gorm:"column:status"`
	PendingCleanupAtRaw *string             `gorm:"column:pending_cleanup_at_raw"`
	DeletedAtRaw        *string             `gorm:"column:deleted_at_raw"`
	LastReferencedAtRaw string              `gorm:"column:last_referenced_at_raw"`
	CreatedAtRaw        string              `gorm:"column:created_at_raw"`
	UpdatedAtRaw        string              `gorm:"column:updated_at_raw"`
	DocumentTitle       string              `gorm:"column:document_title"`
	DocumentStatus      models.EntityStatus `gorm:"column:document_status"`
	SpaceName           string              `gorm:"column:space_name"`
	SpaceOwnerID        string              `gorm:"column:space_owner_id"`
	SpaceOwnerName      string              `gorm:"column:space_owner_name"`
	SpaceOwnerMail      string              `gorm:"column:space_owner_mail"`
}

type documentImageAssetReferenceRowDB struct {
	ImageAssetID        string `gorm:"column:image_asset_id"`
	DocumentID          string `gorm:"column:document_id"`
	DocumentTitle       string `gorm:"column:document_title"`
	SpaceID             string `gorm:"column:space_id"`
	SpaceName           string `gorm:"column:space_name"`
	Status              string `gorm:"column:status"`
	LastReferencedAtRaw string `gorm:"column:last_referenced_at_raw"`
}

type documentAccessRowDB struct {
	ID                int64                 `gorm:"column:id"`
	DocumentID        string                `gorm:"column:document_id"`
	NodeID            string                `gorm:"column:node_id"`
	ThemeID           string                `gorm:"column:theme_id"`
	Format            models.DocumentFormat `gorm:"column:format"`
	DocumentVis       models.Visibility     `gorm:"column:document_vis"`
	DocumentStatus    models.EntityStatus   `gorm:"column:document_status"`
	DocumentBanReason string                `gorm:"column:document_ban_reason"`
	DocumentBannedAt  *time.Time            `gorm:"column:document_banned_at"`
	DocumentDeletedAt *time.Time            `gorm:"column:document_deleted_at"`
	Title             string                `gorm:"column:title"`
	ContentMD         string                `gorm:"column:content_md"`
	Version           int                   `gorm:"column:version"`
	SourceBlobID      *string               `gorm:"column:source_blob_id"`
	SourceFileName    *string               `gorm:"column:source_file_name"`
	SourceMimeType    *string               `gorm:"column:source_mime_type"`
	ContentVersion    int                   `gorm:"column:content_version"`
	CreatedByUserID   *string               `gorm:"column:created_by_user_id"`
	UpdatedByUserID   *string               `gorm:"column:updated_by_user_id"`
	SpaceID           string                `gorm:"column:space_id"`
	SpaceName         string                `gorm:"column:space_name"`
	SpaceVis          models.Visibility     `gorm:"column:space_vis"`
	SpaceStatus       models.EntityStatus   `gorm:"column:space_status"`
	SpaceBanReason    string                `gorm:"column:space_ban_reason"`
	SpaceBannedAt     *time.Time            `gorm:"column:space_banned_at"`
	SpaceDeletedAt    *time.Time            `gorm:"column:space_deleted_at"`
	SpaceOwnerUser    string                `gorm:"column:space_owner_user"`
}

type adminDocumentListRowDB struct {
	ID              int64                 `gorm:"column:id"`
	DocumentID      string                `gorm:"column:document_id"`
	NodeID          string                `gorm:"column:node_id"`
	ReaderSlug      *string               `gorm:"column:reader_slug"`
	ThemeID         string                `gorm:"column:theme_id"`
	Format          models.DocumentFormat `gorm:"column:format"`
	Visibility      models.Visibility     `gorm:"column:visibility"`
	Status          models.EntityStatus   `gorm:"column:status"`
	BannedReason    string                `gorm:"column:banned_reason"`
	BannedAt        *time.Time            `gorm:"column:banned_at"`
	DeletedAt       *time.Time            `gorm:"column:deleted_at"`
	Title           string                `gorm:"column:title"`
	ContentMD       string                `gorm:"column:content_md"`
	Version         int                   `gorm:"column:version"`
	SourceBlobID    *string               `gorm:"column:source_blob_id"`
	SourceFileName  *string               `gorm:"column:source_file_name"`
	SourceMimeType  *string               `gorm:"column:source_mime_type"`
	ContentVersion  int                   `gorm:"column:content_version"`
	CreatedByUserID *string               `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string               `gorm:"column:updated_by_user_id"`
	CreatedAtRaw    string                `gorm:"column:created_at_raw"`
	UpdatedAtRaw    string                `gorm:"column:updated_at_raw"`
	SpaceID         string                `gorm:"column:space_id"`
	SpaceName       string                `gorm:"column:space_name"`
	SpaceOwnerID    string                `gorm:"column:space_owner_id"`
	SpaceOwnerName  string                `gorm:"column:space_owner_name"`
	SpaceOwnerEmail string                `gorm:"column:space_owner_email"`
}

type documentShareRowDB struct {
	ID              int64   `gorm:"column:id"`
	ShareID         string  `gorm:"column:share_id"`
	DocumentID      string  `gorm:"column:document_id"`
	SpaceID         string  `gorm:"column:space_id"`
	Mode            string  `gorm:"column:mode"`
	PasswordHash    *string `gorm:"column:password_hash"`
	PasswordHint    string  `gorm:"column:password_hint"`
	ExpiresAtRaw    *string `gorm:"column:expires_at_raw"`
	DisabledAtRaw   *string `gorm:"column:disabled_at_raw"`
	AccessVersion   int     `gorm:"column:access_version"`
	CreatedByUserID *string `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string `gorm:"column:updated_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw    string  `gorm:"column:updated_at_raw"`
}

type documentShareAccessRowDB struct {
	ShareID          string  `gorm:"column:share_id"`
	DocumentID       string  `gorm:"column:document_id"`
	SpaceID          string  `gorm:"column:space_id"`
	DocumentFormat   string  `gorm:"column:document_format"`
	Mode             string  `gorm:"column:mode"`
	PasswordHash     *string `gorm:"column:password_hash"`
	PasswordHint     string  `gorm:"column:password_hint"`
	ExpiresAtRaw     *string `gorm:"column:expires_at_raw"`
	DisabledAtRaw    *string `gorm:"column:disabled_at_raw"`
	AccessVersion    int     `gorm:"column:access_version"`
	DocumentRouteKey string  `gorm:"column:document_route_key"`
}

type adminDocumentShareListRowDB struct {
	ID               int64   `gorm:"column:id"`
	ShareID          string  `gorm:"column:share_id"`
	DocumentID       string  `gorm:"column:document_id"`
	SpaceID          string  `gorm:"column:space_id"`
	Mode             string  `gorm:"column:mode"`
	PasswordHash     *string `gorm:"column:password_hash"`
	PasswordHint     string  `gorm:"column:password_hint"`
	ExpiresAtRaw     *string `gorm:"column:expires_at_raw"`
	DisabledAtRaw    *string `gorm:"column:disabled_at_raw"`
	AccessVersion    int     `gorm:"column:access_version"`
	CreatedByUserID  *string `gorm:"column:created_by_user_id"`
	UpdatedByUserID  *string `gorm:"column:updated_by_user_id"`
	CreatedAtRaw     string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw     string  `gorm:"column:updated_at_raw"`
	DocumentRouteKey string  `gorm:"column:document_route_key"`
	DocumentTitle    string  `gorm:"column:document_title"`
	DocumentFormat   string  `gorm:"column:document_format"`
	SpaceName        string  `gorm:"column:space_name"`
	SpaceOwnerID     string  `gorm:"column:space_owner_id"`
	SpaceOwnerName   string  `gorm:"column:space_owner_name"`
	SpaceOwnerEmail  string  `gorm:"column:space_owner_email"`
	CreatedByName    string  `gorm:"column:created_by_name"`
	CreatedByEmail   string  `gorm:"column:created_by_email"`
}

type homeSearchMetadataRowDB struct {
	SpaceID      string  `gorm:"column:space_id"`
	SpaceName    string  `gorm:"column:space_name"`
	DocumentID   string  `gorm:"column:document_id"`
	ReaderSlug   *string `gorm:"column:reader_slug"`
	Title        string  `gorm:"column:title"`
	UpdatedAtRaw string  `gorm:"column:updated_at_raw"`
}

type passwordResetTokenRowDB struct {
	ID                int64   `gorm:"column:id"`
	TokenID           string  `gorm:"column:token_id"`
	TokenSecretHash   string  `gorm:"column:token_secret_hash"`
	UserID            string  `gorm:"column:user_id"`
	Source            string  `gorm:"column:source"`
	RequestedByUserID *string `gorm:"column:requested_by_user_id"`
	RequestIPHash     string  `gorm:"column:request_ip_hash"`
	ExpiresAtRaw      string  `gorm:"column:expires_at_raw"`
	ConsumedAtRaw     *string `gorm:"column:consumed_at_raw"`
	InvalidatedAtRaw  *string `gorm:"column:invalidated_at_raw"`
	CreatedAtRaw      string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw      string  `gorm:"column:updated_at_raw"`
}

type readerPageDocumentRowDB struct {
	DocumentID     string                      `gorm:"column:document_id"`
	NodeID         string                      `gorm:"column:node_id"`
	ReaderSlug     *string                     `gorm:"column:reader_slug"`
	ThemeID        string                      `gorm:"column:theme_id"`
	Format         models.DocumentFormat       `gorm:"column:format"`
	Visibility     string                      `gorm:"column:visibility"`
	Title          string                      `gorm:"column:title"`
	ContentMD      string                      `gorm:"column:content_md"`
	RenderStatus   models.DocumentRenderStatus `gorm:"column:render_status"`
	RenderError    string                      `gorm:"column:render_error"`
	RenderedAtRaw  *string                     `gorm:"column:rendered_at_raw"`
	Version        int                         `gorm:"column:version"`
	SourceBlobID   *string                     `gorm:"column:source_blob_id"`
	SourceFileName *string                     `gorm:"column:source_file_name"`
	SourceMimeType *string                     `gorm:"column:source_mime_type"`
	ContentVersion int                         `gorm:"column:content_version"`
	AuthorNickname string                      `gorm:"column:author_nickname"`
	UpdatedAtRaw   string                      `gorm:"column:updated_at_raw"`
	SpaceID        string                      `gorm:"column:space_id"`
}

type readerPageTreeNodeRowDB struct {
	NodeID             string          `gorm:"column:node_id"`
	DocumentID         *string         `gorm:"column:document_id"`
	ReaderSlug         *string         `gorm:"column:reader_slug"`
	ParentNodeID       *string         `gorm:"column:parent_node_id"`
	Type               models.NodeType `gorm:"column:type"`
	Title              string          `gorm:"column:title"`
	Sort               int             `gorm:"column:sort"`
	DocumentVisibility *string         `gorm:"column:document_visibility"`
	DocumentFormat     *string         `gorm:"column:document_format"`
}

type readerPageDocumentIDRowDB struct {
	DocumentID string `gorm:"column:document_id"`
}

type readerPageResolvedDocumentRowDB struct {
	DocumentID string `gorm:"column:document_id"`
}

type searchAnalyzerDictEntryRowDB struct {
	ID              int64   `gorm:"column:id"`
	Analyzer        string  `gorm:"column:analyzer"`
	Term            string  `gorm:"column:term"`
	Weight          *int    `gorm:"column:weight"`
	Tag             string  `gorm:"column:tag"`
	Status          string  `gorm:"column:status"`
	CreatedByUserID *string `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string `gorm:"column:updated_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw    string  `gorm:"column:updated_at_raw"`
}

type searchIndexJobRowDB struct {
	ID           int64   `gorm:"column:id"`
	JobID        string  `gorm:"column:job_id"`
	Provider     string  `gorm:"column:provider"`
	JobType      string  `gorm:"column:job_type"`
	DedupeKey    string  `gorm:"column:dedupe_key"`
	PayloadJSON  string  `gorm:"column:payload_json"`
	Status       string  `gorm:"column:status"`
	Priority     int     `gorm:"column:priority"`
	RetryCount   int     `gorm:"column:retry_count"`
	NextRunAtRaw string  `gorm:"column:next_run_at_raw"`
	StartedAtRaw *string `gorm:"column:started_at_raw"`
	LastError    string  `gorm:"column:last_error"`
	CreatedAtRaw string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw string  `gorm:"column:updated_at_raw"`
}

type searchIndexJobIDRowDB struct {
	ID int64 `gorm:"column:id"`
}

type searchIndexSourceRowDB struct {
	SpaceID         string                    `gorm:"column:space_id"`
	DocumentID      string                    `gorm:"column:document_id"`
	NodeID          string                    `gorm:"column:node_id"`
	Format          models.DocumentFormat     `gorm:"column:format"`
	Title           string                    `gorm:"column:title"`
	ContentMD       string                    `gorm:"column:content_md"`
	SpaceVisibility string                    `gorm:"column:space_visibility"`
	DocVisibility   string                    `gorm:"column:doc_visibility"`
	UpdatedAt       searchIndexSourceScanTime `gorm:"column:updated_at"`
}

type sitemapPublicDocumentSourceRowDB struct {
	SpaceID              string  `gorm:"column:space_id"`
	SpaceUpdatedAtRaw    string  `gorm:"column:space_updated_at_raw"`
	DocumentID           string  `gorm:"column:document_id"`
	ReaderSlug           *string `gorm:"column:reader_slug"`
	DocumentFormat       string  `gorm:"column:document_format"`
	DocumentContentMD    string  `gorm:"column:document_content_md"`
	DocumentUpdatedAtRaw string  `gorm:"column:document_updated_at_raw"`
}

type systemConfigRowDB struct {
	ID              int64   `gorm:"column:id"`
	ConfigKey       string  `gorm:"column:config_key"`
	ConfigValueJSON string  `gorm:"column:config_value_json"`
	Version         int     `gorm:"column:version"`
	UpdatedByUserID *string `gorm:"column:updated_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw    string  `gorm:"column:updated_at_raw"`
}

type themeRowDB struct {
	ID                     int64  `gorm:"column:id"`
	ThemeID                string `gorm:"column:theme_id"`
	Name                   string `gorm:"column:name"`
	Description            string `gorm:"column:description"`
	VariablesJSON          string `gorm:"column:variables_json"`
	SyntaxTheme            string `gorm:"column:syntax_theme"`
	CodeBlockStyleJSON     string `gorm:"column:code_block_style_json"`
	CodeBlockCodeStyleJSON string `gorm:"column:code_block_code_style_json"`
	InlineCodeStyleJSON    string `gorm:"column:inline_code_style_json"`
	CustomCSS              string `gorm:"column:custom_css"`
	IsBuiltin              bool   `gorm:"column:is_builtin"`
	IsEnabled              bool   `gorm:"column:is_enabled"`
	CreatedAtRaw           string `gorm:"column:created_at_raw"`
	UpdatedAtRaw           string `gorm:"column:updated_at_raw"`
}

type userIdentityRowDB struct {
	ID             int64   `gorm:"column:id"`
	UserID         string  `gorm:"column:user_id"`
	ProviderType   string  `gorm:"column:provider_type"`
	ProviderID     string  `gorm:"column:provider_id"`
	ExternalID     string  `gorm:"column:external_id"`
	LoginName      string  `gorm:"column:login_name"`
	LastLoginAtRaw *string `gorm:"column:last_login_at_raw"`
	CreatedAtRaw   string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw   string  `gorm:"column:updated_at_raw"`
}

type documentImageAssetLifecycleExistingRowDB struct {
	ID              int64   `gorm:"column:id"`
	BlobID          *string `gorm:"column:blob_id"`
	StorageProvider string  `gorm:"column:storage_provider"`
	ObjectKey       string  `gorm:"column:object_key"`
}

type documentImageAssetLifecyclePendingCandidateRowDB struct {
	ID              int64   `gorm:"column:id"`
	BlobID          *string `gorm:"column:blob_id"`
	StorageProvider string  `gorm:"column:storage_provider"`
	ObjectKey       string  `gorm:"column:object_key"`
}

type documentImageAssetLifecycleBlobRowDB struct {
	BlobID          string `gorm:"column:blob_id"`
	StorageProvider string `gorm:"column:storage_provider"`
	ObjectKey       string `gorm:"column:object_key"`
}

type documentTemplateListRowDB struct {
	TemplateID   string `gorm:"column:template_id"`
	SceneKey     string `gorm:"column:scene_key"`
	SceneName    string `gorm:"column:scene_name"`
	Name         string `gorm:"column:name"`
	Description  string `gorm:"column:description"`
	DefaultTitle string `gorm:"column:default_title"`
	Sort         int    `gorm:"column:sort"`
	IsBuiltin    bool   `gorm:"column:is_builtin"`
	IsEnabled    bool   `gorm:"column:is_enabled"`
	UpdatedAtRaw string `gorm:"column:updated_at_raw"`
}

type documentTemplateDetailRowDB struct {
	TemplateID      string  `gorm:"column:template_id"`
	SceneKey        string  `gorm:"column:scene_key"`
	SceneName       string  `gorm:"column:scene_name"`
	Name            string  `gorm:"column:name"`
	Description     string  `gorm:"column:description"`
	DefaultTitle    string  `gorm:"column:default_title"`
	ContentMD       string  `gorm:"column:content_md"`
	Sort            int     `gorm:"column:sort"`
	IsBuiltin       bool    `gorm:"column:is_builtin"`
	IsEnabled       bool    `gorm:"column:is_enabled"`
	CreatedByUserID *string `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string `gorm:"column:updated_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw    string  `gorm:"column:updated_at_raw"`
}

type documentTemplateSceneListRowDB struct {
	SceneKey      string `gorm:"column:scene_key"`
	SceneName     string `gorm:"column:scene_name"`
	Description   string `gorm:"column:description"`
	Sort          int    `gorm:"column:sort"`
	IsBuiltin     bool   `gorm:"column:is_builtin"`
	TemplateCount int64  `gorm:"column:template_count"`
	UpdatedAtRaw  string `gorm:"column:updated_at_raw"`
}

type documentTemplateSceneDetailRowDB struct {
	SceneKey        string  `gorm:"column:scene_key"`
	SceneName       string  `gorm:"column:scene_name"`
	Description     string  `gorm:"column:description"`
	Sort            int     `gorm:"column:sort"`
	IsBuiltin       bool    `gorm:"column:is_builtin"`
	CreatedByUserID *string `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string `gorm:"column:updated_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at_raw"`
	UpdatedAtRaw    string  `gorm:"column:updated_at_raw"`
}

type userListRowDB struct {
	ID           int64               `gorm:"column:id"`
	UserID       string              `gorm:"column:user_id"`
	Email        string              `gorm:"column:email"`
	PasswordHash string              `gorm:"column:password_hash"`
	Name         string              `gorm:"column:name"`
	AvatarURL    string              `gorm:"column:avatar_url"`
	Status       models.EntityStatus `gorm:"column:status"`
	BannedReason string              `gorm:"column:banned_reason"`
	BannedAt     *time.Time          `gorm:"column:banned_at"`
	DeletedAt    *time.Time          `gorm:"column:deleted_at"`
	CreatedAtRaw string              `gorm:"column:created_at_raw"`
	UpdatedAtRaw string              `gorm:"column:updated_at_raw"`
}
