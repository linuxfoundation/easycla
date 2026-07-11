# CLAUDE.md

Copyright The Linux Foundation and each contributor to CommunityBridge.

SPDX-License-Identifier: CC-BY-4.0

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

EasyCLA is the Linux Foundation's Contributor License Agreement service. It lets contributors sign ICLAs/CCLAs and gates GitHub PRs and Gerrit/GitLab reviews on CLA authorization. Third-party integrations: DocuSign (e-sign), DocRaptor (PDF), GitHub, Gerrit, GitLab, Auth0 (SSO), and Salesforce via the LFX Platform APIs.

The frontend consoles (Project/Corporate/Contributor) live in **separate repositories** — this repo is backend-only plus supporting scripts/infra. Do not look for UI code here.

## Repository Layout

- `cla-backend-go/` — **primary backend**, Go. Powers `/v3` (EasyCLA v1, us-east-1) and `/v4` (EasyCLA v2, us-east-2, integrates with LFX Platform + Salesforce). Deployed as AWS Lambdas.
- `cla-backend/` — Serverless Framework **deployment stack** (us-east-1). No application code of its own (the Python backend has been removed): it deploys Go binaries built elsewhere — the `/v3` API and worker lambdas (dynamo-events, metrics, zipbuilder, gitlab-repository-check, user-subscribe) from `cla-backend-go/`, the `/v1`/`/v2` `legacy-api-lambda` from `cla-backend-legacy/`, and the API Gateway authorizer from `cla-backend/auth/`.
- `cla-backend-legacy/` — Go module (own `go.mod`): the Go implementation of the legacy `/v1`/`/v2` API surface (replaced the Python backend). Built as `bin/legacy-api-lambda` and deployed via `cla-backend/serverless.yml` on the original `api.*` domains. Responses carry `X-EasyCLA-Backend: cla-backend-legacy` headers; parity tooling lives under `internal/parity`.
- `cla-sss-base/` — standalone Go module (own `go.mod`): client for the Sanctions Screening Service (SSS).
- `scripts/`, `utils/` — operational shell/Python scripts (data audits, DynamoDB manipulation, deploys, credential rotation). Many `utils/*.sh` scripts operate directly against AWS environments.
- `infra/` — infrastructure config.
- `tests/` — functional/REST tests, Postman collections, py2go comparison tests.

## Go Backend (cla-backend-go) — Primary Workflow

Requires Go 1.25+. All commands run from `cla-backend-go/`.

```bash
make setup          # one-time: install swagger, golangci-lint, goimports; sets up swagger venv
make swagger        # regenerate API models/clients from swagger specs into gen/ (see below)
make build-mac      # build local binary -> bin/cla-mac (build-linux for Linux)
make test           # go test -v ./... with coverage
make lint           # golangci-lint (v1.64.8, config .golangci.yaml) + license header check
make fmt            # gofmt + goimports
make mock           # regenerate mocks via tools/regenmocks.sh
make all-mac        # full pipeline: clean swagger deps fmt build test lint (all-linux on Linux)
```

Run a single test: `go test -v ./signatures/ -run TestName`

Run locally (points at a real AWS environment — see below): build, set env, then `./bin/cla-mac`. Health checks at `http://localhost:8080/v3/ops/health` and `/v4/ops/health`. Set `GH_ORG_VALIDATION=false` to bypass GitHub auth checks for local curl/Postman testing.

### Swagger code generation (important)

The API is **swagger-first**. `gen/` is fully generated and is deleted/rebuilt by `make swagger` — never hand-edit files under `gen/`. Source specs are multi-file YAML under `swagger/` (`cla.v1.yaml`, `cla.v2.yaml`) compiled via `swagger/multi-file-swagger.py`. `make swagger` also downloads and generates clients for external LFX platform services (project-service, organization-service, user-service, acs-service) from their live dev swagger endpoints, so it requires network access. After changing an endpoint's spec, regenerate before implementing handlers.

### Module architecture pattern

Feature modules (e.g. `signatures/`, `approval_list/`, `company/`, `project/`, and everything under `v2/`) follow a consistent three-layer split:

- `handlers.go` — a `Configure(api, service, ...)` function that wires generated swagger operations to service calls. Handlers do request/response translation only.
- `service.go` — business logic, defined behind an interface; the unit-testable layer.
- `repository.go` — DynamoDB access. `dbmodels.go` holds table row structs; `converters.go` maps between DB models, generated API models, and internal models.
- `mocks/` — generated mocks (regenerate with `make mock`, don't edit by hand).

`v2/` packages are the newer LFX-Platform-integrated implementations; top-level packages are v1/legacy. Both are wired together in `cmd/server.go`.

`cmd/` holds `main.go` entrypoints for the many Lambda binaries (metrics, dynamo-events, zipbuilder, gitlab-repository-check, user-subscribe, etc.) plus the main API server (`cmd/server.go`, `cmd/server_standalone.go`, `cmd/server_aws_lambda.go`). Config is loaded from SSM via the `init` + `config` + `token` packages at startup.

## Legacy Go Backend (cla-backend-legacy)

Serves the legacy `/v1`/`/v2` API surface (the Python backend it replaced is gone). From the repo root:

```bash
source setenv.sh
cd cla-backend-legacy
go mod tidy
make lambdas        # build deployment artifact bin/legacy-api-lambda
STAGE=dev ADDR=":5000" make run-local   # run locally on :5000 pointing at dev environment
```

Health check: `http://localhost:5000/v2/health` (response includes `X-EasyCLA-Backend: cla-backend-legacy`). Port 5000 matches the Cypress functional-test helpers (`tests/functional/utils/run-single-test-local.sh` with `V=1`/`V=2`).

Deployment: the built binary is copied into `cla-backend/bin/` and deployed from the `cla-backend` Serverless stack (`yarn deploy:<stage>` from `cla-backend/`).

## AWS Environments & Local Development

Local backend runs point at **real shared AWS environments** (dev/test/staging/prod), not a local stack. The four environments map to AWS accounts via `~/.aws` profiles `lfproduct-dev`, `lfproduct-test`, `lfproduct-staging`, `lfproduct-prod` (assume-role from `lfproduct` base profile, MFA required). See `aws_env.md` for the profile setup and `dev.md` for the assume-role helper script and required env vars (`AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `PRODUCT_DOMAIN`, `ROOT_DOMAIN`, `STAGE`).

Be careful: scripts in `utils/` and `scripts/` can mutate DynamoDB in whichever environment your profile targets, including prod.

## Conventions

- **License headers are enforced in CI.** New code files need `// SPDX-License-Identifier: MIT`; docs need `SPDX-License-Identifier: CC-BY-4.0`. `make lint` runs `check-headers.sh`.
- **DCO required** — every commit needs `Signed-off-by`.
- Use "Approved List" terminology, not "whitelist" (renamed across code, APIs, and specs as of 8/2025).
- Bot CLA exemptions: see `BOT_ALLOWLIST.md`. Co-authors support: see `CO_AUTHORS.md`. PR-check author/co-author caching design: see `COMMIT_AUTHORS_CACHING.md`.

## CI

GitHub Actions in `.github/workflows/`: `build-pr.yml` (build/test/lint), `cypress-functional-pr.yml` (functional E2E), CodeQL/security/license scans for the Go backends, and `deploy-dev.yml` / `deploy-prod.yml` for deployment (both build `cla-backend-go` and `cla-backend-legacy`, then deploy through the `cla-backend` stack).
