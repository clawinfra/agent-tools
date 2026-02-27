package registry_test

import (
	"context"
	"testing"

	"github.com/clawinfra/agent-tools/internal/registry"
	"github.com/clawinfra/agent-tools/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newBrokenRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Close()) // close immediately to break all ops
	return registry.New(db, zaptest.NewLogger(t))
}

func TestRegisterTool_BrokenDB(t *testing.T) {
	r := newBrokenRegistry(t)
	_, err := r.RegisterTool(context.Background(), validRegisterReq())
	assert.Error(t, err)
}

func TestListTools_BrokenDB(t *testing.T) {
	r := newBrokenRegistry(t)
	_, err := r.ListTools(context.Background(), 1, 20)
	assert.Error(t, err)
}

func TestSearchTools_BrokenDB_Query(t *testing.T) {
	r := newBrokenRegistry(t)
	_, err := r.SearchTools(context.Background(), &registry.SearchQuery{Query: "test"})
	assert.Error(t, err)
}

func TestSearchTools_BrokenDB_Empty(t *testing.T) {
	r := newBrokenRegistry(t)
	_, err := r.SearchTools(context.Background(), &registry.SearchQuery{})
	assert.Error(t, err)
}

func TestGetTool_BrokenDB(t *testing.T) {
	r := newBrokenRegistry(t)
	_, err := r.GetTool(context.Background(), "some-id")
	assert.Error(t, err)
}

func TestRecordInvocation_BrokenDB(t *testing.T) {
	r := newBrokenRegistry(t)
	_, err := r.RecordInvocation(context.Background(), "tool-id", "consumer-id", map[string]any{"k": "v"})
	assert.Error(t, err)
}

func TestRegisterProvider_BrokenDB(t *testing.T) {
	r := newBrokenRegistry(t)
	_, err := r.RegisterProvider(context.Background(), &registry.Provider{
		ID:       "prov-1",
		Endpoint: "grpc://localhost:50051",
		PubKey:   "key",
	})
	assert.Error(t, err)
}

func TestListProviders_BrokenDB(t *testing.T) {
	r := newBrokenRegistry(t)
	_, err := r.ListProviders(context.Background())
	assert.Error(t, err)
}

func TestGetProvider_BrokenDB(t *testing.T) {
	r := newBrokenRegistry(t)
	_, err := r.GetProvider(context.Background(), "prov-1")
	assert.Error(t, err)
}

func TestDeactivateTool_BrokenDB(t *testing.T) {
	r := newBrokenRegistry(t)
	err := r.DeactivateTool(context.Background(), "tool-id", "prov-1")
	assert.Error(t, err)
}

func TestCompleteInvocation_BrokenDB(t *testing.T) {
	r := newBrokenRegistry(t)
	err := r.CompleteInvocation(context.Background(), "inv-1", "hash", "sig", "1.0")
	assert.Error(t, err)
}

func TestFailInvocation_BrokenDB(t *testing.T) {
	r := newBrokenRegistry(t)
	err := r.FailInvocation(context.Background(), "inv-1", "timeout")
	assert.Error(t, err)
}
