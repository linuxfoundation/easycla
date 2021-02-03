#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

set -o nounset -o pipefail
declare -r SCRIPT_DIR="$( cd "$( dirname "$0" )" && pwd )"

#------------------------------------------------------------------------------
# Load helper scripts
#------------------------------------------------------------------------------
if [[ -f "${SCRIPT_DIR}/colors.sh" ]]; then
  source "${SCRIPT_DIR}/colors.sh"
else
  echo "Unable to load script: ${SCRIPT_DIR}/colors.sh"
  exit 1
fi

if [[ -f "${SCRIPT_DIR}/logger.sh" ]]; then
  source "${SCRIPT_DIR}/logger.sh"
else
  echo "Unable to load script: ${SCRIPT_DIR}/logger.sh"
  exit 1
fi

if [[ -f "${SCRIPT_DIR}/utils.sh" ]]; then
  source "${SCRIPT_DIR}/utils.sh"
else
  echo "Unable to load script: ${SCRIPT_DIR}/utils.sh"
  exit 1
fi

#------------------------------------------------------------------------------
# Check command line arguments
#------------------------------------------------------------------------------
if [[ $# -eq 0 ]]; then
  echo "Missing environment parameter. Expecting one of: 'dev', 'staging', or 'prod'."
  echo "usage:   $0 [environment]"
  echo "example: $0 dev"
  echo "example: $0 staging"
  echo "example: $0 prod"
  exit 1
fi

declare -r env="${1}"
if [[ "${env}" == 'dev' || "${env}" == 'staging' || "${env}" == 'prod' ]]; then
  echo "Using environment '${env}'..."
else
  echo "Environment parameter does not match expected values. Expecting one of: 'dev', 'staging', or 'prod'."
  echo "usage:   $0 [environment]"
  echo "example: $0 dev"
  echo "example: $0 staging"
  echo "example: $0 prod"
  exit 1
fi

#------------------------------------------------------------------------------
# Common variables
#------------------------------------------------------------------------------
declare -r region="us-east-1"
declare -r profile="lfproduct-${env}"
declare -r current_date="$(date +%Y%m%dT%H%M%S)"
declare -r dry_run="false"
declare -a tables=( "cla-${env}-ccla-whitelist-requests"
  "cla-${env}-companies"
  "cla-${env}-company-invites"
  "cla-${env}-events"
  "cla-${env}-gerrit-instances"
  "cla-${env}-github-orgs"
  "cla-${env}-projects"
  "cla-${env}-repositories"
  "cla-${env}-session-store"
  "cla-${env}-signatures"
  "cla-${env}-store"
  "cla-${env}-user-permissions"
  "cla-${env}-users"
  )

#------------------------------------------------------------------------------
# Perform the database table backup
#------------------------------------------------------------------------------
log "Backing up tables for environment ${_Y}${env}${_W} using aws profile ${_Y}${profile}${_W}."
for table in "${tables[@]}"; do
  cmd="aws --profile ${profile} --region ${region} dynamodb create-backup --table-name ${table} --backup-name ${table}-${current_date}"
  log "Running command: ${cmd}"
  if [[ "${dry_run}" == "true" ]]; then
    log "Skipping execution - in ${_Y}dry-run${_W} mode"
  else
    ${cmd}
    exit_code=$?
    if [[ ${exit_code} -ne 0 ]]; then
      log_warn "Error response ${_R}${exit_code}${_W} from the backup command: ${_Y}${cmd}${_W}"
    fi
  fi
done
