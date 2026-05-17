package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

func TestAdminSpaceExportService_StartExport_RejectsUnsupportedFormat(t *testing.T) {
	t.Parallel()

	svc := newAllowExportService()

	_, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      AdminSpaceExportFormat("pdf"),
	})

	if !errors.Is(err, errcode.ErrAdminSpaceExportFormatUnsupported) {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}

func TestAdminSpaceExportService_StartExport_RequiresSpaceID(t *testing.T) {
	t.Parallel()

	svc := newAllowExportService()

	_, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		Format:      AdminSpaceExportFormatMarkdownZip,
	})

	if !errors.Is(err, errcode.ErrAdminSpaceExportSpaceIDRequired) {
		t.Fatalf("expected space id required error, got %v", err)
	}
}

func TestAdminSpaceExportService_StartExport_PersistsSpaceName(t *testing.T) {
	t.Parallel()

	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-export-start-persists-name?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()
	if err := storage.MigrateUp(context.Background(), database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}

	repo := repository.NewGormAdminSpaceTransferJobRepository(database.ORM)
	svc := newAllowExportService()
	svc.transferJobRepo = repo
	svc.spaceReader = &fakeAdminSpaceExportSpaceReader{
		space: &models.Space{
			SpaceID: "space-a",
			Name:    "知识库",
		},
	}

	result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      AdminSpaceExportFormatSourceZip,
	})
	if err != nil {
		t.Fatalf("start export failed: %v", err)
	}

	job, err := repo.GetByKindAndJobID(context.Background(), models.AdminSpaceTransferJobKindExport, result.JobID)
	if err != nil {
		t.Fatalf("get transfer job failed: %v", err)
	}
	if job.SpaceName != "知识库" {
		t.Fatalf("expected persisted space name, got %q", job.SpaceName)
	}
}

func TestAdminSpaceExportService_BuildPackage_IncludesSpaceCover(t *testing.T) {
	t.Parallel()

	coverAssetID := "cover-asset-a"
	coverContent := []byte("cover-webp")
	svc := newAllowExportService()
	svc.spaceReader = &fakeAdminSpaceExportSpaceReader{
		coverAssets: map[string]*models.SpaceCoverAsset{
			coverAssetID: {
				AssetID:         coverAssetID,
				Source:          string(AdminSpaceCoverSourceUserUpload),
				ObjectKey:       "space-covers/2026/05/16/source-cover.webp",
				ObjectURL:       "/uploads/space-covers/2026/05/16/source-cover.webp",
				MimeType:        "image/webp",
				Width:           1600,
				Height:          900,
				SizeBytes:       int64(len(coverContent)),
				Normalized:      true,
				CreatedByUserID: "actor-user",
			},
		},
	}
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{}
	svc.blobReader = fakeAdminSpaceExportBlobReader{
		contents: map[string][]byte{coverAssetID: coverContent},
	}

	pkg, err := svc.buildAdminSpaceExportPackage(
		context.Background(),
		AdminSpaceExportJob{
			ActorUserID:          "actor-user",
			SpaceID:              "space-a",
			Format:               AdminSpaceExportFormatSourceZip,
			IncludeAttachments:   true,
			IncludeOfficeSources: true,
		},
		models.Space{
			SpaceID:      "space-a",
			Name:         "空间 A",
			Visibility:   models.VisibilityMember,
			CoverAssetID: &coverAssetID,
			CoverKey:     "space-covers/2026/05/16/source-cover.webp",
			CoverURL:     "/uploads/space-covers/2026/05/16/source-cover.webp",
			CoverWidth:   1600,
			CoverHeight:  900,
			CoverSource:  string(AdminSpaceCoverSourceUserUpload),
		},
		nil,
		time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("build package failed: %v", err)
	}

	cover := pkg.Manifest.Space.Cover
	if cover == nil {
		t.Fatalf("expected cover manifest entry")
	}
	if cover.Path == "" || cover.MimeType != "image/webp" || cover.Width != 1600 || cover.Height != 900 {
		t.Fatalf("unexpected cover manifest: %#v", cover)
	}
	if string(pkg.Files[cover.Path]) != string(coverContent) {
		t.Fatalf("expected cover file payload to be included")
	}
}

func TestAdminSpaceExportService_StartExport_RejectsDuplicateActiveJob(t *testing.T) {
	t.Parallel()

	svc := newAllowExportService()
	input := StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      AdminSpaceExportFormatMarkdownZip,
	}

	if _, err := svc.StartExport(context.Background(), input); err != nil {
		t.Fatalf("first start export failed: %v", err)
	}
	_, err := svc.StartExport(context.Background(), input)

	if !errors.Is(err, errcode.ErrAdminSpaceExportJobRunningLimit) {
		t.Fatalf("expected running limit error, got %v", err)
	}
}

func TestAdminSpaceExportService_StartExport_UsesManagementPermissionMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		actorID string
		roles   map[string][]models.AdminRole
		scopes  map[string][]string
		ownerID string
		wantErr error
		wantOK  bool
	}{
		{
			name:    "platform admin can export any space",
			actorID: "platform-admin",
			roles: map[string][]models.AdminRole{
				"platform-admin": {models.AdminRolePlatformAdmin},
			},
			ownerID: "owner-user",
			wantOK:  true,
		},
		{
			name:    "space admin without scope cannot export",
			actorID: "space-admin",
			roles: map[string][]models.AdminRole{
				"space-admin": {models.AdminRoleSpaceAdmin},
			},
			ownerID: "owner-user",
			wantErr: errcode.ErrAdminForbidden,
		},
		{
			name:    "space admin with scope can export",
			actorID: "space-admin",
			roles: map[string][]models.AdminRole{
				"space-admin": {models.AdminRoleSpaceAdmin},
			},
			scopes: map[string][]string{
				"space-admin": {"space-a"},
			},
			ownerID: "owner-user",
			wantOK:  true,
		},
		{
			name:    "ordinary owner can export own space",
			actorID: "owner-user",
			ownerID: "owner-user",
			wantOK:  true,
		},
		{
			name:    "ordinary unrelated user cannot export",
			actorID: "other-user",
			ownerID: "owner-user",
			wantErr: errcode.ErrAdminForbidden,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spaceRepo := &fakeAdminSpaceExportPermissionSpaceRepo{
				spaces: map[string]*models.Space{
					"space-a": {
						SpaceID:     "space-a",
						Name:        "空间 A",
						OwnerUserID: tc.ownerID,
						Status:      models.EntityStatusActive,
						Visibility:  models.VisibilityMember,
					},
				},
			}
			adminAccess := NewAdminAccessService(
				newStubAdminRoleRepo(tc.roles),
				newStubSpaceAdminScopeRepo(tc.scopes),
				spaceRepo,
			)
			svc := NewAdminSpaceExportService(
				adminAccess,
				WithAdminSpaceExportRepositories(spaceRepo, nil),
			)

			result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
				ActorUserID: tc.actorID,
				SpaceID:     "space-a",
				Format:      AdminSpaceExportFormatSourceZip,
			})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got result=%#v err=%v", tc.wantErr, result, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("start export failed: %v", err)
			}
			if tc.wantOK && result.JobID == "" {
				t.Fatalf("expected export job id, got %#v", result)
			}
		})
	}
}

func TestAdminSpaceExportService_StartExport_RecordsQueuedAudit(t *testing.T) {
	t.Parallel()

	recorder := &recordingAdminAuditRecorder{}
	svc := newAllowExportService()
	svc.auditRecorder = recorder

	result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID:          "actor-user",
		SpaceID:              "space-a",
		Format:               AdminSpaceExportFormatSourceZip,
		IncludeAttachments:   true,
		IncludeOfficeSources: true,
	})
	if err != nil {
		t.Fatalf("start export failed: %v", err)
	}

	records := recorder.Records()
	if len(records) != 1 {
		t.Fatalf("expected one audit record, got %d", len(records))
	}
	record, detail := findAuditRecordByStatus(t, records, "queued")
	if record.ActorUserID != "actor-user" ||
		record.Module != AdminAuditModuleSpace ||
		record.Action != AdminAuditActionExport ||
		record.TargetType != "space" ||
		record.TargetID != "space-a" {
		t.Fatalf("unexpected audit record: %#v", record)
	}
	if detail["jobId"] != result.JobID ||
		detail["format"] != string(AdminSpaceExportFormatSourceZip) ||
		detail["abilityType"] != "space_manage" ||
		detail["includeAttachments"] != true ||
		detail["includeOfficeSources"] != true {
		t.Fatalf("unexpected audit detail: %#v", detail)
	}
	assertAuditDetailHasNoTransferSecret(t, detail)
}

func TestAdminSpaceExportService_StartExport_NormalizesPackageOptions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                     string
		format                   AdminSpaceExportFormat
		includeAttachments       bool
		includeOfficeSources     bool
		wantIncludeAttachments   bool
		wantIncludeOfficeSources bool
	}{
		{
			name:                     "source zip always exports importable full package",
			format:                   AdminSpaceExportFormatSourceZip,
			wantIncludeAttachments:   true,
			wantIncludeOfficeSources: true,
		},
		{
			name:                     "epub always includes office sources for rendering",
			format:                   AdminSpaceExportFormatEPUB,
			wantIncludeAttachments:   false,
			wantIncludeOfficeSources: true,
		},
		{
			name:                     "markdown zip preserves explicit options",
			format:                   AdminSpaceExportFormatMarkdownZip,
			includeAttachments:       true,
			includeOfficeSources:     false,
			wantIncludeAttachments:   true,
			wantIncludeOfficeSources: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newAllowExportService()
			result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
				ActorUserID:          "actor-user",
				SpaceID:              "space-a",
				Format:               tc.format,
				IncludeAttachments:   tc.includeAttachments,
				IncludeOfficeSources: tc.includeOfficeSources,
			})
			if err != nil {
				t.Fatalf("start export failed: %v", err)
			}

			job, err := svc.store.Get(result.JobID)
			if err != nil {
				t.Fatalf("get export job failed: %v", err)
			}
			if job.IncludeAttachments != tc.wantIncludeAttachments ||
				job.IncludeOfficeSources != tc.wantIncludeOfficeSources {
				t.Fatalf("unexpected normalized options: attachments=%t office=%t", job.IncludeAttachments, job.IncludeOfficeSources)
			}
		})
	}
}

func TestAdminSpaceExportService_RunJob_RecordsCompletedAudit(t *testing.T) {
	t.Parallel()

	recorder := &recordingAdminAuditRecorder{}
	svc := newAllowExportService()
	svc.auditRecorder = recorder
	svc.exportDir = t.TempDir()
	svc.spaceReader = &fakeAdminSpaceExportSpaceReader{
		space: &models.Space{
			SpaceID:    "space-a",
			Name:       "空间 A",
			Visibility: models.VisibilityAuthenticated,
		},
	}
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{}

	result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      AdminSpaceExportFormatMarkdownZip,
	})
	if err != nil {
		t.Fatalf("start export failed: %v", err)
	}
	eventually(t, time.Second, func() bool {
		job, err := svc.store.Get(result.JobID)
		return err == nil && job.Status == AdminSpaceExportStatusCompleted
	})

	record, detail := findAuditRecordByStatus(t, recorder.Records(), "success")
	if record.TargetType != "space" || record.TargetID != "space-a" {
		t.Fatalf("unexpected completed audit target: %#v", record)
	}
	fileName, _ := detail["fileName"].(string)
	if detail["stage"] != "completed" || fileName == "" || detail["sizeBytes"] == nil {
		t.Fatalf("unexpected completed audit detail: %#v", detail)
	}
	assertAuditDetailHasNoTransferSecret(t, detail)
}

func TestAdminSpaceExportService_RunJob_RecordsFailedAudit(t *testing.T) {
	t.Parallel()

	recorder := &recordingAdminAuditRecorder{}
	svc := newAllowExportService()
	svc.auditRecorder = recorder

	result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      AdminSpaceExportFormatMarkdownZip,
	})
	if err != nil {
		t.Fatalf("start export failed: %v", err)
	}

	svc.runAdminSpaceExportJob(context.Background(), result.JobID)

	record, detail := findAuditRecordByStatus(t, recorder.Records(), "failed")
	if record.TargetType != "space" || record.TargetID != "space-a" {
		t.Fatalf("unexpected failed audit target: %#v", record)
	}
	errorMessage, _ := detail["error"].(string)
	if detail["stage"] != "zip" || errorMessage == "" {
		t.Fatalf("unexpected failed audit detail: %#v", detail)
	}
	assertAuditDetailHasNoTransferSecret(t, detail)
}

func TestAdminSpaceExportService_StreamToken_BindsToJobAndActor(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	svc := newAllowExportService()
	svc.nowFn = func() time.Time { return now }

	result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      AdminSpaceExportFormatMarkdownZip,
	})
	if err != nil {
		t.Fatalf("start export failed: %v", err)
	}
	token := tokenQueryValue(t, result.StreamURL)

	if _, _, _, err := svc.Subscribe(context.Background(), result.JobID, "actor-user", token); err != nil {
		t.Fatalf("subscribe with correct token failed: %v", err)
	}
	if _, _, _, err := svc.Subscribe(context.Background(), result.JobID, "other-user", token); !errors.Is(err, errcode.ErrAdminSpaceExportJobTokenInvalid) {
		t.Fatalf("expected token invalid for other actor, got %v", err)
	}
	if _, _, _, err := svc.Subscribe(context.Background(), "other-job", "actor-user", token); !errors.Is(err, errcode.ErrAdminSpaceExportJobNotFound) {
		t.Fatalf("expected job not found for other job, got %v", err)
	}

	svc.nowFn = func() time.Time { return now.Add(defaultAdminSpaceTransferTokenTTL + time.Second) }
	if _, _, _, err := svc.Subscribe(context.Background(), result.JobID, "actor-user", token); !errors.Is(err, errcode.ErrAdminSpaceExportJobTokenInvalid) {
		t.Fatalf("expected token invalid after expiry, got %v", err)
	}
}

func TestAdminSpaceExportService_IssueStreamURL_ReplacesExpiredToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	svc := newAllowExportService()
	svc.nowFn = func() time.Time { return now }

	result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      AdminSpaceExportFormatMarkdownZip,
	})
	if err != nil {
		t.Fatalf("start export failed: %v", err)
	}
	oldToken := tokenQueryValue(t, result.StreamURL)

	svc.nowFn = func() time.Time { return now.Add(defaultAdminSpaceTransferTokenTTL + time.Second) }
	streamURL, err := svc.IssueStreamURL(context.Background(), "actor-user", result.JobID)
	if err != nil {
		t.Fatalf("issue stream url failed: %v", err)
	}
	newToken := tokenQueryValue(t, streamURL)
	if newToken == "" || newToken == oldToken {
		t.Fatalf("expected a fresh stream token, got %q", newToken)
	}
	if _, _, _, err := svc.Subscribe(context.Background(), result.JobID, "actor-user", oldToken); !errors.Is(err, errcode.ErrAdminSpaceExportJobTokenInvalid) {
		t.Fatalf("expected old token invalid, got %v", err)
	}
	if _, _, _, err := svc.Subscribe(context.Background(), result.JobID, "actor-user", newToken); err != nil {
		t.Fatalf("subscribe with fresh token failed: %v", err)
	}
}

func TestAdminSpaceExportService_ReissuesDownloadTokenForLateSubscriber(t *testing.T) {
	t.Parallel()

	exportDir := t.TempDir()
	svc := newAllowExportService()
	svc.exportDir = exportDir
	result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      AdminSpaceExportFormatMarkdownZip,
	})
	if err != nil {
		t.Fatalf("start export failed: %v", err)
	}
	streamToken := tokenQueryValue(t, result.StreamURL)

	filePath := filepath.Join(exportDir, "space-a.zip")
	if err := os.WriteFile(filePath, []byte("zip-content"), 0o644); err != nil {
		t.Fatalf("write export file failed: %v", err)
	}
	completed, err := svc.store.Complete(result.JobID, "space-a.zip", filePath, 11, "download-token-original", tokenHash("download-token-original"), svc.now())
	if err != nil {
		t.Fatalf("complete export failed: %v", err)
	}
	downloadToken := tokenQueryValue(t, completed.DownloadURL)
	if downloadToken == "" {
		t.Fatalf("expected completed event to include one-time download token")
	}

	initial, _, unsubscribe, err := svc.Subscribe(context.Background(), result.JobID, "actor-user", streamToken)
	if err != nil {
		t.Fatalf("subscribe after completion failed: %v", err)
	}
	defer unsubscribe()
	if strings.Contains(initial.DownloadURL, downloadToken) {
		t.Fatalf("initial event replayed plain download token: %s", initial.DownloadURL)
	}
	reissuedToken := tokenQueryValue(t, initial.DownloadURL)
	if reissuedToken == "" {
		t.Fatalf("expected late subscriber completed event to include reissued download token")
	}
	download, err := svc.ConsumeDownloadToken(result.JobID, "actor-user", reissuedToken)
	if err != nil {
		t.Fatalf("consume reissued download token failed: %v", err)
	}
	if download.FileName != "space-a.zip" || download.SizeBytes != 11 {
		t.Fatalf("unexpected download payload: %#v", download)
	}
}

func TestAdminSpaceExportService_ConsumeDownloadToken_EnforcesOneTimeDownload(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	svc := newAllowExportService()
	svc.nowFn = func() time.Time { return now }

	result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      AdminSpaceExportFormatMarkdownZip,
	})
	if err != nil {
		t.Fatalf("start export failed: %v", err)
	}
	exportDir := t.TempDir()
	svc.exportDir = exportDir
	filePath := filepath.Join(exportDir, "space-a.zip")
	if err := os.WriteFile(filePath, []byte("zip-content"), 0o644); err != nil {
		t.Fatalf("write export file failed: %v", err)
	}
	const downloadToken = "download-token"
	if _, err := svc.store.Complete(result.JobID, "space-a.zip", filePath, 11, downloadToken, tokenHash(downloadToken), now); err != nil {
		t.Fatalf("complete export failed: %v", err)
	}

	download, err := svc.ConsumeDownloadToken(result.JobID, "actor-user", downloadToken)
	if err != nil {
		t.Fatalf("consume download token failed: %v", err)
	}
	if download.FileName != "space-a.zip" || download.FilePath != filePath || download.SizeBytes != 11 {
		t.Fatalf("unexpected download payload: %#v", download)
	}

	if _, err := svc.ConsumeDownloadToken(result.JobID, "actor-user", downloadToken); !errors.Is(err, errcode.ErrAdminSpaceExportDownloadForbidden) {
		t.Fatalf("expected replay to be forbidden, got %v", err)
	}
}

func TestAdminSpaceExportService_IssueDownloadURL_ReissuesOneTimeToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	svc := newAllowExportService()
	svc.nowFn = func() time.Time { return now }

	result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      AdminSpaceExportFormatMarkdownZip,
	})
	if err != nil {
		t.Fatalf("start export failed: %v", err)
	}
	exportDir := t.TempDir()
	svc.exportDir = exportDir
	filePath := filepath.Join(exportDir, "space-a.zip")
	if err := os.WriteFile(filePath, []byte("zip-content"), 0o644); err != nil {
		t.Fatalf("write export file failed: %v", err)
	}
	if _, err := svc.store.Complete(result.JobID, "space-a.zip", filePath, 11, "download-token", tokenHash("download-token"), now); err != nil {
		t.Fatalf("complete export failed: %v", err)
	}

	downloadURL, err := svc.IssueDownloadURL(context.Background(), "actor-user", result.JobID)
	if err != nil {
		t.Fatalf("issue download url failed: %v", err)
	}
	downloadToken := tokenQueryValue(t, downloadURL)
	download, err := svc.ConsumeDownloadToken(result.JobID, "actor-user", downloadToken)
	if err != nil {
		t.Fatalf("consume reissued download token failed: %v", err)
	}
	if download.FileName != "space-a.zip" || download.FilePath != filePath || download.SizeBytes != 11 {
		t.Fatalf("unexpected download payload: %#v", download)
	}
	if _, err := svc.ConsumeDownloadToken(result.JobID, "other-user", downloadToken); !errors.Is(err, errcode.ErrAdminSpaceExportDownloadForbidden) {
		t.Fatalf("expected other actor forbidden, got %v", err)
	}
}

func TestAdminSpaceExportService_IssueDownloadURL_RestoresCompletedJobFromRepository(t *testing.T) {
	t.Parallel()

	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-export-download-restore?mode=memory&cache=shared",
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

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	exportDir := t.TempDir()
	filePath := filepath.Join(exportDir, "space-a.plaindoc")
	content := []byte("zip-content")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("write export file failed: %v", err)
	}

	repo := repository.NewGormAdminSpaceTransferJobRepository(database.ORM)
	job := models.AdminSpaceTransferJob{
		JobID:       "01exportrestoredownload001",
		Kind:        models.AdminSpaceTransferJobKindExport,
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      string(AdminSpaceExportFormatSourceZip),
		Status:      models.AdminSpaceTransferJobStatusCompleted,
		Stage:       "done",
		Progress:    100,
		Message:     "导出完成",
		FilePath:    filePath,
		FileName:    "space-a.plaindoc",
		SizeBytes:   int64(len(content)),
		CreatedAt:   now.Add(-time.Minute),
		UpdatedAt:   now,
		ExpiresAt:   now.Add(10 * time.Minute),
	}
	if err := repo.Create(ctx, &job); err != nil {
		t.Fatalf("create transfer job failed: %v", err)
	}

	svc := newAllowExportService()
	svc.nowFn = func() time.Time { return now }
	svc.exportDir = exportDir
	svc.transferJobRepo = repo

	downloadURL, err := svc.IssueDownloadURL(ctx, "actor-user", job.JobID)
	if err != nil {
		t.Fatalf("issue restored download url failed: %v", err)
	}
	downloadToken := tokenQueryValue(t, downloadURL)
	download, err := svc.ConsumeDownloadToken(job.JobID, "actor-user", downloadToken)
	if err != nil {
		t.Fatalf("consume restored download token failed: %v", err)
	}
	if download.FileName != "space-a.plaindoc" || download.FilePath != filePath || download.SizeBytes != int64(len(content)) {
		t.Fatalf("unexpected restored download payload: %#v", download)
	}
	if _, err := svc.ConsumeDownloadToken(job.JobID, "actor-user", downloadToken); !errors.Is(err, errcode.ErrAdminSpaceExportDownloadForbidden) {
		t.Fatalf("expected restored token replay forbidden, got %v", err)
	}
}

func TestAdminSpaceExportService_ConsumeDownloadToken_RejectsInvalidStates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)

	t.Run("未完成任务不能下载", func(t *testing.T) {
		t.Parallel()

		svc := newAllowExportService()
		svc.nowFn = func() time.Time { return now }
		result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
			ActorUserID: "actor-user",
			SpaceID:     "space-a",
			Format:      AdminSpaceExportFormatMarkdownZip,
		})
		if err != nil {
			t.Fatalf("start export failed: %v", err)
		}

		_, err = svc.ConsumeDownloadToken(result.JobID, "actor-user", "download-token")
		if !errors.Is(err, errcode.ErrAdminSpaceExportDownloadForbidden) {
			t.Fatalf("expected unfinished job to be forbidden, got %v", err)
		}
	})

	t.Run("其他用户不能下载", func(t *testing.T) {
		t.Parallel()

		svc, result, token := completeExportForDownloadTest(t, now, "space-a.zip", true)
		_, err := svc.ConsumeDownloadToken(result.JobID, "other-user", token)
		if !errors.Is(err, errcode.ErrAdminSpaceExportDownloadForbidden) {
			t.Fatalf("expected other actor to be forbidden, got %v", err)
		}
	})

	t.Run("过期 token 不能下载", func(t *testing.T) {
		t.Parallel()

		svc, result, token := completeExportForDownloadTest(t, now, "space-a.zip", true)
		svc.nowFn = func() time.Time { return now.Add(defaultAdminSpaceTransferTokenTTL + time.Second) }
		_, err := svc.ConsumeDownloadToken(result.JobID, "actor-user", token)
		if !errors.Is(err, errcode.ErrAdminSpaceExportFileExpired) {
			t.Fatalf("expected expired token error, got %v", err)
		}
	})

	t.Run("文件缺失不能下载", func(t *testing.T) {
		t.Parallel()

		svc, result, token := completeExportForDownloadTest(t, now, "space-a.zip", false)
		_, err := svc.ConsumeDownloadToken(result.JobID, "actor-user", token)
		if !errors.Is(err, errcode.ErrAdminSpaceExportFileNotReady) {
			t.Fatalf("expected file not ready error, got %v", err)
		}
	})

	t.Run("导出私有目录外文件不能下载", func(t *testing.T) {
		t.Parallel()

		svc, result, token := completeExportForDownloadTest(t, now, "space-a.zip", true)
		outsideDir := t.TempDir()
		outsidePath := filepath.Join(outsideDir, "space-a.zip")
		if err := os.WriteFile(outsidePath, []byte("zip-content"), 0o644); err != nil {
			t.Fatalf("write outside export file failed: %v", err)
		}
		job, err := svc.store.Get(result.JobID)
		if err != nil {
			t.Fatalf("get export job failed: %v", err)
		}
		if _, err := svc.store.Complete(result.JobID, job.FileName, outsidePath, 11, token, tokenHash(token), now); err != nil {
			t.Fatalf("replace export file path failed: %v", err)
		}
		_, err = svc.ConsumeDownloadToken(result.JobID, "actor-user", token)
		if !errors.Is(err, errcode.ErrAdminSpaceExportDownloadForbidden) {
			t.Fatalf("expected outside file path to be forbidden, got %v", err)
		}
	})
}

func TestAdminSpaceExportService_Broadcast_DoesNotBlockWhenSubscriberIsFull(t *testing.T) {
	t.Parallel()

	svc := newAllowExportService()
	result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      AdminSpaceExportFormatMarkdownZip,
	})
	if err != nil {
		t.Fatalf("start export failed: %v", err)
	}
	token := tokenQueryValue(t, result.StreamURL)
	_, events, unsubscribe, err := svc.Subscribe(context.Background(), result.JobID, "actor-user", token)
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer unsubscribe()

	for i := 0; i < cap(events)+3; i++ {
		svc.PublishProgress(result.JobID, AdminSpaceTransferEvent{
			Type:     AdminSpaceTransferEventTypeProgress,
			Stage:    "test",
			Progress: i,
			Message:  "progress",
		})
	}
}

func TestAdminSpaceExportPathSanitizer_PreventsZipSlip(t *testing.T) {
	t.Parallel()

	cases := []string{
		"../evil.md",
		`C:\evil.md`,
		"folder/../../evil.md",
		"bad:name?.md",
	}
	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			entry := safeAdminSpaceExportZipEntry("documents", value)
			if path.IsAbs(entry) || strings.Contains(entry, "..") || strings.Contains(entry, "\\") || strings.Contains(entry, ":") {
				t.Fatalf("unsafe zip entry %q from %q", entry, value)
			}
		})
	}
}

func TestAdminSpaceExportService_BuildsNonImportableMarkdownZipPackage(t *testing.T) {
	t.Parallel()

	format := models.DocumentFormatMarkdown
	visibility := string(models.VisibilityMember)
	folderID := "folder-1"
	docA := "doc-a"
	docB := "doc-b"
	svc := newAllowExportService()
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{
		nodes: []repository.WorkspaceTreeNodeRecord{
			{NodeID: folderID, SpaceID: "space-a", Type: models.NodeTypeFolder, Title: "产品:文档", Sort: 1},
			{NodeID: "node-a", DocumentID: &docA, SpaceID: "space-a", ParentNodeID: &folderID, Type: models.NodeTypeDoc, Title: "需求", Sort: 1, DocumentVisibility: &visibility, DocumentFormat: &format},
			{NodeID: "node-b", DocumentID: &docB, SpaceID: "space-a", ParentNodeID: &folderID, Type: models.NodeTypeDoc, Title: "需求", Sort: 2, DocumentVisibility: &visibility, DocumentFormat: &format},
		},
		documents: map[string]*repository.WorkspaceDocumentRecord{
			docA: {DocumentID: docA, NodeID: "node-a", Format: models.DocumentFormatMarkdown, Title: "需求", ContentMD: "# A", SpaceID: "space-a"},
			docB: {DocumentID: docB, NodeID: "node-b", Format: models.DocumentFormatMarkdown, Title: "需求", ContentMD: "# B", SpaceID: "space-a"},
		},
	}

	pkg, err := svc.buildAdminSpaceExportPackage(
		context.Background(),
		AdminSpaceExportJob{JobID: "job-a", SpaceID: "space-a", Format: AdminSpaceExportFormatMarkdownZip},
		models.Space{SpaceID: "space-a", Name: "空间 A", Visibility: models.VisibilityMember, Status: models.EntityStatusActive},
		svc.workspaceReader.(*fakeAdminSpaceExportWorkspaceReader).nodes,
		time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("build package failed: %v", err)
	}
	if pkg.Manifest.Importable {
		t.Fatalf("expected markdown zip manifest to be non-importable")
	}
	if pkg.Manifest.PackageType != AdminSpaceExportPackageType || pkg.Manifest.Version != AdminSpaceExportPackageVersion {
		t.Fatalf("unexpected manifest package: %#v", pkg.Manifest)
	}
	if len(pkg.Manifest.Documents) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(pkg.Manifest.Documents))
	}
	if pkg.Manifest.Documents[0].Path == pkg.Manifest.Documents[1].Path {
		t.Fatalf("duplicate document paths: %s", pkg.Manifest.Documents[0].Path)
	}
	for _, document := range pkg.Manifest.Documents {
		if _, ok := pkg.Files[document.Path]; !ok {
			t.Fatalf("manifest path %q missing from file map", document.Path)
		}
		if document.DocumentID == "" || document.NodeID == "" || document.Title == "" || document.Format == "" || document.Visibility == "" {
			t.Fatalf("manifest document missing required fields: %#v", document)
		}
	}
	if len(pkg.Tree.Root) != 1 || len(pkg.Tree.Root[0].Children) != 2 {
		t.Fatalf("tree did not preserve parent-child relation: %#v", pkg.Tree)
	}

	partPath := t.TempDir() + string(os.PathSeparator) + "export.part"
	if err := writeAdminSpaceExportZip(partPath, pkg); err != nil {
		t.Fatalf("write zip failed: %v", err)
	}
	assertZipEntryExists(t, partPath, pkg.RootEntryPrefix+"/manifest.json")
	assertZipEntryExists(t, partPath, pkg.RootEntryPrefix+"/tree.json")
	for _, document := range pkg.Manifest.Documents {
		assertZipEntryExists(t, partPath, pkg.RootEntryPrefix+"/"+document.Path)
	}
}

func TestAdminSpaceExportService_IncludesEmptyMarkdownDocumentEntry(t *testing.T) {
	t.Parallel()

	format := models.DocumentFormatMarkdown
	documentID := "doc-empty"
	svc := newAllowExportService()
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{
		nodes: []repository.WorkspaceTreeNodeRecord{
			{NodeID: "node-empty", DocumentID: &documentID, SpaceID: "space-a", Type: models.NodeTypeDoc, Title: "空文档", Sort: 1, DocumentFormat: &format},
		},
		documents: map[string]*repository.WorkspaceDocumentRecord{
			documentID: {DocumentID: documentID, NodeID: "node-empty", Format: models.DocumentFormatMarkdown, Title: "空文档", ContentMD: "", SpaceID: "space-a"},
		},
	}

	pkg, err := svc.buildAdminSpaceExportPackage(
		context.Background(),
		AdminSpaceExportJob{JobID: "job-empty", SpaceID: "space-a", Format: AdminSpaceExportFormatMarkdownZip},
		models.Space{SpaceID: "space-a", Name: "空间 A", Visibility: models.VisibilityMember, Status: models.EntityStatusActive},
		svc.workspaceReader.(*fakeAdminSpaceExportWorkspaceReader).nodes,
		time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("build package failed: %v", err)
	}
	if len(pkg.Manifest.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(pkg.Manifest.Documents))
	}
	document := pkg.Manifest.Documents[0]
	content, ok := pkg.Files[document.Path]
	if !ok {
		t.Fatalf("manifest path %q missing from file map", document.Path)
	}
	if len(content) != 0 {
		t.Fatalf("expected empty markdown content, got %q", string(content))
	}
}

func TestAdminSpaceExportService_FailsWhenAttachmentExportDependencyMissing(t *testing.T) {
	t.Parallel()

	format := models.DocumentFormatMarkdown
	documentID := "doc-a"
	svc := newAllowExportService()
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{
		nodes: []repository.WorkspaceTreeNodeRecord{
			{NodeID: "node-a", DocumentID: &documentID, SpaceID: "space-a", Type: models.NodeTypeDoc, Title: "需求", Sort: 1, DocumentFormat: &format},
		},
		documents: map[string]*repository.WorkspaceDocumentRecord{
			documentID: {DocumentID: documentID, NodeID: "node-a", Format: models.DocumentFormatMarkdown, Title: "需求", ContentMD: "# A", SpaceID: "space-a"},
		},
	}

	_, err := svc.buildAdminSpaceExportPackage(
		context.Background(),
		AdminSpaceExportJob{JobID: "job-a", SpaceID: "space-a", Format: AdminSpaceExportFormatMarkdownZip, IncludeAttachments: true},
		models.Space{SpaceID: "space-a", Name: "空间 A", Visibility: models.VisibilityMember, Status: models.EntityStatusActive},
		svc.workspaceReader.(*fakeAdminSpaceExportWorkspaceReader).nodes,
		time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
	)
	if err == nil {
		t.Fatalf("expected missing attachment export dependency to fail")
	}
}

func TestAdminSpaceExportService_IncludesDocumentAttachmentsWhenRequested(t *testing.T) {
	t.Parallel()

	format := models.DocumentFormatMarkdown
	documentID := "doc-a"
	blobID := "blob-a"
	svc := newAllowExportService()
	svc.attachmentReader = &fakeAdminSpaceExportAttachmentReader{
		attachments: map[string][]models.DocumentAttachment{
			documentID: {
				{
					AttachmentID:    "attachment-a",
					BlobID:          blobID,
					DocumentID:      documentID,
					SpaceID:         "space-a",
					FileName:        "../设计图.png",
					Status:          models.EntityStatusActive,
					StorageProvider: string(ImageHostingProviderLocal),
					ObjectKey:       "attachments/blob-a.png",
				},
			},
		},
		blobs: map[string]*models.DocumentAttachmentBlob{
			blobID: {BlobID: blobID, StorageProvider: string(ImageHostingProviderLocal), ObjectKey: "attachments/blob-a.png"},
		},
	}
	svc.blobReader = fakeAdminSpaceExportBlobReader{
		contents: map[string][]byte{blobID: []byte("attachment-content")},
	}
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{
		nodes: []repository.WorkspaceTreeNodeRecord{
			{NodeID: "node-a", DocumentID: &documentID, SpaceID: "space-a", Type: models.NodeTypeDoc, Title: "需求", Sort: 1, DocumentFormat: &format},
		},
		documents: map[string]*repository.WorkspaceDocumentRecord{
			documentID: {DocumentID: documentID, NodeID: "node-a", Format: models.DocumentFormatMarkdown, Title: "需求", ContentMD: "# A", SpaceID: "space-a"},
		},
	}

	pkg, err := svc.buildAdminSpaceExportPackage(
		context.Background(),
		AdminSpaceExportJob{JobID: "job-a", SpaceID: "space-a", Format: AdminSpaceExportFormatSourceZip, IncludeAttachments: true},
		models.Space{SpaceID: "space-a", Name: "空间 A", Visibility: models.VisibilityMember, Status: models.EntityStatusActive},
		svc.workspaceReader.(*fakeAdminSpaceExportWorkspaceReader).nodes,
		time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("build package failed: %v", err)
	}
	if pkg.Manifest.Summary.AttachmentCount != 1 {
		t.Fatalf("expected attachment count 1, got %d", pkg.Manifest.Summary.AttachmentCount)
	}
	attachments := pkg.Manifest.Documents[0].Attachments
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment path, got %#v", attachments)
	}
	attachmentPath := attachments[0]
	if strings.Contains(attachmentPath, "..") || strings.Contains(attachmentPath, "\\") {
		t.Fatalf("unsafe attachment path: %s", attachmentPath)
	}
	if string(pkg.Files[attachmentPath]) != "attachment-content" {
		t.Fatalf("attachment content mismatch: %q", string(pkg.Files[attachmentPath]))
	}
}

func TestAdminSpaceExportService_IncludesOfficeSourceWhenRequested(t *testing.T) {
	t.Parallel()

	format := models.DocumentFormatDOCX
	documentID := "doc-office"
	sourceBlobID := "blob-office"
	svc := newAllowExportService()
	svc.attachmentReader = &fakeAdminSpaceExportAttachmentReader{
		blobs: map[string]*models.DocumentAttachmentBlob{
			sourceBlobID: {BlobID: sourceBlobID, StorageProvider: string(ImageHostingProviderLocal), ObjectKey: "sources/source.docx"},
		},
	}
	svc.blobReader = fakeAdminSpaceExportBlobReader{
		contents: map[string][]byte{sourceBlobID: []byte("office-source")},
	}
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{
		nodes: []repository.WorkspaceTreeNodeRecord{
			{NodeID: "node-office", DocumentID: &documentID, SpaceID: "space-a", Type: models.NodeTypeDoc, Title: "方案", Sort: 1, DocumentFormat: &format},
		},
		documents: map[string]*repository.WorkspaceDocumentRecord{
			documentID: {
				DocumentID:     documentID,
				NodeID:         "node-office",
				Format:         models.DocumentFormatDOCX,
				Title:          "方案",
				SourceBlobID:   &sourceBlobID,
				SourceFileName: new("方案.docx"),
				SpaceID:        "space-a",
			},
		},
	}

	pkg, err := svc.buildAdminSpaceExportPackage(
		context.Background(),
		AdminSpaceExportJob{JobID: "job-office", SpaceID: "space-a", Format: AdminSpaceExportFormatSourceZip, IncludeOfficeSources: true},
		models.Space{SpaceID: "space-a", Name: "空间 A", Visibility: models.VisibilityMember, Status: models.EntityStatusActive},
		svc.workspaceReader.(*fakeAdminSpaceExportWorkspaceReader).nodes,
		time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("build package failed: %v", err)
	}
	if len(pkg.Manifest.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(pkg.Manifest.Documents))
	}
	source := pkg.Manifest.Documents[0].Source
	if source == nil || !source.Included {
		t.Fatalf("expected included source entry, got %#v", source)
	}
	if _, ok := pkg.Files[source.Path]; !ok {
		t.Fatalf("source path %q missing from file map", source.Path)
	}
	if pkg.Manifest.Summary.OfficeSourceCount != 1 {
		t.Fatalf("expected office source count 1, got %d", pkg.Manifest.Summary.OfficeSourceCount)
	}
}

func TestAdminSpaceExportService_WritesEPUBPackageWithMarkdownAndOfficeHTML(t *testing.T) {
	t.Parallel()

	markdownFormat := models.DocumentFormatMarkdown
	docxFormat := models.DocumentFormatDOCX
	markdownDocumentID := "doc-markdown"
	officeDocumentID := "doc-office"
	sourceBlobID := "blob-office"
	coverAssetID := "cover-asset-epub"
	coverContent, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode cover fixture failed: %v", err)
	}
	svc := newAllowExportService()
	svc.exportDir = t.TempDir()
	svc.spaceReader = &fakeAdminSpaceExportSpaceReader{
		space: &models.Space{
			SpaceID:      "space-a",
			Name:         "空间/A:非法*名",
			Visibility:   models.VisibilityMember,
			Status:       models.EntityStatusActive,
			CoverAssetID: &coverAssetID,
			CoverKey:     "space-covers/2026/05/17/epub-cover.png",
			CoverURL:     "/uploads/space-covers/2026/05/17/epub-cover.png",
			CoverWidth:   1600,
			CoverHeight:  2560,
			CoverSource:  string(AdminSpaceCoverSourceUserUpload),
		},
		coverAssets: map[string]*models.SpaceCoverAsset{
			coverAssetID: {
				AssetID:         coverAssetID,
				Source:          string(AdminSpaceCoverSourceUserUpload),
				ObjectKey:       "space-covers/2026/05/17/epub-cover.png",
				ObjectURL:       "/uploads/space-covers/2026/05/17/epub-cover.png",
				MimeType:        "image/png",
				Width:           1600,
				Height:          2560,
				SizeBytes:       int64(len(coverContent)),
				Normalized:      true,
				CreatedByUserID: "actor-user",
			},
		},
	}
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{
		nodes: []repository.WorkspaceTreeNodeRecord{
			{NodeID: "node-md", DocumentID: &markdownDocumentID, SpaceID: "space-a", Type: models.NodeTypeDoc, Title: "Markdown 章节", Sort: 1, DocumentFormat: &markdownFormat},
			{NodeID: "node-office", DocumentID: &officeDocumentID, SpaceID: "space-a", Type: models.NodeTypeDoc, Title: "Office 章节", Sort: 2, DocumentFormat: &docxFormat},
		},
		documents: map[string]*repository.WorkspaceDocumentRecord{
			markdownDocumentID: {
				DocumentID: markdownDocumentID,
				NodeID:     "node-md",
				Format:     models.DocumentFormatMarkdown,
				Title:      "Markdown 章节",
				ContentMD:  "# Markdown 标题\n\nMarkdown 正文\n\n![红点](data:image/png;base64,iVBORw0KGgo=)",
				SpaceID:    "space-a",
			},
			officeDocumentID: {
				DocumentID:     officeDocumentID,
				NodeID:         "node-office",
				Format:         models.DocumentFormatDOCX,
				Title:          "Office 章节",
				SourceBlobID:   &sourceBlobID,
				SourceFileName: new("方案.docx"),
				SpaceID:        "space-a",
			},
		},
	}
	svc.attachmentReader = &fakeAdminSpaceExportAttachmentReader{
		blobs: map[string]*models.DocumentAttachmentBlob{
			sourceBlobID: {BlobID: sourceBlobID, StorageProvider: string(ImageHostingProviderLocal), ObjectKey: "sources/方案.docx"},
		},
	}
	svc.blobReader = fakeAdminSpaceExportBlobReader{
		contents: map[string][]byte{
			sourceBlobID: []byte("office-source"),
			coverAssetID: coverContent,
		},
	}
	officeRenderer := &fakeAdminSpaceExportOfficeHTMLRenderer{
		html: `<article><h1>Office 标题</h1><p>Office 正文</p><img src="data:image/png;base64,iVBORw0KGgo=" alt="红点" /></article>`,
	}
	svc.officeHTMLRenderer = officeRenderer
	markdownRenderer := &fakeAdminSpaceExportReaderHTMLRenderer{
		html: `<html><body><article id="plaindoc-preview-body" class="markdown-body"><h1>SSR Markdown 标题</h1><div class="code-block-copy-shell"><button type="button" class="code-block-copy-button" data-code-copy-button="1" data-copy-state="idle" aria-label="复制代码"><span class="code-block-copy-button__label code-block-copy-button__label--idle">复制</span><span class="code-block-copy-button__label code-block-copy-button__label--success">复制成功</span></button><pre class="code-block-copy-shell__surface"><code data-code-copy-source="1">const answer = 42;</code></pre></div><p>SSR Markdown 正文</p><img src="data:image/png;base64,iVBORw0KGgo=" alt="红点" /></article></body></html>`,
	}
	svc.readerHTMLRenderer = markdownRenderer

	fileName, filePath, sizeBytes, err := svc.exportAdminSpaceZipPackage(
		context.Background(),
		AdminSpaceExportJob{JobID: "job-epub", SpaceID: "space-a", Format: AdminSpaceExportFormatEPUB},
	)
	if err != nil {
		t.Fatalf("export epub failed: %v", err)
	}
	if !strings.HasSuffix(fileName, ".epub") || !strings.HasSuffix(filePath, ".epub") {
		t.Fatalf("expected epub file, got name=%s path=%s", fileName, filePath)
	}
	if fileName != "空间-A-非法-名.epub" {
		t.Fatalf("expected sanitized space name as epub file name, got %s", fileName)
	}
	if sizeBytes <= 0 {
		t.Fatalf("expected non-empty epub, got %d", sizeBytes)
	}
	if officeRenderer.calls != 1 {
		t.Fatalf("expected office renderer to be called once, got %d", officeRenderer.calls)
	}
	if officeRenderer.lastFormat != models.DocumentFormatDOCX || string(officeRenderer.lastSource) != "office-source" {
		t.Fatalf("unexpected renderer input: format=%s source=%q", officeRenderer.lastFormat, string(officeRenderer.lastSource))
	}
	if markdownRenderer.calls != 1 {
		t.Fatalf("expected markdown SSR renderer to be called once, got %d", markdownRenderer.calls)
	}
	if markdownRenderer.lastInput.Document.DocumentID != markdownDocumentID ||
		markdownRenderer.lastInput.Content != "# Markdown 标题\n\nMarkdown 正文\n\n![红点](data:image/png;base64,iVBORw0KGgo=)" {
		t.Fatalf("unexpected markdown renderer input: %#v", markdownRenderer.lastInput)
	}
	assertZipEntryExists(t, filePath, "mimetype")
	assertZipContainsText(t, filePath, "空间/A:非法*名")
	assertZipContainsText(t, filePath, "SSR Markdown 标题")
	assertZipContainsText(t, filePath, "const answer = 42;")
	assertZipDoesNotContainText(t, filePath, "data-code-copy-button")
	assertZipDoesNotContainText(t, filePath, "复制成功")
	assertZipDoesNotContainText(t, filePath, "# Markdown 标题")
	assertZipDoesNotContainText(t, filePath, "![红点]")
	assertZipContainsText(t, filePath, "Office 正文")
	assertZipEntryExists(t, filePath, "EPUB/xhtml/cover.xhtml")
	assertZipEntryMatches(t, filePath, "EPUB/package.opf", regexp.MustCompile(`(?s)<meta[^>]+name="cover"[^>]+content="[^"]+"`))
	assertZipEntryMatches(t, filePath, "EPUB/package.opf", regexp.MustCompile(`(?s)<item[^>]+href="images/cover\.png"[^>]+media-type="image/png"[^>]+properties="cover-image"`))
	assertZipEntryBytes(t, filePath, "EPUB/images/cover.png", coverContent)
	assertZipContainsText(t, filePath, "images/image-")
	assertZipDoesNotContainText(t, filePath, "data:image/png")
}

func TestAdminSpaceExportService_EPUBRejectsUnsafeImageSources(t *testing.T) {
	t.Parallel()

	markdownFormat := models.DocumentFormatMarkdown
	documentID := "doc-markdown"
	unsafeImagePath := filepath.Join(t.TempDir(), "private.png")
	pngPayload, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode png fixture failed: %v", err)
	}
	if err := os.WriteFile(unsafeImagePath, pngPayload, 0o600); err != nil {
		t.Fatalf("write unsafe image fixture failed: %v", err)
	}

	svc := newAllowExportService()
	svc.exportDir = t.TempDir()
	svc.spaceReader = &fakeAdminSpaceExportSpaceReader{
		space: &models.Space{SpaceID: "space-a", Name: "空间 A", Visibility: models.VisibilityMember, Status: models.EntityStatusActive},
	}
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{
		nodes: []repository.WorkspaceTreeNodeRecord{
			{NodeID: "node-md", DocumentID: &documentID, SpaceID: "space-a", Type: models.NodeTypeDoc, Title: "Markdown 章节", Sort: 1, DocumentFormat: &markdownFormat},
		},
		documents: map[string]*repository.WorkspaceDocumentRecord{
			documentID: {
				DocumentID: documentID,
				NodeID:     "node-md",
				Format:     models.DocumentFormatMarkdown,
				Title:      "Markdown 章节",
				ContentMD:  "# Markdown 标题",
				SpaceID:    "space-a",
			},
		},
	}
	svc.readerHTMLRenderer = &fakeAdminSpaceExportReaderHTMLRenderer{
		html: `<html><body><article id="plaindoc-preview-body"><h1>SSR Markdown 标题</h1><p><img src="` + unsafeImagePath + `" alt="本机路径图片" /></p></article></body></html>`,
	}

	_, filePath, _, err := svc.exportAdminSpaceZipPackage(
		context.Background(),
		AdminSpaceExportJob{JobID: "job-epub-unsafe-image", SpaceID: "space-a", Format: AdminSpaceExportFormatEPUB},
	)
	if err != nil {
		t.Fatalf("export epub failed: %v", err)
	}

	assertZipContainsText(t, filePath, "本机路径图片")
	assertZipDoesNotContainText(t, filePath, unsafeImagePath)
	assertZipDoesNotContainText(t, filePath, "images/image-")
}

func TestWriteAdminSpaceEPUBDataImageFile_RejectsOversizedPayload(t *testing.T) {
	rawImage := bytes.Repeat([]byte{'a'}, 20<<20+1)
	source := "data:image/png;base64," + base64.StdEncoding.EncodeToString(rawImage)

	_, err := writeAdminSpaceEPUBDataImageFile(source, t.TempDir())
	if err == nil {
		t.Fatalf("expected oversized data image to be rejected")
	}
	if !strings.Contains(err.Error(), "exceeds max size") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestCopyAdminSpaceEPUBImageFile_RejectsOversizedLocalFile(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "oversized.png")
	sourceFile, err := os.Create(sourcePath)
	if err != nil {
		t.Fatalf("create oversized image fixture failed: %v", err)
	}
	if err := sourceFile.Close(); err != nil {
		t.Fatalf("close oversized image fixture failed: %v", err)
	}
	if err := os.Truncate(sourcePath, 20<<20+1); err != nil {
		t.Fatalf("truncate oversized image fixture failed: %v", err)
	}
	mediaTempDir := t.TempDir()
	targetPath := filepath.Join(mediaTempDir, buildAdminSpaceEPUBImageFileName("/uploads/oversized.png"))

	_, err = copyAdminSpaceEPUBImageFile("/uploads/oversized.png", sourcePath, mediaTempDir)
	if err == nil {
		t.Fatalf("expected oversized local image to be rejected")
	}
	if !strings.Contains(err.Error(), "exceeds max size") {
		t.Fatalf("expected size limit error, got %v", err)
	}
	if _, statErr := os.Stat(targetPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected partial epub image file to be removed, got stat err %v", statErr)
	}
}

func TestAdminSpaceExportService_EPUBMarkdownRequiresReaderSSR(t *testing.T) {
	t.Parallel()

	markdownFormat := models.DocumentFormatMarkdown
	documentID := "doc-markdown"
	svc := newAllowExportService()
	svc.exportDir = t.TempDir()
	svc.spaceReader = &fakeAdminSpaceExportSpaceReader{
		space: &models.Space{SpaceID: "space-a", Name: "空间 A", Visibility: models.VisibilityMember, Status: models.EntityStatusActive},
	}
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{
		nodes: []repository.WorkspaceTreeNodeRecord{
			{NodeID: "node-md", DocumentID: &documentID, SpaceID: "space-a", Type: models.NodeTypeDoc, Title: "Markdown 章节", Sort: 1, DocumentFormat: &markdownFormat},
		},
		documents: map[string]*repository.WorkspaceDocumentRecord{
			documentID: {
				DocumentID: documentID,
				NodeID:     "node-md",
				Format:     models.DocumentFormatMarkdown,
				Title:      "Markdown 章节",
				ContentMD:  "# Markdown 标题\n\nMarkdown 正文",
				SpaceID:    "space-a",
			},
		},
	}

	_, _, _, err := svc.exportAdminSpaceZipPackage(
		context.Background(),
		AdminSpaceExportJob{JobID: "job-epub-fallback", SpaceID: "space-a", Format: AdminSpaceExportFormatEPUB},
	)
	if err == nil {
		t.Fatalf("expected epub export to fail without reader ssr renderer")
	}
}

func TestAdminSpaceExportService_WritesEPUBWithNestedDocumentTree(t *testing.T) {
	t.Parallel()

	markdownFormat := models.DocumentFormatMarkdown
	folderID := "folder-product"
	parentDocumentID := "doc-parent"
	childDocumentID := "doc-child"
	parentNodeID := "node-parent"
	childNodeID := "node-child"
	svc := newAllowExportService()
	svc.exportDir = t.TempDir()
	svc.spaceReader = &fakeAdminSpaceExportSpaceReader{
		space: &models.Space{SpaceID: "space-a", Name: "空间 A", Visibility: models.VisibilityMember, Status: models.EntityStatusActive},
	}
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{
		nodes: []repository.WorkspaceTreeNodeRecord{
			{NodeID: folderID, SpaceID: "space-a", Type: models.NodeTypeFolder, Title: "产品文档", Sort: 1},
			{NodeID: parentNodeID, DocumentID: &parentDocumentID, SpaceID: "space-a", ParentNodeID: &folderID, Type: models.NodeTypeDoc, Title: "父文档", Sort: 1, DocumentFormat: &markdownFormat},
			{NodeID: childNodeID, DocumentID: &childDocumentID, SpaceID: "space-a", ParentNodeID: &parentNodeID, Type: models.NodeTypeDoc, Title: "子文档", Sort: 1, DocumentFormat: &markdownFormat},
		},
		documents: map[string]*repository.WorkspaceDocumentRecord{
			parentDocumentID: {
				DocumentID: parentDocumentID,
				NodeID:     parentNodeID,
				Format:     models.DocumentFormatMarkdown,
				Title:      "父文档",
				ContentMD:  "# 父文档\n\n父文档正文",
				SpaceID:    "space-a",
			},
			childDocumentID: {
				DocumentID: childDocumentID,
				NodeID:     childNodeID,
				Format:     models.DocumentFormatMarkdown,
				Title:      "子文档",
				ContentMD:  "# 子文档\n\n子文档正文",
				SpaceID:    "space-a",
			},
		},
	}
	svc.readerHTMLRenderer = &fakeAdminSpaceExportReaderHTMLRenderer{
		htmlByDocumentID: map[string]string{
			parentDocumentID: `<html><body><article id="plaindoc-preview-body"><h1>父文档</h1><p>父文档正文</p></article></body></html>`,
			childDocumentID:  `<html><body><article id="plaindoc-preview-body"><h1>子文档</h1><p>子文档正文</p></article></body></html>`,
		},
	}

	_, filePath, _, err := svc.exportAdminSpaceZipPackage(
		context.Background(),
		AdminSpaceExportJob{JobID: "job-epub-tree", SpaceID: "space-a", Format: AdminSpaceExportFormatEPUB},
	)
	if err != nil {
		t.Fatalf("export epub failed: %v", err)
	}

	assertZipContainsText(t, filePath, "父文档正文")
	assertZipContainsText(t, filePath, "子文档正文")
	assertZipEntryMatches(t, filePath, "EPUB/nav.xhtml", regexp.MustCompile(`(?s)>产品文档</a>\s*<ol>\s*<li>\s*<a [^>]*>父文档</a>\s*<ol>\s*<li>\s*<a [^>]*>子文档</a>`))
}

func TestAdminSpaceExportService_EPUBPublishProgressByRenderedDocuments(t *testing.T) {
	t.Parallel()

	markdownFormat := models.DocumentFormatMarkdown
	firstDocumentID := "doc-first"
	secondDocumentID := "doc-second"
	svc := newAllowExportService()
	svc.exportDir = t.TempDir()
	svc.spaceReader = &fakeAdminSpaceExportSpaceReader{
		space: &models.Space{SpaceID: "space-a", Name: "空间 A", Visibility: models.VisibilityMember, Status: models.EntityStatusActive},
	}
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{
		nodes: []repository.WorkspaceTreeNodeRecord{
			{NodeID: "node-first", DocumentID: &firstDocumentID, SpaceID: "space-a", Type: models.NodeTypeDoc, Title: "第一章", Sort: 1, DocumentFormat: &markdownFormat},
			{NodeID: "node-second", DocumentID: &secondDocumentID, SpaceID: "space-a", Type: models.NodeTypeDoc, Title: "第二章", Sort: 2, DocumentFormat: &markdownFormat},
		},
		documents: map[string]*repository.WorkspaceDocumentRecord{
			firstDocumentID: {
				DocumentID: firstDocumentID,
				NodeID:     "node-first",
				Format:     models.DocumentFormatMarkdown,
				Title:      "第一章",
				ContentMD:  "# 第一章",
				SpaceID:    "space-a",
			},
			secondDocumentID: {
				DocumentID: secondDocumentID,
				NodeID:     "node-second",
				Format:     models.DocumentFormatMarkdown,
				Title:      "第二章",
				ContentMD:  "# 第二章",
				SpaceID:    "space-a",
			},
		},
	}
	svc.readerHTMLRenderer = &fakeAdminSpaceExportReaderHTMLRenderer{
		htmlByDocumentID: map[string]string{
			firstDocumentID:  `<html><body><article id="plaindoc-preview-body"><h1>第一章</h1></article></body></html>`,
			secondDocumentID: `<html><body><article id="plaindoc-preview-body"><h1>第二章</h1></article></body></html>`,
		},
	}
	const streamToken = "stream-token"
	jobID := "job-epub-progress"
	now := svc.now()
	if err := svc.store.Create(&AdminSpaceExportJob{
		JobID:                jobID,
		ActorUserID:          "actor-user",
		SpaceID:              "space-a",
		Format:               AdminSpaceExportFormatEPUB,
		Status:               AdminSpaceExportStatusRunning,
		StreamTokenHash:      tokenHash(streamToken),
		StreamTokenExpiresAt: now.Add(defaultAdminSpaceTransferTokenTTL),
		LastEvent: AdminSpaceTransferEvent{
			Type:     AdminSpaceTransferEventTypeProgress,
			Stage:    "running",
			Progress: 1,
			Message:  "导出任务开始执行",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create export job failed: %v", err)
	}
	_, events, unsubscribe, err := svc.Subscribe(context.Background(), jobID, "actor-user", streamToken)
	if err != nil {
		t.Fatalf("subscribe export job failed: %v", err)
	}
	defer unsubscribe()

	_, _, _, err = svc.exportAdminSpaceZipPackage(
		context.Background(),
		AdminSpaceExportJob{JobID: jobID, SpaceID: "space-a", Format: AdminSpaceExportFormatEPUB},
	)
	if err != nil {
		t.Fatalf("export epub failed: %v", err)
	}

	progresses := make([]int, 0, 2)
	for {
		select {
		case event := <-events:
			if event.Stage == "epub_documents" {
				progresses = append(progresses, event.Progress)
				if !strings.Contains(event.Message, "/2") {
					t.Fatalf("expected chapter count in progress message, got %q", event.Message)
				}
			}
		default:
			if len(progresses) != 2 {
				t.Fatalf("expected progress events for 2 rendered chapters, got %#v", progresses)
			}
			if progresses[0] <= 55 || progresses[1] != 90 || progresses[0] >= progresses[1] {
				t.Fatalf("expected chapter progress to advance from 55 to 90, got %#v", progresses)
			}
			return
		}
	}
}

func TestAdminSpaceExportService_MarksPackageNotImportableWithoutOfficeSourceExport(t *testing.T) {
	t.Parallel()

	format := models.DocumentFormatDOCX
	documentID := "doc-office"
	sourceBlobID := "blob-office"
	svc := newAllowExportService()
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{
		nodes: []repository.WorkspaceTreeNodeRecord{
			{NodeID: "node-office", DocumentID: &documentID, SpaceID: "space-a", Type: models.NodeTypeDoc, Title: "方案", Sort: 1, DocumentFormat: &format},
		},
		documents: map[string]*repository.WorkspaceDocumentRecord{
			documentID: {
				DocumentID:   documentID,
				NodeID:       "node-office",
				Format:       models.DocumentFormatDOCX,
				Title:        "方案",
				SourceBlobID: &sourceBlobID,
				SpaceID:      "space-a",
			},
		},
	}

	pkg, err := svc.buildAdminSpaceExportPackage(
		context.Background(),
		AdminSpaceExportJob{JobID: "job-office", SpaceID: "space-a", Format: AdminSpaceExportFormatSourceZip, IncludeOfficeSources: false},
		models.Space{SpaceID: "space-a", Name: "空间 A", Visibility: models.VisibilityMember, Status: models.EntityStatusActive},
		svc.workspaceReader.(*fakeAdminSpaceExportWorkspaceReader).nodes,
		time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("build package failed: %v", err)
	}
	if pkg.Manifest.Importable {
		t.Fatalf("expected source package without office source to be non-importable")
	}
	if len(pkg.Files) != 0 {
		t.Fatalf("expected office source file to be omitted, got %#v", pkg.Files)
	}
}

func TestAdminSpaceExportService_MarkdownZipSkipsOfficeSourceAndIsNotImportable(t *testing.T) {
	t.Parallel()

	format := models.DocumentFormatDOCX
	documentID := "doc-office"
	sourceBlobID := "blob-office"
	svc := newAllowExportService()
	svc.workspaceReader = &fakeAdminSpaceExportWorkspaceReader{
		nodes: []repository.WorkspaceTreeNodeRecord{
			{NodeID: "node-office", DocumentID: &documentID, SpaceID: "space-a", Type: models.NodeTypeDoc, Title: "方案", Sort: 1, DocumentFormat: &format},
		},
		documents: map[string]*repository.WorkspaceDocumentRecord{
			documentID: {
				DocumentID:   documentID,
				NodeID:       "node-office",
				Format:       models.DocumentFormatDOCX,
				Title:        "方案",
				SourceBlobID: &sourceBlobID,
				SpaceID:      "space-a",
			},
		},
	}

	pkg, err := svc.buildAdminSpaceExportPackage(
		context.Background(),
		AdminSpaceExportJob{JobID: "job-markdown-office", SpaceID: "space-a", Format: AdminSpaceExportFormatMarkdownZip, IncludeOfficeSources: false},
		models.Space{SpaceID: "space-a", Name: "空间 A", Visibility: models.VisibilityMember, Status: models.EntityStatusActive},
		svc.workspaceReader.(*fakeAdminSpaceExportWorkspaceReader).nodes,
		time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("build markdown package failed: %v", err)
	}
	if pkg.Manifest.Importable {
		t.Fatalf("expected markdown zip to be non-importable")
	}
	if len(pkg.Files) != 0 {
		t.Fatalf("expected office source to be skipped for markdown zip, got %#v", pkg.Files)
	}
}

func TestWriteAdminSpaceExportZipRejectsMissingManifestReference(t *testing.T) {
	t.Parallel()

	pkg := adminSpaceExportPackage{
		RootEntryPrefix: "space-a",
		Manifest: AdminSpaceExportManifest{
			Version:     AdminSpaceExportPackageVersion,
			PackageType: AdminSpaceExportPackageType,
			Importable:  true,
			Documents: []AdminSpaceExportDocumentEntry{
				{DocumentID: "doc-a", NodeID: "node-a", Path: "documents/missing.md", Attachments: []string{"attachments/doc-a/missing.png"}},
				{DocumentID: "doc-office", NodeID: "node-office", Path: "sources/doc-office/source.docx", Source: &AdminSpaceExportSourceEntry{Path: "sources/doc-office/source.docx", Included: true}},
			},
		},
		Tree:  AdminSpaceExportTree{Version: AdminSpaceExportPackageVersion},
		Files: map[string][]byte{},
	}

	err := writeAdminSpaceExportZip(t.TempDir()+string(os.PathSeparator)+"export.part", pkg)
	if err == nil {
		t.Fatalf("expected missing manifest reference to fail")
	}
}

func completeExportForDownloadTest(
	t *testing.T,
	now time.Time,
	fileName string,
	createFile bool,
) (*AdminSpaceExportService, StartAdminSpaceExportResult, string) {
	t.Helper()

	svc := newAllowExportService()
	svc.nowFn = func() time.Time { return now }
	result, err := svc.StartExport(context.Background(), StartAdminSpaceExportInput{
		ActorUserID: "actor-user",
		SpaceID:     "space-a",
		Format:      AdminSpaceExportFormatMarkdownZip,
	})
	if err != nil {
		t.Fatalf("start export failed: %v", err)
	}

	exportDir := t.TempDir()
	svc.exportDir = exportDir
	filePath := filepath.Join(exportDir, fileName)
	var sizeBytes int64
	if createFile {
		content := []byte("zip-content")
		if err := os.WriteFile(filePath, content, 0o644); err != nil {
			t.Fatalf("write export file failed: %v", err)
		}
		sizeBytes = int64(len(content))
	}
	token := "download-token-" + result.JobID
	if _, err := svc.store.Complete(result.JobID, fileName, filePath, sizeBytes, token, tokenHash(token), now); err != nil {
		t.Fatalf("complete export failed: %v", err)
	}
	return svc, result, token
}

func tokenQueryValue(t *testing.T, rawURL string) string {
	t.Helper()

	const marker = "token="
	index := strings.Index(rawURL, marker)
	if index < 0 {
		t.Fatalf("stream url does not contain token: %s", rawURL)
	}
	return rawURL[index+len(marker):]
}

func newAllowExportService() *AdminSpaceExportService {
	svc := NewAdminSpaceExportService(nil)
	svc.canExportSpace = func(context.Context, string, string) (bool, error) {
		return true, nil
	}
	return svc
}

type fakeAdminSpaceExportSpaceReader struct {
	space       *models.Space
	coverAssets map[string]*models.SpaceCoverAsset
	err         error
}

func (f *fakeAdminSpaceExportSpaceReader) GetBySpaceID(context.Context, string) (*models.Space, error) {
	return f.space, f.err
}

func (f *fakeAdminSpaceExportSpaceReader) GetCoverAssetByAssetID(_ context.Context, assetID string) (*models.SpaceCoverAsset, error) {
	asset := f.coverAssets[strings.TrimSpace(assetID)]
	if asset == nil {
		return nil, nil
	}
	copyAsset := *asset
	return &copyAsset, nil
}

type fakeAdminSpaceExportPermissionSpaceRepo struct {
	spaces              map[string]*models.Space
	coverAssets         map[string]*models.SpaceCoverAsset
	updateMetadataCalls int
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) Create(context.Context, *models.Space) error {
	return nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) GetBySpaceID(_ context.Context, spaceID string) (*models.Space, error) {
	if f == nil {
		return nil, nil
	}
	space := f.spaces[spaceID]
	if space == nil {
		return nil, nil
	}
	copySpace := *space
	return &copySpace, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) GetCoverAssetByAssetID(_ context.Context, assetID string) (*models.SpaceCoverAsset, error) {
	if f == nil {
		return nil, nil
	}
	asset := f.coverAssets[strings.TrimSpace(assetID)]
	if asset == nil {
		return nil, nil
	}
	copyAsset := *asset
	return &copyAsset, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) ListByUserID(_ context.Context, userID string) ([]models.Space, error) {
	if f == nil {
		return []models.Space{}, nil
	}
	result := make([]models.Space, 0)
	for _, space := range f.spaces {
		if space != nil && space.OwnerUserID == userID {
			result = append(result, *space)
		}
	}
	return result, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) ListVisibleForHomepage(
	context.Context,
	repository.ListVisibleHomepageSpacesParams,
) ([]repository.HomepageVisibleSpaceRecord, int64, error) {
	return nil, 0, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) ListForAdmin(
	context.Context,
	repository.ListAdminSpacesParams,
) ([]repository.AdminSpaceListRecord, int64, error) {
	return nil, 0, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) ListMembers(context.Context, string) ([]repository.SpaceMemberListRecord, error) {
	return nil, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) UpsertMember(context.Context, repository.UpsertSpaceMemberParams) error {
	return nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) UpdateMemberRole(context.Context, repository.UpdateSpaceMemberRoleParams) (bool, error) {
	return false, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) DeleteMember(context.Context, string, string) (bool, error) {
	return false, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) CreateCoverAsset(context.Context, *models.SpaceCoverAsset) error {
	return nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) UpdateVisibility(context.Context, string, models.Visibility) (*models.Space, error) {
	return nil, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) UpdateStatus(context.Context, repository.UpdateSpaceStatusParams) (bool, error) {
	return false, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) UpdateMetadata(_ context.Context, params repository.UpdateSpaceMetadataParams) (bool, error) {
	if f == nil {
		return false, nil
	}
	f.updateMetadataCalls++
	space := f.spaces[strings.TrimSpace(params.SpaceID)]
	if space == nil {
		return false, nil
	}
	if params.Name != nil {
		space.Name = *params.Name
	}
	if params.Description != nil {
		space.Description = *params.Description
	}
	if params.Visibility != nil {
		space.Visibility = *params.Visibility
	}
	if params.CategoryID != nil {
		space.CategoryID = *params.CategoryID
	}
	if params.Category != nil {
		space.Category = *params.Category
	}
	if params.CoverAssetID != nil {
		value := strings.TrimSpace(*params.CoverAssetID)
		if value == "" {
			space.CoverAssetID = nil
		} else {
			space.CoverAssetID = &value
		}
	}
	if params.CoverKey != nil {
		space.CoverKey = *params.CoverKey
	}
	if params.CoverURL != nil {
		space.CoverURL = *params.CoverURL
	}
	if params.CoverSource != nil {
		space.CoverSource = *params.CoverSource
	}
	if params.CoverWidth != nil {
		space.CoverWidth = *params.CoverWidth
	}
	if params.CoverHeight != nil {
		space.CoverHeight = *params.CoverHeight
	}
	return true, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) IsMember(context.Context, string, string) (bool, error) {
	return false, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) TransferOwnership(context.Context, string, string, string, time.Time) (bool, error) {
	return false, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) SoftDelete(context.Context, string, time.Time) (bool, error) {
	return false, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) HardDelete(context.Context, string) (bool, error) {
	return false, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) HasReaderAccess(context.Context, string, string) (bool, error) {
	return false, nil
}

func (f *fakeAdminSpaceExportPermissionSpaceRepo) HasWriterAccess(context.Context, string, string) (bool, error) {
	return false, nil
}

type fakeAdminSpaceExportWorkspaceReader struct {
	nodes     []repository.WorkspaceTreeNodeRecord
	documents map[string]*repository.WorkspaceDocumentRecord
}

func (f *fakeAdminSpaceExportWorkspaceReader) ListTreeNodesBySpaceID(context.Context, string) ([]repository.WorkspaceTreeNodeRecord, error) {
	return append([]repository.WorkspaceTreeNodeRecord(nil), f.nodes...), nil
}

func (f *fakeAdminSpaceExportWorkspaceReader) GetDocumentByDocumentID(_ context.Context, documentID string) (*repository.WorkspaceDocumentRecord, error) {
	if f.documents == nil {
		return nil, errors.New("document not found")
	}
	document := f.documents[documentID]
	if document == nil {
		return nil, errors.New("document not found")
	}
	copyDocument := *document
	return &copyDocument, nil
}

type fakeAdminSpaceExportAttachmentReader struct {
	attachments map[string][]models.DocumentAttachment
	blobs       map[string]*models.DocumentAttachmentBlob
}

func (f *fakeAdminSpaceExportAttachmentReader) ListByDocumentID(_ context.Context, documentID string, _ bool) ([]models.DocumentAttachment, error) {
	return append([]models.DocumentAttachment(nil), f.attachments[documentID]...), nil
}

func (f *fakeAdminSpaceExportAttachmentReader) GetBlobByBlobID(_ context.Context, blobID string) (*models.DocumentAttachmentBlob, error) {
	blob := f.blobs[blobID]
	if blob == nil {
		return nil, errors.New("blob not found")
	}
	copyBlob := *blob
	return &copyBlob, nil
}

type fakeAdminSpaceExportBlobReader struct {
	contents map[string][]byte
}

func (f fakeAdminSpaceExportBlobReader) ReadBlobContent(_ context.Context, blob models.DocumentAttachmentBlob, _ string) ([]byte, error) {
	content, ok := f.contents[blob.BlobID]
	if !ok {
		return nil, errors.New("blob content not found")
	}
	return append([]byte(nil), content...), nil
}

type fakeAdminSpaceExportOfficeHTMLRenderer struct {
	html       string
	calls      int
	lastFormat models.DocumentFormat
	lastSource []byte
}

func (f *fakeAdminSpaceExportOfficeHTMLRenderer) RenderExportHTML(
	_ context.Context,
	format models.DocumentFormat,
	sourceContent []byte,
	_ string,
	_ string,
) (string, error) {
	f.calls++
	f.lastFormat = format
	f.lastSource = append([]byte(nil), sourceContent...)
	return f.html, nil
}

type fakeAdminSpaceExportReaderHTMLRenderer struct {
	html             string
	htmlByDocumentID map[string]string
	calls            int
	lastInput        AdminSpaceExportReaderHTMLRenderInput
}

func (f *fakeAdminSpaceExportReaderHTMLRenderer) RenderMarkdownHTML(
	_ context.Context,
	input AdminSpaceExportReaderHTMLRenderInput,
) (string, error) {
	f.calls++
	f.lastInput = input
	if f.htmlByDocumentID != nil {
		if html, ok := f.htmlByDocumentID[input.Document.DocumentID]; ok {
			return html, nil
		}
	}
	return f.html, nil
}

func assertZipEntryExists(t *testing.T, zipPath string, entryName string) {
	t.Helper()

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != entryName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s failed: %v", entryName, err)
		}
		defer rc.Close()
		if _, err := io.Copy(io.Discard, rc); err != nil {
			t.Fatalf("read zip entry %s failed: %v", entryName, err)
		}
		return
	}
	names := bytes.Buffer{}
	for _, file := range reader.File {
		_, _ = names.WriteString(file.Name + "\n")
	}
	t.Fatalf("zip entry %q not found; entries:\n%s", entryName, names.String())
}

func assertZipContainsText(t *testing.T, zipPath string, expected string) {
	t.Helper()

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s failed: %v", file.Name, err)
		}
		payload, err := io.ReadAll(rc)
		closeErr := rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s failed: %v", file.Name, err)
		}
		if closeErr != nil {
			t.Fatalf("close zip entry %s failed: %v", file.Name, closeErr)
		}
		if strings.Contains(string(payload), expected) {
			return
		}
	}
	t.Fatalf("zip %s does not contain text %q", zipPath, expected)
}

func assertZipDoesNotContainText(t *testing.T, zipPath string, unexpected string) {
	t.Helper()

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s failed: %v", file.Name, err)
		}
		payload, err := io.ReadAll(rc)
		closeErr := rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s failed: %v", file.Name, err)
		}
		if closeErr != nil {
			t.Fatalf("close zip entry %s failed: %v", file.Name, closeErr)
		}
		if strings.Contains(string(payload), unexpected) {
			t.Fatalf("zip entry %s unexpectedly contains %q", file.Name, unexpected)
		}
	}
}

func assertZipEntryMatches(t *testing.T, zipPath string, entryName string, pattern *regexp.Regexp) {
	t.Helper()

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != entryName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s failed: %v", entryName, err)
		}
		payload, err := io.ReadAll(rc)
		closeErr := rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s failed: %v", entryName, err)
		}
		if closeErr != nil {
			t.Fatalf("close zip entry %s failed: %v", entryName, closeErr)
		}
		if !pattern.Match(payload) {
			t.Fatalf("zip entry %s does not match %s:\n%s", entryName, pattern.String(), string(payload))
		}
		return
	}
	t.Fatalf("zip entry %q not found", entryName)
}

func assertZipEntryBytes(t *testing.T, zipPath string, entryName string, expected []byte) {
	t.Helper()

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != entryName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s failed: %v", entryName, err)
		}
		payload, err := io.ReadAll(rc)
		closeErr := rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s failed: %v", entryName, err)
		}
		if closeErr != nil {
			t.Fatalf("close zip entry %s failed: %v", entryName, closeErr)
		}
		if !bytes.Equal(payload, expected) {
			t.Fatalf("zip entry %s bytes mismatch: got %q want %q", entryName, string(payload), string(expected))
		}
		return
	}
	t.Fatalf("zip entry %q not found", entryName)
}
