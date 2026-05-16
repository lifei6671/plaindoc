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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
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

func TestAdminSpaceImportService_Inspect_RejectsEPUBPackage(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = t.TempDir()

	_, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "book.epub",
		ContentType: "application/epub+zip",
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
	if _, ok := pkg.Files["space-space-source/unreferenced/large.bin"]; ok {
		t.Fatalf("expected unreferenced zip entry to stay out of materialized file map")
	}
	if _, ok := pkg.Files["space-space-source/documents/doc-a.md"]; !ok {
		t.Fatalf("expected referenced document entry to be materialized")
	}
}

func TestAdminSpaceImportService_Commit_PreservesRestoreFailureWhenRollbackFails(t *testing.T) {
	t.Parallel()

	recorder := &recordingAdminAuditRecorder{}
	spaceRepo := &stubAdminSpaceImportSpaceRepo{
		softDeleteErr: errors.New("rollback unavailable"),
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

	if len(spaceRepo.softDeletedSpaceIDs) != 1 || spaceRepo.softDeletedSpaceIDs[0] == "" {
		t.Fatalf("expected rollback soft delete to be attempted, got %#v", spaceRepo.softDeletedSpaceIDs)
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
	createCoverAssetErr  error
	softDeletedSpaceIDs  []string
	coverAssets          []models.SpaceCoverAsset
	deletedCoverAssetIDs []string
}

type stubAdminSpaceImportAttachmentRepo struct {
	mu                 sync.Mutex
	blobs              []models.DocumentAttachmentBlob
	attachments        []models.DocumentAttachment
	hardDeletedBlobIDs []string
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

func (r *stubAdminSpaceImportSpaceRepo) SoftDelete(_ context.Context, spaceID string, _ time.Time) (bool, error) {
	r.softDeletedSpaceIDs = append(r.softDeletedSpaceIDs, strings.TrimSpace(spaceID))
	return true, r.softDeleteErr
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
	r.hardDeletedBlobIDs = append(r.hardDeletedBlobIDs, strings.TrimSpace(blobID))
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
