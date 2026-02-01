.PHONY: build test run clean docker-build docker-run help

# Variables
BINARY_NAME=notes-sync-server
DOCKER_IMAGE=notes-sync-backend
PORT?=8080
DB_PATH?=./notes.db

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the server binary
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) ./cmd/server/main.go
	@echo "Build complete: ./$(BINARY_NAME)"

run: ## Run the server (PORT=8080 DB_PATH=./notes.db)
	@echo "Starting server on port $(PORT)..."
	PORT=$(PORT) DB_PATH=$(DB_PATH) go run ./cmd/server/main.go

test: ## Run all tests
	@echo "Running tests..."
	go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	go test -race -v ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -f notes.db
	@echo "Clean complete"

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

lint: ## Run golangci-lint (requires golangci-lint)
	@echo "Running linter..."
	golangci-lint run

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):latest .
	@echo "Docker image built: $(DOCKER_IMAGE):latest"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run -p $(PORT):8080 -v $(PWD)/data:/data $(DOCKER_IMAGE):latest

docker-stop: ## Stop Docker container
	@echo "Stopping Docker container..."
	docker stop $$(docker ps -q --filter ancestor=$(DOCKER_IMAGE):latest)

dev: ## Run in development mode with auto-reload (requires air)
	@echo "Starting development server..."
	air

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

install: build ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	go install ./cmd/server

all: clean fmt vet test build ## Run all checks and build
	@echo "All tasks complete!"
