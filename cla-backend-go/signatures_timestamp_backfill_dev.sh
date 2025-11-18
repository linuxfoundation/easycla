#!/bin/bash
# source setenv.sh
go build -o signatures_timestamp_backfill cmd/signatures_timestamp_backfill/main.go && ALLOW_CURRENT_TIME='' DEBUG=true STAGE=dev DRY_RUN=true ./signatures_timestamp_backfill
