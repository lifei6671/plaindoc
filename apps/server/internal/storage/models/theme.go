package models

import "time"

// Theme 对应 themes 表。
type Theme struct {
	ID                     int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ThemeID                string    `gorm:"column:theme_id"`
	Name                   string    `gorm:"column:name"`
	Description            string    `gorm:"column:description"`
	VariablesJSON          string    `gorm:"column:variables_json"`
	SyntaxTheme            string    `gorm:"column:syntax_theme"`
	CodeBlockStyleJSON     string    `gorm:"column:code_block_style_json"`
	CodeBlockCodeStyleJSON string    `gorm:"column:code_block_code_style_json"`
	InlineCodeStyleJSON    string    `gorm:"column:inline_code_style_json"`
	CustomCSS              string    `gorm:"column:custom_css"`
	IsBuiltin              bool      `gorm:"column:is_builtin"`
	CreatedAt              time.Time `gorm:"column:created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at"`
}

func (Theme) TableName() string {
	return "themes"
}
