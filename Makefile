BIN      := alogin
CMD      := ./cmd/alogin
INSTALL  := ~/.local/bin/$(BIN)
FRONTEND := web/frontend
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo "dev")
LDFLAGS  := -ldflags "-X github.com/emusal/alogin2/internal/cli.Version=$(VERSION) -s -w"

# ── default ──────────────────────────────────────────────────────────────────

.DEFAULT_GOAL := build

# ── build ─────────────────────────────────────────────────────────────────────

.PHONY: build
build:                          ## Build CLI binary (no web UI embed)
	go build $(LDFLAGS) -o $(BIN) $(CMD)

.PHONY: build-web
build-web: frontend             ## Build CLI binary with embedded Web UI
	go build $(LDFLAGS) -tags web -o $(BIN) $(CMD)

.PHONY: build-race
build-race:                     ## Build with race detector
	go build -race -o $(BIN) $(CMD)

# ── install ───────────────────────────────────────────────────────────────────

.PHONY: install
install: build-web              ## Install CLI with embedded Web UI to $(INSTALL)
	mkdir -p $(dir $(INSTALL))
	cp $(BIN) $(INSTALL)

.PHONY: install-no-web
install-no-web: build           ## Install CLI without Web UI to $(INSTALL)
	mkdir -p $(dir $(INSTALL))
	cp $(BIN) $(INSTALL)

# ── frontend ──────────────────────────────────────────────────────────────────

.PHONY: frontend
frontend: $(FRONTEND)/node_modules  ## Build React frontend (runs npm install if needed)
	cd $(FRONTEND) && npm run build

.PHONY: frontend-dev
frontend-dev: $(FRONTEND)/node_modules  ## Start Vite dev server
	cd $(FRONTEND) && npm run dev

$(FRONTEND)/node_modules: $(FRONTEND)/package.json
	cd $(FRONTEND) && npm install
	@touch $(FRONTEND)/node_modules

# ── run ───────────────────────────────────────────────────────────────────────

.PHONY: run
run: build                      ## Build and run (pass args via ARGS=)
	./$(BIN) $(ARGS)

.PHONY: run-web
run-web: build-web              ## Build with web UI and start web server
	./$(BIN) web

# ── test / lint ───────────────────────────────────────────────────────────────

.PHONY: test
test:                           ## Run all tests
	go test ./...

.PHONY: test-v
test-v:                         ## Run all tests (verbose)
	go test -v ./...

.PHONY: vet
vet:                            ## Run go vet
	go vet ./...

.PHONY: lint
lint: vet                       ## Run vet + basic checks
	@test -z "$$(gofmt -l .)" || (echo "gofmt issues in:"; gofmt -l .; exit 1)

# ── cross-compile ─────────────────────────────────────────────────────────────

.PHONY: dist
dist:                           ## Cross-compile for all release targets
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)-darwin-arm64  $(CMD)
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)-darwin-amd64  $(CMD)
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)-linux-amd64   $(CMD)
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)-linux-arm64   $(CMD)

.PHONY: dist-web
dist-web: frontend              ## Cross-compile all platforms with embedded Web UI
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -tags web -o $(BIN)-web-darwin-arm64 $(CMD)
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -tags web -o $(BIN)-web-darwin-amd64 $(CMD)
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -tags web -o $(BIN)-web-linux-amd64  $(CMD)
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -tags web -o $(BIN)-web-linux-arm64  $(CMD)

.PHONY: checksums
checksums:                      ## Generate SHA256 checksums for release binaries
	shasum -a 256 $(BIN)-* > checksums.txt

# ── clean ─────────────────────────────────────────────────────────────────────

.PHONY: clean
clean:                          ## Remove built binaries
	rm -f $(BIN) $(BIN)-darwin-arm64 $(BIN)-darwin-amd64 $(BIN)-linux-amd64 $(BIN)-linux-arm64 $(BIN)-web-darwin-arm64 $(BIN)-web-darwin-amd64 $(BIN)-web-linux-amd64 $(BIN)-web-linux-arm64 checksums.txt

.PHONY: clean-all
clean-all: clean                ## Remove binaries + frontend build artifacts
	rm -rf $(FRONTEND)/dist

# ── help ──────────────────────────────────────────────────────────────────────

.PHONY: help
help:                           ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) \
	  | awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
