.PHONY: proto build run test clean migrate install-tools docker-build docker-up docker-down help

# Variables
BINARY_NAME=discordliteserver
PROTO_DIR=api/proto
OUTPUT_DIR=bin

# Help command
help:
	@echo "Available targets:"
	@echo "  proto          - Generate Go code from protobuf definitions"
	@echo "  build          - Build the server binary"
	@echo "  run            - Run the server"
	@echo "  test           - Run tests"
	@echo "  clean          - Remove build artifacts"
	@echo "  migrate        - Run database migrations"
	@echo "  install-tools  - Install required development tools"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-up      - Start services with docker-compose"
	@echo "  docker-down    - Stop services with docker-compose"

# Install required tools (protoc, protoc-gen-go, etc.)
install-tools:
	@echo "Installing protoc plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Tools installed successfully"

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/auth.proto
	@echo "Protobuf code generated successfully"

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
	rm -f $(PROTO_DIR)/*.pb.go
	@echo "Clean complete"

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

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Default target
all: proto build
