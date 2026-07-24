package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterDocsIndexRoutesServesBothCanonicalPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	docsIndexPage := []byte("<!doctype html><title>docs</title>")
	registerDocsIndexRoutes(router, docsIndexPage)

	for _, path := range []string{"/docs", "/docs/"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, string(docsIndexPage), response.Body.String())
			assert.Equal(t, "text/html; charset=utf-8", response.Header().Get("Content-Type"))
			assert.Equal(t, "no-cache", response.Header().Get("Cache-Control"))
			assert.Empty(t, response.Header().Get("Location"))
		})
	}
}
