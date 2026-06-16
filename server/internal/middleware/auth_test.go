package middleware

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBasicAuthRateLimitsRepeatedFailures(t *testing.T) {
	t.Setenv("GOBAN_USERNAME", "admin")
	t.Setenv("PASSWORD", "test-password")
	t.Setenv("GOBAN_SECRET_KEY", "test-secret")
	resetBasicAuthLimiterForTest()

	router := testAuthRouter()
	wrongAuth := basicAuthHeader("admin", "wrong-password")
	for i := 0; i < authFailureLimit; i++ {
		resp := performAuthRequest(router, wrongAuth)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, resp.Code)
		}
	}

	resp := performAuthRequest(router, wrongAuth)
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate limited status 429, got %d: %s", resp.Code, resp.Body.String())
	}
	if resp.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestBasicAuthSuccessClearsFailureCounter(t *testing.T) {
	t.Setenv("GOBAN_USERNAME", "admin")
	t.Setenv("PASSWORD", "test-password")
	t.Setenv("GOBAN_SECRET_KEY", "test-secret")
	resetBasicAuthLimiterForTest()

	router := testAuthRouter()
	wrongAuth := basicAuthHeader("admin", "wrong-password")
	for i := 0; i < authFailureLimit-1; i++ {
		resp := performAuthRequest(router, wrongAuth)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, resp.Code)
		}
	}

	success := performAuthRequest(router, basicAuthHeader("admin", "test-password"))
	if success.Code != http.StatusOK {
		t.Fatalf("expected successful auth to pass, got %d: %s", success.Code, success.Body.String())
	}

	resp := performAuthRequest(router, wrongAuth)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected counter reset after success, got %d", resp.Code)
	}
}

func testAuthRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BasicAuth())
	router.GET("/private", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return router
}

func performAuthRequest(router *gin.Engine, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set("Authorization", authorization)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func basicAuthHeader(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}
