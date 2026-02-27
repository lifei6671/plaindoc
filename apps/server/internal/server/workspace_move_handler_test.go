package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
)

func TestRouter_MoveNode_ReorderWithinSameParent(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-move-same-parent@example.com")
	spaceID := "01h1movespace000000000000001"
	seedWorkspaceMoveSpace(t, database, ownerUserID, spaceID)
	seedWorkspaceMoveNode(t, database, "01h1movenode-root-a00000000001", spaceID, nil, "folder", "Root A", 1)
	seedWorkspaceMoveNode(t, database, "01h1movenode-root-b00000000002", spaceID, nil, "folder", "Root B", 2)
	seedWorkspaceMoveNode(t, database, "01h1movenode-root-c00000000003", spaceID, nil, "doc", "Root C", 3)

	moveReq := httptest.NewRequest(
		http.MethodPost,
		"/api/nodes/01h1movenode-root-b00000000002/move",
		bytes.NewReader([]byte(`{"targetParentId":null,"targetIndex":0}`)),
	)
	moveReq.Header.Set("Authorization", "Bearer "+ownerToken)
	moveReq.Header.Set("Content-Type", "application/json")
	moveRec := serve(moveReq)
	if moveRec.Code != http.StatusOK {
		t.Fatalf("expected move status 200, got %d body=%s", moveRec.Code, moveRec.Body.String())
	}
	if decodeJSONResultCode(t, moveRec.Body.Bytes()) != response.SuccessCode {
		t.Fatalf("expected move success code 0, got %d", decodeJSONResultCode(t, moveRec.Body.Bytes()))
	}

	rootChildren := listWorkspaceMoveSiblingIDs(t, database, spaceID, nil)
	assertWorkspaceMoveNodeOrder(
		t,
		rootChildren,
		[]string{
			"01h1movenode-root-b00000000002",
			"01h1movenode-root-a00000000001",
			"01h1movenode-root-c00000000003",
		},
	)
}

func TestRouter_MoveNode_MoveAcrossParentAllowsDocParent(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-move-cross-parent@example.com")
	spaceID := "01h1movecrossspace00000000001"
	seedWorkspaceMoveSpace(t, database, ownerUserID, spaceID)

	sourceFolderNodeID := "01h1movesourcefolder000000001"
	targetDocNodeID := "01h1movetargetdoc000000000002"
	rootDocNodeID := "01h1moverootdoc00000000000003"
	seedWorkspaceMoveNode(t, database, sourceFolderNodeID, spaceID, nil, "folder", "Source Folder", 1)
	seedWorkspaceMoveNode(t, database, targetDocNodeID, spaceID, nil, "doc", "Target Doc Node", 2)
	seedWorkspaceMoveNode(t, database, rootDocNodeID, spaceID, nil, "doc", "Root Doc", 3)

	moveReq := httptest.NewRequest(
		http.MethodPost,
		"/api/nodes/"+sourceFolderNodeID+"/move",
		bytes.NewReader([]byte(`{"targetParentId":"`+targetDocNodeID+`","targetIndex":0}`)),
	)
	moveReq.Header.Set("Authorization", "Bearer "+ownerToken)
	moveReq.Header.Set("Content-Type", "application/json")
	moveRec := serve(moveReq)
	if moveRec.Code != http.StatusOK {
		t.Fatalf("expected move status 200, got %d body=%s", moveRec.Code, moveRec.Body.String())
	}
	if decodeJSONResultCode(t, moveRec.Body.Bytes()) != response.SuccessCode {
		t.Fatalf("expected move success code 0, got %d", decodeJSONResultCode(t, moveRec.Body.Bytes()))
	}

	rootChildren := listWorkspaceMoveSiblingIDs(t, database, spaceID, nil)
	assertWorkspaceMoveNodeOrder(
		t,
		rootChildren,
		[]string{
			targetDocNodeID,
			rootDocNodeID,
		},
	)

	targetChildren := listWorkspaceMoveSiblingIDs(t, database, spaceID, &targetDocNodeID)
	assertWorkspaceMoveNodeOrder(t, targetChildren, []string{sourceFolderNodeID})
}

func TestRouter_MoveNode_RejectsMoveToDescendant(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-move-cycle@example.com")
	spaceID := "01h1movecyclespace000000001"
	seedWorkspaceMoveSpace(t, database, ownerUserID, spaceID)

	rootFolderNodeID := "01h1move-root-folder00000001"
	childFolderNodeID := "01h1move-child-folder00000002"
	grandChildFolderNodeID := "01h1move-grand-folder0000003"
	seedWorkspaceMoveNode(t, database, rootFolderNodeID, spaceID, nil, "folder", "Root", 1)
	seedWorkspaceMoveNode(t, database, childFolderNodeID, spaceID, &rootFolderNodeID, "folder", "Child", 1)
	seedWorkspaceMoveNode(
		t,
		database,
		grandChildFolderNodeID,
		spaceID,
		&childFolderNodeID,
		"folder",
		"Grand Child",
		1,
	)

	moveReq := httptest.NewRequest(
		http.MethodPost,
		"/api/nodes/"+rootFolderNodeID+"/move",
		bytes.NewReader([]byte(`{"targetParentId":"`+grandChildFolderNodeID+`","targetIndex":0}`)),
	)
	moveReq.Header.Set("Authorization", "Bearer "+ownerToken)
	moveReq.Header.Set("Content-Type", "application/json")
	moveRec := serve(moveReq)
	if moveRec.Code != http.StatusOK {
		t.Fatalf("expected move cycle status 200, got %d body=%s", moveRec.Code, moveRec.Body.String())
	}
	if decodeJSONResultCode(t, moveRec.Body.Bytes()) != response.ResolveErrorCode(response.CodeInvalidNodeMove) {
		t.Fatalf(
			"expected code %d, got %d",
			response.ResolveErrorCode(response.CodeInvalidNodeMove),
			decodeJSONResultCode(t, moveRec.Body.Bytes()),
		)
	}
}

func TestRouter_MoveNode_ReaderForbidden(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "owner-move-forbidden@example.com")
	readerUserID, _, readerToken := registerAccessUser(t, serve, "reader-move-forbidden@example.com")
	spaceID := "01h1moveforbiddenspace00001"
	seedWorkspaceMoveSpace(t, database, ownerUserID, spaceID)
	seedWorkspaceMoveNode(t, database, "01h1moveforbiddennode000001", spaceID, nil, "folder", "Folder", 1)
	seedWorkspaceMoveNode(t, database, "01h1moveforbiddennode000002", spaceID, nil, "folder", "Folder2", 2)
	seedSpaceMemberForAccess(t, database, spaceID, readerUserID, "reader")

	moveReq := httptest.NewRequest(
		http.MethodPost,
		"/api/nodes/01h1moveforbiddennode000002/move",
		bytes.NewReader([]byte(`{"targetParentId":null,"targetIndex":0}`)),
	)
	moveReq.Header.Set("Authorization", "Bearer "+readerToken)
	moveReq.Header.Set("Content-Type", "application/json")
	moveRec := serve(moveReq)
	if moveRec.Code != http.StatusForbidden {
		t.Fatalf("expected reader move status 403, got %d body=%s", moveRec.Code, moveRec.Body.String())
	}
	if decodeJSONResultCode(t, moveRec.Body.Bytes()) != response.ResolveErrorCode(response.CodeForbidden) {
		t.Fatalf(
			"expected forbidden code %d, got %d",
			response.ResolveErrorCode(response.CodeForbidden),
			decodeJSONResultCode(t, moveRec.Body.Bytes()),
		)
	}
}

func seedWorkspaceMoveSpace(
	t *testing.T,
	database *storage.Database,
	ownerUserID string,
	spaceID string,
) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("spaces").Create(map[string]any{
		"space_id":      spaceID,
		"name":          "Workspace Move Space",
		"owner_user_id": ownerUserID,
		"visibility":    "member",
		"status":        "active",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("insert move test space failed: %v", err)
	}
}

func seedWorkspaceMoveNode(
	t *testing.T,
	database *storage.Database,
	nodeID string,
	spaceID string,
	parentNodeID *string,
	nodeType string,
	title string,
	sort int,
) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("nodes").Create(map[string]any{
		"node_id":        nodeID,
		"space_id":       spaceID,
		"parent_node_id": parentNodeID,
		"type":           nodeType,
		"title":          title,
		"sort":           sort,
		"created_at":     now,
		"updated_at":     now,
	}).Error; err != nil {
		t.Fatalf("insert move test node failed: %v", err)
	}
}

func listWorkspaceMoveSiblingIDs(
	t *testing.T,
	database *storage.Database,
	spaceID string,
	parentNodeID *string,
) []string {
	t.Helper()

	type siblingRow struct {
		NodeID string `gorm:"column:node_id"`
	}

	query := database.ORM.Table("nodes").
		Select("node_id").
		Where("space_id = ?", spaceID)
	if parentNodeID == nil {
		query = query.Where("parent_node_id IS NULL")
	} else {
		query = query.Where("parent_node_id = ?", *parentNodeID)
	}

	var rows []siblingRow
	if err := query.Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		t.Fatalf("query move sibling nodes failed: %v", err)
	}

	nodeIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		nodeIDs = append(nodeIDs, row.NodeID)
	}
	return nodeIDs
}

func assertWorkspaceMoveNodeOrder(t *testing.T, actual []string, expected []string) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Fatalf("expected %d nodes, got %d nodes: actual=%v expected=%v", len(expected), len(actual), actual, expected)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("expected order=%v, got order=%v", expected, actual)
		}
	}
}
