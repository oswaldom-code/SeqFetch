BINARY  := seqfetch
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build install test lint snapshot clean trivy

build: ## Build a stripped binary into ./bin
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

install: ## Install a stripped binary into $(go env GOPATH)/bin
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/$(BINARY)

test: ## Run all tests with the race detector
	go test -race ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

snapshot: ## Cross-compile all release targets locally into ./dist
	goreleaser build --snapshot --clean

trivy: ## Scan dependencies for vulnerabilities using Trivy and applying the local ignore policy
	@if command -v trivy >/dev/null 2>&1; then \
        trivy fs --ignore-policy trivy-filter.rego go.mod; \
	elif [ -f ~/.local/bin/trivy ]; then \
        ~/.local/bin/trivy fs --ignore-policy trivy-filter.rego go.mod; \
	else \
        echo "Error: Trivy is not installed. Please install it first."; \
        echo "You can run: curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b ~/.local/bin"; \
        exit 1; \
	fi


clean:
	rm -rf bin dist coverage.out
