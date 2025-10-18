# Solana Liquidity Indexer

This repository houses the next-generation Solana market-data indexer and aggregator. It ingests
on-chain activity from Yellowstone Geyser and Helius, normalizes DEX events, computes stateful
metrics in C++, and publishes canonical data products to NATS, ClickHouse, and Parquet.

## High-Level Goals
- Realtime swap, pool, and candle data for Raydium AMM, Orca Whirlpools, and Meteora pools
- Canonical Solana pair normalization (USDC / USDT / SOL priority)
- Exactly-once semantics via NATS JetStream `Msg-Id = "501:<slot>:<sig>:<index>"`
- Backfill parity with live flow using StreamingFast Substreams
- Safe cutover from the legacy Rust market-data service with bridge and shadow comparison

## Structure Overview
```
proto/                     Protobuf contracts (dex.sol.v1)
ingestor/                  Go: stream tailers for Yellowstone and Helius
  geyser/                  Primary Geyser gRPC tailer
  helius/                  LaserStream/WS fallback tailer
  common/                  Shared slot cache, replay & dedupe helpers
decoder/                   Go adapters per DEX program id
  raydium/
  orca_whirlpool/
  meteora/
state/
  candle_cpp/              C++20 candle + VWAP engine
  price_cpp/               C++20 price/fixed-point utilities
sinks/                     Go sinks to NATS, ClickHouse, Parquet
  nats/
  clickhouse/
  parquet/
backfill/
  substreams/              Substreams modules & manifests
  orchestrator/            Go scheduler driving backfills
api/
  http/                    Go GraphQL/REST service
  grpc/                    Go internal gRPC service
bridge/                    Temporary bridge to legacy subjects
legacy/market-data-rs/     Existing Rust market-data binary (lifted in)
ops/                       Operational assets (JetStream, ClickHouse, dashboards)
Makefile                   Build, lint, test, run targets
flake.nix & default.nix    Nix development shells
```

## Getting Started
1. Install Nix or ensure Go (1.22+), Clang 17, and protoc are available.
2. Install the NATS CLI (`brew install nats-io/nats-tools/nats` or `CGO_ENABLED=0 go install github.com/nats-io/natscli/nats@latest`).
3. Run `make bootstrap` to pull toolchains, install git hooks, and fetch proto deps.
4. Use `make proto-gen` whenever protobuf contracts change.
5. Start dependencies locally with `make up` (NATS, ClickHouse, MinIO, Postgres). Set `BOOTSTRAP_JETSTREAM=1 make up` to auto-seed the JetStream stream and consumer after the containers start.
6. Run the geyser ingestor once credentials are set:

   ```bash
   PROGRAMS_YAML_PATH=ops/programs.yaml \
   NATS_URL=nats://127.0.0.1:4222 \
   NATS_STREAM=DEX \
   go run ./cmd/ingestor/geyser
   ```

   Emits Raydium and Orca Whirlpool swap events today; Meteora integration is
   in progress.

   To enable Helius fallback, export `ENABLE_HELIUS_FALLBACK=1` and provide
   `HELIUS_GRPC`, `HELIUS_WS`, and `HELIUS_API_KEY` before launching the binary.

## Cutover Phases (Summary)
1. **Dark launch** – run new ingestors + bridge while legacy Rust stack stays live.
2. **Shadow compare** – diff swap/candle parity for curated pools; alert on variance.
3. **Flip read-paths** – downstream services consume `dex.sol.*` or GraphQL API.
4. **Retire legacy** – stop Rust binary, remove bridge after one-week soak.

See `docs/SPEC.md` for the canonical specification and `docs/CUTOVER.md` for the detailed playbook.
