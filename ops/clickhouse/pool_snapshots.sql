CREATE TABLE IF NOT EXISTS pool_snapshots (
  chain_id       UInt16,
  slot           UInt64,
  ts             DateTime64(3, 'UTC'),
  pool_id        String,
  mint_base      String,
  mint_quote     String,
  sqrt_q64       UInt64,
  reserves_base  Decimal(38, 0),
  reserves_quote Decimal(38, 0),
  fee_bps        UInt16,
  liquidity      Decimal(38, 0)
) ENGINE = MergeTree
PARTITION BY toDate(ts)
ORDER BY (chain_id, pool_id, slot);
