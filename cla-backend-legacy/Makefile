# Build the legacy (v1/v2) API lambda binary consumed by cla-backend/serverless.yml
# Usage:
#   make lambdas
#   cp ./bin/legacy-api-lambda ../cla-backend/bin/

BIN_DIR := bin
GOFLAGS ?= -mod=mod
LEGACY_BIN := $(BIN_DIR)/legacy-api-lambda
LOCAL_BIN := $(BIN_DIR)/legacy-api-local

.PHONY: lambda lambdas local run-local clean lint

lambda: $(LEGACY_BIN)

lambdas: lambda

local: $(LOCAL_BIN)

$(LEGACY_BIN):
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -trimpath -ldflags="-s -w" -o $(LEGACY_BIN) ./cmd/legacy-api

$(LOCAL_BIN):
	@mkdir -p $(BIN_DIR)
	go build $(GOFLAGS) -trimpath -ldflags="-s -w" -o $(LOCAL_BIN) ./cmd/legacy-api-local

run-local:
	go run $(GOFLAGS) ./cmd/legacy-api-local

lint:
	go fmt ./...
	go vet ./...
	@echo "Running comprehensive Go analysis..."
	@echo "Note: golangci-lint configured for CI with Go 1.25"
	@echo "Local: go fmt + go vet completed - CI will run full golangci-lint"
	@echo "✅ Lint completed successfully"

clean:
	rm -rf $(BIN_DIR)

