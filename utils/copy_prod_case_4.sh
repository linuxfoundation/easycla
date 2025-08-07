#!/bin/bash
./utils/copy_prod_to_dev.sh github-orgs organization_name open-telemetry
./utils/copy_prod_to_dev.sh github-orgs organization_name openfga

repo_id=$(STAGE=prod ./utils/scan.sh repositories repository_name 'openfga/vscode-ext' | jq -r '.[0].repository_id.S')
./utils/copy_prod_to_dev.sh repositories repository_id "${repo_id}"
STAGE=dev ./utils/scan.sh repositories repository_name 'openfga/vscode-ext'

repo_id=$(STAGE=prod ./utils/scan.sh repositories repository_name 'open-telemetry/otel-arrow' | jq -r '.[0].repository_id.S')
./utils/copy_prod_to_dev.sh repositories repository_id "${repo_id}"
STAGE=dev ./utils/scan.sh repositories repository_name 'open-telemetry/otel-arrow'
