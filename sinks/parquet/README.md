# Parquet Cold Storage Writer (Scaffold)

This package contains the initial scaffolding for the Parquet cold-storage
writer described in the locked specification. The component will eventually
buffer canonical protobuf events and periodically write Parquet files to S3 or
compatible object storage for archival/historical analytics.

## Responsibilities (future work)

* Batch trades, pool snapshots, and candles into hourly/daily Parquet files.
* Compress output (Snappy/ZSTD) and upload to S3/MinIO using multipart uploads.
* Expose metrics for batch size, flush latency, and storage errors.
* Support resumable uploads and explicit checkpointing for deterministic
  backfills.

At the moment only configuration parsing and a stubbed `Writer` exist so that
the rest of the pipeline can depend on a stable API while Parquet plumbing is
implemented.

## Configuration

Environment variables mirror the spec defaults:

| Variable                  | Description                                |
| ------------------------- | ------------------------------------------ |
| `S3_ENDPOINT`             | S3/MinIO endpoint (`http://minio:9000`).   |
| `S3_BUCKET`               | Target bucket name.                        |
| `S3_ACCESS_KEY`           | Access key / ID.                           |
| `S3_SECRET_KEY`           | Secret key.                                |
| `PARQUET_FLUSH_INTERVAL_S`| Flush cadence in seconds (default 900).    |
| `PARQUET_PREFIX`          | Object key prefix (default `dex/`).        |

See `config.go` for details.

## Next Steps

1. Integrate `github.com/apache/arrow/go/parquet` (or equivalent) to produce
   schema-aligned files.
2. Implement rolling writers keyed by timeframe (hourly/daily).
3. Add integration tests against MinIO in CI.
