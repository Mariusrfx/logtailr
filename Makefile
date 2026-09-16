BINARY   := logtailr
BUILD_DIR := bin
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X logtailr/cmd.version=$(VERSION)

.PHONY: build build-web test vet lint fmt fmt-check coverage docker ci clean run help

## build-web: Build the frontend and copy to internal/web/dist/
build-web:
	cd web && npm install && npm run build
	rm -rf internal/web/dist
	cp -r web/dist internal/web/dist

## build: Compile the binary into bin/ (run build-web first for dashboard)
build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) .

## build-all: Build frontend + Go binary
build-all: build-web build

## test: Run all tests with race detector
test:
	go test -race -timeout 30s ./...

## vet: Run go vet
vet:
	go vet ./...

## lint: Run govulncheck (install with: go install golang.org/x/vuln/cmd/govulncheck@latest)
lint: vet
	@command -v govulncheck >/dev/null 2>&1 && govulncheck ./... || echo "govulncheck not installed, skipping"

## fmt: Format Go and frontend code
fmt:
	go fmt ./...
	cd web && npm run lint -- --fix

## fmt-check: Fail if code is not formatted (used by ci)
fmt-check:
	@files=$$(gofmt -l .); if [ -n "$$files" ]; then echo "gofmt needed for:"; echo "$$files"; exit 1; fi

## coverage: Generate test coverage report (HTML at coverage.html)
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## docker: Build the Docker image
docker:
	@test -f Dockerfile || { echo "error: Dockerfile not found (planned in wave 6)"; exit 1; }
	docker build -t logtailr:$(VERSION) .

## ci: Full CI pipeline (fmt check + vet + lint + test)
ci: fmt-check vet lint test

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

## run: Build and run with default flags
run: build
	./$(BUILD_DIR)/$(BINARY)

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
