# Wallet Heuristics

## Metrics
- `first_seen_slot` – earliest slot observed for wallet.
- `swaps_24h`, `swaps_7d` – rolling counts.
- `is_fresh` – true if first signature < 14 days old.
- `is_sniper` – true if first swap ≤ 120 seconds from pool creation.
- `bundled_pct` – percentage of swaps marked as bundled/MEV.

## Data Sources
- Swap stream events provide signature + slot.
- Pool creation timestamps sourced from registry (Postgres) or Substreams output.
- Provider metadata used to detect bundling (e.g., Jito tags).

## Implementation Plan
1. Track wallet activity in C++ state service (or Go worker if simpler initially).
2. Emit `dex.sol.wallet.heuristics` protobuf and persist to ClickHouse `wallet_activity` table.
3. Provide API endpoint for wallet lookups; include heuristics in GraphQL schema.

## Acceptance
- Daily job recomputes heuristics and emits corrections if values change post-finalization.
- Tests cover first-seen logic, sniper detection, and bundling thresholds.
