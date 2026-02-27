// Package evoclawplugin provides native EvoClaw integration for agent-tools.
//
// Install in evoclaw.toml:
//
//	[[plugins]]
//	name = "agent-tools"
//	source = "github.com/clawinfra/agent-tools/evoclaw-plugin"
//	version = "0.1.0"
//
//	[plugins.config]
//	registry_url  = "http://localhost:8433"
//	auto_register = true
package evoclawplugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/clawinfra/agent-tools/sdk/go/agenttools"
)

// Config holds the plugin configuration loaded from evoclaw.toml.
type Config struct {
	RegistryURL  string `toml:"registry_url"`
	CLAWWallet   string `toml:"claw_wallet"`
	AutoRegister bool   `toml:"auto_register"`
	Consumer     bool   `toml:"consumer"`
	GRPCPort     int    `toml:"grpc_port"`
}

// Plugin is the EvoClaw agent-tools plugin.
// It integrates with the EvoClaw plugin interface to:
//   - Auto-register skills as tools (if auto_register=true)
//   - Expose a tool invocation interface to the agent runtime
type Plugin struct {
	client *agenttools.Client
	cfg    Config
}

// New creates a new Plugin from config.
func New(cfg Config) (*Plugin, error) {
	if cfg.RegistryURL == "" {
		cfg.RegistryURL = "http://localhost:8433"
	}
	if cfg.GRPCPort == 0 {
		cfg.GRPCPort = 50051
	}

	opts := []agenttools.ClientOption{}
	if cfg.CLAWWallet != "" {
		opts = append(opts, agenttools.WithAuthToken(cfg.CLAWWallet))
	}

	return &Plugin{
		cfg:    cfg,
		client: agenttools.NewClient(cfg.RegistryURL, opts...),
	}, nil
}

// Start initializes the plugin.
func (p *Plugin) Start(ctx context.Context) error {
	if err := p.client.Healthz(ctx); err != nil {
		return fmt.Errorf("agent-tools registry unreachable at %s: %w", p.cfg.RegistryURL, err)
	}
	return nil
}

// SearchTools discovers tools matching a query.
func (p *Plugin) SearchTools(ctx context.Context, query string, opts ...agenttools.SearchOption) ([]*agenttools.Tool, error) {
	result, err := p.client.SearchTools(ctx, query, opts...)
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// SkillSpec describes an EvoClaw skill for tool registration.
// This mirrors the evoclaw skill interface — imported without circular deps.
type SkillSpec struct {
	Schema      map[string]any
	Name        string
	Version     string
	Description string
	Endpoint    string
	Tags        []string
	TimeoutMS   int64
	PricingCLAW float64
}

// RegisterSkill registers a skill as a tool in the registry.
func (p *Plugin) RegisterSkill(ctx context.Context, skill *SkillSpec) (*agenttools.Tool, error) {
	pricingModel := "free"
	pricingAmount := ""
	if skill.PricingCLAW > 0 {
		pricingModel = "per_call"
		pricingAmount = agenttools.CLAWAmount(skill.PricingCLAW)
	}

	schema := skill.Schema
	if schema == nil {
		schema = defaultSchema()
	}

	return p.client.RegisterTool(ctx, &agenttools.RegisterToolRequest{
		Name:        skill.Name,
		Version:     skill.Version,
		Description: skill.Description,
		Schema:      schema,
		Pricing: &agenttools.Pricing{
			Model:      pricingModel,
			AmountCLAW: pricingAmount,
		},
		Endpoint:  skill.Endpoint,
		TimeoutMS: skill.TimeoutMS,
		Tags:      skill.Tags,
	})
}

func defaultSchema() map[string]any {
	return map[string]any{
		"input": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		},
		"output": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		},
	}
}

// JSONMarshal is a convenience helper for constructing schemas from structs.
func JSONMarshal(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}
