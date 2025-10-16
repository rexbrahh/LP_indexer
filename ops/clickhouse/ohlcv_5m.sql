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
