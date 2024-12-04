#!/bin/bash
aws --profile lfproduct-dev dynamodb scan --table-name cla-dev-projects --max-items 3
