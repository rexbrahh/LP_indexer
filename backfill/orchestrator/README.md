# Backfill Orchestrator Scaffold

This package will coordinate Substreams backfills and route decoded events into
ClickHouse/Parquet sinks. It is responsible for splitting time/slot ranges,
invoking Substreams jobs, and recording checkpoints so backfills can resume
without duplication.

## Planned Responsibilities

* Range scheduler that splits (start_slot, end_slot) into manageable batches.
* Workers that invoke Substreams and stream results into the Go sinks.
* Progress tracking (slot checkpoints, error handling, retries).
* Metrics for throughput (`backfill_events_per_sec`) and error counts.

The current scaffold provides a config object, skeleton orchestrator type, and
stubbed methods so integration tests can compile while the implementation is
in-flight.
