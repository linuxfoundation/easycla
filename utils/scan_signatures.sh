#!/bin/bash
aws --profile lfproduct-dev dynamodb scan --table-name cla-dev-signatures --max-items 20
