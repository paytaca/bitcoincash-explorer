.PHONY: build build-api build-zmq clean run test deps docker-build docker-up docker-down

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
API_BINARY=bin/api
ZMQ_BINARY=bin/zmq-listener

# Build directories
BUILD_DIR=bin

# Default target
all: build

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Build both binaries
build: build-api build-zmq

# Build API server
build-api: deps
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -o $(API_BINARY) ./cmd/api

# Build ZMQ listener
build-zmq: deps
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -o $(ZMQ_BINARY) ./cmd/zmq-listener

# Run API server (requires Redis and BCH node)
run-api: build-api
	./$(API_BINARY)

# Run ZMQ listener (requires Redis and BCH node with ZMQ)
run-zmq: build-zmq
	./$(ZMQ_BINARY)

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Run tests
test:
	$(GOTEST) -v ./...

# Docker commands
docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Development mode with hot reload (requires air)
dev-api:
	air -c .air.toml || echo "Install air with: go install github.com/cosmtrek/air@latest"

# Format code
fmt:
	gofmt -w internal/ cmd/

# Lint code
lint:
	golangci-lint run || echo "Install golangci-lint: https://golangci-lint.run/usage/install/"

# Security scan
sec:
	gosec ./... || echo "Install gosec with: go install github.com/securego/gosec/v2/cmd/gosec@latest"

# Show help
help:
	@echo "Available targets:"
	@echo "  make build        - Build both binaries"
	@echo "  make build-api    - Build API server only"
	@echo "  make build-zmq    - Build ZMQ listener only"
	@echo "  make run-api      - Run API server"
	@echo "  make run-zmq      - Run ZMQ listener"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make docker-build - Build Docker images"
	@echo "  make docker-up    - Start Docker containers"
	@echo "  make docker-down  - Stop Docker containers"
	@echo "  make fmt          - Format Go code"
	@echo "  make lint         - Run linter"
	@echo "  make deps         - Download dependencies"