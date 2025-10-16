.PHONY: help bootstrap proto-gen test lint build clean up down ops.jetstream.init ops.jetstream.verify

PROTO_FILES := $(shell find proto -name '*.proto' 2>/dev/null)
GOBIN := $(shell go env GOPATH)/bin

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
	@echo "  ops.jetstream.verify - Verify JetStream streams and consumers exist"

# Bootstrap development environment
bootstrap:
	@echo "Installing dependencies..."
	go mod download
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@command -v protoc >/dev/null 2>&1 || { echo "WARNING: protoc not found on PATH. Install from https://github.com/protocolbuffers/protobuf/releases"; }
	@echo "Bootstrap complete"

# Generate protobuf code (if proto directory exists)
proto-gen:
	@if [ -d proto ]; then \
		if [ -z "$(PROTO_FILES)" ]; then \
			echo "No protobuf files found under proto/"; \
			exit 0; \
		fi; \
		command -v protoc >/dev/null 2>&1 || { echo "ERROR: protoc not found. Run 'make bootstrap' to install prerequisites."; exit 1; }; \
		echo "Generating protobuf code..."; \
		mkdir -p gen/go gen/cpp; \
		PATH="$$PATH:$(GOBIN)" protoc -I proto \
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

# Verify JetStream streams and consumers
ops.jetstream.verify:
	@echo "Verifying JetStream streams and consumers..."
	@command -v nats >/dev/null 2>&1 || { echo "ERROR: nats CLI not found. Install with: brew install nats-io/nats-tools/nats"; exit 1; }
	@nats stream info DEX >/dev/null 2>&1 || { echo "ERROR: Stream DEX does not exist"; exit 1; }
	@nats consumer info DEX SWAP_FIREHOSE >/dev/null 2>&1 || { echo "ERROR: Consumer SWAP_FIREHOSE does not exist"; exit 1; }
	@echo "✓ JetStream verification complete!"
