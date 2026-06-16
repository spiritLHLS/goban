package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDebugPprofRequiresAuth(t *testing.T) {
	t.Setenv("PASSWORD", "test-password")
	t.Setenv("GOBAN_SECRET_KEY", "test-secret")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/goroutine", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected pprof route to require auth, got %d", resp.Code)
	}
}

func TestHealthRouteRemainsPublic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected health route to be public, got %d", resp.Code)
	}
}

func TestStaticFallbackDoesNotSwallowMissingAPIRoutes(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	dist := filepath.Join(tmp, "web", "dist")
	if err := os.MkdirAll(dist, 0750); err != nil {
		t.Fatalf("create dist dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte("<html>app</html>"), 0644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected missing API route to return 404, got %d", resp.Code)
	}
	if strings.Contains(resp.Body.String(), "<html>app</html>") {
		t.Fatal("missing API route should not return SPA index.html")
	}
	if contentType := resp.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected JSON response for missing API route, got %q", contentType)
	}
}
