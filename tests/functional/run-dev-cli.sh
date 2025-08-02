#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT

function validate_env_var() {
  if [[ -z "${!1}" ]]; then
    echo "$1 is not set."
    exit 1
  fi
}

validate_env_var AUTH0_TOKEN_API
validate_env_var AUTH0_CLIENT_ID
validate_env_var AUTH0_CLIENT_SECRET
validate_env_var AUTH0_TOKEN_AUDIENCE
validate_env_var LFX_API_TOKEN

ELECTRON_ENABLE_LOGGING=1 ./node_modules/.bin/cypress run \
  --env apiUrl="https://api-gw.dev.platform.linuxfoundation.org/",LFX_API_TOKEN="${LFX_API_TOKEN}",AUTH0_TOKEN_API="${AUTH0_TOKEN_API}",AUTH0_CLIENT_ID="${AUTH0_CLIENT_ID}",AUTH0_CLIENT_SECRET="${AUTH0_CLIENT_SECRET}",AUTH0_TOKEN_AUDIENCE="${AUTH0_TOKEN_AUDIENCE}"