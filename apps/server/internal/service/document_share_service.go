package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	minDocumentSharePasswordLength = 6
)

var (
	ErrDocumentShareNotFound         = errors.New("document share not found")
	ErrDocumentShareInvalidMode      = errors.New("document share mode is invalid")
	ErrDocumentSharePasswordRequired = errors.New("document share password is required")
	ErrDocumentSharePasswordTooShort = errors.New("document share password too short")
	ErrDocumentSharePasswordInvalid  = errors.New("document share password invalid")
	ErrDocumentShareAccessDenied     = errors.New("document share access denied")
	ErrDocumentSharePreviewDenied    = errors.New("document share preview unsupported")
)

// DocumentShareAttachmentPurpose 表示分享态附件访问用途。
type DocumentShareAttachmentPurpose string

const (
	DocumentShareAttachmentPurposeDownload DocumentShareAttachmentPurpose = "download"
	DocumentShareAttachmentPurposePreview  DocumentShareAttachmentPurpose = "preview"
)

// DocumentShareConfig 表示文档当前分享配置。
type DocumentShareConfig struct {
	Enabled       bool
	ShareID       string
	DocumentID    string
	SpaceID       string
	Mode          models.DocumentShareMode
	PasswordHint  string
	HasPassword   bool
	ExpiresAt     *time.Time
	DisabledAt    *time.Time
	AccessVersion int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UpsertDocumentShareInput 定义文档分享配置入参。
type UpsertDocumentShareInput struct {
	DocumentID   string
	SpaceID      string
	ActorUserID  string
	Mode         models.DocumentShareMode
	Password     string
	PasswordHint string
	ExpiresAt    *time.Time
}

// UpdateDocumentShareByIDInput 定义按 share_id 更新分享配置入参。
type UpdateDocumentShareByIDInput struct {
	ShareID      string
	ActorUserID  string
	Mode         *models.DocumentShareMode
	Password     *string
	PasswordHint *string
	ExpiresAtSet bool
	ExpiresAt    *time.Time
}

// ResolveDocumentShareResult 定义分享访问解析结果。
type ResolveDocumentShareResult struct {
	ShareID          string
	DocumentID       string
	SpaceID          string
	Mode             models.DocumentShareMode
	PasswordHash     *string
	PasswordHint     string
	ExpiresAt        *time.Time
	AccessVersion    int
	DocumentRouteKey string
}

// DocumentSharePageResult 表示分享阅读页数据。
type DocumentSharePageResult struct {
	Share ResolveDocumentShareResult
	Page  ReaderPageViewModel
}

// DocumentShareAttachmentAccessResult 表示分享态附件访问链接结果。
type DocumentShareAttachmentAccessResult struct {
	URL             string
	Purpose         DocumentShareAttachmentPurpose
	PreviewKind     string
	PreviewEnabled  bool
	StorageProvider string
	ObjectKey       string
	FileName        string
	MimeType        string
}

// ListAdminDocumentSharesInput 定义后台分享中心查询参数。
type ListAdminDocumentSharesInput struct {
	ActorUserID      string
	RestrictToScopes bool
	View             repository.DocumentShareAdminView
	Keyword          string
	SpaceID          string
	Mode             models.DocumentShareMode
	Expired          string
	Page             int
	PageSize         int
}

// ListAdminDocumentSharesResult 表示后台分享中心查询结果。
type ListAdminDocumentSharesResult struct {
	Items    []repository.AdminDocumentShareListRecord
	Page     int
	PageSize int
	Total    int64
}

// DocumentShareService 封装文档分享配置与分享态读取能力。
type DocumentShareService struct {
	shareRepo              repository.DocumentShareRepository
	readerPageRepo         repository.ReaderPageRepository
	documentAttachmentRepo repository.DocumentAttachmentRepository
	spaceRepo              repository.SpaceRepository
	imageHostingService    *ImageHostingService
	nowFn                  func() time.Time
}

// NewDocumentShareService 创建文档分享服务。
func NewDocumentShareService(
	db *gorm.DB,
	shareRepo repository.DocumentShareRepository,
	documentAttachmentRepo repository.DocumentAttachmentRepository,
	spaceRepo repository.SpaceRepository,
	imageHostingService *ImageHostingService,
) *DocumentShareService {
	resolvedShareRepo := shareRepo
	if resolvedShareRepo == nil && db != nil {
		resolvedShareRepo = repository.NewGormDocumentShareRepository(db)
	}
	resolvedReaderRepo := repository.NewGormReaderPageRepository(db)
	return &DocumentShareService{
		shareRepo:              resolvedShareRepo,
		readerPageRepo:         resolvedReaderRepo,
		documentAttachmentRepo: documentAttachmentRepo,
		spaceRepo:              spaceRepo,
		imageHostingService:    imageHostingService,
		nowFn:                  time.Now,
	}
}

// GetConfigByDocumentID 查询文档当前分享配置。
func (s *DocumentShareService) GetConfigByDocumentID(
	ctx context.Context,
	documentID string,
) (DocumentShareConfig, error) {
	if s == nil || s.shareRepo == nil {
		return DocumentShareConfig{}, errors.New("document share service dependencies are nil")
	}
	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return DocumentShareConfig{}, ErrDocumentShareNotFound
	}

	share, err := s.shareRepo.GetByDocumentID(ctx, normalizedDocumentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DocumentShareConfig{
				Enabled:    false,
				DocumentID: normalizedDocumentID,
				Mode:       models.DocumentShareModePublic,
			}, nil
		}
		return DocumentShareConfig{}, err
	}
	return mapDocumentShareConfig(*share), nil
}

// UpsertByDocumentID 创建或更新文档分享配置。
func (s *DocumentShareService) UpsertByDocumentID(
	ctx context.Context,
	input UpsertDocumentShareInput,
) (DocumentShareConfig, error) {
	if s == nil || s.shareRepo == nil {
		return DocumentShareConfig{}, errors.New("document share service dependencies are nil")
	}
	documentID := strings.TrimSpace(input.DocumentID)
	spaceID := strings.TrimSpace(input.SpaceID)
	actorUserID := strings.TrimSpace(input.ActorUserID)
	if documentID == "" || spaceID == "" || actorUserID == "" {
		return DocumentShareConfig{}, ErrDocumentShareNotFound
	}

	mode := normalizeDocumentShareMode(input.Mode)
	if mode == "" {
		return DocumentShareConfig{}, ErrDocumentShareInvalidMode
	}
	password := strings.TrimSpace(input.Password)
	passwordHint := strings.TrimSpace(input.PasswordHint)
	expiresAt := normalizeOptionalUTC(input.ExpiresAt)
	now := s.nowFn().UTC()

	existing, err := s.shareRepo.GetByDocumentID(ctx, documentID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return DocumentShareConfig{}, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) || existing == nil {
		share, buildErr := s.buildNewShareRecord(documentID, spaceID, actorUserID, mode, password, passwordHint, expiresAt, now)
		if buildErr != nil {
			return DocumentShareConfig{}, buildErr
		}
		if createErr := s.shareRepo.Create(ctx, &share); createErr != nil {
			return DocumentShareConfig{}, createErr
		}
		return mapDocumentShareConfig(share), nil
	}

	var passwordPtr *string
	if password != "" {
		passwordPtr = &password
	}
	updatedShare, updateErr := s.applyShareUpdate(*existing, UpdateDocumentShareByIDInput{
		ShareID:      existing.ShareID,
		ActorUserID:  actorUserID,
		Mode:         &mode,
		Password:     passwordPtr,
		PasswordHint: &passwordHint,
		ExpiresAtSet: true,
		ExpiresAt:    expiresAt,
	}, now)
	if updateErr != nil {
		return DocumentShareConfig{}, updateErr
	}
	if _, updateErr = s.shareRepo.Update(ctx, &updatedShare); updateErr != nil {
		return DocumentShareConfig{}, updateErr
	}
	return mapDocumentShareConfig(updatedShare), nil
}

// DisableByDocumentID 取消文档分享（逻辑失效）。
func (s *DocumentShareService) DisableByDocumentID(
	ctx context.Context,
	documentID string,
	actorUserID string,
) error {
	if s == nil || s.shareRepo == nil {
		return errors.New("document share service dependencies are nil")
	}
	normalizedDocumentID := strings.TrimSpace(documentID)
	normalizedActorUserID := strings.TrimSpace(actorUserID)
	if normalizedDocumentID == "" || normalizedActorUserID == "" {
		return ErrDocumentShareNotFound
	}

	share, err := s.shareRepo.GetByDocumentID(ctx, normalizedDocumentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	now := s.nowFn().UTC()
	share.DisabledAt = &now
	share.UpdatedByUserID = &normalizedActorUserID
	share.UpdatedAt = now
	if share.AccessVersion <= 0 {
		share.AccessVersion = 1
	}
	share.AccessVersion += 1
	_, err = s.shareRepo.Update(ctx, share)
	return err
}

// DisableByShareID 取消指定 share_id 分享（后台使用）。
func (s *DocumentShareService) DisableByShareID(
	ctx context.Context,
	shareID string,
	actorUserID string,
) error {
	if s == nil || s.shareRepo == nil {
		return errors.New("document share service dependencies are nil")
	}
	normalizedShareID := strings.TrimSpace(shareID)
	normalizedActorUserID := strings.TrimSpace(actorUserID)
	if normalizedShareID == "" || normalizedActorUserID == "" {
		return ErrDocumentShareNotFound
	}

	share, err := s.shareRepo.GetByShareID(ctx, normalizedShareID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDocumentShareNotFound
		}
		return err
	}
	now := s.nowFn().UTC()
	share.DisabledAt = &now
	share.UpdatedByUserID = &normalizedActorUserID
	share.UpdatedAt = now
	if share.AccessVersion <= 0 {
		share.AccessVersion = 1
	}
	share.AccessVersion += 1
	updated, updateErr := s.shareRepo.Update(ctx, share)
	if updateErr != nil {
		return updateErr
	}
	if !updated {
		return ErrDocumentShareNotFound
	}
	return nil
}

// UpdateByShareID 更新指定 share_id 配置（后台使用）。
func (s *DocumentShareService) UpdateByShareID(
	ctx context.Context,
	input UpdateDocumentShareByIDInput,
) (DocumentShareConfig, error) {
	if s == nil || s.shareRepo == nil {
		return DocumentShareConfig{}, errors.New("document share service dependencies are nil")
	}
	shareID := strings.TrimSpace(input.ShareID)
	actorUserID := strings.TrimSpace(input.ActorUserID)
	if shareID == "" || actorUserID == "" {
		return DocumentShareConfig{}, ErrDocumentShareNotFound
	}

	share, err := s.shareRepo.GetByShareID(ctx, shareID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DocumentShareConfig{}, ErrDocumentShareNotFound
		}
		return DocumentShareConfig{}, err
	}

	now := s.nowFn().UTC()
	updatedShare, err := s.applyShareUpdate(*share, input, now)
	if err != nil {
		return DocumentShareConfig{}, err
	}
	updated, err := s.shareRepo.Update(ctx, &updatedShare)
	if err != nil {
		return DocumentShareConfig{}, err
	}
	if !updated {
		return DocumentShareConfig{}, ErrDocumentShareNotFound
	}
	return mapDocumentShareConfig(updatedShare), nil
}

// ResolveActiveShareByRouteKey 解析可访问分享配置。
func (s *DocumentShareService) ResolveActiveShareByRouteKey(
	ctx context.Context,
	spaceID string,
	docKey string,
) (ResolveDocumentShareResult, error) {
	if s == nil || s.shareRepo == nil {
		return ResolveDocumentShareResult{}, errors.New("document share service dependencies are nil")
	}
	row, err := s.shareRepo.ResolveBySpaceAndDocKey(ctx, strings.TrimSpace(spaceID), strings.TrimSpace(docKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ResolveDocumentShareResult{}, ErrDocumentShareNotFound
		}
		return ResolveDocumentShareResult{}, err
	}

	mode := normalizeDocumentShareMode(row.Mode)
	if mode == "" {
		mode = models.DocumentShareModePublic
	}
	accessVersion := row.AccessVersion
	if accessVersion <= 0 {
		accessVersion = 1
	}
	return ResolveDocumentShareResult{
		ShareID:          strings.TrimSpace(row.ShareID),
		DocumentID:       strings.TrimSpace(row.DocumentID),
		SpaceID:          strings.TrimSpace(row.SpaceID),
		Mode:             mode,
		PasswordHash:     trimDocumentShareOptionalString(row.PasswordHash),
		PasswordHint:     strings.TrimSpace(row.PasswordHint),
		ExpiresAt:        normalizeOptionalUTC(row.ExpiresAt),
		AccessVersion:    accessVersion,
		DocumentRouteKey: strings.TrimSpace(row.DocumentRouteKey),
	}, nil
}

// VerifyPassword 校验密码分享访问密码。
func (s *DocumentShareService) VerifyPassword(
	ctx context.Context,
	spaceID string,
	docKey string,
	password string,
) (ResolveDocumentShareResult, error) {
	resolved, err := s.ResolveActiveShareByRouteKey(ctx, spaceID, docKey)
	if err != nil {
		return ResolveDocumentShareResult{}, err
	}
	if resolved.Mode != models.DocumentShareModePassword {
		return resolved, nil
	}
	passwordHash := strings.TrimSpace(normalizeDocumentShareString(resolved.PasswordHash))
	if passwordHash == "" {
		return ResolveDocumentShareResult{}, ErrDocumentSharePasswordRequired
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return ResolveDocumentShareResult{}, ErrDocumentSharePasswordInvalid
	}
	return resolved, nil
}

// BuildReaderPageByRouteKey 构建分享阅读页数据。
func (s *DocumentShareService) BuildReaderPageByRouteKey(
	ctx context.Context,
	spaceID string,
	docKey string,
) (DocumentSharePageResult, error) {
	if s == nil || s.readerPageRepo == nil {
		return DocumentSharePageResult{}, errors.New("document share service dependencies are nil")
	}
	resolvedShare, err := s.ResolveActiveShareByRouteKey(ctx, spaceID, docKey)
	if err != nil {
		return DocumentSharePageResult{}, err
	}

	readerSvc := &ReaderPageService{
		readerPageRepo:         s.readerPageRepo,
		documentAttachmentRepo: s.documentAttachmentRepo,
	}
	documentRow, err := readerSvc.loadDocumentRow(ctx, resolvedShare.DocumentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DocumentSharePageResult{}, ErrDocumentShareNotFound
		}
		return DocumentSharePageResult{}, err
	}
	if strings.TrimSpace(documentRow.SpaceID) != strings.TrimSpace(spaceID) {
		return DocumentSharePageResult{}, ErrDocumentShareNotFound
	}

	tree, err := readerSvc.loadTree(ctx, spaceID)
	if err != nil {
		return DocumentSharePageResult{}, err
	}
	attachments, err := readerSvc.loadDocumentAttachments(ctx, resolvedShare.DocumentID)
	if err != nil {
		return DocumentSharePageResult{}, err
	}

	spaceName := strings.TrimSpace(spaceID)
	if s.spaceRepo != nil {
		if space, spaceErr := s.spaceRepo.GetBySpaceID(ctx, strings.TrimSpace(spaceID)); spaceErr == nil && space != nil {
			if candidateName := strings.TrimSpace(space.Name); candidateName != "" {
				spaceName = candidateName
			}
		}
	}

	documentTitle := strings.TrimSpace(documentRow.Title)
	pageTitle := documentTitle
	if pageTitle == "" {
		pageTitle = "未命名文档"
	}
	if spaceName != "" {
		pageTitle = pageTitle + " - " + spaceName
	}
	documentIDValue := strings.TrimSpace(documentRow.DocumentID)
	documentIdentifier := normalizeReaderOptionalString(documentRow.ReaderSlug)
	documentRouteKey := resolveReaderDocumentRouteKey(documentIDValue, documentRow.ReaderSlug)

	page := ReaderPageViewModel{
		Space: ReaderSpaceViewModel{
			ID:    strings.TrimSpace(spaceID),
			Name:  spaceName,
			Title: pageTitle,
		},
		Document: ReaderDocumentViewModel{
			ID:             documentIDValue,
			NodeID:         strings.TrimSpace(documentRow.NodeID),
			Identifier:     normalizeReaderString(documentIdentifier),
			RouteKey:       documentRouteKey,
			ThemeID:        strings.TrimSpace(documentRow.ThemeID),
			Visibility:     normalizeReaderVisibility(documentRow.Visibility),
			Title:          documentTitle,
			ContentMD:      documentRow.ContentMD,
			Version:        documentRow.Version,
			AuthorNickname: normalizeReaderAuthorNickname(documentRow.AuthorNickname),
			UpdatedAt:      formatReaderTime(documentRow.UpdatedAt),
		},
		Attachments: attachments,
		Tree:        tree,
		ActiveDocID: documentIDValue,
	}
	return DocumentSharePageResult{
		Share: resolvedShare,
		Page:  page,
	}, nil
}

// BuildAttachmentAccessURLByRouteKey 构建分享态附件访问链接。
func (s *DocumentShareService) BuildAttachmentAccessURLByRouteKey(
	ctx context.Context,
	spaceID string,
	docKey string,
	attachmentID string,
	purpose DocumentShareAttachmentPurpose,
) (DocumentShareAttachmentAccessResult, error) {
	if s == nil || s.documentAttachmentRepo == nil {
		return DocumentShareAttachmentAccessResult{}, errors.New("document share service dependencies are nil")
	}
	resolvedShare, err := s.ResolveActiveShareByRouteKey(ctx, spaceID, docKey)
	if err != nil {
		return DocumentShareAttachmentAccessResult{}, err
	}
	normalizedAttachmentID := strings.TrimSpace(attachmentID)
	if normalizedAttachmentID == "" {
		return DocumentShareAttachmentAccessResult{}, ErrDocumentShareNotFound
	}

	targetAttachment, err := s.documentAttachmentRepo.GetByAttachmentID(ctx, normalizedAttachmentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DocumentShareAttachmentAccessResult{}, ErrDocumentShareNotFound
		}
		return DocumentShareAttachmentAccessResult{}, err
	}
	if strings.TrimSpace(targetAttachment.DocumentID) != resolvedShare.DocumentID ||
		targetAttachment.Status == models.EntityStatusDeleted {
		return DocumentShareAttachmentAccessResult{}, ErrDocumentShareNotFound
	}

	normalizedPurpose := normalizeDocumentShareAttachmentPurpose(purpose)
	if normalizedPurpose == "" {
		normalizedPurpose = DocumentShareAttachmentPurposeDownload
	}
	previewKind := normalizeReaderAttachmentPreviewKind(strings.TrimSpace(targetAttachment.PreviewKind))
	previewSupported := isReaderAttachmentPreviewSupported(previewKind)
	if normalizedPurpose == DocumentShareAttachmentPurposePreview && !previewSupported {
		return DocumentShareAttachmentAccessResult{}, ErrDocumentSharePreviewDenied
	}

	attachmentURL, err := s.resolveShareAttachmentURL(ctx, *targetAttachment, normalizedPurpose)
	if err != nil {
		return DocumentShareAttachmentAccessResult{}, err
	}
	if strings.TrimSpace(attachmentURL) == "" {
		return DocumentShareAttachmentAccessResult{}, ErrDocumentShareNotFound
	}
	return DocumentShareAttachmentAccessResult{
		URL:             attachmentURL,
		Purpose:         normalizedPurpose,
		PreviewKind:     previewKind,
		PreviewEnabled:  previewSupported,
		StorageProvider: strings.TrimSpace(targetAttachment.StorageProvider),
		ObjectKey:       strings.TrimSpace(targetAttachment.ObjectKey),
		FileName:        strings.TrimSpace(targetAttachment.FileName),
		MimeType:        strings.TrimSpace(targetAttachment.MimeType),
	}, nil
}

// ListForAdmin 查询后台分享中心列表。
func (s *DocumentShareService) ListForAdmin(
	ctx context.Context,
	input ListAdminDocumentSharesInput,
) (ListAdminDocumentSharesResult, error) {
	if s == nil || s.shareRepo == nil {
		return ListAdminDocumentSharesResult{}, errors.New("document share service dependencies are nil")
	}
	page, pageSize := normalizeDocumentSharePagination(input.Page, input.PageSize)
	items, total, err := s.shareRepo.ListForAdmin(ctx, repository.ListAdminDocumentSharesParams{
		ActorUserID:      strings.TrimSpace(input.ActorUserID),
		RestrictToScopes: input.RestrictToScopes,
		View:             input.View,
		Keyword:          strings.TrimSpace(input.Keyword),
		SpaceID:          strings.TrimSpace(input.SpaceID),
		Mode:             normalizeDocumentShareMode(input.Mode),
		Expired:          strings.TrimSpace(input.Expired),
		Limit:            pageSize,
		Offset:           (page - 1) * pageSize,
	})
	if err != nil {
		return ListAdminDocumentSharesResult{}, err
	}
	return ListAdminDocumentSharesResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *DocumentShareService) buildNewShareRecord(
	documentID string,
	spaceID string,
	actorUserID string,
	mode models.DocumentShareMode,
	password string,
	passwordHint string,
	expiresAt *time.Time,
	now time.Time,
) (models.DocumentShare, error) {
	share := models.DocumentShare{
		ShareID:         strings.ToLower(ulid.Make().String()),
		DocumentID:      documentID,
		SpaceID:         spaceID,
		Mode:            mode,
		PasswordHint:    "",
		ExpiresAt:       normalizeOptionalUTC(expiresAt),
		DisabledAt:      nil,
		AccessVersion:   1,
		CreatedByUserID: &actorUserID,
		UpdatedByUserID: &actorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if mode == models.DocumentShareModePassword {
		normalizedPassword := strings.TrimSpace(password)
		if normalizedPassword == "" {
			return models.DocumentShare{}, ErrDocumentSharePasswordRequired
		}
		if len([]rune(normalizedPassword)) < minDocumentSharePasswordLength {
			return models.DocumentShare{}, ErrDocumentSharePasswordTooShort
		}
		passwordHashBytes, hashErr := bcrypt.GenerateFromPassword([]byte(normalizedPassword), bcrypt.DefaultCost)
		if hashErr != nil {
			return models.DocumentShare{}, hashErr
		}
		passwordHash := string(passwordHashBytes)
		share.PasswordHash = &passwordHash
		share.PasswordHint = strings.TrimSpace(passwordHint)
	}
	return share, nil
}

func (s *DocumentShareService) applyShareUpdate(
	current models.DocumentShare,
	input UpdateDocumentShareByIDInput,
	now time.Time,
) (models.DocumentShare, error) {
	next := current
	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return models.DocumentShare{}, ErrDocumentShareAccessDenied
	}

	targetMode := normalizeDocumentShareMode(next.Mode)
	if targetMode == "" {
		targetMode = models.DocumentShareModePublic
	}
	if input.Mode != nil {
		targetMode = normalizeDocumentShareMode(*input.Mode)
		if targetMode == "" {
			return models.DocumentShare{}, ErrDocumentShareInvalidMode
		}
	}

	passwordChanged := false
	passwordHint := strings.TrimSpace(next.PasswordHint)
	if input.PasswordHint != nil {
		passwordHint = strings.TrimSpace(*input.PasswordHint)
	}

	if targetMode == models.DocumentShareModePassword {
		if input.Password != nil {
			normalizedPassword := strings.TrimSpace(*input.Password)
			if normalizedPassword == "" {
				return models.DocumentShare{}, ErrDocumentSharePasswordRequired
			}
			if len([]rune(normalizedPassword)) < minDocumentSharePasswordLength {
				return models.DocumentShare{}, ErrDocumentSharePasswordTooShort
			}
			passwordHashBytes, hashErr := bcrypt.GenerateFromPassword([]byte(normalizedPassword), bcrypt.DefaultCost)
			if hashErr != nil {
				return models.DocumentShare{}, hashErr
			}
			passwordHash := string(passwordHashBytes)
			if current.PasswordHash == nil || bcrypt.CompareHashAndPassword([]byte(strings.TrimSpace(*current.PasswordHash)), []byte(normalizedPassword)) != nil {
				passwordChanged = true
			}
			next.PasswordHash = &passwordHash
		} else if trimDocumentShareOptionalString(current.PasswordHash) == nil {
			return models.DocumentShare{}, ErrDocumentSharePasswordRequired
		}
		next.PasswordHint = passwordHint
	} else {
		if trimDocumentShareOptionalString(next.PasswordHash) != nil {
			passwordChanged = true
		}
		next.PasswordHash = nil
		next.PasswordHint = ""
	}

	if input.ExpiresAtSet {
		next.ExpiresAt = normalizeOptionalUTC(input.ExpiresAt)
	}

	originalMode := normalizeDocumentShareMode(current.Mode)
	if originalMode == "" {
		originalMode = models.DocumentShareModePublic
	}
	modeChanged := originalMode != targetMode
	reenabled := current.DisabledAt != nil

	next.Mode = targetMode
	next.DisabledAt = nil
	next.UpdatedByUserID = &actorUserID
	next.UpdatedAt = now
	if next.AccessVersion <= 0 {
		next.AccessVersion = 1
	}
	if modeChanged || passwordChanged || reenabled {
		next.AccessVersion += 1
	}
	return next, nil
}

func (s *DocumentShareService) resolveShareAttachmentURL(
	ctx context.Context,
	attachment models.DocumentAttachment,
	purpose DocumentShareAttachmentPurpose,
) (string, error) {
	config := DefaultImageHostingConfig()
	if s.imageHostingService != nil {
		loadedConfig, err := s.imageHostingService.GetConfig(ctx)
		if err != nil {
			return "", err
		}
		config = loadedConfig
	}

	provider := strings.ToLower(strings.TrimSpace(attachment.StorageProvider))
	if provider == string(ImageHostingProviderLocal) {
		return resolveDocumentSharePublicURL(config.Local.PublicBaseURL, attachment.ObjectKey, "/uploads"), nil
	}
	if s.imageHostingService == nil {
		return "", errors.New("image hosting service is nil")
	}
	urlValue, err := s.imageHostingService.BuildObjectReadURL(ctx, config, BuildImageHostingObjectReadURLInput{
		Provider:  ImageHostingProvider(provider),
		ObjectKey: strings.TrimSpace(attachment.ObjectKey),
		ObjectURL: strings.TrimSpace(attachment.ObjectURL),
		FileName:  strings.TrimSpace(attachment.FileName),
		Purpose:   normalizeDocumentShareAttachmentLinkPurpose(purpose),
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(urlValue), nil
}

func mapDocumentShareConfig(share models.DocumentShare) DocumentShareConfig {
	mode := normalizeDocumentShareMode(share.Mode)
	if mode == "" {
		mode = models.DocumentShareModePublic
	}
	return DocumentShareConfig{
		Enabled:       share.DisabledAt == nil,
		ShareID:       strings.TrimSpace(share.ShareID),
		DocumentID:    strings.TrimSpace(share.DocumentID),
		SpaceID:       strings.TrimSpace(share.SpaceID),
		Mode:          mode,
		PasswordHint:  strings.TrimSpace(share.PasswordHint),
		HasPassword:   trimDocumentShareOptionalString(share.PasswordHash) != nil,
		ExpiresAt:     normalizeOptionalUTC(share.ExpiresAt),
		DisabledAt:    normalizeOptionalUTC(share.DisabledAt),
		AccessVersion: share.AccessVersion,
		CreatedAt:     share.CreatedAt.UTC(),
		UpdatedAt:     share.UpdatedAt.UTC(),
	}
}

func normalizeDocumentShareMode(mode models.DocumentShareMode) models.DocumentShareMode {
	value := models.DocumentShareMode(strings.ToLower(strings.TrimSpace(string(mode))))
	if !models.IsValidDocumentShareMode(value) {
		return ""
	}
	return value
}

func normalizeDocumentShareAttachmentPurpose(purpose DocumentShareAttachmentPurpose) DocumentShareAttachmentPurpose {
	value := DocumentShareAttachmentPurpose(strings.ToLower(strings.TrimSpace(string(purpose))))
	switch value {
	case DocumentShareAttachmentPurposeDownload:
		return DocumentShareAttachmentPurposeDownload
	case DocumentShareAttachmentPurposePreview:
		return DocumentShareAttachmentPurposePreview
	default:
		return ""
	}
}

func normalizeDocumentShareAttachmentLinkPurpose(
	purpose DocumentShareAttachmentPurpose,
) DocumentAttachmentLinkPurpose {
	switch normalizeDocumentShareAttachmentPurpose(purpose) {
	case DocumentShareAttachmentPurposePreview:
		return DocumentAttachmentLinkPurposePreview
	default:
		return DocumentAttachmentLinkPurposeDownload
	}
}

func normalizeDocumentShareString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func trimDocumentShareOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeOptionalUTC(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func resolveDocumentSharePublicURL(publicBaseURL string, objectKey string, defaultPrefix string) string {
	normalizedObjectKey := strings.Trim(strings.TrimSpace(objectKey), "/")
	if normalizedObjectKey == "" {
		return ""
	}
	baseURL := strings.TrimSpace(publicBaseURL)
	if baseURL == "" {
		prefix := strings.TrimSpace(defaultPrefix)
		if prefix == "" {
			prefix = "/uploads"
		}
		prefix = "/" + strings.Trim(strings.TrimSpace(prefix), "/")
		return prefix + "/" + normalizedObjectKey
	}
	return strings.TrimRight(baseURL, "/") + "/" + normalizedObjectKey
}

func normalizeDocumentSharePagination(page int, pageSize int) (int, int) {
	normalizedPage := page
	if normalizedPage <= 0 {
		normalizedPage = 1
	}
	normalizedPageSize := pageSize
	if normalizedPageSize <= 0 {
		normalizedPageSize = 20
	}
	if normalizedPageSize > 100 {
		normalizedPageSize = 100
	}
	return normalizedPage, normalizedPageSize
}
