BINARY   := logtailr
BUILD_DIR := bin
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X logtailr/cmd.version=$(VERSION)

.PHONY: build test vet lint clean run help

## build: Compile the binary into bin/
build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) .

## test: Run all tests with race detector
test:
	go test -race -timeout 30s ./...

## vet: Run go vet
vet:
	go vet ./...

## lint: Run govulncheck (install with: go install golang.org/x/vuln/cmd/govulncheck@latest)
lint: vet
	@command -v govulncheck >/dev/null 2>&1 && govulncheck ./... || echo "govulncheck not installed, skipping"

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

## run: Build and run with default flags
run: build
	./$(BUILD_DIR)/$(BINARY)

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
