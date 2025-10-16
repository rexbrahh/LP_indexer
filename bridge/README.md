# Legacy Bridge (Scaffold)

Temporary compatibility service that mirrors the new `dex.sol.*` subjects to the
legacy market-data topics used by downstream systems. The locked spec requires
this bridge during dark launch so that old consumers continue working while we
migrate them to the new contracts.

## Responsibilities

* Subscribe to the canonical JetStream subjects (swaps, candles, snapshots).
* Transform payloads into legacy message shapes/subjects.
* Preserve idempotency using the same `Nats-Msg-Id = "501:<slot>:<sig>:<index>"`.
* Expose health metrics and lag monitors.

The current scaffold only wires configuration parsing and a stub `Run` method.
Implementation will follow in a future change once the legacy subject mapping is
finalised.
