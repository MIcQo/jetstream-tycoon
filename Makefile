.PHONY: all build lint lint-go lint-ts deadcode vulncheck audit-ts i18n-check prom-check test test-go test-ts sim-test test-integration test-integration-go test-integration-ts fuzz fuzz-go fuzz-ts mutate mutate-go mutate-ts migrations-check migrate-up migrate-down determinism content-check wasm wasm-size-check balance dev

WASM_OUT := web/public/sim.wasm
WASM_RAW_MAX_BYTES := 10485760
WASM_BROTLI_MAX_BYTES := 2621440
FUZZ_TIME ?= 3s
MUTATION_THRESHOLD ?= 80
POSTGRES_DSN ?= postgres://jetstream:jetstream@localhost:5432/jetstream?sslmode=disable
CLICKHOUSE_DSN ?= clickhouse://localhost:9000?username=default&database=default&x-multi-statement=true

all: lint migrations-check i18n-check test test-integration fuzz mutate content-check determinism wasm-size-check build

lint: lint-go lint-ts

lint-go:
	@test -z "$$(gofmt -l .)" || (echo "gofmt check failed on:" && gofmt -l . && exit 1)
	go vet ./...
	go run ./tools/simlint sim
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	fi

deadcode:
	deadcode -test ./...

vulncheck:
	govulncheck ./...

lint-ts:
	@if [ -d web/node_modules ]; then \
		npm --prefix web run typecheck; \
		npm --prefix web run lint; \
	fi

audit-ts:
	npm --prefix web audit --audit-level=high

i18n-check:
	go run ./tools/i18ncheck

prom-check:
	promtool check config deploy/prometheus/prometheus.yml
	promtool check rules deploy/prometheus/rules/*.yml

migrations-check:
	go run ./tools/migrationcheck

test: test-go test-ts

test-go:
	go test -race ./...

test-ts:
	@if [ -d web/node_modules ]; then \
		npm --prefix web run test:unit; \
	fi

sim-test:
	go test -race ./sim/...

test-integration: test-integration-go test-integration-ts

test-integration-go:
	go test -race -tags=integration -run "Integration" ./...

test-integration-ts:
	@if [ -d web/node_modules ]; then \
		npm --prefix web run test:integration; \
	fi

fuzz: fuzz-go fuzz-ts

fuzz-go:
	go test -fuzz=^FuzzRoundCents$$ -fuzztime=$(FUZZ_TIME) ./sim/detmath
	go test -fuzz=^FuzzStateAdvance$$ -fuzztime=$(FUZZ_TIME) ./sim

fuzz-ts:
	@if [ -d web/node_modules ]; then \
		npm --prefix web run test:fuzz; \
	fi

mutate: mutate-go mutate-ts

mutate-go:
	go run ./tools/gomutate -threshold=$(MUTATION_THRESHOLD) sim server

mutate-ts:
	@if [ -d web/node_modules ]; then \
		npm --prefix web run test:mutate; \
	fi

content-check: i18n-check
	go run ./tools/datagen

determinism: wasm
	go test -race -v -run TestDeterministicStateHash ./sim/...
	@hash=$$(go run ./cmd/simwasm); \
	echo "native state hash: $$hash"; \
	test "$$hash" = "8c94d155a43185c2bf23642b35a9588a72963e6559bf9daad7810b2f70720760"; \
	if command -v node >/dev/null 2>&1; then \
		goroot="$$(go env GOROOT)"; \
		if [ -f "$$goroot/lib/wasm/wasm_exec_node.js" ]; then \
			wasm_exec="$$goroot/lib/wasm/wasm_exec_node.js"; \
		else \
			wasm_exec="$$goroot/misc/wasm/wasm_exec_node.js"; \
		fi; \
		wasm_hash=$$(node "$$wasm_exec" $(WASM_OUT)); \
		echo "wasm state hash:   $$wasm_hash"; \
		test "$$wasm_hash" = "$$hash"; \
	fi

wasm:
	@mkdir -p $(dir $(WASM_OUT))
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o $(WASM_OUT) ./cmd/simwasm
	@if command -v wasm-opt >/dev/null 2>&1; then \
		wasm-opt -Oz --all-features $(WASM_OUT) -o $(WASM_OUT); \
	fi

wasm-size-check: wasm
	@raw_bytes=$$(wc -c < $(WASM_OUT) | tr -d ' '); \
	echo "sim.wasm raw size: $$raw_bytes bytes (budget: $(WASM_RAW_MAX_BYTES))"; \
	if [ "$$raw_bytes" -gt "$(WASM_RAW_MAX_BYTES)" ]; then \
		echo "ERROR: sim.wasm raw size exceeds budget"; exit 1; \
	fi; \
	if command -v brotli >/dev/null 2>&1; then \
		brotli -f -k -q 11 $(WASM_OUT); \
		br_bytes=$$(wc -c < $(WASM_OUT).br | tr -d ' '); \
		rm -f $(WASM_OUT).br; \
		echo "sim.wasm brotli size: $$br_bytes bytes (budget: $(WASM_BROTLI_MAX_BYTES))"; \
		if [ "$$br_bytes" -gt "$(WASM_BROTLI_MAX_BYTES)" ]; then \
			echo "ERROR: sim.wasm brotli size exceeds budget"; exit 1; \
		fi; \
	else \
		gzip_bytes=$$(gzip -c -9 $(WASM_OUT) | wc -c | tr -d ' '); \
		echo "sim.wasm gzip-9 size (brotli not installed): $$gzip_bytes bytes (budget: $(WASM_BROTLI_MAX_BYTES))"; \
		if [ "$$gzip_bytes" -gt "$(WASM_BROTLI_MAX_BYTES)" ]; then \
			echo "ERROR: sim.wasm compressed size exceeds budget"; exit 1; \
		fi; \
	fi

build: wasm
	@mkdir -p bin
	go build -trimpath -o bin/server ./cmd/server
	go build -trimpath -o bin/ingestor ./cmd/ingestor
	go build -trimpath -o bin/datagen ./tools/datagen
	go build -trimpath -o bin/balancebot ./tools/balancebot

balance:
	go run ./tools/balancebot

migrate-up: migrations-check
	migrate -path migrations/postgres -database "$(POSTGRES_DSN)" up
	migrate -path migrations/clickhouse -database "$(CLICKHOUSE_DSN)" up

migrate-down: migrations-check
	migrate -path migrations/postgres -database "$(POSTGRES_DSN)" down -all
	migrate -path migrations/clickhouse -database "$(CLICKHOUSE_DSN)" down -all

dev:
	@echo "dev stub"
