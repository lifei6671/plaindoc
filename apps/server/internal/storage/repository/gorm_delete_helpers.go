package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type workspaceNodeDeleteSnapshot struct {
	NodeID       string          `gorm:"column:node_id"`
	SpaceID      string          `gorm:"column:space_id"`
	ParentNodeID *string         `gorm:"column:parent_node_id"`
	Type         models.NodeType `gorm:"column:type"`
}

type workspaceNodeDeleteScope struct {
	Root        workspaceNodeDeleteSnapshot
	NodeIDs     []string
	DocumentIDs []string
}

// DeleteDocumentsCascadeInTx 统一删除文档主体及其关联的修订、权限、附件与图片引用。
func DeleteDocumentsCascadeInTx(tx *gorm.DB, documentIDSource any) (int64, error) {
	if tx == nil {
		return 0, fmt.Errorf("gorm tx is nil")
	}
	if !hasDeleteSource(documentIDSource) {
		return 0, nil
	}

	if err := tx.Table("document_file_revisions").
		Where("document_id IN (?)", documentIDSource).
		Delete(nil).Error; err != nil {
		return 0, err
	}
	if err := tx.Table("document_revisions").
		Where("document_id IN (?)", documentIDSource).
		Delete(nil).Error; err != nil {
		return 0, err
	}
	if err := tx.Table("document_permissions").
		Where("document_id IN (?)", documentIDSource).
		Delete(nil).Error; err != nil {
		return 0, err
	}
	if err := tx.Table("document_image_assets").
		Where("document_id IN (?)", documentIDSource).
		Delete(nil).Error; err != nil {
		return 0, err
	}
	if err := tx.Table("document_attachments").
		Where("document_id IN (?)", documentIDSource).
		Delete(nil).Error; err != nil {
		return 0, err
	}

	deleteResult := tx.Where("document_id IN (?)", documentIDSource).Delete(&models.Document{})
	if deleteResult.Error != nil {
		return 0, deleteResult.Error
	}
	return deleteResult.RowsAffected, nil
}

func deleteNodesCascadeInTx(tx *gorm.DB, nodeIDSource any) (int64, error) {
	if tx == nil {
		return 0, fmt.Errorf("gorm tx is nil")
	}
	if !hasDeleteSource(nodeIDSource) {
		return 0, nil
	}

	if err := tx.Table("node_permissions").
		Where("node_id IN (?)", nodeIDSource).
		Delete(nil).Error; err != nil {
		return 0, err
	}

	deleteResult := tx.Where("node_id IN (?)", nodeIDSource).Delete(&models.Node{})
	if deleteResult.Error != nil {
		return 0, deleteResult.Error
	}
	return deleteResult.RowsAffected, nil
}

func collectWorkspaceNodeDeleteScopeInTx(
	ctx context.Context,
	tx *gorm.DB,
	rootNodeID string,
) (workspaceNodeDeleteScope, error) {
	if tx == nil {
		return workspaceNodeDeleteScope{}, fmt.Errorf("gorm tx is nil")
	}

	normalizedRootNodeID := strings.TrimSpace(rootNodeID)
	if normalizedRootNodeID == "" {
		return workspaceNodeDeleteScope{}, gorm.ErrRecordNotFound
	}

	var root workspaceNodeDeleteSnapshot
	if err := tx.WithContext(ctx).
		Table("nodes").
		Select("node_id", "space_id", "parent_node_id", "type").
		Where("node_id = ?", normalizedRootNodeID).
		Take(&root).Error; err != nil {
		return workspaceNodeDeleteScope{}, err
	}

	allNodes := make([]workspaceNodeDeleteSnapshot, 0, 32)
	if err := tx.WithContext(ctx).
		Table("nodes").
		Select("node_id", "space_id", "parent_node_id", "type").
		Where("space_id = ?", strings.TrimSpace(root.SpaceID)).
		Find(&allNodes).Error; err != nil {
		return workspaceNodeDeleteScope{}, err
	}

	nodesByID := make(map[string]workspaceNodeDeleteSnapshot, len(allNodes))
	childrenByParent := make(map[string][]string, len(allNodes))
	for _, node := range allNodes {
		nodeID := strings.TrimSpace(node.NodeID)
		if nodeID == "" {
			continue
		}
		nodesByID[nodeID] = node
		parentKey := ""
		if node.ParentNodeID != nil {
			parentKey = strings.TrimSpace(*node.ParentNodeID)
		}
		childrenByParent[parentKey] = append(childrenByParent[parentKey], nodeID)
	}

	if _, exists := nodesByID[normalizedRootNodeID]; !exists {
		return workspaceNodeDeleteScope{}, gorm.ErrRecordNotFound
	}

	nodeIDs := make([]string, 0, 8)
	stack := []string{normalizedRootNodeID}
	visited := make(map[string]struct{}, len(allNodes))
	for len(stack) > 0 {
		last := len(stack) - 1
		currentNodeID := strings.TrimSpace(stack[last])
		stack = stack[:last]
		if currentNodeID == "" {
			continue
		}
		if _, exists := visited[currentNodeID]; exists {
			continue
		}
		if _, exists := nodesByID[currentNodeID]; !exists {
			continue
		}
		visited[currentNodeID] = struct{}{}
		nodeIDs = append(nodeIDs, currentNodeID)
		stack = append(stack, childrenByParent[currentNodeID]...)
	}

	documentIDs := make([]string, 0, len(nodeIDs))
	if len(nodeIDs) > 0 {
		type documentRow struct {
			DocumentID string `gorm:"column:document_id"`
		}
		documentRows := make([]documentRow, 0, len(nodeIDs))
		if err := tx.WithContext(ctx).
			Table("documents").
			Select("document_id").
			Where("node_id IN ?", nodeIDs).
			Find(&documentRows).Error; err != nil {
			return workspaceNodeDeleteScope{}, err
		}
		for _, row := range documentRows {
			documentID := strings.TrimSpace(row.DocumentID)
			if documentID == "" {
				continue
			}
			documentIDs = append(documentIDs, documentID)
		}
	}

	return workspaceNodeDeleteScope{
		Root:        root,
		NodeIDs:     nodeIDs,
		DocumentIDs: uniqueNonEmptyStrings(documentIDs),
	}, nil
}

func hasDeleteSource(source any) bool {
	switch value := source.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(value) != ""
	case []string:
		return len(value) > 0
	default:
		return true
	}
}

func uniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
