# Build the legacy (v1/v2) API lambda binary
# Usage:
#   make lambdas
#   STAGE=dev AWS_REGION=us-east-1 yarn sls deploy -s ${STAGE} -r ${AWS_REGION}

BIN_DIR := bin
GOFLAGS ?= -mod=mod
LEGACY_BIN := $(BIN_DIR)/legacy-api-lambda
LOCAL_BIN := $(BIN_DIR)/legacy-api-local

.PHONY: lambdas local run-local clean lint

lambdas: $(LEGACY_BIN)

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
	@echo "Running golangci-lint with legacy compatibility rules..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, using go vet only"; \
		echo "Install golangci-lint for full linting: https://golangci-lint.run/usage/install/"; \
	fi
	@echo "✅ Lint completed successfully"

clean:
	rm -rf $(BIN_DIR)
