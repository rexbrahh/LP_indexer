.PHONY: help bootstrap proto-gen test lint build clean up down ops.jetstream.init

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
	@echo "  up                 - Start local dependencies (NATS, ClickHouse, etc.)"
	@echo "  down               - Stop local dependencies"
	@echo "  ops.jetstream.init - Initialize JetStream streams and consumers"

# Bootstrap development environment
bootstrap:
	@echo "Installing dependencies..."
	go mod download
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Bootstrap complete"

# Generate protobuf code
proto-gen:
	@echo "Generating protobuf code..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/**/*.proto
	@echo "Protobuf generation complete"

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

# Run linters
lint:
	@echo "Running linters..."
	go vet ./...
	gofmt -l -s .
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping"

# Build all binaries
build:
	@echo "Building binaries..."
	go build -o bin/ingestor ./ingestor/cmd
	go build -o bin/api ./api/cmd
	@echo "Build complete"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	go clean

# Start local development infrastructure
up:
	@echo "Starting local infrastructure..."
	docker-compose up -d
	@echo "Waiting for services to be ready..."
	sleep 5
	@echo "Infrastructure is up"

# Stop local infrastructure
down:
	@echo "Stopping local infrastructure..."
	docker-compose down
	@echo "Infrastructure is down"

# Initialize JetStream streams and consumers
ops.jetstream.init:
	@echo "Initializing JetStream streams and consumers..."
	@command -v nats >/dev/null 2>&1 || { echo "ERROR: nats CLI not found. Install with: brew install nats-io/nats-tools/nats"; exit 1; }
	@echo ""
	@echo "Creating dex-swaps stream..."
	@nats stream add dex-swaps \
		--subjects="dex.sol.swaps" \
		--storage=file \
		--retention=workq \
		--replicas=1 \
		--discard=old \
		--max-age=168h \
		--max-msg-size=1048576 \
		--dupe-window=1h \
		--defaults || echo "Stream dex-swaps may already exist"
	@echo ""
	@echo "Creating dex-candles-1m stream..."
	@nats stream add dex-candles-1m \
		--subjects="dex.sol.candles.1m" \
		--storage=file \
		--retention=workq \
		--replicas=1 \
		--max-age=720h \
		--dupe-window=1h \
		--defaults || echo "Stream dex-candles-1m may already exist"
	@echo ""
	@echo "Creating dex-candles-5m stream..."
	@nats stream add dex-candles-5m \
		--subjects="dex.sol.candles.5m" \
		--storage=file \
		--retention=workq \
		--replicas=1 \
		--max-age=2160h \
		--dupe-window=1h \
		--defaults || echo "Stream dex-candles-5m may already exist"
	@echo ""
	@echo "Creating dex-candles-1h stream..."
	@nats stream add dex-candles-1h \
		--subjects="dex.sol.candles.1h" \
		--storage=file \
		--retention=workq \
		--replicas=1 \
		--max-age=8760h \
		--dupe-window=1h \
		--defaults || echo "Stream dex-candles-1h may already exist"
	@echo ""
	@echo "Creating clickhouse-sink consumer for dex-swaps..."
	@nats consumer add dex-swaps clickhouse-sink \
		--filter="dex.sol.swaps" \
		--ack=explicit \
		--wait=30s \
		--max-deliver=3 \
		--deliver=all \
		--replay=instant \
		--defaults || echo "Consumer clickhouse-sink may already exist"
	@echo ""
	@echo "Creating clickhouse-sink consumer for dex-candles-1m..."
	@nats consumer add dex-candles-1m clickhouse-sink \
		--filter="dex.sol.candles.1m" \
		--ack=explicit \
		--wait=30s \
		--max-deliver=3 \
		--deliver=all \
		--replay=instant \
		--defaults || echo "Consumer clickhouse-sink may already exist"
	@echo ""
	@echo "✓ JetStream initialization complete!"
	@echo ""
	@echo "Verify with:"
	@echo "  nats stream ls"
	@echo "  nats consumer ls dex-swaps"
	@echo ""
	@echo "See ops/jetstream/README.md for detailed documentation"
