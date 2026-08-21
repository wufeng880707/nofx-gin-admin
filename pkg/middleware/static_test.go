package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStaticWithConfigCacheAndFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("app"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "umi.1234abcd.js"), []byte("js"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		path         string
		status       int
		body         string
		cacheControl string
	}{
		{
			name:         "entry is never stored",
			path:         "/",
			status:       http.StatusOK,
			body:         "app",
			cacheControl: "no-cache, no-store, must-revalidate",
		},
		{
			name:         "client route serves fresh entry",
			path:         "/system/role",
			status:       http.StatusOK,
			body:         "app",
			cacheControl: "no-cache, no-store, must-revalidate",
		},
		{
			name:         "hashed asset is immutable",
			path:         "/umi.1234abcd.js",
			status:       http.StatusOK,
			body:         "js",
			cacheControl: "public, max-age=31536000, immutable",
		},
		{
			name:   "missing asset is not an SPA fallback",
			path:   "/missing.1234abcd.js",
			status: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(StaticWithConfig(StaticConfig{Root: root}))
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			router.ServeHTTP(recorder, req)

			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.status)
			}
			if tt.body != "" && recorder.Body.String() != tt.body {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), tt.body)
			}
			if got := recorder.Header().Get("Cache-Control"); got != tt.cacheControl {
				t.Fatalf("Cache-Control = %q, want %q", got, tt.cacheControl)
			}
		})
	}
}
