SHELL := /bin/bash

# Five VS Code target tuples we ship .vsix for in v0.1.
VSIX_TARGETS := linux-x64 linux-arm64 darwin-x64 darwin-arm64 win32-x64

.PHONY: build sidecar extension test test-sidecar test-extension vsix vsix-all clean install-deps

# --- Local dev ---

# Build everything for development on the host platform.
build: sidecar extension

# Build the sidecar for the host platform only (fast iteration).
sidecar:
	cd sidecar && go build -o dist/muninn-sidecar ./cmd/muninn-sidecar

# Compile the extension TypeScript.
extension:
	cd extension && npm install --no-audit --no-fund && npm run compile

# --- Tests ---

test: test-sidecar test-extension

test-sidecar:
	cd sidecar && go vet ./... && go test ./... -race -count=1

test-extension:
	cd extension && npm run typecheck

# --- Cross-compile + package ---

# Package one .vsix for the host platform (defaults to linux-x64; override
# with TARGET=...).
TARGET ?= linux-x64
vsix:
	./scripts/package-vsix.sh $(TARGET)

# Package every supported platform. Output lands in dist/vsix/.
vsix-all:
	@for t in $(VSIX_TARGETS); do \
	  ./scripts/package-vsix.sh $$t || exit $$?; \
	done
	@echo
	@echo 'all targets packaged:'
	@ls -lh dist/vsix/

# --- Cleanup ---

clean:
	rm -rf sidecar/dist
	rm -rf extension/bin
	rm -rf extension/out
	rm -rf dist
