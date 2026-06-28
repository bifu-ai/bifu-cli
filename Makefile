BINARY     := bifu-cli
BINARY_DIR := bin
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-X bifu-cli/cmd.version=$(VERSION) -s -w"

PLUGIN_SKILLS := plugins/bifu/skills

.PHONY: all build install clean tidy lint test help plugins-sync mcpb

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

## plugins-sync: Regenerate the plugin's skills/ from the canonical skills/
plugins-sync:
	@rm -rf $(PLUGIN_SKILLS)
	@mkdir -p $(PLUGIN_SKILLS)
	@for d in skills/*/; do \
		name=$$(basename $$d); \
		mkdir -p $(PLUGIN_SKILLS)/$$name; \
		cp $$d/SKILL.md $(PLUGIN_SKILLS)/$$name/SKILL.md; \
	done
	@echo "Synced $$(ls -1 $(PLUGIN_SKILLS) | wc -l | tr -d ' ') skills into $(PLUGIN_SKILLS)"

## mcpb: Build Claude Desktop .mcpb extensions (all platforms) into dist/mcpb/
mcpb:
	@scripts/build-mcpb.sh "$(VERSION)"

## help: Show this help
help:
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
