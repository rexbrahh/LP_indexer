# Repository Guidelines

## Project Structure & Module Organization
Runtime services follow the ingest -> decode -> publish -> serve flow: core pipelines live in `ingestor/`, `decoder/`, `sinks/`, and `api/`. Operator CLIs and jobs start from `cmd/`, with shared tooling in `ops/` and automation helpers in `scripts/`. Protobuf definitions reside in `proto/`, generated bindings regenerate into `gen/`, and must never be edited by hand. Long-running analytics engines live under `state/candle_cpp/` and `state/price_cpp/`, while dashboards ship from `observability/`. Architecture notes, payload samples, and runbooks belong in `docs/`.

## Build, Test, and Development Commands
- `nix develop` (or `direnv`) provisions Go 1.24, Clang 17, and generators; fall back to `make bootstrap` only when Nix is unavailable.
- `make proto-gen` refreshes bindings after changes in `proto/`.
- `make build` compiles all binaries in `cmd/`; use `go build ./decoder/...` for focused checks.
- `make test` runs the repository suite; scope down with `go test ./ingestor/...`.
- `make up` starts local ClickHouse and NATS, `make down` stops them, and `make ops.jetstream.init` seeds required streams.

## Coding Style & Naming Conventions
Run `go fmt ./...` before committing; Go packages stay lowercase and exported identifiers use CamelCase. Prefer explicit `context.Context`, table-driven tests, and early returns over nested conditionals. C++ modules target C++20 with headers in `include/`, sources in `src/`, snake_case filenames, and the repo Clang-format profile. Keep comments focused on intent rather than mechanics.

## Testing Guidelines
Place Go unit tests beside sources as `*_test.go`; benchmarks live in the same package. Run `go test ./... -race` when touching concurrency or JetStream paths. Guard integration tests that depend on external services behind build tags so CI stays green. Update `docs/` and sample payloads when protocol changes impact downstream consumers.

## Commit & Pull Request Guidelines
Commit subjects use the imperative `Scope message` pattern (for example, `Decoder add whirlpool fee calc`) with related issues in the body. Pull requests should include a concise summary, validation notes (such as `make test` or targeted `go test` runs), artifacts or screenshots for sink and dashboard updates, and confirmation that regenerated outputs (e.g., `make proto-gen`) are committed. Request reviews from the owning team and respond promptly.

## Security & Ops Tips
Never commit secrets; rely on `.envrc` or local Nix variables for credentials. Validate ClickHouse and NATS connectivity with `make up` before shipping infra-sensitive changes. When adjusting telemetry, mirror dashboards in `observability/` and log schema updates in `docs/ops.md` to keep SRE workflows aligned.
