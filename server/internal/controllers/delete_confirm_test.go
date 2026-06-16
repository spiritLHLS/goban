package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireDeleteConfirmation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/items/:id", func(c *gin.Context) {
		if !requireDeleteConfirmation(c, "demo") {
			return
		}
		respondOK(c, gin.H{"deleted": true})
	})

	tests := []struct {
		name       string
		path       string
		body       string
		wantStatus int
	}{
		{
			name:       "missing confirmation",
			path:       "/items/7",
			wantStatus: http.StatusPreconditionRequired,
		},
		{
			name:       "query confirmation",
			path:       "/items/7?confirm_id=7&confirm_text=demo",
			wantStatus: http.StatusOK,
		},
		{
			name:       "json confirmation",
			path:       "/items/7",
			body:       `{"confirm_id":"7","confirm_text":"demo"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "numeric json confirmation",
			path:       "/items/7",
			body:       `{"confirm_id":7,"confirm_text":"DELETE"}`,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, tt.path, strings.NewReader(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}
