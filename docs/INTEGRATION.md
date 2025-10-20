# Integration Notes

## Proto generation
- Guarded in `Makefile` (`make proto-gen`) using Buf when `proto/` exists.
- Generated Go files should live under `generated/` (currently stubbed until protos land).

## JetStream
- Stream config: `ops/jetstream/streams.dex.json`
- Consumer config: `ops/jetstream/consumer.swaps.json`
- Requires the `nats` CLI (`brew install nats-io/nats-tools/nats` or `CGO_ENABLED=0 go install github.com/nats-io/natscli/nats@latest`).
- Use `make ops.jetstream.init` to create resources, `make ops.jetstream.verify` to assert existence. Override the target server via `NATS_URL` (defaults to `nats://127.0.0.1:${NATS_CLIENT_PORT:-4222}`).
- Set `BOOTSTRAP_JETSTREAM=1 make up` to automatically seed the stream/consumer after Docker services start.

## ClickHouse
- Writer configured via `CLICKHOUSE_DSN`; both `tcp://host:port` and `clickhouse://host:port` DSNs are accepted. Use TLS parameters if required by infrastructure.
- Apply schemas via `make ops.clickhouse.apply` (respects `CLICKHOUSE_DSN`, falls back to docker exec when needed).
- Batch size / retry settings defined in `sinks/clickhouse/writer.go`; adjust per environment.

## Redis (API cache)
- Environment variables `API_REDIS_ADDR`, `API_REDIS_DB`, `API_REDIS_TTL`, `API_REDIS_PASSWORD`.
- Cache gracefully disables when `API_REDIS_ADDR` absent.

## Docker Compose (local dev)
- `docker-compose.yml` boots NATS, ClickHouse, MinIO, Postgres.
- `make up` / `make down` wrappers manage lifecycle.

## CI (TODO)
- Add GitHub Actions workflow running `go test ./...`, `make lint`, `scripts/build_candles.sh` once integration settles.
