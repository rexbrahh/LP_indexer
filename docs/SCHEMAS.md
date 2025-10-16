# Protobuf Schemas (`dex.sol.v1`)

## Messages
- **U128** `{ uint64 hi, uint64 lo }`
- **BlockHead** `{ chain_id, slot, ts_sec, status }`
- **TxMeta** `{ chain_id, slot, sig, success, cu_used, cu_price, log_msgs[] }`
- **SwapEvent** `{ chain_id, slot, sig, index, program_id, pool_id, mint_base, mint_quote, dec_base, dec_quote, amounts, sqrt_price_q64_pre/post, reserves, fee_bps, provisional, is_undo }`
- **PoolSnapshot** `{ chain_id, slot, pool_id, mint_base, mint_quote, sqrt_price_q64, reserves, fee_bps, liquidity }`
- **Candle** `{ chain_id, pair_id, pool_id, timeframe, window_start, provisional, is_correction, q32 prices, U128 volumes, trades }`
- **WalletHeuristics** `{ chain_id, wallet, first_seen_slot, swaps_24h, swaps_7d, is_fresh, is_sniper, bundled_pct }`

## Generation
- Proto files to reside under `proto/dex/sol/v1/` (to be committed).
- Use Buf for linting/generation (`make proto-gen`).
- Generated Go code should land under `generated/go/...` with module import path `github.com/rexbrahh/lp-indexer/gen/go/...`.

## Compatibility
- Additive changes only; never renumber existing fields.
- Use optional fields/oneofs for schema evolution.
- Run `buf breaking` against main before merging schema changes.
