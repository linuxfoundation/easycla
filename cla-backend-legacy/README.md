# cla-backend-legacy

`cla-backend-legacy` is the Go implementation of the legacy EasyCLA `/v1` and `/v2` API surface.

It is no longer deployed as a separate `apigo.*` stack. Deployment now happens through
`cla-backend/serverless.yml`, which serves the legacy Go binary on the original `api.*`
domains while keeping the existing `/v3` deployment in the same stack.

## What changed

- No Python fallback.
- No separate shadow/live deployment mode.
- No separate `cla-backend-legacy/serverless.yml` deployment.
- The build artifact from this module (`bin/legacy-api-lambda`) is copied into
  `cla-backend/bin/` and deployed from the existing `cla-backend` stack.
- Responses now include:
  - `X-EasyCLA-Backend: cla-backend-legacy`
  - `X-EasyCLA-Backend-Version: go`
- Logs now include:
  - `LG:api-request-path:<path> backend=cla-backend-legacy`
  - `LG:e2e-request-path:<path> backend=cla-backend-legacy e2e=1 [e2e_run_id=...]`

## Build

From repo root:

```bash
source setenv.sh
cd cla-backend-legacy
go mod tidy
go test ./...
make lint
make lambdas
```

The deployment artifact is:

```text
bin/legacy-api-lambda
```

## Local run

Local v1/v2 testing should use port `5000`, which matches the existing Cypress local helpers.

```bash
source setenv.sh
cd cla-backend-legacy
STAGE=dev ADDR=":5000" make run-local
```

Basic smoke test:

```bash
curl -i http://localhost:5000/v2/health
```

A successful response should include:

```text
X-EasyCLA-Backend: cla-backend-legacy
X-EasyCLA-Backend-Version: go
```

## Cypress functional coverage

Cypress stays pointed at the existing legacy API shape. For local runs, the current helper
already targets `localhost:5000` for `/v1` and `/v2`.

```bash
cd tests/functional
npm ci
V=1 ALL=1 ./utils/run-single-test-local.sh
V=2 ALL=1 ./utils/run-single-test-local.sh
```

Examples:

```bash
V=2 ./utils/run-single-test-local.sh health
V=1 ./utils/run-single-test-local.sh project
```

## Deployment model

Deployment is owned by `cla-backend/serverless.yml`:

- `/v1` -> `bin/legacy-api-lambda`
- `/v2` -> `bin/legacy-api-lambda`
- `/v3` -> existing `cla-backend-go` binary in the same stack

CI builds `cla-backend-legacy`, copies `bin/legacy-api-lambda` into `cla-backend/bin/`,
then deploys the existing `cla-backend` stack. There is no separate `apigo.*` deployment.

## Cutover verification

After deployment to `dev`, verify the live legacy URL directly:

```bash
curl -i https://api.lfcla.dev.platform.linuxfoundation.org/v2/health
```

Look for:

```text
X-EasyCLA-Backend: cla-backend-legacy
```

In logs, search for:

```text
LG:api-request-path:... backend=cla-backend-legacy
LG:e2e-request-path:... backend=cla-backend-legacy
```
