CREATE TABLE IF NOT EXISTS wallet_activity (
  chain_id       UInt16,
  wallet         String,
  first_seen_slot UInt64,
  swaps_24h      UInt32,
  swaps_7d       UInt32,
  is_fresh       UInt8,
  is_sniper      UInt8,
  updated_at     DateTime('UTC') DEFAULT now('UTC')
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (chain_id, wallet);
