package repository

import (
	"strings"

	"gorm.io/gorm/schema"
)

func tableName[T schema.Tabler](model T) string {
	return model.TableName()
}

func tableWithAlias[T schema.Tabler](model T, alias string) string {
	if alias == "" {
		return model.TableName()
	}
	return model.TableName() + " AS " + alias
}

func qualifiedColumn(alias string, column string) string {
	if alias == "" {
		return column
	}
	return alias + "." + column
}

func selectColumns(columns ...string) string {
	return strings.Join(columns, ", ")
}
