#!/bin/bash
./utils/copy_prod_to_dev.sh github-orgs organization_name kubernetes

repo_id=$(STAGE=prod ./utils/scan.sh repositories repository_name 'kubernetes/website' | jq -r '.[0].repository_id.S')
./utils/copy_prod_to_dev.sh repositories repository_id "${repo_id}"

STAGE=dev MODE=put-item ./utils/enable_co_authors_entry.sh 'kubernetes' 'website' t
STAGE=dev MODE=put-item ./utils/skip_cla_entry.sh 'kubernetes' 'website' 'Copilot;re:^\d+\+Copilot@users\.noreply\.github\.com$;copilot-swe-agent[bot]'

STAGE=dev ./utils/scan.sh github-orgs organization_name kubernetes
STAGE=dev ./utils/scan.sh repositories repository_name 'kubernetes/website'
