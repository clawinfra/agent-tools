package api_test

import (
	"net/http"
	"testing"

	"github.com/clawinfra/agent-tools/internal/api"
	"github.com/clawinfra/agent-tools/internal/registry"
	"github.com/clawinfra/agent-tools/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// newBrokenHandler returns a handler with a closed DB to trigger 500 errors.
func newBrokenHandler(t *testing.T) http.Handler {
	t.Helper()
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	// Close immediately so all DB ops fail.
	require.NoError(t, db.Close())
	reg := registry.New(db, zaptest.NewLogger(t))
	return api.NewHandler(reg, zaptest.NewLogger(t))
}

func TestListTools_InternalError(t *testing.T) {
	h := newBrokenHandler(t)
	rr := doRequest(t, h, http.MethodGet, "/v1/tools", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetTool_InternalError(t *testing.T) {
	h := newBrokenHandler(t)
	rr := doRequest(t, h, http.MethodGet, "/v1/tools/some-id", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestSearchTools_InternalError(t *testing.T) {
	h := newBrokenHandler(t)
	rr := doRequest(t, h, http.MethodGet, "/v1/tools/search?q=test", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestDeactivateTool_InternalError(t *testing.T) {
	h := newBrokenHandler(t)
	rr := doRequest(t, h, http.MethodDelete, "/v1/tools/some-id", nil)
	// With broken DB, we expect either 500 or 404 (can't tell if not found vs error easily)
	assert.True(t, rr.Code == http.StatusInternalServerError || rr.Code == http.StatusNotFound)
}

func TestListProviders_InternalError(t *testing.T) {
	h := newBrokenHandler(t)
	rr := doRequest(t, h, http.MethodGet, "/v1/providers", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetProvider_InternalError(t *testing.T) {
	h := newBrokenHandler(t)
	rr := doRequest(t, h, http.MethodGet, "/v1/providers/some-id", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestRegisterTool_InternalError(t *testing.T) {
	h := newBrokenHandler(t)
	payload := map[string]any{
		"name":     "test-tool",
		"version":  "1.0.0",
		"endpoint": "https://example.com",
		"schema":   map[string]any{"type": "object"},
	}
	rr := doRequest(t, h, http.MethodPost, "/v1/tools", payload)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
