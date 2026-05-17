package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

func TestAdminSpaceTransferTaskHandler_ListMyTasks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-transfer-task-handler-list?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	ctx := context.Background()
	if err := storage.MigrateUp(ctx, database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}

	repo := repository.NewGormAdminSpaceTransferJobRepository(database.ORM)
	now := time.Now().UTC()
	for _, job := range []models.AdminSpaceTransferJob{
		{
			JobID:       "01handleractorarun0001",
			Kind:        models.AdminSpaceTransferJobKindExport,
			ActorUserID: "actor-a",
			Status:      models.AdminSpaceTransferJobStatusRunning,
			CreatedAt:   now,
			UpdatedAt:   now,
			ExpiresAt:   now.Add(30 * time.Minute),
		},
		{
			JobID:       "01handleractorbrun0001",
			Kind:        models.AdminSpaceTransferJobKindImport,
			ActorUserID: "actor-b",
			Status:      models.AdminSpaceTransferJobStatusRunning,
			CreatedAt:   now,
			UpdatedAt:   now,
			ExpiresAt:   now.Add(30 * time.Minute),
		},
	} {
		job := job
		if err := repo.Create(ctx, &job); err != nil {
			t.Fatalf("create job failed: %v", err)
		}
	}

	handler := NewAdminSpaceTransferTaskHandler(service.NewAdminSpaceTransferTaskService(repo))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("admin_actor_user_id", "actor-a")
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/space-transfer-tasks?status=active", nil)
	handler.ListMyTasks(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload response.JsonResult[listAdminSpaceTransferTasksResponse]
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(payload.Data.Tasks) != 1 {
		t.Fatalf("expected one task, got %#v", payload.Data.Tasks)
	}
	if payload.Data.Tasks[0].JobID != "01handleractorarun0001" {
		t.Fatalf("unexpected task returned: %#v", payload.Data.Tasks[0])
	}
}
