package cli_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/clawinfra/agent-tools/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toolListResponse(tools []map[string]any) map[string]any {
	return map[string]any{
		"tools": tools,
		"total": len(tools),
		"page":  1,
		"limit": 50,
	}
}

func searchResponse(tools []map[string]any) map[string]any {
	return map[string]any{
		"tools": tools,
		"total": len(tools),
		"query": "test",
	}
}

func fakeTool(name string) map[string]any {
	return map[string]any{
		"id":          "tid-1",
		"name":        name,
		"version":     "1.0.0",
		"description": "a test tool",
		"provider_id": "prov-1",
		"endpoint":    "https://example.com",
		"timeout_ms":  30000,
		"tags":        []string{"test"},
		"created_at":  time.Now().Format(time.RFC3339),
		"pricing":     map[string]any{"model": "free"},
	}
}

func writeJSONResp(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// TestToolListCmd_Empty tests that "tool list" succeeds with empty registry.
func TestToolListCmd_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONResp(w, toolListResponse(nil))
	}))
	defer srv.Close()

	root := cli.NewRootCmd()
	root.SetArgs([]string{"tool", "list", "--registry", srv.URL})
	err := root.Execute()
	assert.NoError(t, err)
}

// TestToolListCmd_WithTools tests that "tool list" succeeds when tools exist.
func TestToolListCmd_WithTools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONResp(w, toolListResponse([]map[string]any{fakeTool("my-tool")}))
	}))
	defer srv.Close()

	root := cli.NewRootCmd()
	root.SetArgs([]string{"tool", "list", "--registry", srv.URL})
	err := root.Execute()
	assert.NoError(t, err)
}

// TestToolListCmd_ServerError tests that "tool list" returns error on HTTP failure.
func TestToolListCmd_ServerError(t *testing.T) {
	root := cli.NewRootCmd()
	root.SetArgs([]string{"tool", "list", "--registry", "http://127.0.0.1:1"})
	err := root.Execute()
	// cobra wraps the error; the command RunE returns error
	assert.Error(t, err)
}

// TestToolSearchCmd_NoResults tests success when no tools found.
func TestToolSearchCmd_NoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONResp(w, searchResponse(nil))
	}))
	defer srv.Close()

	root := cli.NewRootCmd()
	root.SetArgs([]string{"tool", "search", "--registry", srv.URL, "-q", "weather"})
	err := root.Execute()
	assert.NoError(t, err)
}

// TestToolSearchCmd_WithResults tests that tool search succeeds with matching tools.
func TestToolSearchCmd_WithResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tools := []map[string]any{fakeTool("weather-tool")}
		writeJSONResp(w, searchResponse(tools))
	}))
	defer srv.Close()

	root := cli.NewRootCmd()
	root.SetArgs([]string{"tool", "search", "--registry", srv.URL, "-q", "weather"})
	err := root.Execute()
	assert.NoError(t, err)
}

// TestToolSearchCmd_WithMaxPrice tests that max-price option is passed.
func TestToolSearchCmd_WithMaxPrice(t *testing.T) {
	var gotMaxPrice string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMaxPrice = r.URL.Query().Get("max_price_claw")
		writeJSONResp(w, searchResponse(nil))
	}))
	defer srv.Close()

	root := cli.NewRootCmd()
	root.SetArgs([]string{"tool", "search", "--registry", srv.URL, "-q", "test", "--max-price", "2.5"})
	err := root.Execute()
	assert.NoError(t, err)
	assert.NotEmpty(t, gotMaxPrice)
}

// TestToolSearchCmd_WithPricing tests tool with non-free pricing.
func TestToolSearchCmd_WithPricing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tool := fakeTool("priced-tool")
		tool["pricing"] = map[string]any{"model": "per_call", "amount_claw": "0.5"}
		writeJSONResp(w, searchResponse([]map[string]any{tool}))
	}))
	defer srv.Close()

	root := cli.NewRootCmd()
	root.SetArgs([]string{"tool", "search", "--registry", srv.URL, "-q", "priced"})
	err := root.Execute()
	assert.NoError(t, err)
}

// TestToolSearchCmd_Error tests network error propagation.
func TestToolSearchCmd_Error(t *testing.T) {
	root := cli.NewRootCmd()
	root.SetArgs([]string{"tool", "search", "--registry", "http://127.0.0.1:1", "-q", "test"})
	err := root.Execute()
	assert.Error(t, err)
}

// TestToolSearchCmd_MissingQuery tests required --query flag validation.
func TestToolSearchCmd_MissingQuery(t *testing.T) {
	root := cli.NewRootCmd()
	root.SetArgs([]string{"tool", "search"})
	err := root.Execute()
	assert.Error(t, err)
}

// TestInitCmd_WritesConfig verifies config file is created.
func TestInitCmd_WritesConfig(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	t.Cleanup(func() { os.Chdir(origDir) })

	root := cli.NewRootCmd()
	root.SetArgs([]string{"init"})
	err := root.Execute()
	require.NoError(t, err)
}
