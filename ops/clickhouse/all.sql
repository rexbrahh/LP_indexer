-- Complete ClickHouse schema for the Solana liquidity indexer.

CREATE TABLE IF NOT EXISTS trades (
  chain_id      UInt16,
  slot          UInt64,
  ts            DateTime64(3, 'UTC'),
  sig           String,
  idx           UInt32,
  program_id    LowCardinality(String),
  pool_id       String,
  mint_base     String,
  mint_quote    String,
  dec_base      UInt8,
  dec_quote     UInt8,
  base_in       Decimal(38, 0),
  base_out      Decimal(38, 0),
  quote_in      Decimal(38, 0),
  quote_out     Decimal(38, 0),
  price_q32     Int64,
  provisional   UInt8
) ENGINE = MergeTree
PARTITION BY toDate(ts)
ORDER BY (chain_id, pool_id, slot, sig, idx);

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

CREATE TABLE IF NOT EXISTS ohlcv_1s (
  chain_id     UInt16,
  pair_id      String,
  pool_id      String,
  window_start DateTime('UTC'),
  provisional  UInt8,
  open_px_q32  Int64,
  high_px_q32  Int64,
  low_px_q32   Int64,
  close_px_q32 Int64,
  vwap_num     Decimal(38, 0),
  vwap_den     Decimal(38, 0),
  vol_base     Decimal(38, 0),
  vol_quote    Decimal(38, 0),
  trades       UInt32,
  updated_at   DateTime('UTC') DEFAULT now('UTC')
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toDate(window_start)
ORDER BY (chain_id, pair_id, pool_id, window_start);

CREATE TABLE IF NOT EXISTS ohlcv_1m (
  chain_id     UInt16,
  pair_id      String,
  pool_id      String,
  window_start DateTime('UTC'),
  provisional  UInt8,
  open_px_q32  Int64,
  high_px_q32  Int64,
  low_px_q32   Int64,
  close_px_q32 Int64,
  vwap_num     Decimal(38, 0),
  vwap_den     Decimal(38, 0),
  vol_base     Decimal(38, 0),
  vol_quote    Decimal(38, 0),
  trades       UInt32,
  updated_at   DateTime('UTC') DEFAULT now('UTC')
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toDate(window_start)
ORDER BY (chain_id, pair_id, pool_id, window_start);

CREATE TABLE IF NOT EXISTS ohlcv_5m (
  chain_id     UInt16,
  pair_id      String,
  pool_id      String,
  window_start DateTime('UTC'),
  provisional  UInt8,
  open_px_q32  Int64,
  high_px_q32  Int64,
  low_px_q32   Int64,
  close_px_q32 Int64,
  vwap_num     Decimal(38, 0),
  vwap_den     Decimal(38, 0),
  vol_base     Decimal(38, 0),
  vol_quote    Decimal(38, 0),
  trades       UInt32,
  updated_at   DateTime('UTC') DEFAULT now('UTC')
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toDate(window_start)
ORDER BY (chain_id, pair_id, pool_id, window_start);

CREATE TABLE IF NOT EXISTS ohlcv_1h (
  chain_id     UInt16,
  pair_id      String,
  pool_id      String,
  window_start DateTime('UTC'),
  provisional  UInt8,
  open_px_q32  Int64,
  high_px_q32  Int64,
  low_px_q32   Int64,
  close_px_q32 Int64,
  vwap_num     Decimal(38, 0),
  vwap_den     Decimal(38, 0),
  vol_base     Decimal(38, 0),
  vol_quote    Decimal(38, 0),
  trades       UInt32,
  updated_at   DateTime('UTC') DEFAULT now('UTC')
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toDate(window_start)
ORDER BY (chain_id, pair_id, pool_id, window_start);

CREATE TABLE IF NOT EXISTS ohlcv_1d (
  chain_id     UInt16,
  pair_id      String,
  pool_id      String,
  window_start DateTime('UTC'),
  provisional  UInt8,
  open_px_q32  Int64,
  high_px_q32  Int64,
  low_px_q32   Int64,
  close_px_q32 Int64,
  vwap_num     Decimal(38, 0),
  vwap_den     Decimal(38, 0),
  vol_base     Decimal(38, 0),
  vol_quote    Decimal(38, 0),
  trades       UInt32,
  updated_at   DateTime('UTC') DEFAULT now('UTC')
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toDate(window_start)
ORDER BY (chain_id, pair_id, pool_id, window_start);

CREATE TABLE IF NOT EXISTS wallet_activity (
  chain_id        UInt16,
  wallet          String,
  first_seen_slot UInt64,
  swaps_24h       UInt32,
  swaps_7d        UInt32,
  is_fresh        UInt8,
  is_sniper       UInt8,
  updated_at      DateTime('UTC') DEFAULT now('UTC')
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (chain_id, wallet);

CREATE TABLE IF NOT EXISTS holders_estimate (
  chain_id     UInt16,
  mint         String,
  window_start DateTime('UTC'),
  holders      UInt64,
  method       LowCardinality(String),
  updated_at   DateTime('UTC') DEFAULT now('UTC')
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (chain_id, mint, window_start);
