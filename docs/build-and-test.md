# Build & Test Commands

```bash
# Build CLI
go build ./cmd/alogin

# Build with race detector
go build -race ./cmd/alogin

# Run all tests
go test ./...

# Build + run
go run ./cmd/alogin connect

# Cross-compile
GOOS=linux  GOARCH=amd64 go build -o alogin-linux-amd64  ./cmd/alogin
GOOS=darwin GOARCH=arm64 go build -o alogin-darwin-arm64 ./cmd/alogin

# Build Web UI frontend (required once before `go build -tags web`)
cd web/frontend && npm install && npm run build

# Build with embedded Web UI
go build -tags web -o alogin ./cmd/alogin

# Lint
go vet ./...
```
