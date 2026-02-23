package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clawinfra/agent-tools/internal/api"
	"github.com/clawinfra/agent-tools/internal/registry"
	"github.com/clawinfra/agent-tools/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	reg := registry.New(db, zaptest.NewLogger(t))
	return api.NewHandler(reg, zaptest.NewLogger(t))
}

func doRequest(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestHealthz(t *testing.T) {
	h := newTestHandler(t)
	rr := doRequest(t, h, http.MethodGet, "/healthz", nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "0.1.0", resp["version"])
}

func TestListTools_Empty(t *testing.T) {
	h := newTestHandler(t)
	rr := doRequest(t, h, http.MethodGet, "/v1/tools", nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	tools, ok := resp["tools"]
	assert.True(t, ok)
	assert.Nil(t, tools) // empty list
}

func validToolPayload() map[string]any {
	return map[string]any{
		"name":        "test-tool",
		"version":     "1.0.0",
		"description": "A test tool for HTTP tests",
		"schema": map[string]any{
			"input":  map[string]any{"type": "object"},
			"output": map[string]any{"type": "object"},
		},
		"pricing": map[string]any{
			"model":       "per_call",
			"amount_claw": "5.0",
		},
		"endpoint":   "grpc://localhost:50051",
		"timeout_ms": 10000,
		"tags":       []string{"test"},
	}
}

func TestRegisterTool_Success(t *testing.T) {
	h := newTestHandler(t)
	rr := doRequest(t, h, http.MethodPost, "/v1/tools", validToolPayload())
	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, "test-tool", resp["name"])
}

func TestRegisterTool_InvalidJSON(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/tools", bytes.NewBufferString("{not json}"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRegisterTool_Duplicate(t *testing.T) {
	h := newTestHandler(t)
	payload := validToolPayload()

	rr1 := doRequest(t, h, http.MethodPost, "/v1/tools", payload)
	assert.Equal(t, http.StatusCreated, rr1.Code)

	rr2 := doRequest(t, h, http.MethodPost, "/v1/tools", payload)
	assert.Equal(t, http.StatusConflict, rr2.Code)
}

func TestGetTool_NotFound(t *testing.T) {
	h := newTestHandler(t)
	rr := doRequest(t, h, http.MethodGet, "/v1/tools/did:claw:tool:nonexistent", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetTool_Success(t *testing.T) {
	h := newTestHandler(t)

	// Register first
	rr := doRequest(t, h, http.MethodPost, "/v1/tools", validToolPayload())
	require.Equal(t, http.StatusCreated, rr.Code)

	var created map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))
	id := created["id"].(string)

	// Get it
	rr2 := doRequest(t, h, http.MethodGet, "/v1/tools/"+id, nil)
	assert.Equal(t, http.StatusOK, rr2.Code)

	var got map[string]any
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&got))
	assert.Equal(t, id, got["id"])
}

func TestSearchTools(t *testing.T) {
	h := newTestHandler(t)

	// Register a tool with a unique description
	payload := validToolPayload()
	payload["name"] = "solidity-audit"
	payload["description"] = "Analyzes Solidity contracts for security issues"
	rr := doRequest(t, h, http.MethodPost, "/v1/tools", payload)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Search for it
	rr2 := doRequest(t, h, http.MethodGet, "/v1/tools/search?q=solidity", nil)
	assert.Equal(t, http.StatusOK, rr2.Code)
}

func TestDeleteTool_NotFound(t *testing.T) {
	h := newTestHandler(t)
	rr := doRequest(t, h, http.MethodDelete, "/v1/tools/did:claw:tool:nonexistent", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestInvokeTool_NotImplemented(t *testing.T) {
	h := newTestHandler(t)
	rr := doRequest(t, h, http.MethodPost, "/v1/invoke", map[string]any{
		"tool_id": "did:claw:tool:abc",
		"input":   map[string]any{},
	})
	assert.Equal(t, http.StatusNotImplemented, rr.Code)
}
