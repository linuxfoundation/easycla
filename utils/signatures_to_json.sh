#!/bin/bash
aws --profile lfproduct-dev dynamodb scan --table-name cla-dev-signatures --select ALL_ATTRIBUTES --page-size 500 --max-items 100000 --output json > cla-dev-signatures.json.secret
