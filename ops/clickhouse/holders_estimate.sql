CREATE TABLE IF NOT EXISTS holders_estimate (
  chain_id     UInt16,
  mint         String,
  window_start DateTime('UTC'),
  holders      UInt64,
  method       LowCardinality(String),
  updated_at   DateTime('UTC') DEFAULT now('UTC')
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (chain_id, mint, window_start);
