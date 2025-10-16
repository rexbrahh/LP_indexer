# Environment Configuration

## Required Services
- **NATS JetStream** (`NATS_URL`)
- **ClickHouse** (`CLICKHOUSE_DSN`)
- **MinIO/S3** (`S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`)
- **Postgres** (backfill/orchestrator checkpoints)
- **Redis** (API cache; optional)

## Environment Files
- `.env.example` lists vars consumed by Go services and sinks.
- Use `direnv` or `nix develop` (see `flake.nix`) for reproducible dev shells.

## Local Development
```bash
make up          # boots NATS, ClickHouse, MinIO, Postgres
make down        # stops services
make ops.jetstream.init   # create JetStream stream/consumer
make ops.jetstream.verify # confirm resources exist
```

## Production Notes
- Provision NATS cluster with file storage, 3 replicas.
- Use dedicated ClickHouse cluster (or service) with TLS if required.
- Store secrets in 1Password/Credstash; inject via environment or secret manager.
- Ensure Chainstack/Helius credentials rotated per security policy.
