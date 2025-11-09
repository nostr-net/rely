.PHONY: help build run test clean docker-build docker-run init-db

# Variables
BINARY_NAME=nostr-relay
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the relay binary
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) .

build-linux: ## Build for Linux AMD64
	@echo "Building $(BINARY_NAME) for Linux..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .

run: ## Run the relay (requires config.yaml)
	@if [ ! -f config.yaml ]; then \
		echo "Error: config.yaml not found. Copy config.yaml.example first."; \
		exit 1; \
	fi
	go run $(LDFLAGS) .

dev: ## Run in development mode with live reload (requires air)
	air

test: ## Run all tests
	go test -v ./...

test-integration: ## Run integration tests (requires ClickHouse)
	@echo "Running integration tests..."
	CLICKHOUSE_DSN="clickhouse://localhost:9000/nostr_test" go test -v ./internal/storage/clickhouse

test-coverage: ## Run tests with coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

bench: ## Run benchmarks
	go test -bench=. -benchmem ./...

clean: ## Clean build artifacts
	rm -f $(BINARY_NAME) $(BINARY_NAME)-*
	rm -f coverage.out

init-db: ## Initialize ClickHouse database schema
	@echo "Initializing database schema..."
	@if command -v clickhouse-client >/dev/null 2>&1; then \
		clickhouse-client < internal/storage/clickhouse/migrations/001_consolidated_schema.sql; \
		echo "✓ Database schema initialized"; \
	else \
		echo "Error: clickhouse-client not found. Install ClickHouse first."; \
		exit 1; \
	fi

docker-build: ## Build Docker image
	docker build -t nostr-relay:$(VERSION) -f Dockerfile ../..

docker-run: ## Run with Docker Compose
	docker-compose up -d

docker-stop: ## Stop Docker Compose
	docker-compose down

docker-logs: ## Show Docker logs
	docker-compose logs -f relay

docker-clean: ## Clean Docker resources
	docker-compose down -v

install: build ## Install the relay binary to /usr/local/bin
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo install -m 755 $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "✓ Installed successfully"

uninstall: ## Uninstall the relay binary
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "✓ Uninstalled successfully"

fmt: ## Format code
	go fmt ./...
	gofumpt -l -w .

lint: ## Run linters
	golangci-lint run

deps: ## Download dependencies
	go mod download
	go mod tidy

upgrade-deps: ## Upgrade dependencies
	go get -u ./...
	go mod tidy

setup-dev: ## Setup development environment
	@echo "Setting up development environment..."
	@go install github.com/cosmtrek/air@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install mvdan.cc/gofumpt@latest
	@cp config.yaml.example config.yaml
	@echo "✓ Development environment ready"

systemd-install: build ## Install as systemd service
	@echo "Installing systemd service..."
	@sudo install -m 755 $(BINARY_NAME) /opt/nostr-relay/$(BINARY_NAME)
	@sudo cp config.yaml /opt/nostr-relay/config.yaml
	@sudo cp systemd/nostr-relay.service /etc/systemd/system/
	@sudo systemctl daemon-reload
	@echo "✓ Systemd service installed"
	@echo "  Start with: sudo systemctl start nostr-relay"
	@echo "  Enable on boot: sudo systemctl enable nostr-relay"

.DEFAULT_GOAL := help
