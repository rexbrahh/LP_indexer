# Engineering Expectations

## Branching & Reviews
- Use feature branches (`feat/<area>-<slug>` or `chore/<slug>`); keep PRs scoped.
- Update relevant docs within the same PR; reference tests covering new paths.
- Tag reviewers with clear “Testing done” notes and any outstanding TODOs.

## Coding Standards
- Go: `gofmt`, `go vet`, `golangci-lint`; avoid unchecked error drops (errcheck). 
- C++: follow clang-format (LLVM style), prefer RAII over manual new/delete.
- Proto: additive changes only; run `buf lint` / `buf breaking` when schemas evolve.

## Testing
- `go test ./...` and `make lint` must stay green on every push to main.
- C++ modules compiled via `scripts/build_candles.sh`; use GoogleTest under `state/candle_cpp/tests`.
- Integration tests should rely on local dockerized NATS/ClickHouse (see `docker-compose.yml`).

## Observability & Ops
- Emit Prometheus metrics defined in `docs/OBSERVABILITY.md` once service is wired.
- Health endpoints must verify downstream dependencies (`/healthz`, `/readyz`).
- Use `make ops.jetstream.verify` to validate infrastructure before enabling consumers.

## Documentation
- Keep `docs/` authoritative; major decisions should include brief ADRs when applicable.
- README sections should mirror current behaviour (e.g., env vars, CLI commands).

## Communication
- Post daily progress & blockers in the shared standup doc or Slack thread.
- Flag schema or protocol changes early for downstream consumers.

Adhering to these expectations keeps the pipeline reliable and easy to scale.
