version := "0.2.0-dev"

default:
    @just --list

# Build and install to ~/.local/bin
install:
    go build -ldflags "-X main.version={{version}}" -o ~/.local/bin/society ./cmd/society

# Unit tests (no network, no external deps)
test:
    go test ./...

# Integration tests (starts local HTTP/Docker agents, needs Docker socket)
test-integration:
    go test -tags integration -v -run TestHTTP ./...
    go test -tags integration -v -run TestDocker ./...
    go test -tags integration -v -run TestSTDIO ./...

# Test against live Claude CLI (needs claude installed)
test-claude:
    go test -tags claude -v ./...

# Test Tailscale SSH transport (needs Tailscale + arch reachable)
test-tailscale:
    go test -tags tailscale -v ./...

# Format, vet, test, build
check:
    gofmt -w .
    go vet ./...
    go test ./...
    go build -ldflags "-X main.version={{version}}" -o /dev/null ./cmd/society

# Start docs dev server
docs:
    cd docs && bun dev

# Install, clear registry, run onboard (for testing scan/group logic)
try-onboard: install
    @for a in $(society list 2>/dev/null | awk 'NR>3{print $$1}'); do \
        society remove "$$a" <<< "y" 2>/dev/null; \
    done
    society onboard
