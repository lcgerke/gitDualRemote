.PHONY: build clean test install help

# Build variables
BINARY_NAME=githelper
BUILD_DIR=.
CMD_DIR=./cmd/githelper
GO=go

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	rm -rf /tmp/test-*
	rm -rf /tmp/bare-repos
	@echo "Clean complete"

# Run tests
test:
	@echo "Running unit tests..."
	$(GO) test -v -short ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GO) test -v ./test/integration/...

# Run all tests
test-all:
	@echo "Running all tests..."
	$(GO) test -v ./...

# Install to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GO) install $(CMD_DIR)
	@echo "Install complete"

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Show help
help:
	@echo "GitHelper Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build    - Build the githelper binary"
	@echo "  clean    - Remove build artifacts and test directories"
	@echo "  test     - Run tests"
	@echo "  install  - Install to GOPATH/bin"
	@echo "  fmt      - Format code"
	@echo "  lint     - Run linter (requires golangci-lint)"
	@echo "  help     - Show this help message"

# Default target
.DEFAULT_GOAL := build
