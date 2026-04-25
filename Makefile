BIN        := alogin
CMD        := ./cmd/alogin
INSTALL    := ~/.local/bin/$(BIN)
SKILLS_SRC := skills
SKILLS_DST := ~/.agents/skills
FRONTEND   := web/frontend
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
install: build-web install-skills  ## Install CLI with embedded Web UI to $(INSTALL) and skills to $(SKILLS_DST)
	mkdir -p $(dir $(INSTALL))
	cp $(BIN) $(INSTALL)

.PHONY: install-no-web
install-no-web: build install-skills  ## Install CLI without Web UI to $(INSTALL) and skills to $(SKILLS_DST)
	mkdir -p $(dir $(INSTALL))
	cp $(BIN) $(INSTALL)

.PHONY: install-skills
install-skills:                 ## Install skills to $(SKILLS_DST)
	@for skill in $(SKILLS_SRC)/*/; do \
	  name=$$(basename "$$skill"); \
	  mkdir -p $(SKILLS_DST)/$$name; \
	  cp "$$skill/SKILL.md" $(SKILLS_DST)/$$name/SKILL.md; \
	  echo "  installed skill: $$name → $(SKILLS_DST)/$$name/SKILL.md"; \
	done

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
dist: dist-skills               ## Cross-compile for all release targets
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)-darwin-arm64       $(CMD)
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)-darwin-amd64       $(CMD)
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)-linux-amd64        $(CMD)
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)-linux-arm64        $(CMD)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)-windows-amd64.exe  $(CMD)
	GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)-windows-arm64.exe  $(CMD)

.PHONY: dist-web
dist-web: frontend dist-skills  ## Cross-compile all platforms with embedded Web UI
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -tags web -o $(BIN)-web-darwin-arm64      $(CMD)
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -tags web -o $(BIN)-web-darwin-amd64      $(CMD)
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -tags web -o $(BIN)-web-linux-amd64       $(CMD)
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -tags web -o $(BIN)-web-linux-arm64       $(CMD)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -tags web -o $(BIN)-web-windows-amd64.exe $(CMD)
	tar -czf plugins.tar.gz -C testenv plugins

.PHONY: dist-skills
dist-skills:                    ## Package skills into skills.tar.gz for release
	tar -czf skills.tar.gz -C $(SKILLS_SRC) .

.PHONY: checksums
checksums:                      ## Generate SHA256 checksums for release binaries
	shasum -a 256 $(BIN)-* alogin_* plugins.tar.gz skills.tar.gz > checksums.txt

# ── installers ────────────────────────────────────────────────────────────────

.PHONY: pkg
pkg:                            ## Build macOS .pkg installer for current arch (requires macOS + dist-web first)
	$(eval ARCH := $(shell uname -m | sed 's/x86_64/amd64/'))
	mkdir -p pkg-root/usr/local/bin
	cp $(BIN)-web-darwin-$(ARCH) pkg-root/usr/local/bin/alogin
	chmod +x pkg-root/usr/local/bin/alogin
	pkgbuild \
		--root pkg-root \
		--identifier com.emusal.alogin \
		--version "$(VERSION)" \
		--install-location / \
		alogin-$(VERSION)-darwin-$(ARCH).pkg
	rm -rf pkg-root
	@echo "Built: alogin-$(VERSION)-darwin-$(ARCH).pkg"

.PHONY: deb
deb:                            ## Build Linux .deb package for amd64 (requires nfpm + dist-web first)
	@command -v nfpm >/dev/null 2>&1 || (echo "nfpm not found. Install: https://nfpm.goreleaser.com/install/" && exit 1)
	cp $(BIN)-web-linux-amd64 alogin-linux-current
	ARCH=amd64 VERSION=$(VERSION) nfpm package --config nfpm.yaml --packager deb --target alogin_$(VERSION)_amd64.deb
	rm -f alogin-linux-current
	@echo "Built: alogin_$(VERSION)_amd64.deb"

.PHONY: msi
msi:                            ## Build Windows MSI installer for amd64 (requires go-msi + WiX, run on Windows)
	@command -v go-msi >/dev/null 2>&1 || (echo "go-msi not found. Install: go install github.com/mh-cbon/go-msi@latest" && exit 1)
	cp $(BIN)-web-windows-amd64.exe alogin.exe
	go-msi make --msi alogin-$(VERSION)-windows-amd64.msi --version $(VERSION) --src wix.json
	rm -f alogin.exe
	@echo "Built: alogin-$(VERSION)-windows-amd64.msi"

# ── release ───────────────────────────────────────────────────────────────────

.PHONY: release
release:                        ## Tag and push a release: make release TAG=2.5.3
ifndef TAG
	$(error TAG is required — usage: make release TAG=2.5.3)
endif
	@if git ls-remote --tags origin | grep -q "refs/tags/v$(TAG)$$"; then \
	  echo "Error: tag v$(TAG) already exists on remote"; exit 1; \
	fi
	@if [ -n "$$(git status --porcelain)" ]; then \
	  echo "Error: working tree is dirty — commit or stash changes first"; exit 1; \
	fi
	git tag v$(TAG)
	git push origin v$(TAG)
	@echo "Tagged and pushed v$(TAG) — watch CI at https://github.com/emusal/alogin2/actions"

# ── clean ─────────────────────────────────────────────────────────────────────

.PHONY: clean
clean:                          ## Remove built binaries and installer packages
	rm -f $(BIN) $(BIN)-darwin-arm64 $(BIN)-darwin-amd64 $(BIN)-linux-amd64 $(BIN)-linux-arm64 $(BIN)-windows-amd64.exe $(BIN)-windows-arm64.exe $(BIN)-web-darwin-arm64 $(BIN)-web-darwin-amd64 $(BIN)-web-linux-amd64 $(BIN)-web-linux-arm64 $(BIN)-web-windows-amd64.exe checksums.txt plugins.tar.gz skills.tar.gz
	rm -f alogin-linux-current alogin.exe alogin-*.pkg alogin-*.msi alogin_*.deb
	rm -rf pkg-root

.PHONY: clean-all
clean-all: clean                ## Remove binaries + frontend build artifacts
	rm -rf $(FRONTEND)/dist

# ── help ──────────────────────────────────────────────────────────────────────

.PHONY: help
help:                           ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) \
	  | awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
