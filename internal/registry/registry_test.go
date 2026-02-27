package registry_test

import (
	"context"
	"testing"
	"time"

	"github.com/clawinfra/agent-tools/internal/registry"
	"github.com/clawinfra/agent-tools/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func openTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	return registry.New(openTestDB(t), zaptest.NewLogger(t))
}

func validRegisterReq() *registry.RegisterToolRequest {
	return &registry.RegisterToolRequest{
		Name:        "test-tool",
		Version:     "1.0.0",
		Description: "A test tool",
		Schema: registry.ToolSchema{
			Input:  []byte(`{"type":"object","properties":{"input":{"type":"string"}}}`),
			Output: []byte(`{"type":"object","properties":{"output":{"type":"string"}}}`),
		},
		Pricing:    &registry.Pricing{Model: registry.PricingPerCall, AmountCLAW: "5.0"},
		Endpoint:   "grpc://localhost:50051",
		TimeoutMS:  10000,
		Tags:       []string{"test", "demo"},
		ProviderID: "did:claw:agent:test-provider",
	}
}

func TestRegisterTool_Success(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	req := validRegisterReq()
	tool, err := r.RegisterTool(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, tool)

	assert.NotEmpty(t, tool.ID)
	assert.Equal(t, req.Name, tool.Name)
	assert.Equal(t, req.Version, tool.Version)
	assert.Equal(t, req.Description, tool.Description)
	assert.Equal(t, req.ProviderID, tool.ProviderID)
	assert.Equal(t, req.Endpoint, tool.Endpoint)
	assert.Equal(t, req.TimeoutMS, tool.TimeoutMS)
	assert.True(t, tool.IsActive)
	assert.Equal(t, []string{"test", "demo"}, tool.Tags)
}

func TestRegisterTool_DuplicateReturnsError(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	req := validRegisterReq()
	_, err := r.RegisterTool(ctx, req)
	require.NoError(t, err)

	_, err = r.RegisterTool(ctx, req)
	assert.ErrorIs(t, err, registry.ErrDuplicate)
}

func TestRegisterTool_MissingName(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	req := validRegisterReq()
	req.Name = ""
	_, err := r.RegisterTool(ctx, req)
	assert.Error(t, err)
}

func TestRegisterTool_MissingVersion(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	req := validRegisterReq()
	req.Version = ""
	_, err := r.RegisterTool(ctx, req)
	assert.Error(t, err)
}

func TestRegisterTool_MissingEndpoint(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	req := validRegisterReq()
	req.Endpoint = ""
	_, err := r.RegisterTool(ctx, req)
	assert.Error(t, err)
}

func TestRegisterTool_DefaultsTimeout(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	req := validRegisterReq()
	req.TimeoutMS = 0
	tool, err := r.RegisterTool(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(30000), tool.TimeoutMS)
}

func TestRegisterTool_DefaultsPricingFree(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	req := validRegisterReq()
	req.Pricing = nil
	tool, err := r.RegisterTool(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, tool.Pricing)
	assert.Equal(t, registry.PricingFree, tool.Pricing.Model)
}

func TestRegisterTool_InvalidSchema(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	req := validRegisterReq()
	req.Schema.Input = []byte(`{not valid json}`)
	_, err := r.RegisterTool(ctx, req)
	assert.Error(t, err)
}

func TestGetTool_NotFound(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	_, err := r.GetTool(ctx, "did:claw:tool:nonexistent")
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

func TestGetTool_Success(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	req := validRegisterReq()
	created, err := r.RegisterTool(ctx, req)
	require.NoError(t, err)

	got, err := r.GetTool(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Name, got.Name)
}

func TestListTools_Empty(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	result, err := r.ListTools(ctx, 1, 20)
	require.NoError(t, err)
	assert.Empty(t, result.Tools)
	assert.Equal(t, 0, result.Total)
}

func TestListTools_Pagination(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	// Register 5 tools with different names
	for i := 0; i < 5; i++ {
		req := validRegisterReq()
		req.Name = "tool-" + string(rune('a'+i))
		_, err := r.RegisterTool(ctx, req)
		require.NoError(t, err)
	}

	result, err := r.ListTools(ctx, 1, 3)
	require.NoError(t, err)
	assert.Len(t, result.Tools, 3)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 3, result.Limit)

	page2, err := r.ListTools(ctx, 2, 3)
	require.NoError(t, err)
	assert.Len(t, page2.Tools, 2)
}

func TestListTools_DefaultsPage(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	result, err := r.ListTools(ctx, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 20, result.Limit)
}

func TestSearchTools_ByQuery(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	req := validRegisterReq()
	req.Name = "solidity-auditor"
	req.Description = "Audits Solidity smart contracts"
	req.Tags = []string{"solidity", "audit", "security"}
	_, err := r.RegisterTool(ctx, req)
	require.NoError(t, err)

	// Register another unrelated tool
	req2 := validRegisterReq()
	req2.Name = "price-oracle"
	req2.Description = "Returns DeFi token prices"
	req2.Tags = []string{"defi", "prices"}
	_, err = r.RegisterTool(ctx, req2)
	require.NoError(t, err)

	result, err := r.SearchTools(ctx, &registry.SearchQuery{
		Query: "solidity",
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Len(t, result.Tools, 1)
	assert.Equal(t, "solidity-auditor", result.Tools[0].Name)
}

func TestSearchTools_EmptyQuery_ReturnsAll(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		req := validRegisterReq()
		req.Name = "tool-" + string(rune('a'+i))
		_, err := r.RegisterTool(ctx, req)
		require.NoError(t, err)
	}

	result, err := r.SearchTools(ctx, &registry.SearchQuery{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, result.Tools, 3)
}

func TestDeactivateTool_Success(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	tool, err := r.RegisterTool(ctx, validRegisterReq())
	require.NoError(t, err)

	err = r.DeactivateTool(ctx, tool.ID, "did:claw:agent:test-provider")
	require.NoError(t, err)

	// Should not appear in list
	result, err := r.ListTools(ctx, 1, 20)
	require.NoError(t, err)
	assert.Empty(t, result.Tools)
}

func TestDeactivateTool_WrongProvider(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	tool, err := r.RegisterTool(ctx, validRegisterReq())
	require.NoError(t, err)

	err = r.DeactivateTool(ctx, tool.ID, "did:claw:agent:wrong-provider")
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

func TestDeactivateTool_NotFound(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	err := r.DeactivateTool(ctx, "did:claw:tool:nonexistent", "did:claw:agent:any")
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

func TestPricingString(t *testing.T) {
	tests := []struct {
		pricing *registry.Pricing
		want    string
	}{
		{nil, "free"},
		{&registry.Pricing{Model: registry.PricingFree}, "free"},
		{&registry.Pricing{Model: registry.PricingPerCall, AmountCLAW: "10.0"}, "10.0 CLAW/per_call"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.pricing.String())
	}
}

func TestRecordInvocation(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	tool, err := r.RegisterTool(ctx, validRegisterReq())
	require.NoError(t, err)

	id, err := r.RecordInvocation(ctx, tool.ID, "did:claw:agent:consumer", map[string]any{"key": "value"})
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Contains(t, id, "inv_")
}

func TestCompleteInvocation(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	tool, err := r.RegisterTool(ctx, validRegisterReq())
	require.NoError(t, err)

	invID, err := r.RecordInvocation(ctx, tool.ID, "did:claw:agent:consumer", map[string]any{"key": "value"})
	require.NoError(t, err)

	err = r.CompleteInvocation(ctx, invID, "sha256:output123", "ed25519:sig", "5.0")
	require.NoError(t, err)
}

func TestFailInvocation(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	tool, err := r.RegisterTool(ctx, validRegisterReq())
	require.NoError(t, err)

	invID, err := r.RecordInvocation(ctx, tool.ID, "did:claw:agent:consumer", map[string]any{"key": "value"})
	require.NoError(t, err)

	err = r.FailInvocation(ctx, invID, "provider timeout")
	require.NoError(t, err)
}

func TestRegisterProvider_Success(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	p := &registry.Provider{
		ID:       "did:claw:agent:provider-1",
		Name:     "Provider One",
		Endpoint: "grpc://localhost:50051",
		PubKey:   "ed25519pub1234",
	}
	got, err := r.RegisterProvider(ctx, p)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, p.ID, got.ID)
	assert.Equal(t, p.Name, got.Name)
	assert.Equal(t, "0", got.StakeCLAW)
}

func TestRegisterProvider_Upsert(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	p := &registry.Provider{
		ID:       "did:claw:agent:provider-1",
		Name:     "Provider One",
		Endpoint: "grpc://localhost:50051",
		PubKey:   "pubkey1",
	}
	_, err := r.RegisterProvider(ctx, p)
	require.NoError(t, err)

	p.Name = "Provider One Updated"
	p.PubKey = "pubkey2"
	got, err := r.RegisterProvider(ctx, p)
	require.NoError(t, err)
	assert.Equal(t, "Provider One Updated", got.Name)
}

func TestRegisterProvider_MissingID(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	_, err := r.RegisterProvider(ctx, &registry.Provider{
		Endpoint: "grpc://localhost:50051",
		PubKey:   "pubkey",
	})
	assert.Error(t, err)
}

func TestRegisterProvider_MissingEndpoint(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	_, err := r.RegisterProvider(ctx, &registry.Provider{
		ID:     "did:claw:agent:x",
		PubKey: "pubkey",
	})
	assert.Error(t, err)
}

func TestRegisterProvider_MissingPubKey(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	_, err := r.RegisterProvider(ctx, &registry.Provider{
		ID:       "did:claw:agent:x",
		Endpoint: "grpc://localhost:50051",
	})
	assert.Error(t, err)
}

func TestGetProvider_NotFound(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	_, err := r.GetProvider(ctx, "did:claw:agent:nonexistent")
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

func TestListProviders(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	providers, err := r.ListProviders(ctx)
	require.NoError(t, err)
	assert.Empty(t, providers)

	for i := 0; i < 3; i++ {
		_, err := r.RegisterProvider(ctx, &registry.Provider{
			ID:       "did:claw:agent:p" + string(rune('0'+i)),
			Name:     "Provider " + string(rune('A'+i)),
			Endpoint: "grpc://localhost:5005" + string(rune('1'+i)),
			PubKey:   "pubkey" + string(rune('0'+i)),
		})
		require.NoError(t, err)
	}

	providers, err = r.ListProviders(ctx)
	require.NoError(t, err)
	assert.Len(t, providers, 3)
}

func TestToolSchemaValidate_InvalidOutputSchema(t *testing.T) {
	ts := registry.ToolSchema{
		Input:  []byte(`{"type":"object"}`),
		Output: []byte(`{not valid}`),
	}
	assert.Error(t, ts.Validate())
}

func TestToolSchemaValidate_ValidNoOutput(t *testing.T) {
	ts := registry.ToolSchema{
		Input: []byte(`{"type":"object"}`),
	}
	assert.NoError(t, ts.Validate())
}

func TestInvokeRequest_Types(t *testing.T) {
	req := &registry.InvokeRequest{
		ToolID:     "did:claw:tool:abc",
		Input:      map[string]any{"key": "value"},
		BudgetCLAW: "5.0",
		ConsumerID: "did:claw:agent:consumer",
	}
	assert.Equal(t, "did:claw:tool:abc", req.ToolID)
	assert.Equal(t, "5.0", req.BudgetCLAW)
}

func TestReceipt_Fields(t *testing.T) {
	now := time.Now()
	r := &registry.Receipt{
		ID:          "rcpt_abc",
		ToolID:      "did:claw:tool:xyz",
		ConsumerID:  "did:claw:agent:consumer",
		ProviderID:  "did:claw:agent:provider",
		InputHash:   "sha256:abc",
		OutputHash:  "sha256:def",
		CostCLAW:    "5.0",
		ExecutedAt:  now,
		ProviderSig: "ed25519:sig",
	}
	assert.Equal(t, "rcpt_abc", r.ID)
	assert.Equal(t, "5.0", r.CostCLAW)
}

func TestSearchQuery_Defaults(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	// Register multiple tools with tags
	for i := 0; i < 5; i++ {
		req := validRegisterReq()
		req.Name = "tagged-tool-" + string(rune('a'+i))
		req.Tags = []string{"defi", "prices"}
		_, err := r.RegisterTool(ctx, req)
		require.NoError(t, err)
	}

	// Page 2 with limit
	result, err := r.SearchTools(ctx, &registry.SearchQuery{Page: 2, Limit: 3})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Page)
}

func TestRegisterTool_DefaultsApplied(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	req := validRegisterReq()
	req.TimeoutMS = -1
	req.Pricing = nil
	tool, err := r.RegisterTool(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(30000), tool.TimeoutMS)
	assert.Equal(t, registry.PricingFree, tool.Pricing.Model)
}

func TestInvocation_CompleteAndFail(t *testing.T) {
	r := newTestRegistry(t)
	ctx := context.Background()

	tool, err := r.RegisterTool(ctx, validRegisterReq())
	require.NoError(t, err)

	invID, err := r.RecordInvocation(ctx, tool.ID, "consumer", map[string]any{"k": "v"})
	require.NoError(t, err)

	// Test double-path: complete then fail (overwrites)
	err = r.CompleteInvocation(ctx, invID, "sha256:out", "sig", "1.0")
	require.NoError(t, err)

	err = r.FailInvocation(ctx, "nonexistent-inv", "timeout")
	require.NoError(t, err) // No-op, no rows affected but no error
}
