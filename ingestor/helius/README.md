# Helius Fallback Ingestor

This package contains the scaffold for the Helius LaserStream/WebSocket ingestor
described in the locked Solana indexer specification. The implementation is
responsible for acting as the secondary (fallback) data source whenever the
Yellowstone Geyser stream is unavailable or unhealthy.

## Responsibilities

* Establish a LaserStream gRPC tail for program logs and transaction status
  updates filtered to Raydium, Orca Whirlpools, and Meteora programs.
* Maintain a WebSocket backup that can be promoted when LaserStream is degraded.
* Mirror the canonical protobuf contracts (`dex.sol.v1.*`) so downstream sinks
  receive identical payloads from either primary or fallback.
* Honour the replay policy: on reconnect, fetch the previous 64 slots so that
  provisional/final messages remain consistent across sources.
* Surface health signals (`SlotLag`, `LastHeartbeat`, etc.) so the failover
  controller can make an informed switch between Geyser and Helius producers.

At the moment only the scaffolding exists: a validated configuration object, a
client skeleton that brokers failover, and typed update channels that will
eventually feed the shared publishing pipeline. The actual Helius network
integrations will land in a follow-up change.

## Environment Variables

The config mirrors the spec:

| Variable            | Description                                  |
| ------------------- | -------------------------------------------- |
| `HELIUS_GRPC`       | LaserStream gRPC endpoint (host:port).       |
| `HELIUS_WS`         | WebSocket fallback endpoint.                 |
| `HELIUS_API_KEY`    | API key for either transport.                |
| `HELIUS_TIMEOUT_MS` | Request timeout in milliseconds (optional).  |
| `HELIUS_BACKOFF_MS` | Reconnect backoff in milliseconds (optional).|

See `config.go` for defaults and validation rules.

## Next Steps

1. Implement the LaserStream gRPC dialler with authentication headers.
2. Wire the WebSocket fallback path and heartbeat monitoring.
3. Convert the raw responses into `dex.sol.v1.{BlockHead,TxMeta,SwapEvent}` and
   push them into the shared publisher.
4. Add health metrics (Prometheus) and integrate the controller with the Geyser
   ingestor.
