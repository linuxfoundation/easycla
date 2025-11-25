#!/bin/bash
aws --profile lfproduct-prod dynamodb scan --table-name cla-prod-signatures --filter-expression "attribute_not_exists(#a) OR #a = :nullval" --expression-attribute-names '{"#a":"date_created"}' --expression-attribute-values '{":nullval":{"NULL":true}}' --select "COUNT"
aws --profile lfproduct-prod dynamodb scan --table-name cla-prod-signatures --filter-expression "attribute_exists(#a)" --expression-attribute-names '{"#a":"approx_date_created"}' --select "COUNT"
aws --profile lfproduct-prod dynamodb scan --table-name cla-prod-signatures --filter-expression "attribute_not_exists(#a) OR #a = :nullval" --expression-attribute-names '{"#a":"date_modified"}' --expression-attribute-values '{":nullval":{"NULL":true}}' --select "COUNT"
aws --profile lfproduct-prod dynamodb scan --table-name cla-prod-signatures --filter-expression "attribute_exists(#a)" --expression-attribute-names '{"#a":"approx_date_modified"}' --select "COUNT"
