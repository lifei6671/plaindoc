package handler

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestAdminSpaceTransferSSEHeartbeatIntervalStaysBelowCommonProxyIdleTimeout(t *testing.T) {
	t.Parallel()

	if adminSpaceTransferSSEHeartbeatInterval >= 10*time.Second {
		t.Fatalf("SSE heartbeat interval must stay below 10s idle timeout, got %s", adminSpaceTransferSSEHeartbeatInterval)
	}
}

func TestClearAdminSpaceTransferSSEWriteDeadlineReportsUnsupportedWriter(t *testing.T) {
	t.Parallel()

	err := clearAdminSpaceTransferSSEWriteDeadline(httptest.NewRecorder())

	if err == nil {
		t.Fatal("expected unsupported writer error")
	}
}
