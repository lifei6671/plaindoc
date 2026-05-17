package models

import "time"

const (
	// AdminSpaceTransferJobKindExport 表示空间导出任务。
	AdminSpaceTransferJobKindExport = "space_export"
	// AdminSpaceTransferJobKindImport 表示空间导入任务。
	AdminSpaceTransferJobKindImport = "space_import"
)

const (
	// AdminSpaceTransferJobStatusQueued 表示任务已创建，等待执行。
	AdminSpaceTransferJobStatusQueued = "queued"
	// AdminSpaceTransferJobStatusRunning 表示任务执行中。
	AdminSpaceTransferJobStatusRunning = "running"
	// AdminSpaceTransferJobStatusCompleted 表示任务已完成。
	AdminSpaceTransferJobStatusCompleted = "completed"
	// AdminSpaceTransferJobStatusFailed 表示任务失败。
	AdminSpaceTransferJobStatusFailed = "failed"
)

// AdminSpaceTransferJob 对应 admin_space_transfer_jobs 表。
type AdminSpaceTransferJob struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement"`
	JobID        string     `gorm:"column:job_id"`
	Kind         string     `gorm:"column:kind"`
	ActorUserID  string     `gorm:"column:actor_user_id"`
	SpaceID      string     `gorm:"column:space_id"`
	SpaceName    string     `gorm:"column:space_name"`
	Format       string     `gorm:"column:format"`
	ImportID     string     `gorm:"column:import_id"`
	Status       string     `gorm:"column:status"`
	Stage        string     `gorm:"column:stage"`
	Progress     int        `gorm:"column:progress"`
	Message      string     `gorm:"column:message"`
	FilePath     string     `gorm:"column:file_path"`
	FileName     string     `gorm:"column:file_name"`
	SizeBytes    int64      `gorm:"column:size_bytes"`
	NewSpaceID   string     `gorm:"column:new_space_id"`
	ErrorMessage string     `gorm:"column:error_message"`
	StartedAt    *time.Time `gorm:"column:started_at"`
	CompletedAt  *time.Time `gorm:"column:completed_at"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`
	ExpiresAt    time.Time  `gorm:"column:expires_at"`
}

func (AdminSpaceTransferJob) TableName() string {
	return "admin_space_transfer_jobs"
}

var AdminSpaceTransferJobColumns = struct {
	ID           string
	JobID        string
	Kind         string
	ActorUserID  string
	SpaceID      string
	SpaceName    string
	Format       string
	ImportID     string
	Status       string
	Stage        string
	Progress     string
	Message      string
	FilePath     string
	FileName     string
	SizeBytes    string
	NewSpaceID   string
	ErrorMessage string
	StartedAt    string
	CompletedAt  string
	CreatedAt    string
	UpdatedAt    string
	ExpiresAt    string
}{
	ID:           "id",
	JobID:        "job_id",
	Kind:         "kind",
	ActorUserID:  "actor_user_id",
	SpaceID:      "space_id",
	SpaceName:    "space_name",
	Format:       "format",
	ImportID:     "import_id",
	Status:       "status",
	Stage:        "stage",
	Progress:     "progress",
	Message:      "message",
	FilePath:     "file_path",
	FileName:     "file_name",
	SizeBytes:    "size_bytes",
	NewSpaceID:   "new_space_id",
	ErrorMessage: "error_message",
	StartedAt:    "started_at",
	CompletedAt:  "completed_at",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
	ExpiresAt:    "expires_at",
}

// IsActiveAdminSpaceTransferJobStatus 判断任务是否仍处于活跃状态。
func IsActiveAdminSpaceTransferJobStatus(status string) bool {
	return status == AdminSpaceTransferJobStatusQueued || status == AdminSpaceTransferJobStatusRunning
}

// IsValidAdminSpaceTransferJobKind 判断任务类型是否合法。
func IsValidAdminSpaceTransferJobKind(kind string) bool {
	return kind == AdminSpaceTransferJobKindExport || kind == AdminSpaceTransferJobKindImport
}

// IsValidAdminSpaceTransferJobStatus 判断任务状态是否合法。
func IsValidAdminSpaceTransferJobStatus(status string) bool {
	switch status {
	case AdminSpaceTransferJobStatusQueued,
		AdminSpaceTransferJobStatusRunning,
		AdminSpaceTransferJobStatusCompleted,
		AdminSpaceTransferJobStatusFailed:
		return true
	default:
		return false
	}
}
