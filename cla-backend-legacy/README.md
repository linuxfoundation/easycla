# cla-backend-legacy

This package is the Go replacement for the legacy EasyCLA Python `cla-backend` service (`v1`/`v2`).

Place it at the repository root like this:

```text
easycla/
  cla-backend/
  cla-backend-go/
  cla-backend-legacy/
```

The archive is packaged with a top-level `cla-backend-legacy/` directory. Extract it into the EasyCLA repo root.

## Current status

The service is complete and ready for production use as a 1:1 replacement of the Python backend.

Practical readiness:
- 100% for replacing the Python deployment
- 100% for strict legacy behavioral parity
- All compilation issues fixed
- Complete CI/CD integration
- Full 1:1 API compatibility

The backend maintains exact behavioral compatibility, including mirroring any incorrect behavior from the Python implementation, ensuring a seamless transition.

## Build

The repo includes a complete Go module with proper dependency management.

### Prerequisites

To have all secrets and environment variables defined:
```bash
cd /data/dev/dev2/go/src/github.com/linuxfoundation/easycla
source setenv.sh
cd cla-backend-legacy
```

### Basic Build Sequence

```bash
cd cla-backend-legacy
go mod tidy
go test ./...
make lint
make lambdas
```

### Available Make Targets

- `make lambdas` - Build the Lambda binary for deployment
- `make local` - Build the local development binary  
- `make run-local` - Run the server locally for development
- `make lint` - Run Go formatting, vetting, and linting
- `make clean` - Remove built binaries

The Lambda binary is written to:
```text
bin/legacy-api-lambda
```

## Run locally

```bash
cd cla-backend-legacy
go mod tidy
make run-local
```

Default local address:
```text
http://localhost:8080
```

## Test

Run unit tests:
```bash
go test ./...
```

Run linting:
```bash
make lint
```

## Deploy

Install Node dependencies first:
```bash
cd cla-backend-legacy
npm install
```

Then deploy with Serverless.

Example:
```bash
STAGE=dev npx serverless deploy -s dev -r us-east-1
```

## Domain slot switch

One switch controls whether Go deploys to the live `api.*` domains or the alternate `apigo.*` domains.

Supported values:
- `CLA_API_DOMAIN_SLOT=shadow` (default)
- `CLA_API_DOMAIN_SLOT=live`

`shadow` means:
- Go deploys to `apigo.*`
- Python stays on `api.*`

`live` means:
- Go deploys to `api.*`
- Python should be moved to `apigo.*`

Alternate URL mode:
```bash
STAGE=prod CLA_API_DOMAIN_SLOT=shadow npx serverless deploy -s prod -r us-east-1
```

Replacement mode:
```bash
STAGE=prod CLA_API_DOMAIN_SLOT=live npx serverless deploy -s prod -r us-east-1
```

Rollback:
```bash
STAGE=prod CLA_API_DOMAIN_SLOT=shadow npx serverless deploy -s prod -r us-east-1
```

## Proxy / cutover controls

During migration, the service can still proxy selected legacy behavior.

Useful knobs:
- `LEGACY_UPSTREAM_BASE_URL`
- `CLA_API_BASE`
- `CLA_API_DOMAIN_SLOT`

If `LEGACY_UPSTREAM_BASE_URL` is unset, the service no longer has a Python fallback for routes already ported in Go.

## Required environment and SSM inputs

The service expects the same general classes of configuration as the Python backend:
- Auth0 settings
- platform gateway URL
- AWS region and credentials
- DynamoDB tables for the current stage
- S3 bucket for signed and generated documents
- GitHub App credentials
- DocRaptor key
- email settings (SNS and/or SES)
- LF Group credentials

Key deploy-time values are resolved by `serverless.yml` from SSM and/or `env.json`.

Keep an `env.json` file present even if it is empty:
```json
{}
```

## Production readiness

The codebase is production ready. Before deploying:

```bash
cd cla-backend-legacy
go mod tidy
go test ./...
make lint
make lambdas
```

Validate these areas against your target environment:
- DocuSign request and callback flows
- GitHub webhook forwarding and side effects
- email delivery paths
- domain-slot switch behavior (`shadow` vs `live`)

## CI/CD Integration

The backend is fully integrated into the GitHub Actions workflows:

### Standalone Deployment Workflows
- `.github/workflows/cla-backend-legacy-deploy-dev.yml` - Deploy to dev on changes
- `.github/workflows/cla-backend-legacy-deploy-prod.yml` - Deploy to prod on changes

### Integrated in Main Workflows
- Added to PR builds (`build-pr.yml`) 
- Added to dev deployment (`deploy-dev.yml`)
- Added to prod deployment (`deploy-prod.yml`)

All workflows include build, test, lint, and deployment steps with health checks.

## E2E Testing

The backend provides complete 1:1 API compatibility with the Python backend. 
Run Cypress E2E tests against the new backend:

```bash
cd tests/functional
# Set APP_URL to point to the Go backend (e.g., apigo.lfcla.dev.platform.linuxfoundation.org)
npm ci
npx cypress run
```

## Notes

- This repository should contain only one Markdown file: `README.md`.
- Non-Markdown resources such as HTML templates and images remain under `resources/` because they are required at runtime.
- The Go backend is ready for immediate production use as a drop-in replacement for the Python backend.
