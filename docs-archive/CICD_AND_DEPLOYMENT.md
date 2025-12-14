# CI/CD and Deployment Outline

## Current CI Capabilities

- `nix flake check` runs:
  - `go test ./...` via `buildGoModule` (regenerates protobufs, isolated caches).
  - `go test -race ./...` via `buildGoModule`.
  - Tests call `make proto-gen` inside sandboxed env.
- Dev shells (`nix develop`) ship Go 1.24, protoc+plugins, clang/cmake, nats-server, golangci-lint, buf, etc.
- Checked-in protobuf outputs (`gen/…`) keep builds reproducible.
- `golangci-lint` can be run in shell; future flake check pending stable caches.
- NATS publisher tests now tolerate sandboxed builders (skip when JetStream can’t start).

## CI Pipeline Suggestion

1. `nix develop --command gofmt -w ./...` (format).
2. `nix develop --command golangci-lint run ./...`.
3. `nix develop --command go test ./...` (quick tests).
4. `nix develop --command go test -race ./...` (nightly/PR).
5. `nix flake check` for full reproducible run.
6. Optional: `nix store pre-build` for caching.

## Packaging for Production

### Go Services (ingestors, API, sinks, orchestrator)
- Build statically: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/<service>`.
- Containerize (Dockerfile or nix `dockerTools.buildImage`). Example Dockerfile:
  ```Dockerfile
  FROM gcr.io/distroless/base-debian12
  COPY api-http /usr/local/bin/api-http
  ENTRYPOINT ["/usr/local/bin/api-http"]
  ```
- Optionally expose Nix packages: `packages.${system}.api-http` building binary + image.

### C++ Candle Engine
- Build via CMake/Ninja (`cmake -S state/candle_cpp -B build`, `cmake --install`).
- Create container with runtime libs (`clang`, `libstdc++`).
- Provide a flake package for reproducible builds.

### Infrastructure
- JetStream, ClickHouse, MinIO, Redis currently via `make up` (docker-compose).
- For production use IaC (Terraform/K8s) + metrics config.
- Use committed artifacts (`ops/clickhouse/*.sql`, JetStream JSON) during provisioning.

### Backfill + Substreams
- Once implemented, treat orchestrator as Go service.
- Substreams modules produce .spkg or run via CLI (documented in `/backfill/substreams`).

## Deployment Targets

- Provide Helm charts/Nomad jobs per service (common env vars from `.env` pattern).
- Secrets via Vault/KMS; config via ConfigMap or env.
- Each service exposes health metrics per spec.

## Next Steps

1. Implement actual service logic (ingestors, decoders, sinks).
2. Add flake packages for each binary and container outputs.
3. Integrate `golangci-lint` check once we stabilize caches.
4. Add Helm/manifest prototypes for staging/prod.
5. Automate infra bootstrap: `nix run .#nats-dev`, `clickhouse-client --queries-file ops/clickhouse/all.sql`.
6. Optional: extend flake to build release tarballs (`packages.release-api-http`).

Document revisited on: 2025-10-16 04:22:49 
