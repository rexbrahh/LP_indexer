# Integration Notes

## Proto generation
- Guarded in `Makefile` (`make proto-gen`) using Buf when `proto/` exists.
- Generated Go files should live under `generated/` (currently stubbed until protos land).

## JetStream
- Stream config: `ops/jetstream/streams.dex.json`
- Consumer config: `ops/jetstream/consumer.swaps.json`
- Use `make ops.jetstream.init` to create resources, `make ops.jetstream.verify` to assert existence.

## ClickHouse
- Writer configured via `CLICKHOUSE_DSN`; use TLS if required by infrastructure.
- Batch size / retry settings defined in `sinks/clickhouse/writer.go`; adjust per environment.

## Redis (API cache)
- Environment variables `API_REDIS_ADDR`, `API_REDIS_DB`, `API_REDIS_TTL`, `API_REDIS_PASSWORD`.
- Cache gracefully disables when `API_REDIS_ADDR` absent.

## Docker Compose (local dev)
- `docker-compose.yml` boots NATS, ClickHouse, MinIO, Postgres.
- `make up` / `make down` wrappers manage lifecycle.

## CI (TODO)
- Add GitHub Actions workflow running `go test ./...`, `make lint`, `scripts/build_candles.sh` once integration settles.
