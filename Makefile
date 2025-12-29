.PHONY: proto proto-go proto-swift proto-check proto-clean build run test clean migrate install-tools install-buf docker-build docker-up docker-down help fmt fmt-check lint ci

# Variables
BINARY_NAME=discordliteserver
PROTO_DIR=api/proto
OUTPUT_DIR=bin
BUF_VERSION=1.47.2

# Help command
help:
	@echo "Available targets:"
	@echo "  proto          - Generate Go and Swift code from protobuf definitions"
	@echo "  proto-go       - Generate only Go code"
	@echo "  proto-swift    - Generate only Swift code"
	@echo "  proto-check    - Validate protobuf definitions"
	@echo "  proto-clean    - Remove generated protobuf code"
	@echo "  build          - Build the server binary"
	@echo "  run            - Run the server"
	@echo "  test           - Run tests"
	@echo "  clean          - Remove build artifacts"
	@echo "  migrate        - Run database migrations"
	@echo "  install-tools  - Install required development tools"
	@echo "  install-buf    - Install Buf CLI"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-up      - Start services with docker-compose"
	@echo "  docker-down    - Stop services with docker-compose"
	@echo ""
	@echo "CI/Quality targets:"
	@echo "  fmt            - Format code with go fmt"
	@echo "  fmt-check      - Check if code is formatted (CI mode)"
	@echo "  lint           - Run golangci-lint"
	@echo "  ci             - Run all CI checks locally (fmt-check, lint, proto-check, test)"

# Install required tools (protoc, protoc-gen-go, etc.)
install-tools: install-buf
	@echo "All protobuf tools installed successfully"

# Install Buf CLI
install-buf:
	@echo "Checking for Buf CLI..."
	@if ! command -v buf >/dev/null 2>&1; then \
		echo "Installing Buf CLI..."; \
		if [ "$$(uname -s)" = "Darwin" ]; then \
			brew install bufbuild/buf/buf || brew install buf; \
		elif [ "$$(uname -s)" = "Linux" ]; then \
			curl -sSL "https://github.com/bufbuild/buf/releases/download/v$(BUF_VERSION)/buf-$$(uname -s)-$$(uname -m)" -o /tmp/buf; \
			chmod +x /tmp/buf; \
			sudo mv /tmp/buf /usr/local/bin/buf; \
		else \
			echo "Please install Buf manually: https://buf.build/docs/installation"; \
			exit 1; \
		fi; \
	else \
		echo "Buf CLI already installed: $$(buf --version)"; \
	fi

# Generate all protobuf code (Go + Swift)
proto: install-buf
	@echo "Generating Go and Swift protobuf code..."
	cd $(PROTO_DIR) && buf generate
	@echo "Protobuf code generated successfully!"
	@echo "  Go:    api/gen/go/discord/auth/v1/"
	@echo "  Swift: api/gen/swift/discord/auth/v1/"

# Generate only Go code
proto-go: install-buf
	@echo "Generating Go protobuf code..."
	cd $(PROTO_DIR) && buf generate --template buf.gen.go.yaml --path discord/auth/v1
	@echo "Go code generated in api/gen/go/"

# Generate only Swift code
proto-swift: install-buf
	@echo "Generating Swift protobuf code..."
	cd $(PROTO_DIR) && buf generate --template buf.gen.swift.yaml --path discord/auth/v1
	@echo "Swift code generated in api/gen/swift/"

# Validate protobuf definitions
proto-check: install-buf
	@echo "Validating protobuf definitions..."
	cd $(PROTO_DIR) && buf lint
	cd $(PROTO_DIR) && buf breaking --against '.git#branch=main' || true
	@echo "Protobuf validation complete"

# Clean generated protobuf code
proto-clean:
	@echo "Cleaning generated protobuf code..."
	rm -rf api/gen/go/*
	rm -rf api/gen/swift/*
	@echo "Generated code cleaned"

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(OUTPUT_DIR)
	go build -o $(OUTPUT_DIR)/$(BINARY_NAME) cmd/server/main.go
	@echo "Build complete: $(OUTPUT_DIR)/$(BINARY_NAME)"

# Run the server
run:
	@echo "Starting server..."
	go run cmd/server/main.go

# Test targets
.PHONY: test test-unit test-integration test-coverage test-coverage-html clean-test

# Run all tests
test:
	@echo "Running all tests..."
	go test -v -race -timeout=5m ./...

# Run only unit tests (no TestContainers)
test-unit:
	@echo "Running unit tests..."
	go test -v -short -race ./...

# Run only integration tests (with TestContainers)
test-integration:
	@echo "Running integration tests..."
	go test -v -run Integration -timeout=10m ./...

# Generate coverage report
test-coverage:
	@echo "Generating coverage report..."
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out
	@echo "Total coverage:"
	@go tool cover -func=coverage.out | grep total | awk '{print $$3}'

# Generate HTML coverage report
test-coverage-html: test-coverage
	@echo "Generating HTML coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean test artifacts
clean-test:
	@echo "Cleaning test artifacts..."
	rm -f coverage.out coverage.html

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(OUTPUT_DIR)
	@echo "Clean complete"
	@echo "Note: Generated proto files preserved (committed to git)"
	@echo "      Run 'make proto-clean' to remove them"

# Docker commands
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

docker-up:
	@echo "Starting services with docker-compose..."
	docker-compose up -d

docker-down:
	@echo "Stopping services with docker-compose..."
	docker-compose down

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Check code formatting (CI mode)
fmt-check:
	@echo "Checking code formatting..."
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not formatted:"; \
		echo "$$unformatted"; \
		echo ""; \
		echo "Please run 'make fmt' to format your code."; \
		exit 1; \
	fi
	@echo "All files are properly formatted!"

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run --timeout=5m --config=.golangci.yml

# Run all CI checks locally
ci: fmt-check lint proto-check test
	@echo ""
	@echo "✅ All CI checks passed!"

# Default target
all: proto build
