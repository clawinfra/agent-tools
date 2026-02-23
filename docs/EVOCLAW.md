# EvoClaw Integration Guide

`agent-tools` is designed as a first-class EvoClaw extension. This guide explains how to connect them.

## Quick Setup

### 1. Install the plugin

```toml
# In your evoclaw.toml / agent.toml
[[plugins]]
name = "agent-tools"
source = "github.com/clawinfra/agent-tools/evoclaw-plugin"
version = "0.1.0"

[plugins.config]
registry_url  = "http://localhost:8433"   # local registry
claw_wallet   = "did:claw:wallet:your-id"  # your CLAW wallet for payments
auto_register = true                       # auto-expose skills as tools
consumer      = true                       # allow invoking other agents' tools
```

### 2. Skills become Tools

With `auto_register = true`, every skill installed on your EvoClaw agent is automatically registered in the tool registry. Other agents can discover and invoke your skills.

Skills map to tools:

| EvoClaw Skill | agent-tools Tool |
|---|---|
| `shield-agent` | `smart-contract-auditor` |
| `hybrid-memory` | `semantic-search` |
| `whalecli` | `whale-wallet-tracker` |
| `fear-harvester` | `fear-index-trader` |

### 3. Invoke other agents' tools

From your EvoClaw agent code (Go):

```go
import "github.com/clawinfra/agent-tools/sdk/go"

// In your agent's Run() loop
toolClient := agenttools.NewEvoClaw(ctx)

// Discover audit tool
auditor, err := toolClient.Search(ctx, "solidity audit", agenttools.WithMaxPrice(50))
if err != nil || len(auditor) == 0 {
    return fmt.Errorf("no auditor tool found")
}

// Invoke it
result, err := toolClient.Invoke(ctx, auditor[0].ID, map[string]any{
    "source": contractSource,
})
if err != nil {
    return err
}

log.Printf("Audit findings: %v (cost: %s CLAW)", result.Output["findings"], result.Cost)
```

## Skill-to-Tool Schema Mapping

The plugin automatically derives a JSON schema from a skill's SKILL.md:

```
SKILL.md inputs/outputs → JSON Schema  
SKILL.md description    → tool description  
skill.toml pricing      → tool pricing policy  
```

For custom schema control, add a `schema.json` to your skill:

```json
// skills/my-skill/schema.json
{
  "input": {
    "type": "object",
    "properties": {
      "query": { "type": "string" }
    },
    "required": ["query"]
  },
  "output": {
    "type": "object",
    "properties": {
      "result": { "type": "string" }
    }
  }
}
```

## Pricing Configuration

In `skill.toml`:

```toml
[agent_tools]
pricing_model = "per_call"     # free | per_call | per_token | subscription
price_claw    = "5.0"          # CLAW per invocation
timeout_ms    = 10000
tags          = ["search", "memory", "retrieval"]
```

## Provider Endpoint

The plugin starts a gRPC server on your EvoClaw agent for receiving invocations:

```toml
[plugins.config]
grpc_port = 50051  # default
```

Other agents connect to `grpc://<your-ip>:50051` to invoke your tools.
