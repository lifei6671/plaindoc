package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

func TestAdminSpaceImportService_Commit_RequiresImportCapability(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)

	_, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "",
		ImportID:    "import-a",
		SpaceName:   "导入空间",
	})

	if !errors.Is(err, errcode.ErrAdminSpaceImportCommitForbidden) {
		t.Fatalf("expected commit forbidden error, got %v", err)
	}
}

func TestAdminSpaceImportService_Inspect_RequiresImportCapability(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)

	_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
	})

	if !errors.Is(err, errcode.ErrAdminSpaceImportCommitForbidden) {
		t.Fatalf("expected commit forbidden error, got %v", err)
	}
}

func TestAdminSpaceImportService_Commit_FailsJobWhenImportCapabilityRevoked(t *testing.T) {
	t.Parallel()

	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
	}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(nil, workspaceRepo, nil, nil),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
	)
	svc.stagingDir = t.TempDir()
	var permissionChecks int
	svc.canImportSpace = func(context.Context, string) (bool, error) {
		permissionChecks++
		return permissionChecks <= 2, nil
	}

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}

	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed before worker recheck: %v", err)
	}

	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusFailed && job.LastEvent.Stage == "permission"
	})
}

func TestAdminSpaceImportService_Commit_QueuesEPUBWithCreateSpaceSemantics(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()
	var permissionChecks int
	svc.canImportSpace = func(_ context.Context, actorUserID string) (bool, error) {
		permissionChecks++
		return strings.TrimSpace(actorUserID) == "member-user", nil
	}

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "member-user",
		FileName:    "book.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUB3(t)),
	})
	if err != nil {
		t.Fatalf("inspect epub failed: %v", err)
	}
	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "member-user",
		ImportID:    result.ImportID,
		SpaceID:     "client-should-be-ignored",
		CategoryID:  "cat-a",
		Visibility:  "public",
	})
	if err != nil {
		t.Fatalf("commit epub failed: %v", err)
	}

	job, err := svc.store.GetJob(commitResult.JobID)
	if err != nil {
		t.Fatalf("get epub job failed: %v", err)
	}
	if job.PackageType != AdminSpaceImportPackageTypeEPUB {
		t.Fatalf("expected epub package type, got %q", job.PackageType)
	}
	if job.RequestedSpaceID != "" {
		t.Fatalf("expected epub commit to ignore client space id, got %q", job.RequestedSpaceID)
	}
	if job.RequestedSpaceName != "EPUB 示例书" {
		t.Fatalf("expected epub default space name from inspect title, got %q", job.RequestedSpaceName)
	}
	if job.RequestedCategoryID != "cat-a" || job.RequestedVisibility != "public" {
		t.Fatalf("expected epub commit to keep category and visibility, got %#v", job)
	}
	if permissionChecks < 2 {
		t.Fatalf("expected inspect and commit to both check create-space capability, got %d checks", permissionChecks)
	}
}

func TestAdminSpaceImportService_Commit_UsesCustomEPUBSpaceName(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()
	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "member-user",
		FileName:    "book.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUB3(t)),
	})
	if err != nil {
		t.Fatalf("inspect epub failed: %v", err)
	}
	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "member-user",
		ImportID:    result.ImportID,
		SpaceName:   "自定义书籍空间",
	})
	if err != nil {
		t.Fatalf("commit epub failed: %v", err)
	}

	job, err := svc.store.GetJob(commitResult.JobID)
	if err != nil {
		t.Fatalf("get epub job failed: %v", err)
	}
	if job.RequestedSpaceName != "自定义书籍空间" {
		t.Fatalf("expected custom epub space name, got %q", job.RequestedSpaceName)
	}
}

func TestAdminSpaceImportService_RestoreEPUBPackageCreatesSpaceTreeDocumentsAndRevisions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
	}
	spaceRepo := &stubAdminSpaceImportSpaceRepo{}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{localRootDir: t.TempDir()}
	imageAssetSyncer := &stubAdminSpaceImportDocumentImageAssetSyncer{}
	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()
	svc.workspaceWriter = workspaceRepo
	svc.spaceWriter = spaceRepo
	svc.attachmentWriter = attachmentRepo
	svc.documentImageAssetSyncer = imageAssetSyncer
	svc.localBlobRootDir = attachmentRepo.localRootDir

	result, err := svc.Inspect(ctx, InspectAdminSpaceImportInput{
		ActorUserID: "member-user",
		FileName:    "book.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUB3(t)),
	})
	if err != nil {
		t.Fatalf("inspect epub failed: %v", err)
	}
	staging, err := svc.store.GetStaging(result.ImportID, "member-user", svc.now())
	if err != nil {
		t.Fatalf("get staging failed: %v", err)
	}
	if _, err := os.Stat(staging.FilePath); err != nil {
		t.Fatalf("expected staging file before restore: %v", err)
	}

	newSpaceID, err := svc.restoreAdminSpaceImportEPUBPackage(ctx, AdminSpaceImportJob{
		JobID:               "job-epub",
		ImportID:            result.ImportID,
		ActorUserID:         "member-user",
		PackageType:         AdminSpaceImportPackageTypeEPUB,
		RequestedSpaceName:  "导入后的 EPUB 空间",
		RequestedVisibility: "public",
	})
	if err != nil {
		t.Fatalf("restore epub failed: %v", err)
	}
	if newSpaceID == "" {
		t.Fatal("expected new space id")
	}
	if _, err := os.Stat(staging.FilePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected staging file to be removed after successful restore, got %v", err)
	}
	if len(workspaceRepo.spaces) != 1 {
		t.Fatalf("expected one created space, got %d", len(workspaceRepo.spaces))
	}
	space := workspaceRepo.spaces[0]
	if space.SpaceID != newSpaceID || space.Name != "导入后的 EPUB 空间" || space.Visibility != models.VisibilityPublic {
		t.Fatalf("unexpected created space: %#v", space)
	}

	folderNode := findAdminSpaceImportCreatedNode(t, workspaceRepo.nodes, models.NodeTypeFolder, "第二部分")
	if folderNode.Node.ParentNodeID != nil {
		t.Fatalf("expected root folder without parent, got %v", *folderNode.Node.ParentNodeID)
	}
	firstChapter := findAdminSpaceImportCreatedNode(t, workspaceRepo.nodes, models.NodeTypeDoc, "第一章")
	if firstChapter.Document == nil || firstChapter.Revision == nil {
		t.Fatalf("expected first chapter document and revision, got %#v", firstChapter)
	}
	if firstChapter.Document.Format != models.DocumentFormatMarkdown {
		t.Fatalf("expected markdown document, got %q", firstChapter.Document.Format)
	}
	if !strings.Contains(firstChapter.Document.ContentMD, "第一章") || !strings.Contains(firstChapter.Document.ContentMD, "正文") {
		t.Fatalf("expected converted markdown content, got %q", firstChapter.Document.ContentMD)
	}
	if firstChapter.Document.ContentMD != firstChapter.Revision.ContentMD ||
		firstChapter.Document.Version != 1 ||
		firstChapter.Revision.Version != 1 ||
		firstChapter.Revision.BaseVersion != 0 {
		t.Fatalf("expected first revision to mirror document content, document=%#v revision=%#v", firstChapter.Document, firstChapter.Revision)
	}

	secondChapter := findAdminSpaceImportCreatedNode(t, workspaceRepo.nodes, models.NodeTypeDoc, "第二章")
	if secondChapter.Node.ParentNodeID == nil || *secondChapter.Node.ParentNodeID != folderNode.Node.NodeID {
		t.Fatalf("expected second chapter under folder %q, got %#v", folderNode.Node.NodeID, secondChapter.Node.ParentNodeID)
	}
	if secondChapter.Document == nil || !strings.Contains(secondChapter.Document.ContentMD, "/uploads/") {
		t.Fatalf("expected second chapter image to be localized, got %#v", secondChapter.Document)
	}
	if len(attachmentRepo.blobs) != 1 {
		t.Fatalf("expected one localized image blob, got %d", len(attachmentRepo.blobs))
	}
	imageAssetSyncer.mu.Lock()
	syncInputs := append([]SyncDocumentImageAssetsInput(nil), imageAssetSyncer.inputs...)
	imageAssetSyncer.mu.Unlock()
	if len(syncInputs) != 2 {
		t.Fatalf("expected image asset sync for both imported EPUB documents, got %#v", syncInputs)
	}
	if syncInputs[1].DocumentID != secondChapter.Document.DocumentID ||
		syncInputs[1].SpaceID != newSpaceID ||
		!strings.Contains(syncInputs[1].ContentMD, "/uploads/") {
		t.Fatalf("unexpected image asset sync input for localized chapter: %#v", syncInputs[1])
	}
}

func TestAdminSpaceImportService_RestoreEPUBPackageTracksLocalizedBlobsWhenMarkdownConversionFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	epubPath := filepath.Join(t.TempDir(), "book.epub")
	if err := os.WriteFile(epubPath, buildAdminSpaceImportTestEPUB3(t), 0o644); err != nil {
		t.Fatalf("write epub fixture failed: %v", err)
	}
	epubPackage, err := readAdminSpaceEPUBImportPackage(epubPath)
	if err != nil {
		t.Fatalf("read epub package failed: %v", err)
	}
	defer func() {
		if closeErr := epubPackage.Close(); closeErr != nil {
			t.Fatalf("close epub package failed: %v", closeErr)
		}
	}()
	nodeSeq := 0
	documentSeq := 0
	plan, _ := planAdminSpaceEPUBImportTree(adminSpaceEPUBPlanInput{
		OPFRoot:                    epubPackage.OPFRoot,
		Items:                      epubPackage.NavItems,
		ChapterHTMLByCanonicalHref: epubPackage.ChapterHTMLByCanonicalHref,
		NewNodeID: func() string {
			nodeSeq++
			return "node-" + strconv.Itoa(nodeSeq)
		},
		NewDocumentID: func() string {
			documentSeq++
			return "doc-" + strconv.Itoa(documentSeq)
		},
	})
	var imageChapter adminSpaceEPUBPlannedNode
	parentNodeID := ""
	if len(plan.Root) >= 2 && len(plan.Root[1].Children) >= 1 {
		imageChapter = plan.Root[1].Children[0]
		parentNodeID = plan.Root[1].NodeID
	}
	if imageChapter.DocumentID == "" {
		t.Fatalf("expected EPUB fixture to contain image chapter, plan=%#v", plan.Root)
	}

	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{localRootDir: t.TempDir()}
	svc := NewAdminSpaceImportService(nil)
	svc.workspaceWriter = &stubAdminSpaceImportWorkspaceRepo{}
	svc.attachmentWriter = attachmentRepo
	svc.localBlobRootDir = attachmentRepo.localRootDir
	importer := adminSpaceEPUBPackageImporter{
		service:        svc,
		job:            AdminSpaceImportJob{JobID: "job-epub-convert-fail", ActorUserID: "member-user"},
		pkg:            epubPackage,
		newSpaceID:     "space-epub-convert-fail",
		plan:           plan,
		converter:      failingHTMLMarkdownConverter{err: errors.New("convert failed")},
		totalDocuments: 1,
		createdBlobs:   make([]models.DocumentAttachmentBlob, 0),
	}

	err = importer.restoreNode(ctx, imageChapter, &parentNodeID, 0)
	if err == nil || !strings.Contains(err.Error(), "convert failed") {
		t.Fatalf("expected markdown conversion failure, got %v", err)
	}
	if len(importer.createdBlobs) != 1 {
		t.Fatalf("expected localized blob to be tracked for cleanup, got %#v", importer.createdBlobs)
	}
}

func TestAdminSpaceImportService_RestoreEPUBPackageUsesCoverAndDescription(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
	}
	spaceRepo := &stubAdminSpaceImportSpaceRepo{}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{localRootDir: t.TempDir()}
	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()
	svc.workspaceWriter = workspaceRepo
	svc.spaceWriter = spaceRepo
	svc.attachmentWriter = attachmentRepo
	svc.localBlobRootDir = attachmentRepo.localRootDir

	result, err := svc.Inspect(ctx, InspectAdminSpaceImportInput{
		ActorUserID: "member-user",
		FileName:    "book.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUB3WithCoverAndDescription(t)),
	})
	if err != nil {
		t.Fatalf("inspect epub failed: %v", err)
	}
	if !result.Space.HasCover {
		t.Fatalf("expected EPUB preview to report cover")
	}

	newSpaceID, err := svc.restoreAdminSpaceImportEPUBPackage(ctx, AdminSpaceImportJob{
		JobID:               "job-epub-cover",
		ImportID:            result.ImportID,
		ActorUserID:         "member-user",
		PackageType:         AdminSpaceImportPackageTypeEPUB,
		RequestedVisibility: "member",
	})
	if err != nil {
		t.Fatalf("restore epub failed: %v", err)
	}
	if len(workspaceRepo.spaces) != 1 {
		t.Fatalf("expected one created space, got %d", len(workspaceRepo.spaces))
	}
	space := workspaceRepo.spaces[0]
	if space.SpaceID != newSpaceID {
		t.Fatalf("expected created space id %q, got %#v", newSpaceID, space)
	}
	if space.Description != "这是 EPUB 简介" {
		t.Fatalf("expected EPUB description to become space description, got %q", space.Description)
	}
	if space.CoverAssetID == nil || strings.TrimSpace(*space.CoverAssetID) == "" {
		t.Fatalf("expected EPUB cover asset id, got %#v", space)
	}
	if space.CoverURL == "" || space.CoverWidth != 2 || space.CoverHeight != 3 || space.CoverSource != string(AdminSpaceCoverSourceUserUpload) {
		t.Fatalf("unexpected EPUB cover fields: %#v", space)
	}
	if len(spaceRepo.coverAssets) != 1 {
		t.Fatalf("expected one EPUB cover asset, got %#v", spaceRepo.coverAssets)
	}
	coverAsset := spaceRepo.coverAssets[0]
	if coverAsset.AssetID != *space.CoverAssetID ||
		coverAsset.MimeType != "image/webp" ||
		coverAsset.Width != 2 ||
		coverAsset.Height != 3 ||
		!coverAsset.Normalized ||
		coverAsset.Source != string(AdminSpaceCoverSourceUserUpload) {
		t.Fatalf("unexpected persisted EPUB cover asset: %#v", coverAsset)
	}
}

func TestAdminSpaceImportService_RestoreEPUBPackageCleansSpaceOnCreateNodeFailureAndKeepsStaging(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
		createNodeErr:   errors.New("create node failed"),
	}
	spaceRepo := &stubAdminSpaceImportSpaceRepo{}
	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()
	svc.workspaceWriter = workspaceRepo
	svc.spaceWriter = spaceRepo
	svc.attachmentWriter = &stubAdminSpaceImportAttachmentRepo{localRootDir: t.TempDir()}
	svc.localBlobRootDir = t.TempDir()

	result, err := svc.Inspect(ctx, InspectAdminSpaceImportInput{
		ActorUserID: "member-user",
		FileName:    "book.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUB3(t)),
	})
	if err != nil {
		t.Fatalf("inspect epub failed: %v", err)
	}
	staging, err := svc.store.GetStaging(result.ImportID, "member-user", svc.now())
	if err != nil {
		t.Fatalf("get staging failed: %v", err)
	}

	newSpaceID, err := svc.restoreAdminSpaceImportEPUBPackage(ctx, AdminSpaceImportJob{
		JobID:       "job-epub-fail",
		ImportID:    result.ImportID,
		ActorUserID: "member-user",
		PackageType: AdminSpaceImportPackageTypeEPUB,
	})
	if err == nil {
		t.Fatal("expected restore epub to fail")
	}
	if newSpaceID == "" {
		t.Fatal("expected restore to return created space id for audit target")
	}
	if len(spaceRepo.hardDeletedSpaceIDs) != 1 || spaceRepo.hardDeletedSpaceIDs[0] != newSpaceID {
		t.Fatalf("expected failed restore to hard delete created space %q, got %#v", newSpaceID, spaceRepo.hardDeletedSpaceIDs)
	}
	if _, statErr := os.Stat(staging.FilePath); statErr != nil {
		t.Fatalf("expected failed restore to keep staging file for short-term investigation, got %v", statErr)
	}
}

func TestAdminSpaceImportService_RestoreEPUBPackageUsesEPUB2TOCTree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
	}
	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()
	svc.workspaceWriter = workspaceRepo
	svc.spaceWriter = &stubAdminSpaceImportSpaceRepo{}
	svc.attachmentWriter = &stubAdminSpaceImportAttachmentRepo{localRootDir: t.TempDir()}
	svc.localBlobRootDir = t.TempDir()

	result, err := svc.Inspect(ctx, InspectAdminSpaceImportInput{
		ActorUserID: "member-user",
		FileName:    "book.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUB2(t)),
	})
	if err != nil {
		t.Fatalf("inspect epub2 failed: %v", err)
	}
	if _, err := svc.restoreAdminSpaceImportEPUBPackage(ctx, AdminSpaceImportJob{
		JobID:       "job-epub2",
		ImportID:    result.ImportID,
		ActorUserID: "member-user",
		PackageType: AdminSpaceImportPackageTypeEPUB,
	}); err != nil {
		t.Fatalf("restore epub2 failed: %v", err)
	}

	folderNode := findAdminSpaceImportCreatedNode(t, workspaceRepo.nodes, models.NodeTypeFolder, "第一部分")
	bodyNode := findAdminSpaceImportCreatedNode(t, workspaceRepo.nodes, models.NodeTypeDoc, "正文")
	if bodyNode.Node.ParentNodeID == nil || *bodyNode.Node.ParentNodeID != folderNode.Node.NodeID {
		t.Fatalf("expected toc parent body doc under folder, got %#v", bodyNode.Node.ParentNodeID)
	}
	childNode := findAdminSpaceImportCreatedNode(t, workspaceRepo.nodes, models.NodeTypeDoc, "第二章")
	if childNode.Node.ParentNodeID == nil || *childNode.Node.ParentNodeID != folderNode.Node.NodeID {
		t.Fatalf("expected toc child doc under folder, got %#v", childNode.Node.ParentNodeID)
	}
}

func TestAdminSpaceImportService_RunEPUBJobPublishesProgressAndCompletedNewSpaceID(t *testing.T) {
	t.Parallel()

	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-import-epub-progress?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()
	ctx := context.Background()
	if err := storage.MigrateUp(ctx, database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}

	transferJobRepo := repository.NewGormAdminSpaceTransferJobRepository(database.ORM)
	svc := NewAdminSpaceImportService(nil, WithAdminSpaceImportTransferJobRepository(transferJobRepo))
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(ctx, InspectAdminSpaceImportInput{
		ActorUserID: "member-user",
		FileName:    "book.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUB3(t)),
	})
	if err != nil {
		t.Fatalf("inspect epub failed: %v", err)
	}
	commitResult, err := svc.Commit(ctx, CommitAdminSpaceImportInput{
		ActorUserID: "member-user",
		ImportID:    result.ImportID,
		SpaceName:   "进度 EPUB 空间",
	})
	if err != nil {
		t.Fatalf("commit epub failed: %v", err)
	}
	token := tokenQueryValue(t, commitResult.StreamURL)
	initial, events, unsubscribe, err := svc.Subscribe(ctx, commitResult.JobID, "member-user", token)
	if err != nil {
		t.Fatalf("subscribe epub import failed: %v", err)
	}
	defer unsubscribe()
	if initial.Stage != "queued" {
		t.Fatalf("expected queued initial event, got %#v", initial)
	}

	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
	}
	svc.workspaceWriter = workspaceRepo
	svc.spaceWriter = &stubAdminSpaceImportSpaceRepo{}
	svc.attachmentWriter = &stubAdminSpaceImportAttachmentRepo{localRootDir: t.TempDir()}
	svc.localBlobRootDir = t.TempDir()

	svc.runAdminSpaceImportJob(ctx, commitResult.JobID)
	received := collectAdminSpaceImportEventsUntilTerminal(t, events)
	stages := make(map[string]bool, len(received))
	var completed AdminSpaceTransferEvent
	documentProgresses := make([]int, 0, 2)
	for _, event := range received {
		stages[event.Stage] = true
		if event.Stage == "epub_documents" {
			documentProgresses = append(documentProgresses, event.Progress)
		}
		if event.Type == AdminSpaceTransferEventTypeCompleted {
			completed = event
		}
	}
	for _, stage := range []string{"running", "epub_parse", "epub_space", "epub_convert", "epub_documents", "epub_done", "completed"} {
		if !stages[stage] {
			t.Fatalf("expected epub import progress stage %q, got events %#v", stage, received)
		}
	}
	if len(documentProgresses) != 2 || documentProgresses[0] != 62 || documentProgresses[1] != 90 {
		t.Fatalf("expected document progress to follow imported/total documents 62 -> 90, got %#v events=%#v", documentProgresses, received)
	}
	if completed.NewSpaceID == "" || completed.SpaceID != completed.NewSpaceID {
		t.Fatalf("expected completed event to include spaceId and newSpaceId, got %#v", completed)
	}
	persistedJob, err := transferJobRepo.GetByKindAndJobID(ctx, models.AdminSpaceTransferJobKindImport, commitResult.JobID)
	if err != nil {
		t.Fatalf("get persisted transfer job failed: %v", err)
	}
	if persistedJob.Status != models.AdminSpaceTransferJobStatusCompleted || persistedJob.NewSpaceID != completed.NewSpaceID {
		t.Fatalf("unexpected persisted transfer job: %#v completed=%#v", persistedJob, completed)
	}
}

func TestAdminSpaceImportStore_PublishKeepsTerminalEventWhenSubscriberBufferIsFull(t *testing.T) {
	t.Parallel()

	store := NewAdminSpaceImportStore()
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	token := "stream-token"
	if err := store.CreateJob(AdminSpaceImportJob{
		JobID:                "job-buffer",
		ImportID:             "import-buffer",
		ActorUserID:          "actor-user",
		Status:               AdminSpaceImportStatusRunning,
		StreamTokenHash:      tokenHash(token),
		StreamTokenExpiresAt: now.Add(time.Minute),
		CreatedAt:            now,
		UpdatedAt:            now,
	}); err != nil {
		t.Fatalf("create job failed: %v", err)
	}
	_, events, unsubscribe, err := store.Subscribe("job-buffer", "actor-user", token, now)
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer unsubscribe()

	for index := 0; index < adminSpaceTransferEventBufferSize+3; index++ {
		store.Publish("job-buffer", AdminSpaceTransferEvent{
			Type:     AdminSpaceTransferEventTypeProgress,
			Stage:    "epub_convert",
			Progress: index,
			Message:  "章节进度",
		}, now)
	}
	store.Complete("job-buffer", "new-space", now)

	eventsSeen := make([]AdminSpaceTransferEvent, 0, adminSpaceTransferEventBufferSize+1)
	for {
		select {
		case event := <-events:
			eventsSeen = append(eventsSeen, event)
			if event.Type == AdminSpaceTransferEventTypeCompleted {
				if event.NewSpaceID != "new-space" {
					t.Fatalf("expected completed newSpaceId, got %#v", event)
				}
				return
			}
		case <-time.After(time.Second):
			t.Fatalf("expected terminal event to survive full subscriber buffer, got %#v", eventsSeen)
		}
	}
}

func TestAdminSpaceImportService_Inspect_RejectsEPUBWithoutCreateSpaceCapability(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.canImportSpace = func(context.Context, string) (bool, error) {
		return false, nil
	}
	_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "member-user",
		FileName:    "book.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUB3(t)),
	})

	if !errors.Is(err, errcode.ErrAdminSpaceImportCommitForbidden) {
		t.Fatalf("expected epub inspect to require create-space capability, got %v", err)
	}
}

func TestAdminSpaceImportService_Inspect_RejectsMissingFile(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)

	_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
	})

	if !errors.Is(err, errcode.ErrAdminSpaceImportFileRequired) {
		t.Fatalf("expected file required error, got %v", err)
	}
}

func TestAdminSpaceImportService_Inspect_RejectsEmptyAndUnsupportedUpload(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		fileName    string
		contentType string
		reader      *bytes.Reader
		want        error
	}{
		{
			name:        "empty zip",
			fileName:    "space.plaindoc",
			contentType: "application/zip",
			reader:      bytes.NewReader(nil),
			want:        errcode.ErrAdminSpaceImportZipInvalid,
		},
		{
			name:        "zip extension",
			fileName:    "space.zip",
			contentType: "application/zip",
			reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
			want:        errcode.ErrAdminSpaceImportPackageUnsupported,
		},
		{
			name:        "plain text",
			fileName:    "space.txt",
			contentType: "text/plain",
			reader:      bytes.NewReader([]byte("not a zip")),
			want:        errcode.ErrAdminSpaceImportPackageUnsupported,
		},
		{
			name:        "oversized header",
			fileName:    "space.plaindoc",
			contentType: "application/zip",
			reader:      bytes.NewReader([]byte("zip")),
			want:        errcode.ErrAdminSpaceImportZipInvalid,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := NewAdminSpaceImportService(nil)
			svc.stagingDir = t.TempDir()
			input := InspectAdminSpaceImportInput{
				ActorUserID: "actor-user",
				FileName:    tc.fileName,
				ContentType: tc.contentType,
				Reader:      tc.reader,
			}
			if tc.name == "oversized header" {
				input.SizeBytes = maxAdminSpaceImportUploadBytes + 1
			}
			_, err := svc.Inspect(context.Background(), input)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestAdminSpaceImportService_Inspect_AcceptsEPUBPreview(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "demo.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUB3(t)),
	})
	if err != nil {
		t.Fatalf("inspect epub failed: %v", err)
	}

	if result.PackageType != AdminSpaceImportPackageTypeEPUB {
		t.Fatalf("expected epub package type, got %q", result.PackageType)
	}
	if result.ExportedAt != "" {
		t.Fatalf("epub inspect must not expose exportedAt, got %q", result.ExportedAt)
	}
	if result.SourcePublishedAt != "2026-05-17" {
		t.Fatalf("expected source published date, got %q", result.SourcePublishedAt)
	}
	if len(result.SourceAuthors) != 1 || result.SourceAuthors[0] != "作者甲" {
		t.Fatalf("expected epub author, got %#v", result.SourceAuthors)
	}
	if result.Space.Name != "EPUB 示例书" {
		t.Fatalf("expected epub title as preview space name, got %q", result.Space.Name)
	}
	if result.Summary.DocumentCount != 2 {
		t.Fatalf("expected two spine documents, got %d", result.Summary.DocumentCount)
	}
	if result.Summary.ImageCount != 1 {
		t.Fatalf("expected one image resource, got %d", result.Summary.ImageCount)
	}
	if result.Summary.MaxDepth != 2 {
		t.Fatalf("expected max depth 2, got %d", result.Summary.MaxDepth)
	}
	if result.Warnings == nil {
		t.Fatal("expected empty warnings slice instead of nil warnings")
	}
	if !result.Importable {
		t.Fatal("expected epub preview to be importable")
	}
}

func TestAdminSpaceImportService_Inspect_RejectsInvalidEPUBPreview(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		epub []byte
		want error
	}{
		{
			name: "missing mimetype",
			epub: buildAdminSpaceImportTestEPUB3WithOptions(t, false, true, true, true),
			want: errcode.ErrAdminSpaceImportZipInvalid,
		},
		{
			name: "missing container",
			epub: buildAdminSpaceImportTestEPUB3WithOptions(t, true, false, true, true),
			want: errcode.ErrAdminSpaceImportZipInvalid,
		},
		{
			name: "missing opf",
			epub: buildAdminSpaceImportTestEPUB3WithOptions(t, true, true, false, true),
			want: errcode.ErrAdminSpaceImportZipInvalid,
		},
		{
			name: "missing spine",
			epub: buildAdminSpaceImportTestEPUB3WithOptions(t, true, true, true, false),
			want: errcode.ErrAdminSpaceImportPackageNotImportable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := NewAdminSpaceImportService(nil)
			svc.stagingDir = t.TempDir()
			_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
				ActorUserID: "actor-user",
				FileName:    "demo.epub",
				ContentType: "application/epub+zip",
				Reader:      bytes.NewReader(tc.epub),
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestCollectAdminSpaceEPUBEntries_RejectsUnsafeEntries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		files []*zip.File
	}{
		{
			name: "absolute path",
			files: []*zip.File{
				buildAdminSpaceEPUBZipFileForTest("/OPS/content.opf", 1),
			},
		},
		{
			name: "parent traversal",
			files: []*zip.File{
				buildAdminSpaceEPUBZipFileForTest("OPS/../content.opf", 1),
			},
		},
		{
			name: "duplicate entry",
			files: []*zip.File{
				buildAdminSpaceEPUBZipFileForTest("OPS/content.opf", 1),
				buildAdminSpaceEPUBZipFileForTest("OPS/content.opf", 1),
			},
		},
		{
			name: "single entry too large",
			files: []*zip.File{
				buildAdminSpaceEPUBZipFileForTest("OPS/content.opf", uint64(maxAdminSpaceEPUBEntryBytes+1)),
			},
		},
		{
			name: "total uncompressed size too large",
			files: []*zip.File{
				buildAdminSpaceEPUBZipFileForTest("OPS/a.xhtml", uint64(maxAdminSpaceEPUBEntryBytes)),
				buildAdminSpaceEPUBZipFileForTest("OPS/b.xhtml", uint64(maxAdminSpaceEPUBEntryBytes)),
				buildAdminSpaceEPUBZipFileForTest("OPS/c.xhtml", uint64(maxAdminSpaceEPUBEntryBytes)),
				buildAdminSpaceEPUBZipFileForTest("OPS/d.xhtml", uint64(maxAdminSpaceEPUBEntryBytes)),
				buildAdminSpaceEPUBZipFileForTest("OPS/e.xhtml", 1),
			},
		},
		{
			name: "directory depth too large",
			files: []*zip.File{
				buildAdminSpaceEPUBZipFileForTest("OPS/a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p/q.xhtml", 1),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := collectAdminSpaceEPUBEntries(&zip.Reader{File: tc.files})
			if !errors.Is(err, errcode.ErrAdminSpaceImportZipInvalid) {
				t.Fatalf("expected invalid zip error, got %v", err)
			}
		})
	}
}

func TestCollectAdminSpaceEPUBEntries_RejectsTooManyEntries(t *testing.T) {
	t.Parallel()

	files := make([]*zip.File, 0, maxAdminSpaceEPUBEntries+1)
	for i := 0; i <= maxAdminSpaceEPUBEntries; i++ {
		files = append(files, buildAdminSpaceEPUBZipFileForTest("OPS/chapter-"+strconv.Itoa(i)+".xhtml", 1))
	}

	_, err := collectAdminSpaceEPUBEntries(&zip.Reader{File: files})
	if !errors.Is(err, errcode.ErrAdminSpaceImportZipInvalid) {
		t.Fatalf("expected invalid zip error, got %v", err)
	}
}

func TestAdminSpaceImportService_Inspect_AcceptsEPUB2TOCPreview(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "epub2.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUB2(t)),
	})
	if err != nil {
		t.Fatalf("inspect epub2 failed: %v", err)
	}
	if result.Space.Name != "EPUB2 示例书" {
		t.Fatalf("expected epub2 title, got %q", result.Space.Name)
	}
	if result.Summary.DocumentCount != 2 {
		t.Fatalf("expected two spine documents, got %d", result.Summary.DocumentCount)
	}
	if result.Summary.MaxDepth != 2 {
		t.Fatalf("expected toc depth 2, got %d", result.Summary.MaxDepth)
	}
}

func TestAdminSpaceImportService_Inspect_FallsBackToFlatSpineWithoutNavOrTOC(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "flat.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUBWithoutNavOrTOC(t)),
	})
	if err != nil {
		t.Fatalf("inspect flat epub failed: %v", err)
	}
	if result.Summary.DocumentCount != 2 {
		t.Fatalf("expected two spine documents, got %d", result.Summary.DocumentCount)
	}
	if result.Summary.MaxDepth != 1 {
		t.Fatalf("expected flat depth 1, got %d", result.Summary.MaxDepth)
	}
}

func TestReadAdminSpaceEPUBNavItems_RebasesSubdirNavHrefToOPFRoot(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/Text/nav.xhtml", []byte(`<!doctype html>
<html xmlns="http://www.w3.org/1999/xhtml">
  <body>
    <nav epub:type="toc"><ol><li><a href="chapter1.xhtml#intro">第一章</a></li></ol></nav>
  </body>
</html>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/Text/chapter1.xhtml", []byte(`<html><body><h1 id="intro">第一章</h1></body></html>`))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close epub failed: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatalf("open zip reader failed: %v", err)
	}
	entries, err := collectAdminSpaceEPUBEntries(reader)
	if err != nil {
		t.Fatalf("collect epub entries failed: %v", err)
	}

	items := readAdminSpaceEPUBNavItems(entries, "OPS", adminSpaceEPUBOPFPackage{
		Manifest: []adminSpaceEPUBOPFItem{
			{ID: "nav", Href: "Text/nav.xhtml", MediaType: "application/xhtml+xml", Properties: "nav"},
			{ID: "chapter-1", Href: "Text/chapter1.xhtml", MediaType: "application/xhtml+xml"},
		},
	}, map[string]adminSpaceEPUBOPFItem{
		"nav":       {ID: "nav", Href: "Text/nav.xhtml", MediaType: "application/xhtml+xml", Properties: "nav"},
		"chapter-1": {ID: "chapter-1", Href: "Text/chapter1.xhtml", MediaType: "application/xhtml+xml"},
	})

	if len(items) != 1 {
		t.Fatalf("expected one nav item, got %#v", items)
	}
	if got, want := items[0].Href, "Text/chapter1.xhtml#intro"; got != want {
		t.Fatalf("expected rebased nav href %q, got %q", want, got)
	}
}

func TestReadAdminSpaceEPUBNavItems_RebasesSubdirTOCHrefToOPFRoot(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/Text/toc.ncx", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <navMap>
    <navPoint><navLabel><text>第一章</text></navLabel><content src="chapter1.xhtml#intro"/></navPoint>
  </navMap>
</ncx>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/Text/chapter1.xhtml", []byte(`<html><body><h1 id="intro">第一章</h1></body></html>`))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close epub failed: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatalf("open zip reader failed: %v", err)
	}
	entries, err := collectAdminSpaceEPUBEntries(reader)
	if err != nil {
		t.Fatalf("collect epub entries failed: %v", err)
	}

	items := readAdminSpaceEPUBNavItems(entries, "OPS", adminSpaceEPUBOPFPackage{
		Manifest: []adminSpaceEPUBOPFItem{
			{ID: "toc", Href: "Text/toc.ncx", MediaType: "application/x-dtbncx+xml"},
			{ID: "chapter-1", Href: "Text/chapter1.xhtml", MediaType: "application/xhtml+xml"},
		},
	}, map[string]adminSpaceEPUBOPFItem{
		"toc":       {ID: "toc", Href: "Text/toc.ncx", MediaType: "application/x-dtbncx+xml"},
		"chapter-1": {ID: "chapter-1", Href: "Text/chapter1.xhtml", MediaType: "application/xhtml+xml"},
	})

	if len(items) != 1 {
		t.Fatalf("expected one toc item, got %#v", items)
	}
	if got, want := items[0].Href, "Text/chapter1.xhtml#intro"; got != want {
		t.Fatalf("expected rebased toc href %q, got %q", want, got)
	}
}

func TestAdminSpaceImportService_Inspect_WarnsForNonStandardMediaTypeFallback(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "fallback.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUBWithNonStandardMediaTypes(t)),
	})
	if err != nil {
		t.Fatalf("inspect fallback epub failed: %v", err)
	}
	if result.Summary.DocumentCount != 1 || result.Summary.ImageCount != 1 {
		t.Fatalf("expected extension fallback counts, got summary %#v", result.Summary)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for non-standard media type fallback")
	}
}

func TestAdminSpaceImportService_Inspect_WarnsForNonUTF8XMLDeclaration(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "latin.epub",
		ContentType: "application/epub+zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUBWithNonUTF8OPFDeclaration(t)),
	})
	if err != nil {
		t.Fatalf("inspect non utf-8 epub failed: %v", err)
	}
	if result.Space.Name != "Latin EPUB" {
		t.Fatalf("expected title from non utf-8 declared opf, got %q", result.Space.Name)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for non utf-8 xml declaration")
	}
}

func TestAdminSpaceImportService_LocalizeEPUBImagesUsesImportedBlobStorage(t *testing.T) {
	t.Parallel()

	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(nil, nil, nil, attachmentRepo),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
	)
	entries := collectAdminSpaceEPUBEntriesForImageTest(t, map[string][]byte{
		"OPS/images/cover.png": []byte("png-payload"),
	})

	rewritten, warnings, createdBlobs, err := svc.localizeAdminSpaceEPUBChapterImages(
		context.Background(),
		adminSpaceEPUBImageLocalizeInput{
			SourceKey:           "OPS/chapter.xhtml",
			SourceCanonicalHref: "OPS/chapter.xhtml",
			HTML:                []byte(`<body><img src="images/cover.png" alt="封面"></body>`),
			Entries:             entries,
		},
		"space-1",
		"doc-1",
	)
	if err != nil {
		t.Fatalf("localizeAdminSpaceEPUBChapterImages returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if len(createdBlobs) != 1 {
		t.Fatalf("expected one created blob, got %#v", createdBlobs)
	}
	if !strings.Contains(rewritten, createdBlobs[0].ObjectURL) || !strings.HasPrefix(createdBlobs[0].ObjectURL, "/uploads/") {
		t.Fatalf("expected rewritten html to use local upload url, html=%q blobs=%#v", rewritten, createdBlobs)
	}
	if createdBlobs[0].MimeType != "image/png" {
		t.Fatalf("expected image/png blob, got %#v", createdBlobs[0])
	}
}

func TestAdminSpaceImportService_Commit_RejectsOtherActorsStaging(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	markStagingImportableForTest(svc, result.ImportID, "actor-user")

	_, err = svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "other-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})

	if !errors.Is(err, errcode.ErrAdminSpaceImportStagingNotFound) {
		t.Fatalf("expected staging not found for other actor, got %v", err)
	}
}

func TestAdminSpaceImportService_Commit_RejectsExpiredStaging(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	svc := NewAdminSpaceImportService(nil)
	svc.nowFn = func() time.Time { return now }
	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	markStagingImportableForTest(svc, result.ImportID, "actor-user")

	svc.nowFn = func() time.Time { return now.Add(defaultAdminSpaceImportStagingTTL + time.Second) }
	_, err = svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})

	if !errors.Is(err, errcode.ErrAdminSpaceImportStagingExpired) {
		t.Fatalf("expected staging expired error, got %v", err)
	}
}

func TestAdminSpaceImportService_StreamToken_BindsToJobAndActor(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	markStagingImportableForTest(svc, result.ImportID, "actor-user")
	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	token := tokenQueryValue(t, commitResult.StreamURL)
	if _, _, _, err := svc.Subscribe(context.Background(), commitResult.JobID, "actor-user", token); err != nil {
		t.Fatalf("subscribe with correct token failed: %v", err)
	}
	if _, _, _, err := svc.Subscribe(context.Background(), commitResult.JobID, "other-user", token); !errors.Is(err, errcode.ErrAdminSpaceImportJobTokenInvalid) {
		t.Fatalf("expected token invalid for other actor, got %v", err)
	}
}

func TestAdminSpaceImportService_Commit_PersistsStagingFilePathInTransferJob(t *testing.T) {
	t.Parallel()

	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-import-transfer-file-path?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()
	ctx := context.Background()
	if err := storage.MigrateUp(ctx, database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}

	transferJobRepo := repository.NewGormAdminSpaceTransferJobRepository(database.ORM)
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportTransferJobRepository(transferJobRepo),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(ctx, InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	staging, err := svc.store.GetStaging(result.ImportID, "actor-user", svc.now())
	if err != nil {
		t.Fatalf("get staging failed: %v", err)
	}
	markStagingImportableForTest(svc, result.ImportID, "actor-user")

	commitResult, err := svc.Commit(ctx, CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	job, err := transferJobRepo.GetByKindAndJobID(ctx, models.AdminSpaceTransferJobKindImport, commitResult.JobID)
	if err != nil {
		t.Fatalf("get transfer job failed: %v", err)
	}
	if strings.TrimSpace(job.FilePath) != staging.FilePath {
		t.Fatalf("expected transfer job file path %q, got %q", staging.FilePath, job.FilePath)
	}
}

func TestAdminSpaceImportService_IssueStreamURL_ReplacesExpiredToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	svc := NewAdminSpaceImportService(nil)
	svc.nowFn = func() time.Time { return now }
	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	markStagingImportableForTest(svc, result.ImportID, "actor-user")
	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	oldToken := tokenQueryValue(t, commitResult.StreamURL)

	svc.nowFn = func() time.Time { return now.Add(defaultAdminSpaceTransferTokenTTL + time.Second) }
	streamURL, err := svc.IssueStreamURL(context.Background(), "actor-user", commitResult.JobID)
	if err != nil {
		t.Fatalf("issue stream url failed: %v", err)
	}
	newToken := tokenQueryValue(t, streamURL)
	if newToken == "" || newToken == oldToken {
		t.Fatalf("expected a fresh stream token, got %q", newToken)
	}
	if _, _, _, err := svc.Subscribe(context.Background(), commitResult.JobID, "actor-user", oldToken); !errors.Is(err, errcode.ErrAdminSpaceImportJobTokenInvalid) {
		t.Fatalf("expected old token invalid, got %v", err)
	}
	if _, _, _, err := svc.Subscribe(context.Background(), commitResult.JobID, "actor-user", newToken); err != nil {
		t.Fatalf("subscribe with fresh token failed: %v", err)
	}
}

func TestAdminSpaceImportService_Commit_RecordsQueuedAudit(t *testing.T) {
	t.Parallel()

	recorder := &recordingAdminAuditRecorder{}
	svc := NewAdminSpaceImportService(nil, WithAdminSpaceImportAuditRecorder(recorder))
	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	markStagingImportableForTest(svc, result.ImportID, "actor-user")

	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
		SpaceID:     "target-space",
		CategoryID:  "cat-a",
		Visibility:  "authenticated",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	records := recorder.Records()
	if len(records) != 1 {
		t.Fatalf("expected one audit record, got %d", len(records))
	}
	record, detail := findAuditRecordByStatus(t, records, "queued")
	if record.ActorUserID != "actor-user" ||
		record.Module != AdminAuditModuleSpace ||
		record.Action != AdminAuditActionImport ||
		record.TargetType != "space_import" ||
		record.TargetID != result.ImportID {
		t.Fatalf("unexpected audit record: %#v", record)
	}
	if detail["jobId"] != commitResult.JobID ||
		detail["importId"] != result.ImportID ||
		detail["requestedSpaceId"] != "target-space" ||
		detail["requestedSpaceName"] != "导入空间" ||
		detail["requestedCategoryId"] != "cat-a" ||
		detail["requestedVisibility"] != "authenticated" ||
		detail["abilityType"] != "space_create" {
		t.Fatalf("unexpected audit detail: %#v", detail)
	}
	assertAuditDetailHasNoTransferSecret(t, detail)
}

func TestAdminSpaceImportService_Inspect_ParsesImportableSpacePackage(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if result.ImportID == "" {
		t.Fatalf("expected import id")
	}
	if !result.Importable {
		t.Fatalf("expected importable package")
	}
	if result.PackageType != AdminSpaceExportPackageType || result.PackageVersion != AdminSpaceExportPackageVersion {
		t.Fatalf("unexpected package metadata: %#v", result)
	}
	if result.Space.SpaceID != "space-source" || result.Space.Name != "源空间" || result.Space.Visibility != "member" {
		t.Fatalf("unexpected preview space: %#v", result.Space)
	}
	if result.Summary.DocumentCount != 1 || result.Summary.AttachmentCount != 1 || result.Summary.OfficeSourceCount != 1 {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", result.Warnings)
	}

	staging, err := svc.store.GetStaging(result.ImportID, "actor-user", svc.now())
	if err != nil {
		t.Fatalf("get staging failed: %v", err)
	}
	if staging.FilePath == "" || strings.Contains(staging.FilePath, "uploads") {
		t.Fatalf("expected private staging path, got %q", staging.FilePath)
	}
	if _, err := os.Stat(staging.FilePath); err != nil {
		t.Fatalf("expected staging file to exist: %v", err)
	}
}

func TestAdminSpaceImportService_Inspect_RejectsMissingManifestAndTree(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		data []byte
		want error
	}{
		{
			name: "missing manifest",
			data: buildAdminSpaceImportTestZip(t, false, true, true),
			want: errcode.ErrAdminSpaceImportManifestMissing,
		},
		{
			name: "missing tree",
			data: buildAdminSpaceImportTestZip(t, true, false, true),
			want: errcode.ErrAdminSpaceImportTreeMissing,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := NewAdminSpaceImportService(nil)
			svc.stagingDir = t.TempDir()
			_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
				ActorUserID: "actor-user",
				FileName:    "space.plaindoc",
				ContentType: "application/zip",
				Reader:      bytes.NewReader(tc.data),
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestAdminSpaceImportService_Inspect_RejectsEPUBPayloadWithPlaindocExtension(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "book.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestEPUB(t)),
	})

	if !errors.Is(err, errcode.ErrAdminSpaceImportPackageUnsupported) {
		t.Fatalf("expected package unsupported error, got %v", err)
	}
}

func TestAdminSpaceImportService_Inspect_RejectsMissingReferencedFiles(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, false)),
	})

	if !errors.Is(err, errcode.ErrAdminSpaceImportPackageNotImportable) {
		t.Fatalf("expected package not importable error, got %v", err)
	}
}

func TestAdminSpaceImportService_Inspect_RejectsMissingAttachmentEntryFile(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportMissingAttachmentEntryZip(t)),
	})

	if !errors.Is(err, errcode.ErrAdminSpaceImportPackageNotImportable) {
		t.Fatalf("expected package not importable error, got %v", err)
	}
}

func TestAdminSpaceImportService_Inspect_RejectsMismatchedManifestAndTreeRoots(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportMismatchedRootZip(t)),
	})

	if !errors.Is(err, errcode.ErrAdminSpaceImportTreeMissing) {
		t.Fatalf("expected tree missing error, got %v", err)
	}
}

func TestAdminSpaceImportService_Inspect_RejectsUnsafeZipEntry(t *testing.T) {
	t.Parallel()

	cases := []string{
		"../manifest.json",
		"/manifest.json",
		"C:/manifest.json",
		"space-space-source/../manifest.json",
		"space-space-source\\manifest.json",
		"space-space-source\\..\\manifest.json",
	}
	for _, entryName := range cases {
		t.Run(entryName, func(t *testing.T) {
			t.Parallel()

			svc := NewAdminSpaceImportService(nil)
			svc.stagingDir = t.TempDir()

			_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
				ActorUserID: "actor-user",
				FileName:    "space.plaindoc",
				ContentType: "application/zip",
				Reader:      bytes.NewReader(buildAdminSpaceImportUnsafeEntryZip(t, entryName)),
			})

			if !errors.Is(err, errcode.ErrAdminSpaceImportZipInvalid) {
				t.Fatalf("expected zip invalid error, got %v", err)
			}
		})
	}
}

func TestAdminSpaceImportService_Inspect_RejectsDuplicateZipEntry(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportDuplicateEntryZip(t)),
	})

	if !errors.Is(err, errcode.ErrAdminSpaceImportZipInvalid) {
		t.Fatalf("expected zip invalid error, got %v", err)
	}
}

func TestAdminSpaceImportService_Inspect_RejectsLargeMetadataEntry(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportLargeManifestZip(t)),
	})

	if !errors.Is(err, errcode.ErrAdminSpaceImportZipInvalid) {
		t.Fatalf("expected zip invalid error, got %v", err)
	}
}

func TestAdminSpaceImportService_Inspect_RejectsImportableDocumentWithEmptyPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		data []byte
	}{
		{name: "document path", data: buildAdminSpaceImportEmptyDocumentPathZip(t)},
		{name: "office source path", data: buildAdminSpaceImportEmptySourcePathZip(t)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := NewAdminSpaceImportService(nil)
			svc.stagingDir = t.TempDir()

			_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
				ActorUserID: "actor-user",
				FileName:    "space.plaindoc",
				ContentType: "application/zip",
				Reader:      bytes.NewReader(tc.data),
			})

			if !errors.Is(err, errcode.ErrAdminSpaceImportPackageNotImportable) {
				t.Fatalf("expected package not importable error, got %v", err)
			}
		})
	}
}

func TestAdminSpaceImportService_Inspect_AllowsPreviewForNotImportablePackage(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZipWithImportable(t, false)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if result.Importable {
		t.Fatalf("expected not importable preview")
	}
	if result.Space.Name != "源空间" || len(result.Warnings) == 0 {
		t.Fatalf("expected preview with warning, got %#v", result)
	}

	_, err = svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if !errors.Is(err, errcode.ErrAdminSpaceImportPackageNotImportable) {
		t.Fatalf("expected package not importable on commit, got %v", err)
	}
}

func TestAdminSpaceImportService_Commit_StartsWorkerAndRestoresMarkdownTree(t *testing.T) {
	t.Parallel()

	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
	}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(nil, workspaceRepo, nil, attachmentRepo),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}

	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
		Visibility:  "authenticated",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	if commitResult.JobID == "" {
		t.Fatalf("expected job id")
	}

	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusCompleted
	})

	workspaceRepo.mu.Lock()
	defer workspaceRepo.mu.Unlock()
	if len(workspaceRepo.spaces) != 1 {
		t.Fatalf("expected one imported space, got %d", len(workspaceRepo.spaces))
	}
	importedSpace := workspaceRepo.spaces[0]
	if importedSpace.SpaceID == "space-source" {
		t.Fatalf("expected new space id, got source id")
	}
	if importedSpace.Name != "导入空间" || importedSpace.Visibility != models.VisibilityAuthenticated {
		t.Fatalf("unexpected imported space: %#v", importedSpace)
	}
	if len(workspaceRepo.nodes) != 2 {
		t.Fatalf("expected folder and document nodes, got %d", len(workspaceRepo.nodes))
	}
	if workspaceRepo.nodes[0].Node.Type != models.NodeTypeFolder || workspaceRepo.nodes[0].Node.ParentNodeID != nil {
		t.Fatalf("unexpected folder node: %#v", workspaceRepo.nodes[0])
	}
	docParams := workspaceRepo.nodes[1]
	if docParams.Node.Type != models.NodeTypeDoc || docParams.Document == nil || docParams.Revision == nil {
		t.Fatalf("expected document node with document and revision, got %#v", docParams)
	}
	if docParams.Node.ParentNodeID == nil || *docParams.Node.ParentNodeID != workspaceRepo.nodes[0].Node.NodeID {
		t.Fatalf("expected document parent to use new folder id, got %#v", docParams.Node.ParentNodeID)
	}
	if docParams.Document.ThemeID != "default" {
		t.Fatalf("expected imported document to use default theme, got %q", docParams.Document.ThemeID)
	}
	if docParams.Document.ContentMD != "# A" || docParams.Revision.ContentMD != "# A" {
		t.Fatalf("expected markdown content restored, got document=%q revision=%q", docParams.Document.ContentMD, docParams.Revision.ContentMD)
	}
}

func TestAdminSpaceImportService_Commit_RestoresSpaceCover(t *testing.T) {
	t.Parallel()

	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
	}
	spaceRepo := &stubAdminSpaceImportSpaceRepo{}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(spaceRepo, workspaceRepo, nil, attachmentRepo),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/octet-stream",
		Reader:      bytes.NewReader(buildAdminSpaceImportCoverZip(t)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if !result.Space.HasCover {
		t.Fatalf("expected preview to report package cover")
	}

	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusCompleted
	})

	workspaceRepo.mu.Lock()
	defer workspaceRepo.mu.Unlock()
	if len(workspaceRepo.spaces) != 1 {
		t.Fatalf("expected one imported space, got %d", len(workspaceRepo.spaces))
	}
	importedSpace := workspaceRepo.spaces[0]
	if importedSpace.CoverAssetID == nil || strings.TrimSpace(*importedSpace.CoverAssetID) == "" {
		t.Fatalf("expected imported space cover asset id, got %#v", importedSpace)
	}
	if importedSpace.CoverWidth != 1600 || importedSpace.CoverHeight != 900 || importedSpace.CoverSource != string(AdminSpaceCoverSourceUserUpload) {
		t.Fatalf("unexpected imported cover fields: %#v", importedSpace)
	}
	if len(spaceRepo.coverAssets) != 1 || spaceRepo.coverAssets[0].AssetID != *importedSpace.CoverAssetID {
		t.Fatalf("expected cover asset to be persisted, got %#v", spaceRepo.coverAssets)
	}
}

func TestAdminSpaceImportService_Inspect_RejectsInvalidCoverSource(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/octet-stream",
		Reader:      bytes.NewReader(buildAdminSpaceImportCoverZipWithSource(t, "invalid-source")),
	})
	if !errors.Is(err, errcode.ErrAdminSpaceImportPackageNotImportable) {
		t.Fatalf("expected package not importable for invalid cover source, got %v", err)
	}
}

func TestAdminSpaceImportService_Commit_RejectsInvalidCoverPayload(t *testing.T) {
	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
	}
	spaceRepo := &stubAdminSpaceImportSpaceRepo{}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(spaceRepo, workspaceRepo, nil, attachmentRepo),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/octet-stream",
		Reader: bytes.NewReader(buildAdminSpaceImportCoverZipWithPayload(
			t,
			string(AdminSpaceCoverSourceUserUpload),
			"image/webp",
			[]byte("not-a-real-image"),
		)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}

	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed before worker restore: %v", err)
	}
	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusFailed
	})

	if len(spaceRepo.coverAssets) != 0 {
		t.Fatalf("expected invalid cover payload to be rejected before persist, got %#v", spaceRepo.coverAssets)
	}
}

func TestAdminSpaceImportService_Commit_CleansCoverObjectWhenAssetPersistFails(t *testing.T) {
	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
	}
	spaceRepo := &stubAdminSpaceImportSpaceRepo{createCoverAssetErr: errors.New("persist cover failed")}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(spaceRepo, workspaceRepo, nil, attachmentRepo),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/octet-stream",
		Reader:      bytes.NewReader(buildAdminSpaceImportCoverZip(t)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}

	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed before worker restore: %v", err)
	}
	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusFailed
	})

	if len(spaceRepo.coverAssets) != 1 {
		t.Fatalf("expected one attempted cover asset, got %#v", spaceRepo.coverAssets)
	}
	objectPath := filepath.Join(adminSpaceCoverStorageRootDir, filepath.FromSlash(spaceRepo.coverAssets[0].ObjectKey))
	t.Cleanup(func() {
		_ = os.Remove(objectPath)
	})
	if _, err := os.Stat(objectPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected failed cover object to be removed, stat err=%v", err)
	}
}

func TestAdminSpaceImportService_Commit_CleansCoverAssetWhenCreateSpaceFails(t *testing.T) {
	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
		createSpaceErr:  errors.New("create space failed"),
	}
	spaceRepo := &stubAdminSpaceImportSpaceRepo{}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(spaceRepo, workspaceRepo, nil, attachmentRepo),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/octet-stream",
		Reader:      bytes.NewReader(buildAdminSpaceImportCoverZip(t)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}

	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed before worker restore: %v", err)
	}
	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusFailed
	})

	if len(spaceRepo.coverAssets) != 1 {
		t.Fatalf("expected one attempted cover asset, got %#v", spaceRepo.coverAssets)
	}
	if len(spaceRepo.deletedCoverAssetIDs) != 1 || spaceRepo.deletedCoverAssetIDs[0] != spaceRepo.coverAssets[0].AssetID {
		t.Fatalf("expected failed cover asset to be deleted, got %#v", spaceRepo.deletedCoverAssetIDs)
	}
	objectPath := filepath.Join(adminSpaceCoverStorageRootDir, filepath.FromSlash(spaceRepo.coverAssets[0].ObjectKey))
	t.Cleanup(func() {
		_ = os.Remove(objectPath)
	})
	if _, err := os.Stat(objectPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected failed cover object to be removed, stat err=%v", err)
	}
}

func TestAdminSpaceImportService_RunJob_RecordsCompletedAuditWithNewSpaceID(t *testing.T) {
	t.Parallel()

	recorder := &recordingAdminAuditRecorder{}
	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
	}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(nil, workspaceRepo, nil, attachmentRepo),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
		WithAdminSpaceImportAuditRecorder(recorder),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusCompleted
	})

	job, err := svc.store.GetJob(commitResult.JobID)
	if err != nil {
		t.Fatalf("get job failed: %v", err)
	}
	record, detail := findAuditRecordByStatus(t, recorder.Records(), "success")
	if record.TargetType != "space" || record.TargetID != job.NewSpaceID || job.NewSpaceID == "" {
		t.Fatalf("unexpected completed audit target: record=%#v job=%#v", record, job)
	}
	if detail["stage"] != "completed" || detail["newSpaceId"] != job.NewSpaceID {
		t.Fatalf("unexpected completed audit detail: %#v", detail)
	}
	assertAuditDetailHasNoTransferSecret(t, detail)
}

func TestAdminSpaceImportService_Commit_RestoresOfficeAndAttachmentMetadata(t *testing.T) {
	t.Parallel()

	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
	}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{}
	renderer := &stubAdminSpaceImportOfficeRenderer{}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(nil, workspaceRepo, nil, attachmentRepo),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
		WithAdminSpaceImportOfficeHTMLRenderer(renderer),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportOfficeZip(t)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusCompleted
	})

	workspaceRepo.mu.Lock()
	var officeDocument *models.Document
	var officeFileRevision *models.DocumentFileRevision
	for index := range workspaceRepo.nodes {
		params := workspaceRepo.nodes[index]
		if params.Document != nil && params.Document.Format == models.DocumentFormatDOCX {
			officeDocument = params.Document
			officeFileRevision = params.FileRevision
			break
		}
	}
	workspaceRepo.mu.Unlock()
	if officeDocument == nil || officeFileRevision == nil {
		t.Fatalf("expected imported office document with file revision")
	}
	if officeDocument.RenderStatus != models.DocumentRenderStatusPending {
		t.Fatalf("expected office render status pending, got %q", officeDocument.RenderStatus)
	}
	const expectedDOCXMimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	if officeDocument.SourceMimeType == nil || *officeDocument.SourceMimeType != expectedDOCXMimeType {
		t.Fatalf("expected imported DOCX mime type %q, got %+v", expectedDOCXMimeType, officeDocument.SourceMimeType)
	}
	if officeFileRevision.MimeType != expectedDOCXMimeType {
		t.Fatalf("expected imported DOCX file revision mime type %q, got %q", expectedDOCXMimeType, officeFileRevision.MimeType)
	}

	renderer.mu.Lock()
	if len(renderer.tasks) != 1 {
		t.Fatalf("expected one office render task, got %d", len(renderer.tasks))
	}
	if string(renderer.tasks[0].SourceContent) != "office-source" || renderer.tasks[0].DocumentID != officeDocument.DocumentID {
		t.Fatalf("unexpected render task: %#v", renderer.tasks[0])
	}
	renderer.mu.Unlock()

	attachmentRepo.mu.Lock()
	if len(attachmentRepo.attachments) != 1 {
		t.Fatalf("expected one attachment, got %d", len(attachmentRepo.attachments))
	}
	attachment := attachmentRepo.attachments[0]
	if attachment.FileName != "原始 图.png" || attachment.MimeType != "image/png" || attachment.SizeBytes != int64(len("image")) {
		t.Fatalf("expected attachment metadata to be preserved, got %#v", attachment)
	}
	attachmentRepo.mu.Unlock()

	job, err := svc.store.GetJob(commitResult.JobID)
	if err != nil {
		t.Fatalf("get job failed: %v", err)
	}
	if job.AttachmentIDMappings["attach-a"] != attachment.AttachmentID {
		t.Fatalf("expected old attachment id mapping, got %#v want %q", job.AttachmentIDMappings, attachment.AttachmentID)
	}
}

func TestAdminSpaceImportService_Commit_CompletesWhenOfficeRenderQueueFails(t *testing.T) {
	t.Parallel()

	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
	}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{}
	renderer := &stubAdminSpaceImportOfficeRenderer{enqueueErr: errors.New("queue unavailable")}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(nil, workspaceRepo, nil, attachmentRepo),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
		WithAdminSpaceImportOfficeHTMLRenderer(renderer),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportOfficeZip(t)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusCompleted
	})
}

func TestAdminSpaceImportService_Commit_CleansCreatedBlobsWhenRestoreFails(t *testing.T) {
	t.Parallel()

	recorder := &recordingAdminAuditRecorder{}
	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory:             &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
		createNodeErrOnFileRevision: errors.New("create node failed"),
	}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(nil, workspaceRepo, nil, attachmentRepo),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
		WithAdminSpaceImportAuditRecorder(recorder),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportOfficeZip(t)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusFailed
	})

	attachmentRepo.mu.Lock()
	createdBlobs := append([]models.DocumentAttachmentBlob(nil), attachmentRepo.blobs...)
	deletedBlobIDs := append([]string(nil), attachmentRepo.hardDeletedBlobIDs...)
	attachmentRepo.mu.Unlock()
	if len(createdBlobs) == 0 {
		t.Fatalf("expected created blobs before restore failure")
	}
	if len(deletedBlobIDs) != len(createdBlobs) {
		t.Fatalf("expected created blobs to be cleaned, deleted=%v blobs=%v", deletedBlobIDs, createdBlobs)
	}
	for _, blob := range createdBlobs {
		targetPath, err := svc.resolveImportedLocalBlobPath(blob.ObjectKey)
		if err != nil {
			t.Fatalf("resolve imported blob path failed: %v", err)
		}
		if _, err := os.Stat(targetPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected imported blob file to be removed, path=%s err=%v", targetPath, err)
		}
	}

	record, detail := findAuditRecordByStatus(t, recorder.Records(), "failed")
	if record.TargetType != "space" || record.TargetID == "" {
		t.Fatalf("unexpected failed audit target: %#v", record)
	}
	errorMessage, _ := detail["error"].(string)
	if detail["stage"] != "restore" || errorMessage == "" {
		t.Fatalf("unexpected failed audit detail: %#v", detail)
	}
	assertAuditDetailHasNoTransferSecret(t, detail)
}

func TestAdminSpaceImportService_Commit_CleansCoverWhenRestoreFails(t *testing.T) {
	t.Parallel()

	spaceRepo := &stubAdminSpaceImportSpaceRepo{}
	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
		createNodeErr:   errors.New("create node failed"),
	}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(spaceRepo, workspaceRepo, nil, attachmentRepo),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportCoverZip(t)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusFailed
	})

	if len(spaceRepo.coverAssets) != 1 {
		t.Fatalf("expected imported cover asset to be created, got %d", len(spaceRepo.coverAssets))
	}
	coverAsset := spaceRepo.coverAssets[0]
	if len(spaceRepo.deletedCoverAssetIDs) != 1 || spaceRepo.deletedCoverAssetIDs[0] != coverAsset.AssetID {
		t.Fatalf("expected imported cover asset to be deleted, deleted=%v asset=%s", spaceRepo.deletedCoverAssetIDs, coverAsset.AssetID)
	}
	coverPath := filepath.Join(adminSpaceCoverStorageRootDir, filepath.FromSlash(coverAsset.ObjectKey))
	if _, err := os.Stat(coverPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected imported cover object to be removed, path=%s err=%v", coverPath, err)
	}
}

func TestReadAdminSpaceImportPackageDoesNotMaterializeUnreferencedEntries(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/manifest.json", buildAdminSpaceImportTestManifest(true))
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/tree.json", buildAdminSpaceImportTestTree())
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/documents/doc-a.md", []byte("# A"))
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/attachments/doc-a/image.png", []byte("image"))
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/sources/doc-a/source.docx", []byte("source"))
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/unreferenced/large.bin", bytes.Repeat([]byte("x"), 128<<10))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	zipPath := filepath.Join(t.TempDir(), "space.plaindoc")
	if err := os.WriteFile(zipPath, buffer.Bytes(), 0o600); err != nil {
		t.Fatalf("write zip failed: %v", err)
	}

	pkg, err := readAdminSpaceImportPackage(zipPath)
	if err != nil {
		t.Fatalf("read import package failed: %v", err)
	}
	defer func() {
		if err := pkg.Close(); err != nil {
			t.Fatalf("close import package failed: %v", err)
		}
	}()
	documentPayload, err := readAdminSpaceImportPackageFile(pkg, "documents/doc-a.md")
	if err != nil {
		t.Fatalf("read referenced document failed: %v", err)
	}
	if string(documentPayload) != "# A" {
		t.Fatalf("unexpected document payload: %q", string(documentPayload))
	}
	if _, ok := pkg.Entries["space-space-source/unreferenced/large.bin"]; !ok {
		t.Fatalf("expected unreferenced zip entry to remain indexed for package-level validation")
	}
}

func TestReadAdminSpaceImportPackageDefersReferencedPayloadReadErrors(t *testing.T) {
	t.Parallel()

	zipPath := filepath.Join(t.TempDir(), "space.plaindoc")
	if err := os.WriteFile(zipPath, buildAdminSpaceImportCorruptStoredDocumentZip(t), 0o600); err != nil {
		t.Fatalf("write zip failed: %v", err)
	}

	pkg, err := readAdminSpaceImportPackage(zipPath)
	if err != nil {
		t.Fatalf("read import package should only validate referenced entry existence, got %v", err)
	}
	defer func() {
		if err := pkg.Close(); err != nil {
			t.Fatalf("close import package failed: %v", err)
		}
	}()

	if _, err := readAdminSpaceImportPackageFile(pkg, "documents/doc-a.md"); err == nil {
		t.Fatalf("expected corrupt referenced payload to fail only when the entry is read")
	}
}

func TestAdminSpaceImportService_Commit_RollbackHardDeletesSpaceBeforeBlobFiles(t *testing.T) {
	t.Parallel()

	blobRoot := t.TempDir()
	var mu sync.Mutex
	events := make([]string, 0, 4)
	recordEvent := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	}

	spaceRepo := &stubAdminSpaceImportSpaceRepo{
		onHardDelete: func(spaceID string) {
			recordEvent("space-hard-delete:" + spaceID)
		},
	}
	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory:             &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
		createNodeErrOnFileRevision: errors.New("create office node failed"),
	}
	attachmentRepo := &stubAdminSpaceImportAttachmentRepo{
		localRootDir: blobRoot,
		onHardDeleteBlob: func(blob models.DocumentAttachmentBlob) {
			targetPath := filepath.Join(blobRoot, filepath.FromSlash(strings.TrimLeft(blob.ObjectKey, "/")))
			if _, err := os.Stat(targetPath); err != nil {
				t.Fatalf("expected blob file to exist before DB hard delete, path=%s err=%v", targetPath, err)
			}
			recordEvent("blob-hard-delete:" + blob.BlobID)
		},
	}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(spaceRepo, workspaceRepo, nil, attachmentRepo),
		WithAdminSpaceImportBlobStorage(blobRoot),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportOfficeZip(t)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusFailed
	})

	mu.Lock()
	defer mu.Unlock()
	if len(spaceRepo.softDeletedSpaceIDs) != 0 {
		t.Fatalf("expected failed import rollback to use hard delete, got soft deletes %#v", spaceRepo.softDeletedSpaceIDs)
	}
	if len(events) < 2 {
		t.Fatalf("expected hard delete and blob cleanup events, got %#v", events)
	}
	if !strings.HasPrefix(events[0], "space-hard-delete:") || !strings.HasPrefix(events[1], "blob-hard-delete:") {
		t.Fatalf("expected space hard delete before blob DB cleanup, got %#v", events)
	}
}

func TestAdminSpaceImportService_Commit_PreservesRestoreFailureWhenRollbackFails(t *testing.T) {
	t.Parallel()

	recorder := &recordingAdminAuditRecorder{}
	spaceRepo := &stubAdminSpaceImportSpaceRepo{
		hardDeleteErr: errors.New("rollback unavailable"),
	}
	workspaceRepo := &stubAdminSpaceImportWorkspaceRepo{
		defaultCategory: &models.SpaceCategory{CategoryID: "cat-default", Name: "默认分类", IsDefault: true},
		createNodeErr:   errors.New("create node failed"),
	}
	svc := NewAdminSpaceImportService(
		nil,
		WithAdminSpaceImportRepositories(spaceRepo, workspaceRepo, nil, nil),
		WithAdminSpaceImportBlobStorage(t.TempDir()),
		WithAdminSpaceImportAuditRecorder(recorder),
	)
	svc.stagingDir = t.TempDir()

	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.plaindoc",
		ContentType: "application/zip",
		Reader:      bytes.NewReader(buildAdminSpaceImportTestZip(t, true, true, true)),
	})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	commitResult, err := svc.Commit(context.Background(), CommitAdminSpaceImportInput{
		ActorUserID: "actor-user",
		ImportID:    result.ImportID,
		SpaceName:   "导入空间",
	})
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	eventually(t, time.Second, func() bool {
		job, err := svc.store.GetJob(commitResult.JobID)
		return err == nil && job.Status == AdminSpaceImportStatusFailed
	})

	job, err := svc.store.GetJob(commitResult.JobID)
	if err != nil {
		t.Fatalf("get job failed: %v", err)
	}
	if job.LastEvent.Stage != "restore" {
		t.Fatalf("expected restore failure stage to be preserved, got %#v", job.LastEvent)
	}
	if !strings.Contains(job.LastEvent.Message, "create node failed") {
		t.Fatalf("expected original restore error in job message, got %q", job.LastEvent.Message)
	}
	if !strings.Contains(job.LastEvent.Message, "rollback unavailable") {
		t.Fatalf("expected rollback error in job message, got %q", job.LastEvent.Message)
	}

	if len(spaceRepo.hardDeletedSpaceIDs) != 1 || spaceRepo.hardDeletedSpaceIDs[0] == "" {
		t.Fatalf("expected rollback hard delete to be attempted, got %#v", spaceRepo.hardDeletedSpaceIDs)
	}
	_, detail := findAuditRecordByStatus(t, recorder.Records(), "failed")
	errorMessage, _ := detail["error"].(string)
	if detail["stage"] != "restore" || !strings.Contains(errorMessage, "create node failed") {
		t.Fatalf("unexpected failed audit detail: %#v", detail)
	}
}

func markStagingImportableForTest(svc *AdminSpaceImportService, importID string, actorUserID string) {
	if svc == nil || svc.store == nil {
		return
	}
	svc.store.mu.Lock()
	defer svc.store.mu.Unlock()
	staging, ok := svc.store.stagings[importStagingKey(importID, actorUserID)]
	if ok && staging != nil {
		staging.Importable = true
	}
}

func collectAdminSpaceImportEventsUntilTerminal(
	t *testing.T,
	events <-chan AdminSpaceTransferEvent,
) []AdminSpaceTransferEvent {
	t.Helper()
	received := make([]AdminSpaceTransferEvent, 0)
	deadline := time.After(time.Second)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return received
			}
			received = append(received, event)
			if event.Type == AdminSpaceTransferEventTypeCompleted || event.Type == AdminSpaceTransferEventTypeFailed {
				return received
			}
		case <-deadline:
			t.Fatalf("timed out waiting for terminal import event, got %#v", received)
			return received
		}
	}
}

func buildAdminSpaceImportTestZip(t *testing.T, includeManifest bool, includeTree bool, includeDocument bool) []byte {
	t.Helper()

	return buildAdminSpaceImportTestZipWithOptions(t, includeManifest, includeTree, includeDocument, true)
}

func buildAdminSpaceImportTestZipWithImportable(t *testing.T, importable bool) []byte {
	t.Helper()

	return buildAdminSpaceImportTestZipWithOptions(t, true, true, true, importable)
}

func buildAdminSpaceImportTestZipWithOptions(
	t *testing.T,
	includeManifest bool,
	includeTree bool,
	includeDocument bool,
	importable bool,
) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	prefix := "space-space-source"
	if includeManifest {
		writeAdminSpaceImportTestJSON(t, zipWriter, prefix+"/manifest.json", buildAdminSpaceImportTestManifest(importable))
	}
	if includeTree {
		writeAdminSpaceImportTestJSON(t, zipWriter, prefix+"/tree.json", buildAdminSpaceImportTestTree())
	}
	if includeDocument {
		writeAdminSpaceImportTestFile(t, zipWriter, prefix+"/documents/doc-a.md", []byte("# A"))
		writeAdminSpaceImportTestFile(t, zipWriter, prefix+"/attachments/doc-a/image.png", []byte("image"))
		writeAdminSpaceImportTestFile(t, zipWriter, prefix+"/sources/doc-a/source.docx", []byte("source"))
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportTestEPUB3(t *testing.T) []byte {
	t.Helper()

	return buildAdminSpaceImportTestEPUB3WithOptions(t, true, true, true, true)
}

func buildAdminSpaceImportTestEPUB3WithCoverAndDescription(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestFile(t, zipWriter, "mimetype", []byte("application/epub+zip"))
	writeAdminSpaceImportTestFile(t, zipWriter, "META-INF/container.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/content.opf", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package version="3.0" xmlns="http://www.idpf.org/2007/opf" unique-identifier="bookid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>带封面 EPUB</dc:title>
    <dc:creator>作者甲</dc:creator>
    <dc:description>这是 EPUB 简介</dc:description>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chapter-1" href="chapters/chapter1.xhtml" media-type="application/xhtml+xml"/>
    <item id="cover" href="images/cover.png" media-type="image/png" properties="cover-image"/>
  </manifest>
  <spine>
    <itemref idref="chapter-1"/>
  </spine>
</package>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/nav.xhtml", []byte(`<!doctype html>
<html xmlns="http://www.w3.org/1999/xhtml">
  <body>
    <nav epub:type="toc"><ol><li><a href="chapters/chapter1.xhtml">第一章</a></li></ol></nav>
  </body>
</html>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/chapters/chapter1.xhtml", []byte(`<html><body><h1>第一章</h1><p>正文</p></body></html>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/images/cover.png", buildValidAdminSpaceImportPNGPayload(t, 2, 3))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close epub failed: %v", err)
	}
	return buffer.Bytes()
}

func findAdminSpaceImportCreatedNode(
	t *testing.T,
	nodes []repository.WorkspaceCreateNodeParams,
	nodeType models.NodeType,
	title string,
) repository.WorkspaceCreateNodeParams {
	t.Helper()
	for _, params := range nodes {
		if params.Node == nil {
			continue
		}
		if params.Node.Type == nodeType && params.Node.Title == title {
			return params
		}
	}
	t.Fatalf("created node %s/%s not found in %#v", nodeType, title, nodes)
	return repository.WorkspaceCreateNodeParams{}
}

func buildAdminSpaceImportTestEPUB3WithOptions(
	t *testing.T,
	includeMimetype bool,
	includeContainer bool,
	includeOPF bool,
	includeSpine bool,
) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	if includeMimetype {
		writeAdminSpaceImportTestFile(t, zipWriter, "mimetype", []byte("application/epub+zip"))
	}
	if includeContainer {
		writeAdminSpaceImportTestFile(t, zipWriter, "META-INF/container.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`))
	}
	if includeOPF {
		spine := ""
		if includeSpine {
			spine = `<spine>
    <itemref idref="chapter-1"/>
    <itemref idref="chapter-2"/>
  </spine>`
		}
		writeAdminSpaceImportTestFile(t, zipWriter, "OPS/content.opf", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package version="3.0" xmlns="http://www.idpf.org/2007/opf" unique-identifier="bookid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>EPUB 示例书</dc:title>
    <dc:creator>作者甲</dc:creator>
    <dc:date>2026-05-17</dc:date>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chapter-1" href="chapters/chapter1.xhtml" media-type="application/xhtml+xml"/>
    <item id="chapter-2" href="chapters/chapter2.xhtml" media-type="application/xhtml+xml"/>
    <item id="cover" href="images/cover.png" media-type="image/png"/>
  </manifest>
  `+spine+`
</package>`))
	}
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/nav.xhtml", []byte(`<!doctype html>
<html xmlns="http://www.w3.org/1999/xhtml">
  <body>
    <nav epub:type="toc">
      <ol>
        <li><a href="chapters/chapter1.xhtml">第一章</a></li>
        <li><span>第二部分</span><ol><li><a href="chapters/chapter2.xhtml">第二章</a></li></ol></li>
      </ol>
    </nav>
  </body>
</html>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/chapters/chapter1.xhtml", []byte(`<html><body><h1>第一章</h1><p>正文</p></body></html>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/chapters/chapter2.xhtml", []byte(`<html><body><h1>第二章</h1><img src="../images/cover.png" alt="封面"/></body></html>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/images/cover.png", []byte("png"))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close epub failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceEPUBZipFileForTest(name string, uncompressedSize uint64) *zip.File {
	return &zip.File{
		FileHeader: zip.FileHeader{
			Name:               name,
			UncompressedSize64: uncompressedSize,
		},
	}
}

func buildAdminSpaceImportTestEPUB2(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestFile(t, zipWriter, "mimetype", []byte("application/epub+zip"))
	writeAdminSpaceImportTestFile(t, zipWriter, "META-INF/container.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles>
</container>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OEBPS/content.opf", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package version="2.0" xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>EPUB2 示例书</dc:title>
    <dc:creator>作者乙</dc:creator>
  </metadata>
  <manifest>
    <item id="toc" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="chapter-1" href="chapter1.xhtml" media-type="application/xhtml+xml"/>
    <item id="chapter-2" href="chapter2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine toc="toc">
    <itemref idref="chapter-1"/>
    <itemref idref="chapter-2"/>
  </spine>
</package>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OEBPS/toc.ncx", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/">
  <navMap>
    <navPoint><navLabel><text>第一部分</text></navLabel><content src="chapter1.xhtml"/>
      <navPoint><navLabel><text>第二章</text></navLabel><content src="chapter2.xhtml"/></navPoint>
    </navPoint>
  </navMap>
</ncx>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OEBPS/chapter1.xhtml", []byte(`<html><body><h1>第一章</h1></body></html>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OEBPS/chapter2.xhtml", []byte(`<html><body><h1>第二章</h1></body></html>`))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close epub2 failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportTestEPUBWithoutNavOrTOC(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestFile(t, zipWriter, "mimetype", []byte("application/epub+zip"))
	writeAdminSpaceImportTestFile(t, zipWriter, "META-INF/container.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OPS/content.opf"/></rootfiles>
</container>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/content.opf", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package version="3.0" xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>扁平 EPUB</dc:title></metadata>
  <manifest>
    <item id="chapter-1" href="chapter1.xhtml" media-type="application/xhtml+xml"/>
    <item id="chapter-2" href="chapter2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="chapter-1"/><itemref idref="chapter-2"/></spine>
</package>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/chapter1.xhtml", []byte(`<html><body><h1>第一章</h1></body></html>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/chapter2.xhtml", []byte(`<html><body><h1>第二章</h1></body></html>`))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close flat epub failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportTestEPUBWithNonStandardMediaTypes(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestFile(t, zipWriter, "mimetype", []byte("application/epub+zip"))
	writeAdminSpaceImportTestFile(t, zipWriter, "META-INF/container.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OPS/content.opf"/></rootfiles>
</container>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/content.opf", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package version="3.0" xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Fallback EPUB</dc:title></metadata>
  <manifest>
    <item id="chapter-1" href="chapter1.xhtml" media-type="application/octet-stream"/>
    <item id="image-1" href="cover.jpg" media-type="application/octet-stream"/>
  </manifest>
  <spine><itemref idref="chapter-1"/></spine>
</package>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/chapter1.xhtml", []byte(`<html><body><img src="cover.jpg"/></body></html>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/cover.jpg", []byte("jpg"))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close fallback epub failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportTestEPUBWithNonUTF8OPFDeclaration(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestFile(t, zipWriter, "mimetype", []byte("application/epub+zip"))
	writeAdminSpaceImportTestFile(t, zipWriter, "META-INF/container.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OPS/content.opf"/></rootfiles>
</container>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/content.opf", []byte(`<?xml version="1.0" encoding="ISO-8859-1"?>
<package version="3.0" xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Latin EPUB</dc:title></metadata>
  <manifest><item id="chapter-1" href="chapter1.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="chapter-1"/></spine>
</package>`))
	writeAdminSpaceImportTestFile(t, zipWriter, "OPS/chapter1.xhtml", []byte(`<html><body><h1>Latin</h1></body></html>`))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close non utf-8 epub failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportCoverZip(t *testing.T) []byte {
	t.Helper()

	return buildAdminSpaceImportCoverZipWithSource(t, string(AdminSpaceCoverSourceUserUpload))
}

func buildAdminSpaceImportCoverZipWithSource(t *testing.T, source string) []byte {
	t.Helper()

	return buildAdminSpaceImportCoverZipWithPayload(t, source, "image/webp", buildValidAdminSpaceImportCoverPayload(t))
}

func buildAdminSpaceImportCoverZipWithPayload(t *testing.T, source string, mimeType string, coverPayload []byte) []byte {
	t.Helper()

	const coverPath = "covers/source-cover.webp"
	coverHash := sha256.Sum256(coverPayload)
	manifest := buildAdminSpaceImportTestManifest(true)
	manifest.Space.Cover = &AdminSpaceExportCoverEntry{
		AssetID:    "source-cover",
		Path:       coverPath,
		FileName:   "source-cover.webp",
		MimeType:   mimeType,
		SizeBytes:  int64(len(coverPayload)),
		Width:      1600,
		Height:     900,
		Source:     source,
		Normalized: true,
		SHA256:     hex.EncodeToString(coverHash[:]),
	}

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/manifest.json", manifest)
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/tree.json", buildAdminSpaceImportTestTree())
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/documents/doc-a.md", []byte("# A"))
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/attachments/doc-a/image.png", []byte("image"))
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/sources/doc-a/source.docx", []byte("source"))
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/"+coverPath, coverPayload)
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buffer.Bytes()
}

func buildValidAdminSpaceImportCoverPayload(t *testing.T) []byte {
	t.Helper()

	payload, err := encodeAdminSpaceCoverWebP(image.NewRGBA(image.Rect(0, 0, 1600, 900)), adminSpaceCoverDefaultQuality)
	if err != nil {
		t.Fatalf("encode test cover failed: %v", err)
	}
	return payload
}

func buildValidAdminSpaceImportPNGPayload(t *testing.T, width int, height int) []byte {
	t.Helper()

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("encode test png failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportTestEPUB(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestFile(t, zipWriter, "mimetype", []byte("application/epub+zip"))
	writeAdminSpaceImportTestFile(t, zipWriter, "META-INF/container.xml", []byte("<container></container>"))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close epub failed: %v", err)
	}
	return buffer.Bytes()
}

func writeAdminSpaceImportTestJSON(t *testing.T, zipWriter *zip.Writer, name string, value any) {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s failed: %v", name, err)
	}
	writeAdminSpaceImportTestFile(t, zipWriter, name, payload)
}

func writeAdminSpaceImportTestFile(t *testing.T, zipWriter *zip.Writer, name string, payload []byte) {
	t.Helper()

	writer, err := zipWriter.Create(name)
	if err != nil {
		t.Fatalf("create zip entry %s failed: %v", name, err)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatalf("write zip entry %s failed: %v", name, err)
	}
}

func buildAdminSpaceImportCorruptStoredDocumentZip(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/manifest.json", buildAdminSpaceImportTestManifest(true))
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/tree.json", buildAdminSpaceImportTestTree())
	writeAdminSpaceImportStoredTestFile(t, zipWriter, "space-space-source/documents/doc-a.md", []byte("# A"))
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/attachments/doc-a/image.png", []byte("image"))
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/sources/doc-a/source.docx", []byte("source"))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	payload := buffer.Bytes()
	documentOffset := bytes.Index(payload, []byte("# A"))
	if documentOffset < 0 {
		t.Fatalf("stored document payload not found in zip")
	}
	payload[documentOffset] = '!'
	return payload
}

func writeAdminSpaceImportStoredTestFile(t *testing.T, zipWriter *zip.Writer, name string, payload []byte) {
	t.Helper()

	header := &zip.FileHeader{Name: name, Method: zip.Store}
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		t.Fatalf("create stored zip entry %s failed: %v", name, err)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatalf("write stored zip entry %s failed: %v", name, err)
	}
}

func buildAdminSpaceImportMismatchedRootZip(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	manifest := buildAdminSpaceImportTestManifest(true)
	tree := buildAdminSpaceImportTestTree()
	writeAdminSpaceImportTestJSON(t, zipWriter, "root-a/manifest.json", manifest)
	writeAdminSpaceImportTestJSON(t, zipWriter, "root-b/tree.json", tree)
	writeAdminSpaceImportTestFile(t, zipWriter, "root-a/documents/doc-a.md", []byte("# A"))
	writeAdminSpaceImportTestFile(t, zipWriter, "root-a/attachments/doc-a/image.png", []byte("image"))
	writeAdminSpaceImportTestFile(t, zipWriter, "root-a/sources/doc-a/source.docx", []byte("source"))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportUnsafeEntryZip(t *testing.T, entryName string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestFile(t, zipWriter, entryName, []byte("{}"))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportDuplicateEntryZip(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/manifest.json", buildAdminSpaceImportTestManifest(true))
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/manifest.json", buildAdminSpaceImportTestManifest(true))
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/tree.json", buildAdminSpaceImportTestTree())
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportLargeManifestZip(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/manifest.json", bytes.Repeat([]byte("{"), maxAdminSpaceImportMetadataBytes+1))
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/tree.json", buildAdminSpaceImportTestTree())
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportEmptyDocumentPathZip(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	manifest := buildAdminSpaceImportTestManifest(true)
	manifest.Documents[0].Path = ""
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/manifest.json", manifest)
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/tree.json", buildAdminSpaceImportTestTree())
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportEmptySourcePathZip(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	manifest := buildAdminSpaceImportTestManifest(true)
	manifest.Documents[0].Source.Path = ""
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/manifest.json", manifest)
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/tree.json", buildAdminSpaceImportTestTree())
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/documents/doc-a.md", []byte("# A"))
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/attachments/doc-a/image.png", []byte("image"))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportMissingAttachmentEntryZip(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	manifest := buildAdminSpaceImportTestManifest(true)
	manifest.Documents[0].Attachments = nil
	manifest.Documents[0].AttachmentEntries = []AdminSpaceExportAttachmentEntry{
		{Path: "attachments/doc-a/missing.png", FileName: "missing.png"},
	}
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/manifest.json", manifest)
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/tree.json", buildAdminSpaceImportTestTree())
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/documents/doc-a.md", []byte("# A"))
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/sources/doc-a/source.docx", []byte("source"))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buffer.Bytes()
}

func buildAdminSpaceImportTestManifest(importable bool) AdminSpaceExportManifest {
	return AdminSpaceExportManifest{
		Version:     AdminSpaceExportPackageVersion,
		PackageType: AdminSpaceExportPackageType,
		ExportedAt:  "2026-05-16T00:00:00Z",
		Format:      AdminSpaceExportFormatMarkdownZip,
		Importable:  importable,
		Space: AdminSpaceExportManifestSpace{
			SpaceID:    "space-source",
			Name:       "源空间",
			Visibility: "member",
		},
		Summary: AdminSpaceExportSummary{FolderCount: 1, DocumentCount: 1, AttachmentCount: 1, OfficeSourceCount: 1},
		Documents: []AdminSpaceExportDocumentEntry{
			{
				DocumentID:  "doc-a",
				NodeID:      "node-a",
				Title:       "文档 A",
				Format:      "markdown",
				Visibility:  "member",
				Path:        "documents/doc-a.md",
				Attachments: []string{"attachments/doc-a/image.png"},
				Source:      &AdminSpaceExportSourceEntry{Path: "sources/doc-a/source.docx", Included: true},
			},
		},
	}
}

func buildAdminSpaceImportTestTree() AdminSpaceExportTree {
	return AdminSpaceExportTree{
		Version: AdminSpaceExportPackageVersion,
		Root: []AdminSpaceExportTreeNode{
			{NodeID: "folder-a", Type: "folder", Title: "目录", Children: []AdminSpaceExportTreeNode{
				{NodeID: "node-a", DocumentID: "doc-a", Type: "doc", Title: "文档 A", Format: "markdown"},
			}},
		},
	}
}

func buildAdminSpaceImportOfficeZip(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	manifest := AdminSpaceExportManifest{
		Version:     AdminSpaceExportPackageVersion,
		PackageType: AdminSpaceExportPackageType,
		ExportedAt:  "2026-05-16T00:00:00Z",
		Format:      AdminSpaceExportFormatSourceZip,
		Importable:  true,
		Space: AdminSpaceExportManifestSpace{
			SpaceID:    "space-source",
			Name:       "源空间",
			Visibility: "member",
		},
		Summary: AdminSpaceExportSummary{DocumentCount: 2, AttachmentCount: 1, OfficeSourceCount: 1},
		Documents: []AdminSpaceExportDocumentEntry{
			{
				DocumentID:  "doc-markdown",
				NodeID:      "node-markdown",
				Title:       "文档",
				Format:      "markdown",
				Visibility:  "member",
				Path:        "documents/doc.md",
				Attachments: []string{"attachments/doc-markdown/image.png"},
				AttachmentEntries: []AdminSpaceExportAttachmentEntry{
					{
						AttachmentID: "attach-a",
						Path:         "attachments/doc-markdown/image.png",
						FileName:     "原始 图.png",
						MimeType:     "image/png",
						SizeBytes:    int64(len("image")),
					},
				},
			},
			{
				DocumentID: "doc-office",
				NodeID:     "node-office",
				Title:      "Office",
				Format:     "docx",
				Visibility: "member",
				Path:       "sources/doc-office/source.docx",
				Source: &AdminSpaceExportSourceEntry{
					Path:     "sources/doc-office/source.docx",
					Included: true,
				},
			},
		},
	}
	tree := AdminSpaceExportTree{
		Version: AdminSpaceExportPackageVersion,
		Root: []AdminSpaceExportTreeNode{
			{NodeID: "node-markdown", DocumentID: "doc-markdown", Type: "doc", Title: "文档", Format: "markdown"},
			{NodeID: "node-office", DocumentID: "doc-office", Type: "doc", Title: "Office", Format: "docx"},
		},
	}
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/manifest.json", manifest)
	writeAdminSpaceImportTestJSON(t, zipWriter, "space-space-source/tree.json", tree)
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/documents/doc.md", []byte("# Doc"))
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/attachments/doc-markdown/image.png", []byte("image"))
	writeAdminSpaceImportTestFile(t, zipWriter, "space-space-source/sources/doc-office/source.docx", []byte("office-source"))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buffer.Bytes()
}

type stubAdminSpaceImportWorkspaceRepo struct {
	mu                          sync.Mutex
	defaultCategory             *models.SpaceCategory
	createSpaceErr              error
	createNodeErr               error
	createNodeErrOnFileRevision error
	spaces                      []models.Space
	nodes                       []repository.WorkspaceCreateNodeParams
}

type stubAdminSpaceImportSpaceRepo struct {
	softDeleteErr        error
	hardDeleteErr        error
	createCoverAssetErr  error
	softDeletedSpaceIDs  []string
	hardDeletedSpaceIDs  []string
	coverAssets          []models.SpaceCoverAsset
	deletedCoverAssetIDs []string
	onHardDelete         func(spaceID string)
}

type stubAdminSpaceImportAttachmentRepo struct {
	mu                 sync.Mutex
	blobs              []models.DocumentAttachmentBlob
	attachments        []models.DocumentAttachment
	hardDeletedBlobIDs []string
	localRootDir       string
	onHardDeleteBlob   func(blob models.DocumentAttachmentBlob)
}

type stubAdminSpaceImportOfficeRenderer struct {
	mu         sync.Mutex
	tasks      []OfficeHTMLRenderTask
	enqueueErr error
}

func (r *stubAdminSpaceImportOfficeRenderer) Enqueue(_ context.Context, task OfficeHTMLRenderTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks = append(r.tasks, task)
	return r.enqueueErr
}

type stubAdminSpaceImportDocumentImageAssetSyncer struct {
	mu     sync.Mutex
	inputs []SyncDocumentImageAssetsInput
	err    error
}

func (s *stubAdminSpaceImportDocumentImageAssetSyncer) SyncDocumentImageAssets(
	_ context.Context,
	input SyncDocumentImageAssetsInput,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inputs = append(s.inputs, input)
	return s.err
}

type failingHTMLMarkdownConverter struct {
	err error
}

func (c failingHTMLMarkdownConverter) Convert(
	_ context.Context,
	_ ConvertHTMLMarkdownInput,
) (ConvertHTMLMarkdownResult, error) {
	return ConvertHTMLMarkdownResult{}, c.err
}

func (r *stubAdminSpaceImportSpaceRepo) SoftDelete(_ context.Context, spaceID string, _ time.Time) (bool, error) {
	r.softDeletedSpaceIDs = append(r.softDeletedSpaceIDs, strings.TrimSpace(spaceID))
	return true, r.softDeleteErr
}

func (r *stubAdminSpaceImportSpaceRepo) HardDelete(_ context.Context, spaceID string) (bool, error) {
	normalizedSpaceID := strings.TrimSpace(spaceID)
	r.hardDeletedSpaceIDs = append(r.hardDeletedSpaceIDs, normalizedSpaceID)
	if r.onHardDelete != nil {
		r.onHardDelete(normalizedSpaceID)
	}
	return true, r.hardDeleteErr
}

func (r *stubAdminSpaceImportSpaceRepo) CreateCoverAsset(_ context.Context, asset *models.SpaceCoverAsset) error {
	if asset == nil {
		return errors.New("cover asset is nil")
	}
	r.coverAssets = append(r.coverAssets, *asset)
	return r.createCoverAssetErr
}

func (r *stubAdminSpaceImportSpaceRepo) DeleteCoverAssetByAssetID(_ context.Context, assetID string) (bool, error) {
	r.deletedCoverAssetIDs = append(r.deletedCoverAssetIDs, strings.TrimSpace(assetID))
	return true, nil
}

func (r *stubAdminSpaceImportAttachmentRepo) FindBlobByHash(
	_ context.Context,
	storageProvider string,
	contentHashAlgo string,
	contentHash string,
	sizeBytes int64,
) (*models.DocumentAttachmentBlob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index := range r.blobs {
		blob := r.blobs[index]
		if blob.StorageProvider == storageProvider &&
			blob.ContentHashAlgo == contentHashAlgo &&
			blob.ContentHash == contentHash &&
			blob.SizeBytes == sizeBytes {
			return &blob, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *stubAdminSpaceImportAttachmentRepo) CreateBlob(_ context.Context, blob *models.DocumentAttachmentBlob) error {
	if blob == nil {
		return errors.New("blob is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.blobs = append(r.blobs, *blob)
	return nil
}

func (r *stubAdminSpaceImportAttachmentRepo) Create(_ context.Context, attachment *models.DocumentAttachment) error {
	if attachment == nil {
		return errors.New("attachment is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attachments = append(r.attachments, *attachment)
	return nil
}

func (r *stubAdminSpaceImportAttachmentRepo) HardDeleteBlobIfUnreferenced(_ context.Context, blobID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	normalizedBlobID := strings.TrimSpace(blobID)
	for _, blob := range r.blobs {
		if strings.TrimSpace(blob.BlobID) == normalizedBlobID && r.onHardDeleteBlob != nil {
			r.onHardDeleteBlob(blob)
			break
		}
	}
	r.hardDeletedBlobIDs = append(r.hardDeletedBlobIDs, normalizedBlobID)
	return true, nil
}

func (r *stubAdminSpaceImportWorkspaceRepo) GetDefaultCategory(context.Context) (*models.SpaceCategory, error) {
	if r.defaultCategory == nil {
		return nil, errors.New("default category is nil")
	}
	category := *r.defaultCategory
	return &category, nil
}

func (r *stubAdminSpaceImportWorkspaceRepo) CreateSpace(_ context.Context, space *models.Space) error {
	if space == nil {
		return errors.New("space is nil")
	}
	if r.createSpaceErr != nil {
		return r.createSpaceErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spaces = append(r.spaces, *space)
	return nil
}

func (r *stubAdminSpaceImportWorkspaceRepo) CreateNode(_ context.Context, params repository.WorkspaceCreateNodeParams) error {
	if params.Node == nil {
		return errors.New("node is nil")
	}
	if r.createNodeErr != nil {
		return r.createNodeErr
	}
	if params.FileRevision != nil && r.createNodeErrOnFileRevision != nil {
		return r.createNodeErrOnFileRevision
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes = append(r.nodes, params)
	return nil
}

func eventually(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if condition() {
		return
	}
	t.Fatalf("condition was not met within %s", timeout)
}
