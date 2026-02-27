package evoclawplugin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	evoclawplugin "github.com/clawinfra/agent-tools/evoclaw-plugin"
	"github.com/clawinfra/agent-tools/sdk/go/agenttools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toolResp(name string) map[string]any {
	return map[string]any{
		"id":          "tid-1",
		"name":        name,
		"version":     "1.0.0",
		"description": "test",
		"provider_id": "prov-1",
		"endpoint":    "http://example.com",
		"timeout_ms":  30000,
		"tags":        []string{},
		"created_at":  time.Now().Format(time.RFC3339),
		"pricing":     map[string]any{"model": "free"},
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func TestNew_Defaults(t *testing.T) {
	p, err := evoclawplugin.New(evoclawplugin.Config{})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestNew_WithConfig(t *testing.T) {
	p, err := evoclawplugin.New(evoclawplugin.Config{
		RegistryURL:  "http://localhost:8433",
		CLAWWallet:   "mytoken",
		AutoRegister: true,
		GRPCPort:     50051,
	})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestStart_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	p, err := evoclawplugin.New(evoclawplugin.Config{RegistryURL: srv.URL})
	require.NoError(t, err)
	assert.NoError(t, p.Start(context.Background()))
}

func TestStart_Unreachable(t *testing.T) {
	p, err := evoclawplugin.New(evoclawplugin.Config{RegistryURL: "http://127.0.0.1:1"})
	require.NoError(t, err)
	err = p.Start(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unreachable")
}

func TestSearchTools_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"tools": []map[string]any{toolResp("weather-tool")},
			"total": 1,
		})
	}))
	defer srv.Close()

	p, err := evoclawplugin.New(evoclawplugin.Config{RegistryURL: srv.URL})
	require.NoError(t, err)
	tools, err := p.SearchTools(context.Background(), "weather")
	require.NoError(t, err)
	assert.Len(t, tools, 1)
}

func TestSearchTools_WithOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"tools": []map[string]any{}, "total": 0})
	}))
	defer srv.Close()

	p, err := evoclawplugin.New(evoclawplugin.Config{RegistryURL: srv.URL})
	require.NoError(t, err)
	tools, err := p.SearchTools(context.Background(), "test",
		agenttools.WithLimit(5),
		agenttools.WithTag("ai"),
	)
	require.NoError(t, err)
	assert.Empty(t, tools)
}

func TestSearchTools_Error(t *testing.T) {
	p, err := evoclawplugin.New(evoclawplugin.Config{RegistryURL: "http://127.0.0.1:1"})
	require.NoError(t, err)
	_, err = p.SearchTools(context.Background(), "test")
	assert.Error(t, err)
}

func TestRegisterSkill_Free(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 201, toolResp("my-skill"))
	}))
	defer srv.Close()

	p, err := evoclawplugin.New(evoclawplugin.Config{RegistryURL: srv.URL})
	require.NoError(t, err)
	tool, err := p.RegisterSkill(context.Background(), evoclawplugin.SkillSpec{
		Name:     "my-skill",
		Version:  "1.0.0",
		Endpoint: "http://example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "my-skill", tool.Name)
}

func TestRegisterSkill_WithPricing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		pricing := body["pricing"].(map[string]any)
		assert.Equal(t, "per_call", pricing["model"])
		assert.Equal(t, "1.5", pricing["amount_claw"])
		writeJSON(w, 201, toolResp("priced-skill"))
	}))
	defer srv.Close()

	p, err := evoclawplugin.New(evoclawplugin.Config{RegistryURL: srv.URL})
	require.NoError(t, err)
	tool, err := p.RegisterSkill(context.Background(), evoclawplugin.SkillSpec{
		Name:        "priced-skill",
		Version:     "1.0.0",
		Endpoint:    "http://example.com",
		PricingCLAW: 1.5,
	})
	require.NoError(t, err)
	assert.NotNil(t, tool)
}

func TestRegisterSkill_WithSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 201, toolResp("schema-skill"))
	}))
	defer srv.Close()

	p, err := evoclawplugin.New(evoclawplugin.Config{RegistryURL: srv.URL})
	require.NoError(t, err)
	tool, err := p.RegisterSkill(context.Background(), evoclawplugin.SkillSpec{
		Name:     "schema-skill",
		Version:  "1.0.0",
		Endpoint: "http://example.com",
		Schema:   map[string]any{"type": "object"},
	})
	require.NoError(t, err)
	assert.NotNil(t, tool)
}

func TestRegisterSkill_Error(t *testing.T) {
	p, err := evoclawplugin.New(evoclawplugin.Config{RegistryURL: "http://127.0.0.1:1"})
	require.NoError(t, err)
	_, err = p.RegisterSkill(context.Background(), evoclawplugin.SkillSpec{
		Name:    "x",
		Version: "1.0.0",
	})
	assert.Error(t, err)
}

func TestJSONMarshal(t *testing.T) {
	type MyStruct struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar"`
	}
	m := evoclawplugin.JSONMarshal(MyStruct{Foo: "hello", Bar: 42})
	assert.Equal(t, "hello", m["foo"])
	assert.Equal(t, float64(42), m["bar"])
}
