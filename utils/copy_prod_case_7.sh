#!/bin/bash
repo_id=$(STAGE=prod ./utils/scan.sh repositories repository_name 'fluxnova-modeler' | jq -r '.[0].repository_id.S')
./utils/copy_prod_to_dev.sh repositories repository_id "${repo_id}"
STAGE=dev ./utils/scan.sh repositories repository_name 'fluxnova-modeler'
