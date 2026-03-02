package models

import "time"

const (
	// SearchAnalyzerDictEntryStatusActive 表示词条生效中。
	SearchAnalyzerDictEntryStatusActive = "active"
	// SearchAnalyzerDictEntryStatusDeleted 表示词条已删除。
	SearchAnalyzerDictEntryStatusDeleted = "deleted"
)

// SearchAnalyzerDictEntry 对应 search_analyzer_dict_entries 表，记录分词词典词条。
type SearchAnalyzerDictEntry struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Analyzer        string    `gorm:"column:analyzer"`
	Term            string    `gorm:"column:term"`
	Weight          *int      `gorm:"column:weight"`
	Tag             string    `gorm:"column:tag"`
	Status          string    `gorm:"column:status"`
	CreatedByUserID *string   `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string   `gorm:"column:updated_by_user_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (SearchAnalyzerDictEntry) TableName() string {
	return "search_analyzer_dict_entries"
}
