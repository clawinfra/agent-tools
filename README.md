# 🔧 agent-tools

[![CI](https://github.com/clawinfra/agent-tools/actions/workflows/ci.yml/badge.svg)](https://github.com/clawinfra/agent-tools/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.22+-blue)](https://go.dev)
[![License](https://img.shields.io/badge/license-Apache%202.0-green)](LICENSE)
[![Coverage](https://img.shields.io/badge/coverage-90%25-brightgreen)](https://github.com/clawinfra/agent-tools/actions)

**The picks-and-shovels layer for autonomous AI agents.**

`agent-tools` is a decentralized tool registry and invocation protocol for autonomous AI agents. Agents register tools they offer, discover tools they need, invoke them with cryptographic receipts, and settle payments in CLAW tokens via [ClawChain](https://github.com/clawinfra/claw-chain).

Built for [EvoClaw](https://github.com/clawinfra/evoclaw) agents. Works with any agent framework.

---

## Why Now?

The AI agent ecosystem is maturing. Agents no longer just chat — they execute arbitrage strategies, manage liquidity positions, audit smart contracts, and coordinate with other agents to accomplish complex tasks.

The missing layer is **trustless tool composition**: an agent on device A should be able to discover a specialized tool running on device B, invoke it, receive a cryptographic receipt proving execution, and pay for it — all autonomously, without a centralized broker.

That's `agent-tools`.

### The Gap

| Capability | Coinbase AgentKit | Fetch.ai uAgents | ElizaOS | **agent-tools** |
|---|---|---|---|---|
| Wallet management | ✅ | ✅ | ❌ | ✅ (via ClawChain) |
| Agent discovery | ❌ | ✅ | ❌ | ✅ |
| Tool registry | ❌ | ❌ | plugin-only | ✅ |
| Metered invocation | ❌ | ❌ | ❌ | ✅ |
| Execution receipts | ❌ | ❌ | ❌ | ✅ |
| Agent micropayments | partial | ❌ | ❌ | ✅ |
| EvoClaw native | ❌ | ❌ | ❌ | ✅ |

---

## Core Concepts

### Tool
A named, versioned, schema-defined capability that an agent exposes. Tools have:
- An input/output JSON schema
- A pricing policy (free, per-call, per-token, subscription)
- An execution SLA (timeout, retry policy)
- A cryptographic identity (Ed25519 keypair tied to the provider's DID)

### Provider
An agent (EvoClaw or compatible) that registers one or more tools. Providers stake CLAW tokens as collateral — bad actors lose their stake.

### Consumer
An agent that discovers tools, invokes them, and pays in CLAW. Payments are escrowed before invocation and released upon valid receipt.

### Receipt
A cryptographically signed proof of execution. Contains: tool ID, input hash, output hash, execution timestamp, provider signature. Receipts are anchored to ClawChain for auditability.

---

## Quick Start

### Install

```bash
# One-liner (Linux/macOS)
curl -fsSL https://raw.githubusercontent.com/clawinfra/agent-tools/main/install.sh | sh

# Build from source
git clone https://github.com/clawinfra/agent-tools
cd agent-tools
go build -ldflags="-s -w" -o agent-tools ./cmd/agent-tools
```

### Start the Registry Server

```bash
# Initialize config
agent-tools init

# Start registry (default: :8433)
agent-tools serve

# Check health
curl http://localhost:8433/healthz
```

### Register a Tool (Provider)

```bash
agent-tools tool register \
  --name "solidity-auditor" \
  --version "1.0.0" \
  --schema ./schemas/solidity-auditor.json \
  --price "10 CLAW/call" \
  --timeout 30s
```

Or via Go SDK:

```go
import "github.com/clawinfra/agent-tools/sdk/go/agenttools"

client := agenttools.NewClient("http://localhost:8433")

tool := &agenttools.ToolSpec{
    Name:    "solidity-auditor",
    Version: "1.0.0",
    Schema:  agenttools.MustLoadSchema("./schemas/solidity-auditor.json"),
    Pricing: agenttools.PerCallPricing(10), // 10 CLAW per call
    Timeout: 30 * time.Second,
}

registration, err := client.RegisterTool(ctx, tool)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Tool registered: %s\n", registration.ID)
```

### Discover Tools (Consumer)

```bash
# Search by capability
agent-tools tool search --query "solidity audit"

# List all tools
agent-tools tool list
```

```go
results, err := client.SearchTools(ctx, &agenttools.SearchQuery{
    Query:    "solidity audit",
    MaxPrice: agenttools.CLAWAmount(100),
})
for _, tool := range results.Tools {
    fmt.Printf("Found: %s (%s) — %s\n", tool.Name, tool.Version, tool.Pricing)
}
```

### Invoke a Tool

```go
receipt, err := client.InvokeTool(ctx, &agenttools.InvokeRequest{
    ToolID: "did:claw:tool:abc123",
    Input:  map[string]any{"contract": soliditySource},
    Budget: agenttools.CLAWAmount(50), // max spend
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Output: %v\n", receipt.Output)
fmt.Printf("Receipt: %s\n", receipt.ID) // anchored to ClawChain
fmt.Printf("Cost: %s CLAW\n", receipt.Cost)
```

---

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full system design.

**Components:**
- **Registry API** — REST + gRPC endpoints for tool CRUD and discovery
- **Invocation Router** — Routes invoke requests to provider agents
- **Receipt Engine** — Generates and verifies cryptographic execution receipts
- **Payment Gateway** — Escrow + settlement via ClawChain CLAW tokens
- **EvoClaw Plugin** — Native integration for EvoClaw agents

---

## Integration with EvoClaw

`agent-tools` is designed as a first-class EvoClaw plugin:

```toml
# evoclaw.toml
[[plugins]]
name = "agent-tools"
source = "github.com/clawinfra/agent-tools/evoclaw-plugin"
version = "0.1.0"

[plugins.config]
registry_url = "http://localhost:8433"
claw_wallet   = "did:claw:wallet:your-wallet-id"
auto_register = true  # auto-register all skills as tools
```

When `auto_register = true`, EvoClaw automatically exposes all installed skills as agent-tools tools, making them discoverable and monetizable by other agents.

---

## Roadmap

### v0.1 — Foundation (2 weeks)
- [x] Tool registration + discovery (REST API)
- [x] Local SQLite storage
- [x] JSON schema validation
- [x] Go SDK
- [x] CLI (`agent-tools tool register/list/search`)
- [x] EvoClaw plugin scaffold
- [x] CI/CD with 90%+ test coverage

### v0.2 — Invocation (4 weeks)
- [ ] Tool invocation protocol (gRPC)
- [ ] Execution receipts (Ed25519 signatures)
- [ ] Basic usage metering (invocation count, latency)
- [ ] Rate limiting + circuit breaker
- [ ] Timeout + retry policy enforcement

### v0.3 — Payments (6 weeks)
- [ ] ClawChain payment integration (CLAW escrow + settlement)
- [ ] Provider staking + slashing
- [ ] Consumer credit system
- [ ] Receipt anchoring on ClawChain

### v1.0 — Production
- [ ] Tool marketplace UI
- [ ] Multi-registry federation
- [ ] SLA enforcement + reputation scoring
- [ ] Audit log on ClawChain

---

## Project Structure

```
agent-tools/
├── cmd/
│   └── agent-tools/        # CLI entrypoint
├── internal/
│   ├── registry/           # Tool registry core
│   ├── router/             # Invocation routing
│   ├── receipts/           # Receipt generation + verification
│   ├── payment/            # ClawChain payment gateway
│   └── store/              # SQLite persistence
├── sdk/
│   └── go/                 # Go SDK for consumers + providers
├── evoclaw-plugin/         # EvoClaw native plugin
├── proto/                  # gRPC protobuf definitions
├── schemas/                # Tool schema examples
├── docs/
│   ├── ARCHITECTURE.md     # System design
│   ├── API.md              # API reference
│   ├── EVOCLAW.md          # EvoClaw integration guide
│   └── PAYMENTS.md         # Payment protocol spec
├── .github/
│   ├── workflows/          # CI/CD
│   ├── ISSUE_TEMPLATE/     # Issue templates
│   └── PULL_REQUEST_TEMPLATE.md
├── go.mod
├── go.sum
├── Makefile
├── LICENSE
└── README.md
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). We welcome contributions!

**Standards (non-negotiable):**
- Test coverage ≥ 90%
- All new code fully typed
- Docs updated with every PR
- CI must be green before merge

```bash
# Dev setup
make dev-setup

# Run tests
make test

# Check coverage
make coverage

# Lint
make lint
```

---

## License

Apache 2.0 — see [LICENSE](LICENSE).

Built by [ClawInfra](https://github.com/clawinfra) · Powered by [EvoClaw](https://github.com/clawinfra/evoclaw) + [ClawChain](https://github.com/clawinfra/claw-chain)
