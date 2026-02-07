.PHONY: build install test clean fmt lint

# Build variables
BINARY_NAME=tdls-easy-k8s
VERSION?=dev
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X github.com/user/tdls-easy-k8s/internal/cli.version=$(VERSION) -X github.com/user/tdls-easy-k8s/internal/cli.commit=$(COMMIT) -X github.com/user/tdls-easy-k8s/internal/cli.buildDate=$(BUILD_DATE)"

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/tdls-easy-k8s

# Install the binary to $GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME) to $(GOPATH)/bin..."
	@mkdir -p $(GOPATH)/bin
	@cp bin/$(BINARY_NAME) $(GOPATH)/bin/
	@echo "Installation complete!"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out
	@echo "Clean complete!"

# Run the binary locally
run: build
	./bin/$(BINARY_NAME)

# Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  install        - Build and install to GOPATH/bin"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  clean          - Remove build artifacts"
	@echo "  run            - Build and run the binary"
	@echo "  help           - Show this help message"
