package server

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestStartupStateTracksMigrationProgressAndReadySnapshot(t *testing.T) {
	state := NewStartupState()

	state.SetPhase(StartupPhaseOpeningDatabase, "正在连接数据库")
	state.SetMigrationProgress(MigrationStartupProgress{
		Phase:          "applying",
		TotalCount:     34,
		PendingCount:   4,
		AppliedCount:   31,
		CurrentVersion: 32,
		CurrentName:    "reader_slug",
	})
	state.MarkReady()

	snapshot := state.Snapshot()
	if !snapshot.Ready || snapshot.Failed {
		t.Fatalf("expected ready snapshot without failure, got ready=%v failed=%v", snapshot.Ready, snapshot.Failed)
	}
	if snapshot.Phase != StartupPhaseReady {
		t.Fatalf("expected ready phase, got %q", snapshot.Phase)
	}
	if snapshot.TotalCount != 34 || snapshot.PendingCount != 4 || snapshot.AppliedCount != 31 {
		t.Fatalf("unexpected migration counts: %+v", snapshot)
	}
	if snapshot.CurrentVersion != 32 || snapshot.CurrentName != "reader_slug" {
		t.Fatalf("unexpected current migration: %+v", snapshot)
	}
	if snapshot.StartedAt == "" || snapshot.UpdatedAt == "" {
		t.Fatalf("expected timestamps in snapshot: %+v", snapshot)
	}
}

func TestStartupStateFailedSnapshotUsesSafeMessage(t *testing.T) {
	state := NewStartupState()
	state.MarkFailed("数据库连接失败，请检查服务日志和数据库配置。", errors.New("dsn postgres://user:secret@localhost/plain failed"))

	snapshot := state.Snapshot()
	if !snapshot.Failed || snapshot.Ready {
		t.Fatalf("expected failed snapshot without ready, got ready=%v failed=%v", snapshot.Ready, snapshot.Failed)
	}
	if snapshot.Phase != StartupPhaseFailed {
		t.Fatalf("expected failed phase, got %q", snapshot.Phase)
	}
	if snapshot.Message != "数据库连接失败，请检查服务日志和数据库配置。" {
		t.Fatalf("unexpected safe message %q", snapshot.Message)
	}
	if strings.Contains(snapshot.Message, "secret") || strings.Contains(snapshot.Message, "postgres://") {
		t.Fatalf("safe message leaked sensitive detail: %q", snapshot.Message)
	}
}

func TestStartupStateSnapshotIsConcurrentSafe(t *testing.T) {
	state := NewStartupState()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(version int) {
			defer wg.Done()
			state.SetMigrationProgress(MigrationStartupProgress{
				Phase:          "applying",
				TotalCount:     20,
				PendingCount:   10,
				AppliedCount:   version,
				CurrentVersion: version,
				CurrentName:    "concurrent",
			})
			_ = state.Snapshot()
		}(i)
	}
	wg.Wait()

	snapshot := state.Snapshot()
	if snapshot.Phase != StartupPhaseMigrating {
		t.Fatalf("expected migrating phase after concurrent progress, got %q", snapshot.Phase)
	}
}
