# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
#!/bin/bash
# source setenv-prod.sh.secret
go build -o signatures_timestamp_backfill cmd/signatures_timestamp_backfill/main.go && DEBUG='' STAGE=prod DRY_RUN=true ./signatures_timestamp_backfill
# go build -o signatures_timestamp_backfill cmd/signatures_timestamp_backfill/main.go && DEBUG=true STAGE=prod DRY_RUN='' ./signatures_timestamp_backfill
