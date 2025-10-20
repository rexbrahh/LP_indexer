.PHONY: help bootstrap proto-gen test lint build clean ops.clickhouse.apply up down ops.jetstream.init ops.jetstream.verify run.bridge check.bridge.metrics run.ingestor.geyser candle-e2e demo.geyser

PROTO_FILES := $(shell find proto -name '*.proto' 2>/dev/null)
GOBIN := $(shell go env GOPATH)/bin

ifeq ($(shell command -v docker-compose >/dev/null 2>&1 && echo present),present)
DOCKER_COMPOSE := docker-compose
else
DOCKER_COMPOSE := docker compose
endif

DOCKER_COMPOSE_FILE ?= docker-compose.yml

# Default target
help:
	@echo "Solana Liquidity Indexer - Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  bootstrap           - Install toolchains and dependencies"
	@echo "  proto-gen           - Generate protobuf code"
	@echo "  test               - Run all tests"
	@echo "  lint               - Run linters"
	@echo "  build              - Build all binaries"
	@echo "  clean              - Clean build artifacts"
	@echo "  ops.clickhouse.apply - Apply ClickHouse schemas in ops/clickhouse"
	@echo "  up                 - Start local dependencies (NATS, ClickHouse, etc.)"
	@echo "  down               - Stop local dependencies"
	@echo "  ops.jetstream.init - Initialize JetStream streams and consumers"
	@echo "  ops.jetstream.verify - Verify JetStream streams and consumers exist"
	@echo "  run.bridge          - Run the legacy bridge with local subject map"
	@echo "  run.ingestor.geyser - Run the geyser ingestor (Raydium swaps -> JetStream)"
	@echo "  candle-e2e         - Run candle replay + ClickHouse validation harness"
	@echo "  check.bridge.metrics - Assert bridge Prometheus metrics respond"
	@echo "  demo.geyser        - Run Geyser streaming demo against a configured endpoint"

# Bootstrap development environment
bootstrap:
	@echo "Installing dependencies..."
	go mod download
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/bufbuild/buf/cmd/buf@latest
	@command -v protoc >/dev/null 2>&1 || { echo "WARNING: protoc not found on PATH. Install from https://github.com/protocolbuffers/protobuf/releases"; }
	@echo "Bootstrap complete"

# Generate protobuf code (if proto directory exists)
proto-gen:
	@if [ -d proto ]; then \
		if [ -z "$(PROTO_FILES)" ]; then \
			echo "No protobuf files found under proto/"; \
			exit 0; \
		fi; \
		command -v buf >/dev/null 2>&1 || { echo "ERROR: buf not found. Install via 'go install github.com/bufbuild/buf/cmd/buf@latest' or use nix develop."; exit 1; }; \
		echo "Linting protobuf schemas..."; \
		buf lint; \
		PROTOC_BIN=""; \
		if [ -n "$${PROTOBUF_PREFIX:-}" ]; then \
			if [ -x "$${PROTOBUF_PREFIX}/bin/protoc" ]; then \
				PROTOC_BIN="$${PROTOBUF_PREFIX}/bin/protoc"; \
			fi; \
		fi; \
		if [ -z "$$PROTOC_BIN" ]; then \
			if [ -x /opt/homebrew/opt/protobuf/bin/protoc ]; then \
				PROTOC_BIN="/opt/homebrew/opt/protobuf/bin/protoc"; \
			elif [ -x /usr/local/opt/protobuf/bin/protoc ]; then \
				PROTOC_BIN="/usr/local/opt/protobuf/bin/protoc"; \
			fi; \
		fi; \
		if [ -z "$$PROTOC_BIN" ]; then \
			if command -v protoc >/dev/null 2>&1; then \
				PROTOC_BIN="$$(command -v protoc)"; \
			else \
				echo "ERROR: protoc not found. Run 'make bootstrap' to install prerequisites."; \
				exit 1; \
			fi; \
		fi; \
		echo "Generating protobuf code..."; \
		rm -rf gen/go gen/cpp; \
		mkdir -p gen/go gen/cpp; \
		PATH="$$PATH:$(GOBIN)" "$$PROTOC_BIN" -I proto \
			--go_out=gen/go --go_opt=paths=source_relative \
			--go-grpc_out=gen/go --go-grpc_opt=paths=source_relative \
			--cpp_out=gen/cpp \
			$(PROTO_FILES); \
		echo "Protobuf generation complete"; \
	else \
		echo "No protobuf definitions found"; \
	fi

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Run linters
lint:
	@echo "Running linters..."
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 || { echo "ERROR: golangci-lint not found. Install via 'go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest' or use nix develop."; exit 1; }
	golangci-lint run ./...

# Build all binaries
build:
	@echo "Building Go packages..."
	go build ./...
	@echo "Build complete"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	go clean

ops.clickhouse.apply:
	@./scripts/apply_clickhouse_schema.sh

# Start local development infrastructure
up:
	@echo "Starting local infrastructure..."
	@if [ ! -f "$(DOCKER_COMPOSE_FILE)" ]; then \
		echo "ERROR: $(DOCKER_COMPOSE_FILE) not found. Generate or configure your local docker-compose file."; \
		exit 1; \
	fi
	@$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) up -d
	@echo "Waiting for services to be ready..."
	sleep 5
	@if [ "$(BOOTSTRAP_JETSTREAM)" = "1" ]; then \
		echo "Bootstrapping JetStream stream/consumers..."; \
		NATS_URL=$${NATS_URL:-nats://127.0.0.1:$${NATS_CLIENT_PORT:-4222}} \
		$(MAKE) --no-print-directory ops.jetstream.init; \
	fi
	@echo "Infrastructure is up"

# Stop local infrastructure
down:
	@echo "Stopping local infrastructure..."
	@if [ ! -f "$(DOCKER_COMPOSE_FILE)" ]; then \
		echo "WARN: $(DOCKER_COMPOSE_FILE) not found, skipping docker compose teardown."; \
		exit 0; \
	fi
	@$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) down
	@echo "Infrastructure is down"

# Initialize JetStream streams and consumers
ops.jetstream.init:
	@echo "Initializing JetStream streams and consumers..."
	@command -v nats >/dev/null 2>&1 || { echo "ERROR: nats CLI not found. Install with: brew install nats-io/nats-tools/nats"; exit 1; }
	@NATS_SERVER=$${NATS_URL:-nats://127.0.0.1:$${NATS_CLIENT_PORT:-4222}}; \
	nats --server $$NATS_SERVER stream add --config ops/jetstream/streams.dex.json || echo "Stream DEX may already exist"; \
	nats --server $$NATS_SERVER consumer add DEX --config ops/jetstream/consumer.swaps.json || echo "Consumer SWAP_FIREHOSE may already exist"
	@echo "✓ JetStream initialization complete!"

# Verify JetStream streams and consumers
ops.jetstream.verify:
	@echo "Verifying JetStream streams and consumers..."
	@command -v nats >/dev/null 2>&1 || { echo "ERROR: nats CLI not found. Install with: brew install nats-io/nats-tools/nats"; exit 1; }
	@NATS_SERVER=$${NATS_URL:-nats://127.0.0.1:$${NATS_CLIENT_PORT:-4222}}; \
	nats --server $$NATS_SERVER stream info DEX >/dev/null 2>&1 || { echo "ERROR: Stream DEX does not exist"; exit 1; }; \
	nats --server $$NATS_SERVER consumer info DEX SWAP_FIREHOSE >/dev/null 2>&1 || { echo "ERROR: Consumer SWAP_FIREHOSE does not exist"; exit 1; }
	@echo "✓ JetStream verification complete!"

run.bridge:
	@echo "Starting legacy bridge..."
	@BRIDGE_SOURCE_NATS_URL=$${BRIDGE_SOURCE_NATS_URL:-nats://127.0.0.1:4222}; \
	BRIDGE_TARGET_NATS_URL=$${BRIDGE_TARGET_NATS_URL:-$$BRIDGE_SOURCE_NATS_URL}; \
	BRIDGE_SOURCE_STREAM=$${BRIDGE_SOURCE_STREAM:-DEX}; \
	BRIDGE_TARGET_STREAM=$${BRIDGE_TARGET_STREAM:-legacy}; \
	BRIDGE_SUBJECT_MAP_PATH=$${BRIDGE_SUBJECT_MAP_PATH:-ops/bridge/subject_map.yaml}; \
	BRIDGE_METRICS_ADDR=$${BRIDGE_METRICS_ADDR:-:9090}; \
	if [ ! -f "$$BRIDGE_SUBJECT_MAP_PATH" ]; then echo "ERROR: subject map $$BRIDGE_SUBJECT_MAP_PATH not found"; exit 1; fi; \
	env \
		BRIDGE_SOURCE_NATS_URL="$$BRIDGE_SOURCE_NATS_URL" \
		BRIDGE_TARGET_NATS_URL="$$BRIDGE_TARGET_NATS_URL" \
		BRIDGE_SOURCE_STREAM="$$BRIDGE_SOURCE_STREAM" \
		BRIDGE_TARGET_STREAM="$$BRIDGE_TARGET_STREAM" \
		BRIDGE_SUBJECT_MAP_PATH="$$BRIDGE_SUBJECT_MAP_PATH" \
		BRIDGE_METRICS_ADDR="$$BRIDGE_METRICS_ADDR" \
		go run ./cmd/bridge

run.ingestor.geyser:
	@echo "Starting geyser ingestor..."
	@PROGRAMS_YAML_PATH=$${PROGRAMS_YAML_PATH:-ops/programs.yaml}; \
	NATS_URL=$${NATS_URL:-nats://127.0.0.1:4222}; \
	NATS_STREAM=$${NATS_STREAM:-DEX}; \
	NATS_SUBJECT_ROOT=$${NATS_SUBJECT_ROOT:-dex.sol}; \
	INGESTOR_METRICS_ADDR=$${INGESTOR_METRICS_ADDR:-:9101}; \
	if [ ! -f "$$PROGRAMS_YAML_PATH" ]; then echo "ERROR: programs file $$PROGRAMS_YAML_PATH not found"; exit 1; fi; \
	env \
		PROGRAMS_YAML_PATH="$$PROGRAMS_YAML_PATH" \
		NATS_URL="$$NATS_URL" \
		NATS_STREAM="$$NATS_STREAM" \
		NATS_SUBJECT_ROOT="$$NATS_SUBJECT_ROOT" \
		INGESTOR_METRICS_ADDR="$$INGESTOR_METRICS_ADDR" \
		go run ./cmd/ingestor/geyser

check.bridge.metrics:
	@echo "Checking bridge metrics endpoint..."
	BRIDGE_METRICS_URL?=http://127.0.0.1:9090/metrics
	BRIDGE_METRICS_URL=$${BRIDGE_METRICS_URL} ./scripts/check_bridge_metrics.sh

candle-e2e:
	@scripts/run_candle_e2e.sh $(INPUT)

demo.geyser:
	@GEYSER_ENDPOINT=$${GEYSER_ENDPOINT} \
	GEYSER_API_KEY=$${GEYSER_API_KEY} \
	GEYSER_PROGRAMS_JSON=$${GEYSER_PROGRAMS_JSON:-ops/geyser_programs_demo.json} \
	go run ./cmd/ingestor/geyser-demo
