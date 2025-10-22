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
  reserves_base  Decimal(38, 0),
  reserves_quote Decimal(38, 0),
  fee_bps        UInt16,
  provisional    UInt8,
  is_undo        UInt8
) ENGINE = MergeTree
PARTITION BY toDate(ts)
ORDER BY (chain_id, pool_id, slot, sig, idx);
