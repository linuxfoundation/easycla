#!/bin/bash
aws --profile lfproduct-dev dynamodb query --table-name cla-dev-projects --index-name project-name-lower-search-index --key-condition-expression "project_name_lower  = :name" --expression-attribute-values  '{":name":{"S":"child group earths"}}'
