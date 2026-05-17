package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestTimeoutSkipsServerSentEventRequests(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Timeout(10 * time.Second))
	router.GET("/api/admin/spaces/:spaceId/exports/:jobId/events", func(c *gin.Context) {
		if _, ok := c.Request.Context().Deadline(); ok {
			t.Fatal("SSE request context should not inherit business request timeout")
		}
		c.Status(http.StatusOK)
	})
	router.GET("/api/admin/space-imports/:jobId/events", func(c *gin.Context) {
		if _, ok := c.Request.Context().Deadline(); ok {
			t.Fatal("SSE request context should not inherit business request timeout")
		}
		c.Status(http.StatusOK)
	})

	for _, path := range []string{
		"/api/admin/spaces/space-a/exports/job-a/events",
		"/api/admin/space-imports/job-a/events",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Accept", "text/event-stream")
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200 for %s, got %d", path, recorder.Code)
		}
	}
}

func TestTimeoutKeepsDeadlineForRegularRequests(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Timeout(10 * time.Second))
	router.GET("/api/admin/spaces", func(c *gin.Context) {
		if _, ok := c.Request.Context().Deadline(); !ok {
			t.Fatal("regular request context should inherit business request timeout")
		}
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/admin/spaces", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestTimeoutKeepsDeadlineForEventsPathWithoutSSEAccept(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Timeout(10 * time.Second))
	router.GET("/api/internal/events", func(c *gin.Context) {
		if _, ok := c.Request.Context().Deadline(); !ok {
			t.Fatal("non-SSE /events request should still inherit business request timeout")
		}
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/internal/events", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestTimeoutKeepsDeadlineForRegularRequestsWithSSEAccept(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Timeout(10 * time.Second))
	router.GET("/api/admin/spaces", func(c *gin.Context) {
		if _, ok := c.Request.Context().Deadline(); !ok {
			t.Fatal("regular request with SSE Accept should still inherit business request timeout")
		}
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/admin/spaces", nil)
	request.Header.Set("Accept", "text/event-stream")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}
