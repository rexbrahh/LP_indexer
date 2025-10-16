#!/usr/bin/env bash
set -euo pipefail

METRICS_URL=${BRIDGE_METRICS_URL:-http://127.0.0.1:9090/metrics}

echo "Fetching metrics from $METRICS_URL" >&2
curl -sf "$METRICS_URL" | grep -E 'dex_bridge_(forward_total|dropped_total|publish_errors_total|source_lag_seconds)' || {
  echo "Bridge metrics not found" >&2
  exit 1
}

echo "✓ Bridge metrics exposed"
