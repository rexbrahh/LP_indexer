#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SCHEMA_DIR="$ROOT_DIR/ops/clickhouse"

if [ ! -d "$SCHEMA_DIR" ]; then
  echo "ERROR: ClickHouse schema directory not found at $SCHEMA_DIR" >&2
  exit 1
fi

CLICKHOUSE_CLIENT_CMD=${CLICKHOUSE_CLIENT:-clickhouse-client}

# Resolve clickhouse-client command or fall back to docker exec if possible.
if ! command -v "${CLICKHOUSE_CLIENT_CMD%% *}" >/dev/null 2>&1; then
  if command -v docker >/dev/null 2>&1; then
    CLICKHOUSE_CONTAINER=$(docker ps --format '{{.Names}}' | grep -E 'clickhouse' | head -n1 || true)
    if [ -n "$CLICKHOUSE_CONTAINER" ]; then
      CLICKHOUSE_CLIENT_CMD="docker exec -i $CLICKHOUSE_CONTAINER clickhouse-client"
    else
      echo "ERROR: clickhouse-client not found and no running ClickHouse container detected." >&2
      exit 1
    fi
  else
    echo "ERROR: clickhouse-client not found in PATH and Docker unavailable." >&2
    exit 1
  fi
fi

CLICKHOUSE_DSN_DEFAULT="clickhouse://127.0.0.1:9000/default"
CLICKHOUSE_DSN=${CLICKHOUSE_DSN:-$CLICKHOUSE_DSN_DEFAULT}

readarray -t CLICKHOUSE_ARGS < <(
  python3 - "$CLICKHOUSE_DSN" <<'PY'
import shlex
import sys
from urllib.parse import urlparse, parse_qs

dsn = sys.argv[1]
res = urlparse(dsn)

if res.scheme and res.scheme not in ("clickhouse", "tcp"):
    print(f"ERROR: unsupported DSN scheme '{res.scheme}'. Expected clickhouse:// or tcp://", file=sys.stderr)
    sys.exit(1)

args = []
if res.hostname:
    args.extend(["--host", res.hostname])
if res.port:
    args.extend(["--port", str(res.port)])
if res.username:
    args.extend(["--user", res.username])
if res.password:
    args.extend(["--password", res.password])

database = res.path.lstrip("/") if res.path and res.path != "/" else ""
query_db = parse_qs(res.query).get("database")
if query_db:
    database = query_db[0]
if database:
    args.extend(["--database", database])

for arg in args:
    print(arg)
PY
)

if [ "${CLICKHOUSE_ARGS[0]:-}" = "ERROR:" ]; then
  printf '%s\n' "${CLICKHOUSE_ARGS[@]}" >&2
  exit 1
fi

IFS=' ' read -r -a CLIENT_ARR <<<"$CLICKHOUSE_CLIENT_CMD"

SCHEMA_FILES=()
while IFS= read -r -d '' file; do
  SCHEMA_FILES+=("$file")
done < <(find "$SCHEMA_DIR" -type f -name '*.sql' -print0 | sort -z)

if [ ${#SCHEMA_FILES[@]} -eq 0 ]; then
  echo "No ClickHouse schema files found in $SCHEMA_DIR" >&2
  exit 0
fi

for file in "${SCHEMA_FILES[@]}"; do
  echo "Applying schema: ${file#$ROOT_DIR/}"
  "${CLIENT_ARR[@]}" "${CLICKHOUSE_ARGS[@]}" --multiquery --queries-file "$file"
done

echo "✓ ClickHouse schema applied successfully"
