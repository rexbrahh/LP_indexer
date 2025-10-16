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

# Generate protobuf code (if proto directory exists)
proto-gen:
	@if [ -f buf.yaml ] && [ -d proto ]; then \
		echo "Generating protobuf code..."; \
		buf generate; \
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
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping"

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
	@nats stream add --config ops/jetstream/streams.dex.json || echo "Stream DEX may already exist"
	@nats consumer add DEX --config ops/jetstream/consumer.swaps.json || echo "Consumer SWAP_FIREHOSE may already exist"
	@echo "✓ JetStream initialization complete!"
