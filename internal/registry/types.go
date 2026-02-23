// Package registry implements the agent-tools tool registry core.
package registry

import (
	"encoding/json"
	"fmt"
	"time"
)

// Tool represents a registered tool in the registry.
type Tool struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Version     string     `json:"version"`
	Description string     `json:"description"`
	Schema      ToolSchema `json:"schema"`
	Pricing     *Pricing   `json:"pricing"`
	ProviderID  string     `json:"provider_id"`
	Endpoint    string     `json:"endpoint"`
	TimeoutMS   int64      `json:"timeout_ms"`
	Tags        []string   `json:"tags"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	IsActive    bool       `json:"is_active"`
}

// ToolSchema defines the input and output JSON schemas for a tool.
type ToolSchema struct {
	Input  json.RawMessage `json:"input"`
	Output json.RawMessage `json:"output"`
}

// Validate checks that the schema is valid JSON.
func (s ToolSchema) Validate() error {
	var v any
	if err := json.Unmarshal(s.Input, &v); err != nil {
		return fmt.Errorf("invalid input schema: %w", err)
	}
	if len(s.Output) > 0 {
		if err := json.Unmarshal(s.Output, &v); err != nil {
			return fmt.Errorf("invalid output schema: %w", err)
		}
	}
	return nil
}

// PricingModel enumerates how a tool charges for invocations.
type PricingModel string

const (
	PricingFree         PricingModel = "free"
	PricingPerCall      PricingModel = "per_call"
	PricingPerToken     PricingModel = "per_token"
	PricingSubscription PricingModel = "subscription"
)

// Pricing describes the cost structure for invoking a tool.
type Pricing struct {
	Model      PricingModel `json:"model"`
	AmountCLAW string       `json:"amount_claw,omitempty"` // decimal string
}

// String returns a human-readable pricing description.
func (p *Pricing) String() string {
	if p == nil || p.Model == PricingFree {
		return "free"
	}
	return fmt.Sprintf("%s CLAW/%s", p.AmountCLAW, p.Model)
}

// Provider represents an agent that provides tools.
type Provider struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Endpoint   string    `json:"endpoint"`
	PubKey     string    `json:"pubkey"`
	StakeCLAW  string    `json:"stake_claw"`
	Reputation int64     `json:"reputation"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeen   time.Time `json:"last_seen"`
}

// RegisterToolRequest is the input for tool registration.
type RegisterToolRequest struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Schema      ToolSchema      `json:"schema"`
	Pricing     *Pricing        `json:"pricing"`
	Endpoint    string          `json:"endpoint"`
	TimeoutMS   int64           `json:"timeout_ms"`
	Tags        []string        `json:"tags"`
	ProviderID  string          `json:"-"` // set from auth context
	RawSchema   json.RawMessage `json:"-"` // original schema JSON
}

// Validate checks that a registration request is valid.
func (r *RegisterToolRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.Version == "" {
		return fmt.Errorf("version is required")
	}
	if r.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if r.TimeoutMS <= 0 {
		r.TimeoutMS = 30000
	}
	if r.Pricing == nil {
		r.Pricing = &Pricing{Model: PricingFree}
	}
	return r.Schema.Validate()
}

// SearchQuery defines parameters for tool discovery.
type SearchQuery struct {
	Query    string  `json:"q"`
	Tag      string  `json:"tag"`
	Provider string  `json:"provider"`
	MaxPrice float64 `json:"max_price_claw"`
	Page     int     `json:"page"`
	Limit    int     `json:"limit"`
}

// SearchResult is the response from a tool search.
type SearchResult struct {
	Tools   []*Tool `json:"tools"`
	Total   int     `json:"total"`
	Page    int     `json:"page"`
	Limit   int     `json:"limit"`
	Query   string  `json:"query,omitempty"`
}

// Invocation tracks a single tool invocation lifecycle.
type Invocation struct {
	ID          string    `json:"id"`
	ToolID      string    `json:"tool_id"`
	ConsumerID  string    `json:"consumer_id"`
	InputHash   string    `json:"input_hash"`
	OutputHash  string    `json:"output_hash,omitempty"`
	ReceiptSig  string    `json:"receipt_sig,omitempty"`
	Status      string    `json:"status"`
	CostCLAW    string    `json:"cost_claw,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// InvokeRequest is the input for invoking a tool.
type InvokeRequest struct {
	ToolID         string         `json:"tool_id"`
	Input          map[string]any `json:"input"`
	BudgetCLAW     string         `json:"budget_claw,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	ConsumerID     string         `json:"-"` // set from auth context
}

// InvokeResponse is returned from a tool invocation.
type InvokeResponse struct {
	InvocationID string         `json:"invocation_id"`
	ToolID       string         `json:"tool_id"`
	Output       map[string]any `json:"output"`
	Receipt      *Receipt       `json:"receipt,omitempty"`
	CostCLAW     string         `json:"cost_claw,omitempty"`
	DurationMS   int64          `json:"duration_ms"`
}

// Receipt is a cryptographically signed proof of tool execution.
type Receipt struct {
	ID          string    `json:"id"`
	ToolID      string    `json:"tool_id"`
	ConsumerID  string    `json:"consumer_id"`
	ProviderID  string    `json:"provider_id"`
	InputHash   string    `json:"input_hash"`
	OutputHash  string    `json:"output_hash"`
	CostCLAW    string    `json:"cost_claw,omitempty"`
	ExecutedAt  time.Time `json:"executed_at"`
	ProviderSig string    `json:"provider_sig"`
}
