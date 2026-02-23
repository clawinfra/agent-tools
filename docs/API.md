# API Reference — agent-tools

Base URL: `http://<host>:8433/v1`

All requests accept and return `application/json`.

Authentication: Bearer token (DID-signed JWT) — `Authorization: Bearer <token>`

---

## Health

### GET /healthz

No auth required.

**Response 200:**
```json
{
  "status": "ok",
  "version": "0.1.0",
  "uptime_seconds": 3600
}
```

---

## Tools

### POST /v1/tools

Register a new tool.

**Request:**
```json
{
  "name": "solidity-auditor",
  "version": "1.0.0",
  "description": "Audits Solidity smart contracts for vulnerabilities",
  "schema": {
    "input": {
      "type": "object",
      "properties": {
        "source": { "type": "string", "description": "Solidity source code" }
      },
      "required": ["source"]
    },
    "output": {
      "type": "object",
      "properties": {
        "findings": { "type": "array" },
        "severity": { "type": "string", "enum": ["low", "medium", "high", "critical"] }
      }
    }
  },
  "pricing": {
    "model": "per_call",
    "amount_claw": "10.0"
  },
  "endpoint": "grpc://10.0.0.44:50051",
  "timeout_ms": 30000,
  "tags": ["security", "solidity", "audit"]
}
```

**Response 201:**
```json
{
  "id": "did:claw:tool:abc123...",
  "name": "solidity-auditor",
  "version": "1.0.0",
  "provider_id": "did:claw:agent:xyz789...",
  "created_at": "2026-02-23T05:00:00Z"
}
```

---

### GET /v1/tools

List all active tools (paginated).

**Query params:** `?page=1&limit=20&provider=<did>&tag=security`

**Response 200:**
```json
{
  "tools": [...],
  "total": 42,
  "page": 1,
  "limit": 20
}
```

---

### GET /v1/tools/search

Full-text search across tool name, description, and tags.

**Query params:** `?q=solidity+audit&max_price_claw=50&page=1&limit=20`

**Response 200:**
```json
{
  "tools": [...],
  "total": 3,
  "query": "solidity audit"
}
```

---

### GET /v1/tools/:id

Get a specific tool by DID.

**Response 200:** Full tool object including schema.

**Response 404:** Tool not found.

---

### PUT /v1/tools/:id

Update a tool (provider only).

Can update: `description`, `pricing`, `endpoint`, `timeout_ms`, `tags`.
Cannot update: `name`, `version`, `schema` (create a new version instead).

---

### DELETE /v1/tools/:id

Deactivate a tool (provider only). Soft delete — existing invocations continue.

---

## Invocations

### POST /v1/invoke

Invoke a tool.

**Request:**
```json
{
  "tool_id": "did:claw:tool:abc123...",
  "input": {
    "source": "pragma solidity ^0.8.0; ..."
  },
  "budget_claw": "50.0",
  "idempotency_key": "invoke-2026-02-23-001"
}
```

**Response 200:**
```json
{
  "invocation_id": "inv_xyz789...",
  "tool_id": "did:claw:tool:abc123...",
  "output": {
    "findings": [...],
    "severity": "medium"
  },
  "receipt": {
    "id": "rcpt_abc123...",
    "provider_sig": "ed25519:...",
    "executed_at": "2026-02-23T05:01:00Z"
  },
  "cost_claw": "10.0",
  "duration_ms": 4200
}
```

---

### GET /v1/invoke/:id

Get invocation status (for async invocations).

**Response 200:**
```json
{
  "invocation_id": "inv_xyz789...",
  "status": "completed",
  "output": {...},
  "receipt": {...},
  "cost_claw": "10.0"
}
```

Status values: `pending`, `running`, `completed`, `failed`, `timeout`

---

## Providers

### POST /v1/providers

Register as a tool provider.

**Request:**
```json
{
  "name": "EvoClaw Edge Node A",
  "endpoint": "grpc://10.0.0.44:50051",
  "pubkey": "ed25519:aabbcc...",
  "stake_claw": "1000.0"
}
```

---

### GET /v1/providers/:id

Get provider info including reputation score and active tools.

---

## Error Responses

All errors follow:

```json
{
  "error": {
    "code": "TOOL_NOT_FOUND",
    "message": "Tool did:claw:tool:abc123 not found",
    "details": {}
  }
}
```

| HTTP | Code | Meaning |
|---|---|---|
| 400 | `INVALID_SCHEMA` | Tool schema fails validation |
| 400 | `INVALID_INPUT` | Invocation input fails tool schema |
| 401 | `UNAUTHORIZED` | Missing or invalid auth token |
| 403 | `FORBIDDEN` | Not tool owner |
| 404 | `TOOL_NOT_FOUND` | Tool ID not found |
| 409 | `DUPLICATE_TOOL` | Tool name+version already registered |
| 429 | `RATE_LIMITED` | Too many requests |
| 408 | `INVOKE_TIMEOUT` | Tool invocation timed out |
| 500 | `INTERNAL_ERROR` | Server error |
| 503 | `PROVIDER_UNAVAILABLE` | Provider agent unreachable |
