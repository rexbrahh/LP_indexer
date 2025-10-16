PROJECT_NAME := solana-liquidity-indexer
GO_FILES := $(shell find . -name '*.go' -not -path './legacy/*')
PROTO_DIR := proto
PROTO_OUT := generated

.PHONY: all bootstrap proto-gen fmt lint test up down clean

all: build

bootstrap:
	@echo '>> Installing tooling (protoc plugins, pre-commit hooks)'
	@./scripts/bootstrap.sh

proto-gen:
	@echo '>> Generating protobuf code'
	@./scripts/gen_protos.sh $(PROTO_DIR) $(PROTO_OUT)

fmt:
	@echo '>> Formatting Go and C++ sources'
	@./scripts/fmt.sh

lint:
	@echo '>> Running linters'
	@./scripts/lint.sh

build:
	@echo '>> Building Go services'
	@./scripts/build.sh

candles:
	@echo '>> Building C++ candle engine'
	@./scripts/build_candles.sh

run.ingestor:
	@echo '>> Starting Geyser + Helius ingestors'
	@./scripts/run_ingestors.sh

run.candles:
	@echo '>> Starting C++ candle engine'
	@./scripts/run_candles.sh

run.sinks:
	@echo '>> Starting sinks (NATS, ClickHouse, Parquet)'
	@./scripts/run_sinks.sh

run.bridge:
	@echo '>> Starting legacy compatibility bridge'
	@./scripts/run_bridge.sh

up:
	@echo '>> Starting local dependencies (NATS, ClickHouse, MinIO, Postgres)'
	@docker compose up -d

down:
	@docker compose down

clean:
	@rm -rf $(PROTO_OUT) cmake-build-* build dist
