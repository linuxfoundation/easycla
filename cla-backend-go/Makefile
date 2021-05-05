# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
SERVICE = cla
SHELL = bash
LAMBDA_BIN = backend-aws-lambda
METRICS_BIN = metrics-aws-lambda
METRICS_REPORT_BIN = metrics-report-lambda
DYNAMO_EVENTS_BIN = dynamo-events-lambda
ZIPBUILDER_SCHEDULER_BIN = zipbuilder-scheduler-lambda
ZIPBUILDER_BIN = zipbuilder-lambda
FUNCTIONAL_TESTS_BIN = functional-tests
USER_SUBSCRIBE_BIN = user-subscribe-lambda
MAKEFILE_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BUILD_TIME=$(shell sh -c 'date -u +%FT%T%z')
VERSION := $(shell sh -c 'git describe --always --tags')
BRANCH := $(shell sh -c 'git rev-parse --abbrev-ref HEAD')
COMMIT := $(shell sh -c 'git rev-parse --short HEAD')
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.branch=$(BRANCH) -X main.buildDate=$(BUILD_TIME)"
BUILD_TAGS=-tags aws_lambda

LINT_TOOL=$(shell go env GOPATH)/bin/golangci-lint
LINT_VERSION=v1.37.0
SWAGGER_TOOL_VERSION=v0.24.0
GO_PKGS=$(shell go list ./... | grep -v /vendor/ | grep -v /node_modules/)
GO_FILES=$(shell find . -type f -name '*.go' -not -path './vendor/*')

.PHONY: generate setup tool-setup setup-dev setup-deploy clean-all clean swagger up fmt test run deps build build-mac build-aws-lambda user-subscribe-lambda qc lint

all: all-mac
all-mac: clean swagger deps fmt build-mac build-aws-lambda-mac build-user-subscribe-lambda-mac build-metrics-lambda-mac build-dynamo-events-lambda-mac build-zipbuilder-scheduler-lambda-mac build-zipbuilder-lambda-mac test lint
all-linux: clean swagger deps fmt build-linux build-aws-lambda-linux build-user-subscribe-lambda-linux build-metrics-lambda-linux build-dynamo-events-lambda-linux build-zipbuilder-scheduler-lambda-linux build-zipbuilder-lambda-linux test lint
build-lambdas-mac: build-aws-lambda-mac build-user-subscribe-lambda-mac build-metrics-lambda-mac build-metrics-report-lambda-mac build-dynamo-events-lambda-mac build-zipbuilder-scheduler-lambda-mac build-zipbuilder-lambda-mac
build-lambdas-linux: build-aws-lambda-linux build-user-subscribe-lambda-linux build-metrics-lambda-linux build-metrics-report-lambda-linux build-dynamo-events-lambda-linux build-zipbuilder-scheduler-lambda-linux build-zipbuilder-lambda-linux

generate: swagger

setup: $(LINT_TOOL) setup-dev setup-deploy

tool-setup:
	@echo "Installing gobin for installing tools..."
	@# gobin is the equivalent of 'go get' whilst in module-aware mode but this does not modify your go.mod
	GO111MODULE=off go get -u github.com/myitcv/gobin

setup_dev: setup-dev
setup-dev: tool-setup
	@echo "Removing previously install version of swagger..."
	@rm -Rf $(shell echo $(GOPATH))/bin/swagger $(shell echo $(GOPATH))/src/github.com/go-swagger
	@echo "Installing swagger version: '$(SWAGGER_TOOL_VERSION)'..."
	gobin github.com/go-swagger/go-swagger/cmd/swagger@$(SWAGGER_TOOL_VERSION)
	@echo "Installing goimports..."
	gobin golang.org/x/tools/cmd/goimports
	@echo "Installing cover..."
	gobin golang.org/x/tools/cmd/cover
	@echo "Installing multi-file-swagger tool..."
	@echo "Downloading golangci-lint version $(LINT_VERSION)..."
	@# Latest releases: https://github.com/golangci/golangci-lint/releases
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(shell go env GOPATH)/bin $(LINT_VERSION)
	cd $(dir $(realpath $(firstword $(MAKEFILE_LIST))))swagger && pip3 install virtualenv && virtualenv .venv && source .venv/bin/activate && pip3 install -r requirements.txt

setup_deploy: setup-deploy
setup-deploy:
	@yarn add serverless && yarn install

clean:
	@rm -rf cla cla-mac cla-linux \
		./v2/project-service/client ./v2/project-service/models \
		./v2/organization-service/client ./v2/organization-service/models \
		./v2/user-service/client ./v2/user-service/models \
		backend-aws-lambda* dynamo-events-lambda* \
		functional-tests* metrics-aws-lambda* metrics-report-lambda* \
		user-subscribe-lambda* zipbuild-lambda* zipbuilder-scheduler-lambda*

clean-swagger:
	@rm -rf gen/

clean-all: clean clean-swagger
	@rm -rf vendor/

swagger: clean-swagger swagger-build swagger-validate

swagger-prep:
	@mkdir gen

swagger-build-v1-services:
	@echo
	@echo "Generating v1 legacy API models..."
	cd swagger; source .venv/bin/activate && python3 multi-file-swagger.py --spec-input-file cla.v1.yaml --spec-output-file cla.v1.compiled.yaml
	swagger -q generate server \
		-t gen \
		-f swagger/cla.v1.compiled.yaml \
		--copyright-file=copyright-header.txt \
		--exclude-main \
		-A cla \
		-P user.CLAUser

swagger-build-v2-services:
	@echo
	@echo "Generating v2 API models..."
	cd swagger; source .venv/bin/activate && python3 multi-file-swagger.py --spec-input-file cla.v2.yaml --spec-output-file cla.v2.compiled.yaml
	swagger -q generate server \
		-t gen \
		-f swagger/cla.v2.compiled.yaml \
		--copyright-file=copyright-header.txt \
		--server-package=v2/restapi \
		--model-package=v2/models \
		--exclude-main \
		-A easycla \
		-P auth.User

swagger-build-project-service:
	@echo
	@echo "Generating swagger client for the project-service..."
	@mkdir -p v2/project-service
	curl -sfL https://api-gw.dev.platform.linuxfoundation.org/project-service/swagger.json --output swagger/project-service.yaml
	sed -i.bak 's/X-ACL/Empty-Header/g' swagger/project-service.yaml
	swagger -q generate client \
		--copyright-file=copyright-header.txt \
		-t v2/project-service \
		-f swagger/project-service.yaml

swagger-build-organization-service:
	@echo
	@echo "Generating swagger client for the organization-service..."
	@mkdir -p v2/organization-service
	curl -sfL https://api-gw.dev.platform.linuxfoundation.org/organization-service/swagger.json --output swagger/organization-service.yaml
	sed -i.bak 's/X-ACL/Empty-Header/g' swagger/organization-service.yaml
	swagger -q generate client \
		--copyright-file=copyright-header.txt \
		-t v2/organization-service \
		-f swagger/organization-service.yaml

swagger-build-user-service:
	@echo
	@echo "Generating swagger client for the user-service..."
	@mkdir -p v2/user-service
	curl -sfL https://api-gw.dev.platform.linuxfoundation.org/user-service/swagger.json --output swagger/user-service.yaml
	sed -i.bak 's/X-ACL/Empty-Header/g' swagger/user-service.yaml
	swagger -q generate client \
		--copyright-file=copyright-header.txt \
		-t v2/user-service \
		-f swagger/user-service.yaml

swagger-build-acs-service:
	@echo
	@echo "Generating swagger client for the acs-service..."
	@mkdir -p v2/acs-service
	curl -sfL https://api-gw.dev.platform.linuxfoundation.org/acs/v1/api-docs/swagger/swagger.json --output swagger/acs-service.yaml
	sed -i.bak 's/X-ACL/X-API-KEY/g' swagger/acs-service.yaml
	swagger -q generate client \
		--copyright-file=copyright-header.txt \
		-t v2/acs-service \
		-f swagger/acs-service.yaml

build-swagger: swagger-build
swagger-build: clean-swagger swagger-prep swagger-build-v1-services swagger-build-v2-services swagger-build-project-service swagger-build-organization-service swagger-build-user-service swagger-build-acs-service

swagger-validate: swagger-v1-validate swagger-v2-validate

swagger-v1-validate:
	@echo ""
	@echo "Validating EasyCLA v1 legacy API specification..."
	@swagger validate --stop-on-error swagger/cla.v1.compiled.yaml

swagger-v2-validate:
	@echo ""
	@echo "Validating EasyCLA v2 API specification..."
	@swagger validate --stop-on-error swagger/cla.v2.compiled.yaml

fmt:
	@echo "Formatting code and optimizing imports..."
	@gofmt -w -l -s $(GO_FILES)
	@goimports -w -l $(GO_FILES)

test:
	@echo "Running unit tests..."
	@go test -v $(shell go list ./... | grep -v /vendor/ | grep -v /node_modules/) -coverprofile=cover.out

mock:
	@echo "Re-Generating mocks"
	@cd $(MAKEFILE_DIR) && mkdir -p repositories/mock
	@cd $(MAKEFILE_DIR) && mockgen -copyright_file=copyright-header.txt -source=repositories/service.go -package=mock -destination=repositories/mock/mock_service.go
	@cd $(MAKEFILE_DIR) && mockgen -copyright_file=copyright-header.txt -source=repositories/repository.go -package=mock -destination=repositories/mock/mock_repository.go
    # github organizations
	@cd $(MAKEFILE_DIR) && mockgen -copyright_file=copyright-header.txt -package=github_organizations -destination=github_organizations/mock.go -self_package=github.com/communitybridge/easycla/cla-backend-go/github_organizations github.com/communitybridge/easycla/cla-backend-go/github_organizations Repository
    # events
	@cd $(MAKEFILE_DIR) && mockgen -copyright_file=copyright-header.txt -package=events -destination=events/mock.go -self_package=github.com/communitybridge/easycla/cla-backend-go/events github.com/communitybridge/easycla/cla-backend-go/events Service
	# mocks for github
	@cd $(MAKEFILE_DIR) && mockgen -copyright_file=copyright-header.txt -package=branch_protection -destination=github/branch_protection/mock.go -self_package=github.com/communitybridge/easycla/cla-backend-go/github/branch_protection github.com/communitybridge/easycla/cla-backend-go/github/branch_protection CombinedRepository


run:
	go run main.go

deps:
	go env -w GOPRIVATE=github.com/LF-Engineering/*
	go mod download

build: build-linux
build-linux: deps
	@echo "Building Linux amd64 binary..."
	env GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(SERVICE) main.go
	@chmod +x $(SERVICE)

build-mac: deps
	@echo "Building Mac OSX amd64 binary..."
	env GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(SERVICE)-mac main.go
	@chmod +x $(SERVICE)-mac

rebuild-mac: clean fmt build-mac lint
	./$(SERVICE)-mac

build-aws-lambda: build-aws-lambda-linux
build-aws-lambda-linux: deps
	@echo "Building a statically linked Linux amd64 binary..."
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) $(BUILD_TAGS) -o $(LAMBDA_BIN) main.go
	@chmod +x $(LAMBDA_BIN)

build-aws-lambda-mac: deps
	@echo "Building a statically linked Mac OSX amd64 binary..."
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) $(BUILD_TAGS) -o $(LAMBDA_BIN)-mac main.go
	@chmod +x $(LAMBDA_BIN)-mac

build-user-subscribe-lambda: build-user-subscribe-lambda-linux
build-user-subscribe-lambda-linux: deps
	@echo "Building a statically linked Linux amd64 binary..."
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) $(BUILD_TAGS) -o $(USER_SUBSCRIBE_BIN) userSubscribeLambda/main.go
	@chmod +x $(USER_SUBSCRIBE_BIN)

build-user-subscribe-lambda-mac: deps
	@echo "Building a statically linked Mac OSX amd64 binary..."
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) $(BUILD_TAGS) -o $(USER_SUBSCRIBE_BIN)-mac userSubscribeLambda/main.go
	@chmod +x $(USER_SUBSCRIBE_BIN)-mac

build-metrics-lambda: build-metrics-lambda-linux
build-metrics-lambda-linux: deps
	@echo "Building a statically linked Linux amd64 binary..."
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(METRICS_BIN) cmd/metrics_lambda/main.go
	@chmod +x $(METRICS_BIN)

build-metrics-lambda-mac: deps
	@echo "Building a statically linked Mac OSX amd64 binary..."
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(METRICS_BIN)-mac cmd/metrics_lambda/main.go
	@chmod +x $(METRICS_BIN)-mac

build-metrics-report-lambda: build-metrics-report-lambda-linux
build-metrics-report-lambda-linux: deps
	@echo "Building a statically linked Linux amd64 binary..."
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(METRICS_REPORT_BIN) cmd/metrics_report_lambda/main.go
	@chmod +x $(METRICS_REPORT_BIN)

build-metrics-report-lambda-mac: deps
	@echo "Building a statically linked Mac OSX amd64 binary..."
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(METRICS_REPORT_BIN)-mac cmd/metrics_report_lambda/main.go
	@chmod +x $(METRICS_REPORT_BIN)-mac


build-dynamo-events-lambda: build-dynamo-events-lambda-linux
build-dynamo-events-lambda-linux: deps
	@echo "Building a statically linked Linux amd64 binary..."
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DYNAMO_EVENTS_BIN) cmd/dynamo_events_lambda/main.go
	@chmod +x $(DYNAMO_EVENTS_BIN)

build-dynamo-events-lambda-mac: deps
	@echo "Building a statically linked Mac OSX amd64 binary..."
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DYNAMO_EVENTS_BIN)-mac cmd/dynamo_events_lambda/main.go
	@chmod +x $(DYNAMO_EVENTS_BIN)-mac

build-zipbuilder-scheduler-lambda: build-zipbuilder-scheduler-lambda-linux
build-zipbuilder-scheduler-lambda-linux: deps
	@echo "Building a statically linked Linux amd64 binary..."
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(ZIPBUILDER_SCHEDULER_BIN) cmd/zipbuilder_scheduler_lambda/main.go
	@chmod +x $(ZIPBUILDER_SCHEDULER_BIN)

build-zipbuilder-scheduler-lambda-mac: deps
	@echo "Building a statically linked Mac OSX amd64 binary..."
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(ZIPBUILDER_SCHEDULER_BIN)-mac cmd/zipbuilder_scheduler_lambda/main.go
	@chmod +x $(ZIPBUILDER_SCHEDULER_BIN)-mac

build-zipbuilder-lambda: build-zipbuilder-lambda-linux
build-zipbuilder-lambda-linux: deps
	@echo "Building a statically linked Linux amd64 binary..."
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(ZIPBUILDER_BIN) cmd/zipbuilder_lambda/main.go
	@chmod +x $(ZIPBUILDER_BIN)

build-zipbuilder-lambda-mac: deps
	@echo "Building a statically linked Mac OSX amd64 binary..."
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(ZIPBUILDER_BIN)-mac cmd/zipbuilder_lambda/main.go
	@chmod +x $(ZIPBUILDER_BIN)-mac

build-functional-tests: build-functional-tests-linux
build-functional-tests-linux: deps
	@echo "Building Functional Tests for Linux amd64 binary..."
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(FUNCTIONAL_TESTS_BIN) cmd/functional_tests/main.go
	@chmod +x $(FUNCTIONAL_TESTS_BIN)

build-functional-tests-mac: deps
	@echo "Building Functional Tests for OSX amd64 binary..."
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(FUNCTIONAL_TESTS_BIN)-mac cmd/functional_tests/main.go
	@chmod +x $(FUNCTIONAL_TESTS_BIN)-mac

lint:
	@cd $(MAKEFILE_DIR) && echo "Running lint..." && $(LINT_TOOL) --version && $(LINT_TOOL) run --exclude="this method will not auto-escape HTML. Verify data is well formed" --allow-parallel-runners --config=.golangci.yaml ./... && echo "Lint check passed."
	@cd $(MAKEFILE_DIR) && ./check-headers.sh

