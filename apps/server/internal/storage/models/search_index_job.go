package models

import "time"

const (
	// SearchIndexJobTypeDocUpsert 表示按文档 ID 增量 upsert 索引任务。
	SearchIndexJobTypeDocUpsert = "DOC_UPSERT"
	// SearchIndexJobTypeDocDelete 表示按文档 ID 删除索引任务。
	SearchIndexJobTypeDocDelete = "DOC_DELETE"
	// SearchIndexJobTypeSpacePurge 表示按空间 ID 清空索引任务。
	SearchIndexJobTypeSpacePurge = "SPACE_PURGE"
	// SearchIndexJobTypeRebuildSpace 表示按空间 ID 重建索引任务。
	SearchIndexJobTypeRebuildSpace = "REBUILD_SPACE"
)

const (
	// SearchIndexJobStatusPending 表示任务待执行。
	SearchIndexJobStatusPending = "pending"
	// SearchIndexJobStatusRunning 表示任务执行中。
	SearchIndexJobStatusRunning = "running"
	// SearchIndexJobStatusSuccess 表示任务执行成功。
	SearchIndexJobStatusSuccess = "success"
	// SearchIndexJobStatusFailed 表示任务执行失败（等待重试）。
	SearchIndexJobStatusFailed = "failed"
)

const (
	// SearchIndexJobPriorityHigh 表示高优先级任务（删除/权限收紧）。
	SearchIndexJobPriorityHigh = 10
	// SearchIndexJobPriorityNormal 表示普通优先级任务。
	SearchIndexJobPriorityNormal = 100
)

// SearchIndexJob 对应 search_index_jobs 表。
type SearchIndexJob struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement"`
	JobID       string     `gorm:"column:job_id"`
	Provider    string     `gorm:"column:provider"`
	JobType     string     `gorm:"column:job_type"`
	DedupeKey   string     `gorm:"column:dedupe_key"`
	PayloadJSON string     `gorm:"column:payload_json"`
	Status      string     `gorm:"column:status"`
	Priority    int        `gorm:"column:priority"`
	RetryCount  int        `gorm:"column:retry_count"`
	NextRunAt   time.Time  `gorm:"column:next_run_at"`
	StartedAt   *time.Time `gorm:"column:started_at"`
	LastError   string     `gorm:"column:last_error"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (SearchIndexJob) TableName() string {
	return "search_index_jobs"
}

// IsValidSearchIndexJobType 判断任务类型是否合法。
func IsValidSearchIndexJobType(value string) bool {
	switch value {
	case SearchIndexJobTypeDocUpsert,
		SearchIndexJobTypeDocDelete,
		SearchIndexJobTypeSpacePurge,
		SearchIndexJobTypeRebuildSpace:
		return true
	default:
		return false
	}
}

// IsValidSearchIndexJobStatus 判断任务状态是否合法。
func IsValidSearchIndexJobStatus(value string) bool {
	switch value {
	case SearchIndexJobStatusPending,
		SearchIndexJobStatusRunning,
		SearchIndexJobStatusSuccess,
		SearchIndexJobStatusFailed:
		return true
	default:
		return false
	}
}
