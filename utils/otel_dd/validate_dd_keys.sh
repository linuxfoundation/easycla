#!/bin/bash
curl -sS -H "DD-API-KEY: ${DD_API_KEY}" -H "DD-APPLICATION-KEY: ${DD_APP_KEY}" https://api.datadoghq.com/api/v2/validate_keys | jq -r
