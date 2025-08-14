# Makefile for Minkube
# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Binary names
BINARY_NAME=minkube
BINARY_UNIX=$(BINARY_NAME)_unix
BINARY_WINDOWS=$(BINARY_NAME).exe

# Build directory
BUILD_DIR=./build

# Default target
.PHONY: all
all: test build

# Build the binary
.PHONY: build
build: | $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v ./...

# Build for multiple platforms
.PHONY: build-all
build-all: build-linux build-windows build-darwin

.PHONY: build-linux
build-linux: | $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_UNIX) -v ./...

.PHONY: build-windows
build-windows: | $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_WINDOWS) -v ./...

.PHONY: build-darwin
build-darwin: | $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)_darwin -v ./...

# Test
.PHONY: test
test:
	$(GOTEST) -v ./...

# Test with coverage
.PHONY: test-coverage
test-coverage:
	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Clean
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Run the application

# Default port for worker
WORKER_PORT ?= 8000
WORKER_HOST ?= localhost

# Run worker with configurable port
.PHONY: run-worker
run-worker:
    MINKUBE_HOST=$(WORKER_HOST) MINKUBE_PORT=$(WORKER_PORT) MINKUBE_ROLE=worker $(GOCMD) run main.go

# Convenience targets for multiple workers
.PHONY: run-worker1 run-worker2 run-worker3
run-worker1:
    MINKUBE_HOST=localhost MINKUBE_PORT=8001 MINKUBE_ROLE=worker $(GOCMD) run main.go

run-worker2:
    MINKUBE_HOST=localhost MINKUBE_PORT=8002 MINKUBE_ROLE=worker $(GOCMD) run main.go

run-worker3:
    MINKUBE_HOST=localhost MINKUBE_PORT=8003 MINKUBE_ROLE=worker $(GOCMD) run main.go

# Run multiple workers in background (for testing)
.PHONY: run-workers-dev
run-workers-dev:
	@echo "Starting 3 workers in background..."
	MINKUBE_HOST=localhost MINKUBE_PORT=8001 MINKUBE_ROLE=worker $(GOCMD) run main.go &
	MINKUBE_HOST=localhost MINKUBE_PORT=8002 MINKUBE_ROLE=worker $(GOCMD) run main.go &
	MINKUBE_HOST=localhost MINKUBE_PORT=8003 MINKUBE_ROLE=worker $(GOCMD) run main.go &
	@echo "Workers started on ports 8001, 8002, 8003"

# Stop background workers
.PHONY: stop-workers
stop-workers:
	@echo "Stopping workers..."
	pkill -f "MINKUBE_ROLE=worker" || true
	@echo "Workers stopped"

.PHONY: run-manager
run-manager:
	MINKUBE_WORKERS=localhost:8000 MINKUBE_ROLE=manager $(GOCMD) run main.go

.PHONY: run-manager-multi
run-manager-multi:
	MINKUBE_WORKERS=localhost:8001,localhost:8002,localhost:8003 MINKUBE_ROLE=manager $(GOCMD) run main.go

# Run with custom environment
.PHONY: run-dev
run-dev:
    MINKUBE_HOST=localhost MINKUBE_PORT=9000 $(GOCMD) run main.go

.PHONY: run-prod
run-prod:
    MINKUBE_HOST=0.0.0.0 MINKUBE_PORT=8080 $(GOCMD) run main.go

# Development helpers
.PHONY: fmt
fmt:
	$(GOFMT) ./...

.PHONY: vet
vet:
	$(GOCMD) vet ./...

.PHONY: lint
lint:
	golangci-lint run

# Dependency management
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) verify

.PHONY: deps-update
deps-update:
	$(GOMOD) get -u ./...
	$(GOMOD) tidy

.PHONY: tidy
tidy:
	$(GOMOD) tidy

# Docker commands
.PHONY: docker-build
docker-build:
	docker build -t minkube:latest .

.PHONY: docker-run
docker-run:
	docker run -p 9000:9000 -e MINKUBE_HOST=0.0.0.0 -e MINKUBE_PORT=9000 minkube:latest

# Development workflow
.PHONY: dev
dev: fmt vet test build

# Production workflow
.PHONY: prod
prod: clean deps test build-all

# Install development tools
.PHONY: install-tools
install-tools:
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Create build directory
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary for current platform"
	@echo "  build-all     - Build binaries for all platforms"
	@echo "  build-linux   - Build binary for Linux"
	@echo "  build-windows - Build binary for Windows"
	@echo "  build-darwin  - Build binary for macOS"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  run-worker    - Run worker (default port 8000, use WORKER_PORT=xxxx to override)"
	@echo "  run-worker1   - Run worker on port 8001"
	@echo "  run-worker2   - Run worker on port 8002"
	@echo "  run-worker3   - Run worker on port 8003"
	@echo "  run-workers-dev - Start 3 workers in background (ports 8001-8003)"
	@echo "  stop-workers  - Stop all background workers"
	@echo "  run-manager   - Run manager with single worker (localhost:8000)"
	@echo "  run-manager-multi - Run manager with multiple workers (8001-8003)"
	@echo "  run-dev       - Run in development mode"
	@echo "  run-prod      - Run in production mode"
	@echo "  clean         - Clean build artifacts"
	@echo "  fmt           - Format Go code"
	@echo "  vet           - Run go vet"
	@echo "  lint          - Run golangci-lint"
	@echo "  deps          - Download dependencies"
	@echo "  deps-update   - Update dependencies"
	@echo "  tidy          - Tidy go modules"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  dev           - Development workflow (fmt, vet, test, build)"
	@echo "  prod          - Production workflow (clean, deps, test, build-all)"
	@echo "  install-tools - Install development tools"
	@echo "  help          - Show this help message"