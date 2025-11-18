#!/bin/bash
# source setenv-prod.sh.secret
go build -o signatures_timestamp_backfill cmd/signatures_timestamp_backfill/main.go && ALLOW_CURRENT_TIME='' DEBUG='' STAGE=prod DRY_RUN=true ./signatures_timestamp_backfill
