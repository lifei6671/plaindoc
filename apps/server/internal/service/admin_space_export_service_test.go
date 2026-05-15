package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
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

func TestAdminSpaceExportService_BuildsImportableMarkdownZipPackage(t *testing.T) {
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
	if !pkg.Manifest.Importable {
		t.Fatalf("expected importable manifest")
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
	space *models.Space
	err   error
}

func (f *fakeAdminSpaceExportSpaceReader) GetBySpaceID(context.Context, string) (*models.Space, error) {
	return f.space, f.err
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
