BINARY     := bifu-cli
BINARY_DIR := bin
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-X bifu-cli/cmd.version=$(VERSION) -s -w"

.PHONY: all build install clean tidy lint test help

## build: Compile bifu-cli binary to ./bin/
build:
	@mkdir -p $(BINARY_DIR)
	go build $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY) .

## install: Install bifu-cli to GOPATH/bin
install:
	go install $(LDFLAGS) .

## tidy: Run go mod tidy
tidy:
	go mod tidy

## clean: Remove compiled binaries
clean:
	rm -rf $(BINARY_DIR)

## test: Run unit tests
test:
	go test ./... -v

## lint: Run staticcheck linter (requires: go install honnef.co/go/tools/cmd/staticcheck@latest)
lint:
	staticcheck ./...

## help: Show this help
help:
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
