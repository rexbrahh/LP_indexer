# Candle Pipeline Validation Harness

This harness exercises the end-to-end candle flow: synthetic swaps feed the C++ candle engine, which publishes `dex.sol.v1.Candle` messages to JetStream, and the Go bridge (`cmd/candles`) persists them to ClickHouse.

## Prerequisites

- Docker or Podman (for NATS + ClickHouse containers)
- Go toolchain (1.22 per repo `go.mod`)
- Ninja/CMake + protoc (for the C++ candle module)

## Components

1. **NATS JetStream** (`nats:2.10`) hosting the `DEX` stream
2. **ClickHouse** (`clickhouse/clickhouse-server:24`) for persistence
3. **C++ candle engine driver** (`candle_replay`) that replays recorded swap events via `CandleWorker`
4. **Go bridge** (`go run ./cmd/candles`) consuming JetStream candles and writing to ClickHouse
5. **Validation query** that checks expected rows in ClickHouse

## Setup

```bash
# 1. Start infrastructure
make test-env-up   # starts NATS and ClickHouse via docker-compose

# 2. Seed ClickHouse schema
make clickhouse-init   # runs ops/clickhouse/all.sql against local instance

# 3. Build C++ candle driver
cmake -S state/candle_cpp -B build/candle_cpp && cmake --build build/candle_cpp

# 4. Replay sample swaps into the candle engine
./build/candle_cpp/candle_replay --input fixtures/swaps_sample.csv \
  --nats-url nats://127.0.0.1:4222 --subject-root dex.sol

# 5. Run the Go candle bridge
CLICKHOUSE_DSN=tcp://127.0.0.1:9000 \
CLICKHOUSE_DB=default \
go run ./cmd/candles

# 6. Validate persisted candles
clickhouse-client --query "SELECT count() FROM default.candles WHERE pool_id='RAYDIUM_POOL'"
```

## Expected Result

- ClickHouse `candles` table contains at least one row per replayed window
- `provisional` flag is `0` (finalized)
- Metrics (future): latency < 2s; throughput > 50k swaps/minute in stress mode

## Future Enhancements

- Automate swap replay with recorded datasets (Raydium, Orca, Meteora)
- Add Parquet sink verification once writer implementation lands
- Integrate harness into CI nightly job
