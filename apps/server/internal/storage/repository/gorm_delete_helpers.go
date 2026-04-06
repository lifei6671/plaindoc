package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type workspaceNodeDeleteSnapshot = workspaceNodeDeleteSnapshotDB

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

	if err := tx.Model(&models.DocumentFileRevision{}).
		Where(qualifiedColumn("", models.DocumentFileRevisionColumns.DocumentID)+" IN (?)", documentIDSource).
		Delete(nil).Error; err != nil {
		return 0, err
	}
	if err := tx.Model(&models.DocumentRevision{}).
		Where(qualifiedColumn("", models.DocumentRevisionColumns.DocumentID)+" IN (?)", documentIDSource).
		Delete(nil).Error; err != nil {
		return 0, err
	}
	if err := tx.Model(&models.DocumentPermission{}).
		Where(qualifiedColumn("", models.DocumentColumns.DocumentID)+" IN (?)", documentIDSource).
		Delete(nil).Error; err != nil {
		return 0, err
	}
	if err := tx.Model(&models.DocumentImageAsset{}).
		Where(qualifiedColumn("", models.DocumentImageAssetColumns.DocumentID)+" IN (?)", documentIDSource).
		Delete(nil).Error; err != nil {
		return 0, err
	}
	if err := tx.Model(&models.DocumentAttachment{}).
		Where(qualifiedColumn("", models.DocumentAttachmentColumns.DocumentID)+" IN (?)", documentIDSource).
		Delete(nil).Error; err != nil {
		return 0, err
	}

	deleteResult := tx.Where(qualifiedColumn("", models.DocumentColumns.DocumentID)+" IN (?)", documentIDSource).Delete(&models.Document{})
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

	if err := tx.Model(&models.NodePermission{}).
		Where(qualifiedColumn("", models.NodeColumns.NodeID)+" IN (?)", nodeIDSource).
		Delete(nil).Error; err != nil {
		return 0, err
	}

	deleteResult := tx.Where(qualifiedColumn("", models.NodeColumns.NodeID)+" IN (?)", nodeIDSource).Delete(&models.Node{})
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
		Model(&models.Node{}).
		Select(selectColumns(
			qualifiedColumn("", models.NodeColumns.NodeID),
			qualifiedColumn("", models.NodeColumns.SpaceID),
			qualifiedColumn("", models.NodeColumns.ParentNodeID),
			qualifiedColumn("", models.NodeColumns.Type),
		)).
		Where(qualifiedColumn("", models.NodeColumns.NodeID)+" = ?", normalizedRootNodeID).
		Take(&root).Error; err != nil {
		return workspaceNodeDeleteScope{}, err
	}

	allNodes := make([]workspaceNodeDeleteSnapshot, 0, 32)
	if err := tx.WithContext(ctx).
		Model(&models.Node{}).
		Select(selectColumns(
			qualifiedColumn("", models.NodeColumns.NodeID),
			qualifiedColumn("", models.NodeColumns.SpaceID),
			qualifiedColumn("", models.NodeColumns.ParentNodeID),
			qualifiedColumn("", models.NodeColumns.Type),
		)).
		Where(qualifiedColumn("", models.NodeColumns.SpaceID)+" = ?", strings.TrimSpace(root.SpaceID)).
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
			Model(&models.Document{}).
			Select(qualifiedColumn("", models.DocumentColumns.DocumentID)).
			Where(qualifiedColumn("", models.DocumentColumns.NodeID)+" IN ?", nodeIDs).
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
