# Contributing to agent-tools

Thanks for your interest! `agent-tools` is part of the [ClawInfra](https://github.com/clawinfra) ecosystem. We welcome contributions.

## Standards

These are non-negotiable for all PRs:

- **Test coverage ≥ 90%** — CI blocks merges below this threshold
- **All code typed** — no `any` escapes without explicit justification
- **Docs updated** — new features need API/architecture doc updates
- **CI green** — all tests, lint, and vet must pass
- **Commit messages** — follow [Conventional Commits](https://www.conventionalcommits.org/)

## Dev Setup

**Requirements:**
- Go 1.22+
- `protoc` + `protoc-gen-go` (for gRPC changes)
- SQLite3

```bash
git clone https://github.com/clawinfra/agent-tools
cd agent-tools

# Install tools
make dev-setup

# Run tests
make test

# Check coverage (must be ≥90%)
make coverage

# Run linter
make lint

# Start dev server (hot reload via air)
make dev
```

## Project Structure

```
cmd/agent-tools/    — CLI entrypoint (cobra)
internal/registry/  — Tool registration + storage
internal/router/    — Invocation routing
internal/receipts/  — Receipt generation + verification  
internal/payment/   — ClawChain payment gateway (v0.3)
internal/store/     — SQLite persistence layer
sdk/go/             — Public Go SDK
evoclaw-plugin/     — EvoClaw native plugin
proto/              — gRPC protobuf definitions
```

## Pull Request Process

1. Fork the repo
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Write tests first (TDD preferred)
4. Implement the feature
5. Run `make test && make coverage && make lint`
6. Open a PR with a clear description

## Commit Format

```
feat(registry): add FTS5 search endpoint
fix(router): handle provider timeout correctly
docs(api): add invoke error codes
test(receipts): add receipt verification edge cases
```

## Issue Reporting

Use the issue templates:
- `bug_report.md` — for bugs
- `feature_request.md` — for new features

## License

By contributing, you agree your contributions are licensed under Apache 2.0.
