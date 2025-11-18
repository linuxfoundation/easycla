# Signature Timestamp Backfill Utility

This utility fixes missing `date_created` and `date_modified` timestamps in the EasyCLA signatures DynamoDB table, addressing [GitHub Issue #4862](https://github.com/linuxfoundation/easycla/issues/4862).

## Problem Description

Approximately 1,293 signatures in production were missing both `date_created` and `date_modified` timestamps, with ~4k signatures missing `date_created` alone. This was caused by bugs in both the Python and Go backends.

## Usage Instructions

### Prerequisites
```bash
# Set AWS profile with access to cla-{stage}-signatures table
export AWS_PROFILE=your-aws-profile
```

### Quick Start
```bash
# Test on staging first (dry-run, recommended)
./scripts/backfill-signatures-timestamps.sh --stage staging --dry-run

# Apply fix to production
./scripts/backfill-signatures-timestamps.sh --stage prod
```

### Manual Build and Run
```bash
# Build the utility
go build -o bin/signatures-timestamp-backfill ./cmd/signatures_timestamp_backfill

# Run with dry-run
STAGE=staging DRY_RUN=true AWS_PROFILE=your-profile ./bin/signatures-timestamp-backfill
```