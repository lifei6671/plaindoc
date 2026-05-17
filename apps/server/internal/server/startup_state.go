package server

import (
	"errors"
	"sync"
	"time"
)

// StartupPhase 表示服务启动过程中可对外展示的阶段。
type StartupPhase string

const (
	StartupPhaseBooting         StartupPhase = "booting"
	StartupPhaseOpeningDatabase StartupPhase = "opening_database"
	StartupPhaseMigrating       StartupPhase = "migrating"
	StartupPhaseBuildingRouter  StartupPhase = "building_router"
	StartupPhaseReady           StartupPhase = "ready"
	StartupPhaseFailed          StartupPhase = "failed"
)

// StartupSnapshot 是启动中间页和状态 API 暴露的安全快照。
type StartupSnapshot struct {
	Phase          StartupPhase `json:"phase"`
	Ready          bool         `json:"ready"`
	Failed         bool         `json:"failed"`
	Message        string       `json:"message"`
	CurrentVersion int          `json:"currentVersion,omitempty"`
	CurrentName    string       `json:"currentName,omitempty"`
	AppliedCount   int          `json:"appliedCount"`
	PendingCount   int          `json:"pendingCount"`
	TotalCount     int          `json:"totalCount"`
	StartedAt      string       `json:"startedAt"`
	UpdatedAt      string       `json:"updatedAt"`
}

// MigrationStartupProgress 是迁移器回调到启动状态的轻量进度结构。
type MigrationStartupProgress struct {
	Phase          string
	TotalCount     int
	PendingCount   int
	AppliedCount   int
	CurrentVersion int
	CurrentName    string
}

// StartupState 记录启动阶段；页面只读取安全文案，完整错误由调用方写日志。
type StartupState struct {
	mu       sync.RWMutex
	snapshot StartupSnapshot
	err      error
}

// NewStartupState 创建初始启动状态。
func NewStartupState() *StartupState {
	now := time.Now().UTC().Format(time.RFC3339)
	return &StartupState{
		snapshot: StartupSnapshot{
			Phase:     StartupPhaseBooting,
			Ready:     false,
			Failed:    false,
			Message:   "服务正在启动",
			StartedAt: now,
			UpdatedAt: now,
		},
	}
}

// SetPhase 设置启动阶段和面向用户的安全文案。
func (s *StartupState) SetPhase(phase StartupPhase, message string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Phase = phase
	s.snapshot.Message = normalizeStartupMessage(message, defaultStartupMessage(phase))
	s.snapshot.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

// SetMigrationProgress 将迁移进度更新到启动快照。
func (s *StartupState) SetMigrationProgress(progress MigrationStartupProgress) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Phase = StartupPhaseMigrating
	s.snapshot.Message = "正在迁移数据库"
	s.snapshot.TotalCount = progress.TotalCount
	s.snapshot.PendingCount = progress.PendingCount
	s.snapshot.AppliedCount = progress.AppliedCount
	s.snapshot.CurrentVersion = progress.CurrentVersion
	s.snapshot.CurrentName = progress.CurrentName
	if progress.Phase == "complete" {
		s.snapshot.Message = "数据库迁移已完成"
	}
	s.snapshot.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

// MarkReady 标记启动完成。
func (s *StartupState) MarkReady() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Phase = StartupPhaseReady
	s.snapshot.Ready = true
	s.snapshot.Failed = false
	s.snapshot.Message = "服务启动完成"
	s.snapshot.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.err = nil
}

// MarkFailed 标记启动失败。safeMessage 必须是可展示给用户的安全文案。
func (s *StartupState) MarkFailed(safeMessage string, err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Phase = StartupPhaseFailed
	s.snapshot.Ready = false
	s.snapshot.Failed = true
	s.snapshot.Message = normalizeStartupMessage(safeMessage, "服务初始化失败，请查看服务日志。")
	s.snapshot.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.err = err
}

// Err 返回启动失败的原始错误，仅供日志和测试使用，不能直接暴露到页面。
func (s *StartupState) Err() error {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

// Snapshot 返回当前启动状态的副本。
func (s *StartupState) Snapshot() StartupSnapshot {
	if s == nil {
		return StartupSnapshot{
			Phase:     StartupPhaseFailed,
			Failed:    true,
			Message:   "服务初始化状态不可用",
			StartedAt: time.Now().UTC().Format(time.RFC3339),
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

func normalizeStartupMessage(message string, fallback string) string {
	if message != "" {
		return message
	}
	return fallback
}

func defaultStartupMessage(phase StartupPhase) string {
	switch phase {
	case StartupPhaseOpeningDatabase:
		return "正在连接数据库"
	case StartupPhaseMigrating:
		return "正在迁移数据库"
	case StartupPhaseBuildingRouter:
		return "正在初始化服务"
	case StartupPhaseReady:
		return "服务启动完成"
	case StartupPhaseFailed:
		return "服务初始化失败，请查看服务日志。"
	default:
		return "服务正在启动"
	}
}

var errStartupHandlerNotSet = errors.New("startup switch handler is not set")
