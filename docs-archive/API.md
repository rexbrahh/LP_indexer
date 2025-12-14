# HTTP / GraphQL API Overview

## REST Endpoints
| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Liveness check (always 200 when process running) |
| GET | `/readyz` *(todo)* | Readiness check (JetStream subscription, Redis connectivity) |
| GET | `/v1/pool/:id` | Returns latest pool snapshot (stub data today) |
| GET | `/v1/pool/:id/candles?tf=<tf>` | Returns candles for pool, timeframe in {`1s`,`1m`,`5m`,`1h`,`1d`} |

## Example
```bash
curl "http://localhost:8080/v1/pool/SOL-USDC/candles?tf=1m"
```

## GraphQL (planned)
- `pool(id: ID!): Pool`
- `candles(poolId: ID!, timeframe: Timeframe!, limit: Int): [Candle!]`
- `wallet(id: ID!): WalletHeuristics`

## Authentication & Rate Limits
- Stubbed for now; plan to integrate with upstream gateway providing API keys + token bucket.

## Caching
- Redis optional (`API_REDIS_ADDR`). Cache disabled when env var missing.
- TTL default 5 minutes (configurable via `API_REDIS_TTL`).

Detailed schema lives in `docs/api/openapi.yaml` and will evolve alongside endpoint changes.
