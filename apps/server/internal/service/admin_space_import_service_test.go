package service

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
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

func TestAdminSpaceImportService_Commit_RejectsOtherActorsStaging(t *testing.T) {
	t.Parallel()

	svc := NewAdminSpaceImportService(nil)
	result, err := svc.Inspect(context.Background(), InspectAdminSpaceImportInput{
		ActorUserID: "actor-user",
		FileName:    "space.zip",
		ContentType: "application/zip",
		Reader:      bytes.NewReader([]byte("zip")),
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
		FileName:    "space.zip",
		ContentType: "application/zip",
		Reader:      bytes.NewReader([]byte("zip")),
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
		FileName:    "space.zip",
		ContentType: "application/zip",
		Reader:      bytes.NewReader([]byte("zip")),
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
