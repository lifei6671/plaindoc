package service

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

func TestAdminAuditService_Record_AllowsSpaceTransferAuditForNonAdmin(t *testing.T) {
	t.Parallel()

	auditRepo := &stubSpaceTransferAuditLogRepo{}
	adminAccessService := NewAdminAccessService(newStubAdminRoleRepo(nil), newStubSpaceAdminScopeRepo(nil), nil)
	svc := NewAdminAuditService(auditRepo, adminAccessService)

	err := svc.Record(context.Background(), RecordAdminAuditInput{
		ActorUserID: "actor-user",
		Module:      AdminAuditModuleSpace,
		Action:      AdminAuditActionExport,
		TargetType:  "space",
		TargetID:    "space-a",
		Summary:     "space export queued: space-a",
		Detail: map[string]any{
			"status":      "queued",
			"abilityType": "space_manage",
		},
	})
	if err != nil {
		t.Fatalf("record transfer audit failed: %v", err)
	}
	if len(auditRepo.created) != 1 {
		t.Fatalf("expected one audit log, got %d", len(auditRepo.created))
	}
	if auditRepo.created[0].Module != string(AdminAuditModuleSpace) ||
		auditRepo.created[0].Action != string(AdminAuditActionExport) ||
		auditRepo.created[0].TargetType != "space" ||
		auditRepo.created[0].TargetID != "space-a" {
		t.Fatalf("unexpected audit log: %#v", auditRepo.created[0])
	}
}

type recordingAdminAuditRecorder struct {
	mu      sync.Mutex
	records []RecordAdminAuditInput
	err     error
}

func (r *recordingAdminAuditRecorder) Record(_ context.Context, input RecordAdminAuditInput) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, input)
	return r.err
}

func (r *recordingAdminAuditRecorder) Records() []RecordAdminAuditInput {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]RecordAdminAuditInput(nil), r.records...)
}

func findAuditRecordByStatus(
	t *testing.T,
	records []RecordAdminAuditInput,
	status string,
) (RecordAdminAuditInput, map[string]any) {
	t.Helper()

	for _, record := range records {
		detail := auditDetailMap(t, record)
		if detailStatus, _ := detail["status"].(string); detailStatus == status {
			return record, detail
		}
	}
	t.Fatalf("audit record with status %q not found in %#v", status, records)
	return RecordAdminAuditInput{}, nil
}

func auditDetailMap(t *testing.T, record RecordAdminAuditInput) map[string]any {
	t.Helper()

	detail, ok := record.Detail.(map[string]any)
	if !ok {
		t.Fatalf("expected audit detail map, got %T", record.Detail)
	}
	return detail
}

func assertAuditDetailHasNoTransferSecret(t *testing.T, detail map[string]any) {
	t.Helper()

	for key := range detail {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if strings.Contains(normalized, "token") || normalized == "filepath" || normalized == "file_path" {
			t.Fatalf("audit detail leaked transfer secret key %q: %#v", key, detail)
		}
	}
}

func TestSanitizeAdminSpaceTransferAuditMessage_HidesSecretsAndPrivatePaths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		message string
		hints   []string
		want    string
	}{
		{
			name:    "token",
			message: "download token invalid",
			want:    adminSpaceTransferAuditHiddenError,
		},
		{
			name:    "private relative dir",
			message: "rename data/exports/admin-space/job.part data/exports/admin-space/job.zip: permission denied",
			hints:   []string{defaultAdminSpaceExportDir},
			want:    adminSpaceTransferAuditHiddenError,
		},
		{
			name:    "absolute path",
			message: "open /tmp/plaindoc/import.zip: permission denied",
			want:    adminSpaceTransferAuditHiddenError,
		},
		{
			name:    "business message",
			message: "manifest 引用文件缺失: documents/missing.md",
			want:    "manifest 引用文件缺失: documents/missing.md",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := sanitizeAdminSpaceTransferAuditMessage(tc.message, tc.hints...)
			if got != tc.want {
				t.Fatalf("unexpected sanitized message: got %q want %q", got, tc.want)
			}
		})
	}
}

type stubSpaceTransferAuditLogRepo struct {
	created []models.AuditLog
}

func (r *stubSpaceTransferAuditLogRepo) Create(_ context.Context, auditLog *models.AuditLog) error {
	if auditLog != nil {
		r.created = append(r.created, *auditLog)
	}
	return nil
}

func (r *stubSpaceTransferAuditLogRepo) List(
	context.Context,
	repository.ListAuditLogsParams,
) ([]repository.AdminAuditLogListRecord, int64, error) {
	return nil, 0, nil
}
