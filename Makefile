# Makefile for cclogviewer

# Binary names
BINARY_NAME=cclogviewer
MCP_BINARY_NAME=cclogviewer-mcp

# Build directory
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build variables
VERSION?=1.0.0
BUILD_TIME=$(shell date +%FT%T%z)
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

# Installation directory - defaults to GOPATH/bin or GOBIN if set
GOPATH?=$(shell go env GOPATH)
GOBIN?=$(shell go env GOBIN)
ifeq ($(GOBIN),)
    INSTALL_DIR=$(GOPATH)/bin
else
    INSTALL_DIR=$(GOBIN)
endif

# Default target
.DEFAULT_GOAL := build

# Create build directory
$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

# Build the binary
build: $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v ./cmd/cclogviewer

# Build the MCP server
build-mcp: $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(MCP_BINARY_NAME) -v ./cmd/cclogviewer-mcp

# Build all binaries
build-all-binaries: build build-mcp

# Build with version info
build-release: $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) -v ./cmd/cclogviewer
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(MCP_BINARY_NAME) -v ./cmd/cclogviewer-mcp


# Install the binary
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)"
	@mkdir -p $(INSTALL_DIR)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	@chmod 755 $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installation complete. You can now run '$(BINARY_NAME)' from anywhere."

# Install the MCP server
install-mcp: build-mcp
	@echo "Installing $(MCP_BINARY_NAME) to $(INSTALL_DIR)"
	@mkdir -p $(INSTALL_DIR)
	@cp $(BUILD_DIR)/$(MCP_BINARY_NAME) $(INSTALL_DIR)/
	@chmod 755 $(INSTALL_DIR)/$(MCP_BINARY_NAME)
	@echo "Installation complete. You can now run '$(MCP_BINARY_NAME)' from anywhere."

# Install all binaries
install-all: install install-mcp

# Uninstall the binary
uninstall:
	@echo "Removing $(BINARY_NAME) and $(MCP_BINARY_NAME) from $(INSTALL_DIR)"
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@rm -f $(INSTALL_DIR)/$(MCP_BINARY_NAME)
	@echo "Uninstall complete."

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f test_output.html
	rm -f example_*.html
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
fmt:
	$(GOCMD) fmt ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Show coverage report in terminal
test-coverage-report: test-coverage
	@$(GOCMD) tool cover -func=coverage.out | grep total | awk '{print "Total Coverage: " $$3}'

# Run integration tests
test-integration:
	$(GOTEST) -tags=integration -v ./...

# Run benchmarks
benchmark:
	$(GOTEST) -bench=. -benchmem ./...

# Run all tests (unit + integration)
test-all: test test-integration

# Build for multiple platforms
build-all: build-linux build-darwin build-windows

build-linux: $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 -v ./cmd/cclogviewer
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 -v ./cmd/cclogviewer

build-darwin: $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 -v ./cmd/cclogviewer
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 -v ./cmd/cclogviewer

build-windows: $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe -v ./cmd/cclogviewer

# Create release archives
release: build-all
	mkdir -p dist
	tar -czf dist/$(BINARY_NAME)-linux-amd64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-linux-amd64 -C .. README.md
	tar -czf dist/$(BINARY_NAME)-linux-arm64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-linux-arm64 -C .. README.md
	tar -czf dist/$(BINARY_NAME)-darwin-amd64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-darwin-amd64 -C .. README.md
	tar -czf dist/$(BINARY_NAME)-darwin-arm64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-darwin-arm64 -C .. README.md
	cd $(BUILD_DIR) && zip ../dist/$(BINARY_NAME)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe && cd .. && zip -j dist/$(BINARY_NAME)-windows-amd64.zip README.md

# Show help
help:
	@echo "Available targets:"
	@echo "  make build          - Build the binary"
	@echo "  make build-mcp      - Build the MCP server"
	@echo "  make build-all-binaries - Build all binaries"
	@echo "  make install        - Install binary to Go bin directory ($(INSTALL_DIR))"
	@echo "  make install-mcp    - Install MCP server to Go bin directory"
	@echo "  make install-all    - Install all binaries"
	@echo "  make uninstall      - Remove binaries from Go bin directory"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make deps           - Download and tidy dependencies"
	@echo "  make fmt            - Format Go code"
	@echo "  make lint           - Run linter (requires golangci-lint)"
	@echo "  make test           - Run unit tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-all       - Run all tests"
	@echo "  make benchmark      - Run benchmarks"
	@echo "  make build-all      - Build for all platforms"
	@echo "  make release        - Create release archives"
	@echo ""
	@echo "Installation directory is determined by:"
	@echo "  - GOBIN if set, otherwise"
	@echo "  - GOPATH/bin (currently: $(INSTALL_DIR))"

.PHONY: build build-mcp build-all-binaries build-release install install-mcp install-all uninstall clean deps fmt lint test test-coverage test-coverage-report test-integration benchmark test-all build-all build-linux build-darwin build-windows release help