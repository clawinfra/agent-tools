# Architecture — agent-tools

> **Status:** Design v0.1 — living document, updated with each major release.

---

## Problem Statement

Autonomous AI agents need to compose capabilities. An arbitrage agent needs a price oracle tool. A governance agent needs a contract auditor. A DeFi agent needs a sentiment analyzer.

Today, each of these tools is a hard-coded dependency — bundled into the agent binary, updated manually, paid for through centralized APIs. This creates:

1. **Brittle coupling** — update one tool, redeploy everything
2. **No fair payment** — tools are either free (unsustainable) or SaaS-priced (not agent-native)
3. **No provenance** — you can't prove what tool was used or what it returned
4. **Vendor lock-in** — Coinbase AgentKit requires CDP; Eliza plugins are JS-only

`agent-tools` solves this with a decentralized tool registry + invocation protocol + payment settlement layer, all native to the EvoClaw + ClawChain ecosystem.

---

## System Components

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              agent-tools                                    │
│                                                                             │
│  ┌──────────────────────┐    ┌───────────────────────┐                     │
│  │   Registry API       │    │   Invocation Router   │                     │
│  │   (REST + gRPC)      │    │   (gRPC)              │                     │
│  │                      │    │                       │                     │
│  │  • POST /tools       │    │  • Route to provider  │                     │
│  │  • GET  /tools       │    │  • Escrow payment     │                     │
│  │  • GET  /tools/search│    │  • Enforce timeout    │                     │
│  │  • DELETE /tools/:id │    │  • Generate receipt   │                     │
│  └────────┬─────────────┘    └──────────┬────────────┘                     │
│           │                             │                                   │
│  ┌────────▼─────────────────────────────▼────────────┐                     │
│  │                  Core Services                     │                     │
│  │                                                    │                     │
│  │  ┌────────────┐  ┌────────────┐  ┌─────────────┐  │                     │
│  │  │  Registry  │  │  Receipt   │  │  Payment    │  │                     │
│  │  │  Store     │  │  Engine    │  │  Gateway    │  │                     │
│  │  │  (SQLite)  │  │  (Ed25519) │  │  (CLAW)    │  │                     │
│  │  └────────────┘  └────────────┘  └──────┬──────┘  │                     │
│  └────────────────────────────────────────┬┼──────────┘                     │
│                                           ││                                │
└───────────────────────────────────────────┼┼────────────────────────────────┘
                                            ││
                            ┌───────────────▼▼───────────────┐
                            │         ClawChain L1           │
                            │   (Substrate-based blockchain) │
                            │                                │
                            │  • AgentRegistry pallet        │
                            │  • ClawToken pallet            │
                            │  • Escrow pallet (v0.3)        │
                            │  • ReceiptAnchor pallet (v0.3) │
                            └────────────────────────────────┘
```

---

## Data Flow

### Tool Registration

```
Provider Agent                Registry API              ClawChain
     │                             │                        │
     │  POST /tools                │                        │
     │  {name, schema, pricing}    │                        │
     │────────────────────────────▶│                        │
     │                             │ Validate schema        │
     │                             │ Generate tool DID      │
     │                             │ Store in SQLite        │
     │                             │                        │
     │                             │ (v0.3) anchor on-chain │
     │                             │───────────────────────▶│
     │                             │                        │ tx confirmed
     │                             │◀───────────────────────│
     │  201 {tool_id, did}         │                        │
     │◀────────────────────────────│                        │
```

### Tool Discovery

```
Consumer Agent                Registry API
     │                             │
     │  GET /tools/search?q=...    │
     │────────────────────────────▶│
     │                             │ FTS5 search
     │                             │ Apply filters
     │                             │ Rank by reputation
     │  200 [{tool, provider, ...}]│
     │◀────────────────────────────│
```

### Tool Invocation (v0.1 — direct, no payment)

```
Consumer Agent                Registry API           Provider Agent
     │                             │                       │
     │  POST /invoke               │                       │
     │  {tool_id, input, budget}   │                       │
     │────────────────────────────▶│                       │
     │                             │ Resolve provider addr │
     │                             │ Validate input schema │
     │                             │ Forward invocation    │
     │                             │──────────────────────▶│
     │                             │                       │ Execute tool
     │                             │                       │ Sign receipt
     │                             │ {output, receipt_sig} │
     │                             │◀──────────────────────│
     │                             │ Verify receipt sig    │
     │                             │ Return to consumer    │
     │  {output, receipt_id}       │                       │
     │◀────────────────────────────│                       │
```

### Tool Invocation with Payment (v0.3)

```
Consumer Agent       Payment Gateway    ClawChain    Provider Agent
     │                     │               │               │
     │  POST /invoke       │               │               │
     │  {tool_id, budget}  │               │               │
     │────────────────────▶│               │               │
     │                     │ Lock escrow   │               │
     │                     │──────────────▶│               │
     │                     │ escrow_id     │               │
     │                     │◀──────────────│               │
     │                     │               │               │
     │                     │ Forward invoke with escrow_id │
     │                     │───────────────────────────────▶│
     │                     │               │               │ Execute
     │                     │               │               │ Sign receipt
     │                     │ {output, sig, escrow_id}      │
     │                     │◀───────────────────────────────│
     │                     │               │               │
     │                     │ Verify sig    │               │
     │                     │ Release escrow│               │
     │                     │──────────────▶│               │
     │                     │               │ CLAW → provider│
     │  {output, receipt}  │               │               │
     │◀────────────────────│               │               │
```

---

## Components Detail

### 1. Registry Store (SQLite)

Schema:

```sql
CREATE TABLE tools (
    id          TEXT PRIMARY KEY,  -- tool DID: did:claw:tool:<hash>
    name        TEXT NOT NULL,
    version     TEXT NOT NULL,
    description TEXT,
    schema      TEXT NOT NULL,     -- JSON Schema (input + output)
    pricing     TEXT NOT NULL,     -- JSON pricing policy
    provider_id TEXT NOT NULL,     -- provider DID: did:claw:agent:<hash>
    endpoint    TEXT NOT NULL,     -- gRPC endpoint for invocation
    timeout_ms  INTEGER NOT NULL DEFAULT 30000,
    tags        TEXT,              -- comma-separated tags
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    is_active   INTEGER NOT NULL DEFAULT 1
);

CREATE VIRTUAL TABLE tools_fts USING fts5(
    name, description, tags,
    content='tools',
    content_rowid='rowid'
);

CREATE TABLE invocations (
    id              TEXT PRIMARY KEY,
    tool_id         TEXT NOT NULL REFERENCES tools(id),
    consumer_id     TEXT NOT NULL,
    input_hash      TEXT NOT NULL,  -- SHA-256 of input JSON
    output_hash     TEXT,           -- SHA-256 of output JSON
    receipt_sig     TEXT,           -- Ed25519 signature from provider
    status          TEXT NOT NULL,  -- pending|completed|failed|timeout
    cost_claw       TEXT,           -- CLAW amount (decimal string)
    escrow_id       TEXT,           -- ClawChain escrow tx hash
    started_at      INTEGER NOT NULL,
    completed_at    INTEGER,
    error           TEXT
);

CREATE TABLE providers (
    id          TEXT PRIMARY KEY,  -- provider DID
    name        TEXT,
    endpoint    TEXT NOT NULL,
    pubkey      TEXT NOT NULL,     -- Ed25519 public key (hex)
    stake_claw  TEXT,              -- staked CLAW (decimal string)
    reputation  INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    last_seen   INTEGER NOT NULL
);
```

### 2. Registry API (REST)

Base URL: `http://<host>:8433/v1`

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check |
| `POST` | `/tools` | Register a tool |
| `GET` | `/tools` | List tools (paginated) |
| `GET` | `/tools/search` | Full-text search + filter |
| `GET` | `/tools/:id` | Get tool by ID |
| `PUT` | `/tools/:id` | Update tool (provider only) |
| `DELETE` | `/tools/:id` | Deactivate tool |
| `POST` | `/invoke` | Invoke a tool |
| `GET` | `/invoke/:id` | Get invocation status |
| `GET` | `/providers` | List providers |
| `POST` | `/providers` | Register a provider |
| `GET` | `/providers/:id` | Get provider |

Full spec: [docs/API.md](docs/API.md)

### 3. Invocation Router (gRPC)

The router handles the invocation lifecycle:
- Input schema validation (jsonschema)
- Provider health check (last-seen < 30s)
- Payment escrow (v0.3+)
- Forwarding to provider gRPC endpoint
- Timeout enforcement
- Receipt verification
- Payment release (v0.3+)

Provider agents expose a single gRPC endpoint:

```protobuf
service ToolExecutor {
  rpc Execute(ExecuteRequest) returns (ExecuteResponse);
  rpc Health(HealthRequest) returns (HealthResponse);
}
```

See [proto/executor.proto](proto/executor.proto).

### 4. Receipt Engine

Receipts are Ed25519-signed blobs:

```json
{
  "receipt_id": "rcpt_abc123",
  "tool_id": "did:claw:tool:xyz789",
  "consumer_id": "did:claw:agent:consumer",
  "provider_id": "did:claw:agent:provider",
  "input_hash": "sha256:aabbcc...",
  "output_hash": "sha256:ddeeff...",
  "cost_claw": "10.0",
  "executed_at": 1708683600,
  "provider_sig": "ed25519:base64sig..."
}
```

In v0.3, receipts are anchored to ClawChain via the `ReceiptAnchor` pallet, providing immutable audit trails.

### 5. Payment Gateway (v0.3)

Integrates with ClawChain via the existing `clawchain` skill:

```go
// Escrow before invocation
escrowID, err := clawchain.Escrow(ctx, &clawchain.EscrowRequest{
    Consumer: consumerDID,
    Provider: providerDID,
    Amount:   tool.Pricing.MaxCost(budget),
    ToolID:   toolID,
    Timeout:  tool.Timeout + 5*time.Second,
})

// Release after valid receipt
err = clawchain.ReleaseEscrow(ctx, escrowID, actualCost, receipt)
```

### 6. EvoClaw Plugin

Makes every EvoClaw skill automatically available as an `agent-tools` tool:

```go
// evoclaw-plugin/plugin.go
type AgentToolsPlugin struct {
    client   *agenttools.Client
    evoclaw  *evoclaw.Runtime
}

func (p *AgentToolsPlugin) OnSkillLoaded(skill evoclaw.Skill) {
    spec := skillToToolSpec(skill)
    reg, err := p.client.RegisterTool(ctx, spec)
    // now discoverable by other agents
}
```

---

## Tech Stack Rationale

| Layer | Choice | Rationale |
|---|---|---|
| Language | **Go** | EvoClaw is Go; strong stdlib; single binary; excellent concurrency |
| Storage | **SQLite** (FTS5) | Zero dependencies; proven in hybrid-memory; FTS5 for fast text search |
| API | **REST + gRPC** | REST for human tooling + CLIs; gRPC for agent-to-agent (typed, streaming) |
| Crypto | **Ed25519** | Fast; small sigs; used by ClawChain substrate |
| Blockchain | **ClawChain** | Our L1; CLAW token; zero gas; AgentRegistry pallet already exists |
| Schema | **JSON Schema** | Universal; existing Go libraries; human-readable |
| Testing | **Go testing + testify** | Native; fast; 90%+ coverage enforced in CI |

### Why Go over Rust?

EvoClaw's orchestrator is Go. The edge agent is Rust. `agent-tools` is a **server-side registry** that coordinates with the Go orchestrator — Go is the natural fit. Rust would be appropriate for an edge-embedded version (future ClawOS integration).

### Why SQLite over Postgres?

Same reasoning as `hybrid-memory` — zero ops burden, easy to embed, FTS5 is powerful enough for tool search at any realistic scale (millions of tools). Postgres can be added as an optional backend in v1.0 for large deployments.

### Why not IPFS/on-chain storage?

Tool schemas and metadata change frequently during development. Full on-chain storage adds latency and cost. Instead, we store metadata in SQLite and **anchor content hashes** on ClawChain — best of both worlds.

---

## Security Model

### Provider Authentication
- Providers sign registration requests with their DID keypair
- Registry verifies the signature before storing the tool
- Provider public keys are anchored on ClawChain's `AgentRegistry` pallet

### Consumer Authentication
- Consumers authenticate with their DID keypair (same mechanism)
- Rate limiting per consumer DID prevents abuse

### Invocation Integrity
- Consumer signs the invoke request (including input hash)
- Provider signs the receipt (including output hash)
- Registry verifies both signatures before releasing payment

### Staking + Slashing (v0.3)
- Providers stake CLAW as collateral
- Failed SLAs (timeout, wrong output) result in stake slashing
- Consumers can dispute receipts within a challenge window

---

## Deployment

### Standalone Server

```bash
agent-tools serve \
  --addr :8433 \
  --db ./data/agent-tools.db \
  --clawchain ws://testnet.clawchain.win:9944
```

### As EvoClaw Skill

```bash
# Installed as a skill, managed by EvoClaw orchestrator
evoclaw skill install github.com/clawinfra/agent-tools/evoclaw-plugin
```

### Docker / Podman

```bash
podman run -d \
  --name agent-tools \
  -p 8433:8433 \
  -v agent-tools-data:/data \
  ghcr.io/clawinfra/agent-tools:latest
```

---

## Key Design Decisions

### Decision 1: What to build first?

**Options considered:**
1. Payment rails first
2. Tool registry first
3. Execution sandbox first

**Decision: Tool registry first.**

Rationale: Payment rails without tools = nothing to pay for. Execution sandbox is a bigger lift. The registry is the foundation everything else builds on — once developers can register and discover tools, they immediately see value. Payments can be wired in as an upgrade (escrow + receipt already designed for it).

### Decision 2: gRPC vs REST for invocation

**Decision: Both.** REST for developer experience (curl-friendly, easier to debug). gRPC for production agent-to-agent calls (typed contracts, streaming, bidirectional). Same service; REST is a JSON transcoding layer.

### Decision 3: On-chain vs off-chain tool storage

**Decision: Off-chain (SQLite) with on-chain anchoring.**

Full on-chain storage would require a `ToolRegistry` pallet, chain upgrades, and adds ~6 second latency per registration (block time). Instead, we store metadata locally and anchor content hashes + provider DIDs on ClawChain. This gives us auditability without the operational overhead.

### Decision 4: EvoClaw integration approach

**Decision: Native plugin (auto-register = true).**

This means every EvoClaw installation becomes a potential tool provider with zero extra config. Agents can immediately start monetizing their skills. This is the network effect flywheel — more EvoClaw users = more tools in the registry = more value for consumers.

---

## Future Work

- **Multi-registry federation** — registries discover each other; global tool graph
- **Tool versioning + migration** — semver, deprecation, automatic consumer upgrades
- **Agent reputation scoring** — on-chain track record of successful invocations
- **ClawOS integration** — embedded registry for edge device clusters
- **Tool composition** — chain tools together into pipelines with single invocation
- **ZK execution proofs** — prove correct execution without revealing input/output
