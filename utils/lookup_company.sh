#!/bin/bash
aws --profile lfproduct-dev dynamodb query --table-name cla-dev-companies --index-name company-name-index --key-condition-expression "company_name = :name" --expression-attribute-values  '{":name":{"S":"Google LLC"}}'
