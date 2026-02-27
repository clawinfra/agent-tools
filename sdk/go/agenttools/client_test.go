package agenttools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/clawinfra/agent-tools/sdk/go/agenttools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// toolJSON returns a minimal tool JSON object.
func toolJSON(id, name string) map[string]any {
	return map[string]any{
		"id":          id,
		"name":        name,
		"version":     "1.0.0",
		"description": "test tool",
		"provider_id": "prov-1",
		"endpoint":    "https://example.com/tool",
		"timeout_ms":  30000,
		"tags":        []string{"test"},
		"created_at":  time.Now().Format(time.RFC3339),
		"pricing":     map[string]any{"model": "free"},
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// --- NewClient & options ---

func TestNewClient_Defaults(t *testing.T) {
	c := agenttools.NewClient("http://localhost:8433")
	require.NotNil(t, c)
}

func TestNewClient_WithAuthToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeJSON(w, 200, map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	c := agenttools.NewClient(srv.URL, agenttools.WithAuthToken("mytoken"))
	err := c.Healthz(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Bearer mytoken", gotAuth)
}

func TestNewClient_WithHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	custom := &http.Client{Timeout: 5 * time.Second}
	c := agenttools.NewClient(srv.URL, agenttools.WithHTTPClient(custom))
	err := c.Healthz(context.Background())
	require.NoError(t, err)
}

// --- Healthz ---

func TestHealthz_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/healthz", r.URL.Path)
		writeJSON(w, 200, map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	c := agenttools.NewClient(srv.URL)
	assert.NoError(t, c.Healthz(context.Background()))
}

func TestHealthz_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 503, map[string]any{
			"error": map[string]string{"code": "unavailable", "message": "down"},
		})
	}))
	defer srv.Close()

	c := agenttools.NewClient(srv.URL)
	err := c.Healthz(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestHealthz_HTTPError_NoBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := agenttools.NewClient(srv.URL)
	err := c.Healthz(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// --- RegisterTool ---

func TestRegisterTool_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/tools", r.URL.Path)
		writeJSON(w, 201, toolJSON("tool-1", "my-tool"))
	}))
	defer srv.Close()

	c := agenttools.NewClient(srv.URL)
	tool, err := c.RegisterTool(context.Background(), &agenttools.RegisterToolRequest{
		Name:        "my-tool",
		Version:     "1.0.0",
		Description: "test",
		Schema:      map[string]any{"type": "object"},
		Endpoint:    "https://example.com",
		Tags:        []string{"test"},
	})
	require.NoError(t, err)
	assert.Equal(t, "tool-1", tool.ID)
	assert.Equal(t, "my-tool", tool.Name)
}

func TestRegisterTool_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 400, map[string]any{
			"error": map[string]string{"code": "bad_request", "message": "invalid tool"},
		})
	}))
	defer srv.Close()

	c := agenttools.NewClient(srv.URL)
	_, err := c.RegisterTool(context.Background(), &agenttools.RegisterToolRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad_request")
}

// --- GetTool ---

func TestGetTool_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/tools/tool-abc", r.URL.Path)
		writeJSON(w, 200, toolJSON("tool-abc", "abc"))
	}))
	defer srv.Close()

	c := agenttools.NewClient(srv.URL)
	tool, err := c.GetTool(context.Background(), "tool-abc")
	require.NoError(t, err)
	assert.Equal(t, "tool-abc", tool.ID)
}

func TestGetTool_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{
			"error": map[string]string{"code": "not_found", "message": "tool not found"},
		})
	}))
	defer srv.Close()

	c := agenttools.NewClient(srv.URL)
	_, err := c.GetTool(context.Background(), "missing")
	assert.Error(t, err)
}

// --- ListTools ---

func TestListTools_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/tools", r.URL.Path)
		writeJSON(w, 200, map[string]any{
			"tools": []map[string]any{toolJSON("t1", "tool1")},
			"total": 1, "page": 1, "limit": 20,
		})
	}))
	defer srv.Close()

	c := agenttools.NewClient(srv.URL)
	list, err := c.ListTools(context.Background(), nil)
	require.NoError(t, err)
	assert.Len(t, list.Tools, 1)
}

func TestListTools_WithPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "2", r.URL.Query().Get("page"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		writeJSON(w, 200, map[string]any{
			"tools": []map[string]any{},
			"total": 0, "page": 2, "limit": 10,
		})
	}))
	defer srv.Close()

	c := agenttools.NewClient(srv.URL)
	list, err := c.ListTools(context.Background(), &agenttools.ListToolsRequest{Page: 2, Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, list.Tools)
}

// --- SearchTools ---

func TestSearchTools_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/tools/search", r.URL.Path)
		assert.Equal(t, "weather", r.URL.Query().Get("q"))
		writeJSON(w, 200, map[string]any{
			"tools": []map[string]any{toolJSON("t1", "weather-tool")},
			"total": 1,
			"query": "weather",
		})
	}))
	defer srv.Close()

	c := agenttools.NewClient(srv.URL)
	result, err := c.SearchTools(context.Background(), "weather")
	require.NoError(t, err)
	assert.Len(t, result.Tools, 1)
	assert.Equal(t, "weather", result.Query)
}

func TestSearchTools_WithOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "5", q.Get("limit"))
		assert.NotEmpty(t, q.Get("max_price_claw"))
		assert.Equal(t, "ai", q.Get("tag"))
		writeJSON(w, 200, map[string]any{
			"tools": []map[string]any{},
			"total": 0,
		})
	}))
	defer srv.Close()

	c := agenttools.NewClient(srv.URL)
	result, err := c.SearchTools(context.Background(), "test",
		agenttools.WithLimit(5),
		agenttools.WithMaxPrice(1.5),
		agenttools.WithTag("ai"),
	)
	require.NoError(t, err)
	assert.Empty(t, result.Tools)
}

// --- Pricing.String ---

func TestPricing_String_Free(t *testing.T) {
	p := &agenttools.Pricing{Model: "free"}
	assert.Equal(t, "free", p.String())
}

func TestPricing_String_Nil(t *testing.T) {
	var p *agenttools.Pricing
	assert.Equal(t, "free", p.String())
}

func TestPricing_String_PerCall(t *testing.T) {
	p := &agenttools.Pricing{Model: "per_call", AmountCLAW: "0.5"}
	assert.Equal(t, "0.5 CLAW/per_call", p.String())
}

// --- CLAWAmount ---

func TestCLAWAmount(t *testing.T) {
	assert.Equal(t, "1.5", agenttools.CLAWAmount(1.5))
	assert.Equal(t, "0.0", agenttools.CLAWAmount(0))
	assert.Equal(t, "100.0", agenttools.CLAWAmount(100))
}

// --- network errors ---

func TestGetTool_NetworkError(t *testing.T) {
	c := agenttools.NewClient("http://127.0.0.1:1") // nothing listening
	_, err := c.GetTool(context.Background(), "x")
	assert.Error(t, err)
}

func TestRegisterTool_NetworkError(t *testing.T) {
	c := agenttools.NewClient("http://127.0.0.1:1")
	_, err := c.RegisterTool(context.Background(), &agenttools.RegisterToolRequest{})
	assert.Error(t, err)
}
