#!/bin/bash
go build -o signatures_timestamp_backfill cmd/signatures_timestamp_backfill/main.go && ALLOW_CURRENT_TIME='' STAGE=prod DRY_RUN=true ./signatures_timestamp_backfill
