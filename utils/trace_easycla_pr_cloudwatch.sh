#!/usr/bin/env bash
set -euo pipefail

# source setenv.sh
# STAGE=dev PROFILE=lfproduct-dev PR=56 REPO_ID=792406957 USER_LOGIN=lukaszgryglicki ORG=mlehotskylf-org2 REPO=easycla-dev START_UTC='2026-04-21 13:05:00' END_UTC='2026-04-21 13:20:00' ./utils/trace_easycla_pr_cloudwatch.sh

STAGE="${STAGE:-dev}"
PROFILE="${PROFILE:-lfproduct-${STAGE}}"

# Set these for the PR being traced.
PR="${PR:-56}"
REPO_ID="${REPO_ID:-792406957}"
USER_LOGIN="${USER_LOGIN:-lukaszgryglicki}"
ORG="${ORG:-mlehotskylf-org2}"
REPO="${REPO:-easycla-dev}"

# UTC time window. Adjust as needed.
START_UTC="${START_UTC:-2026-04-21 13:05:00}"
END_UTC="${END_UTC:-2026-04-21 13:20:00}"

START_TS="$(date -u -d "${START_UTC}" +%s)"
END_TS="$(date -u -d "${END_UTC}" +%s)"

run_query() {
  local region="$1"
  local title="$2"
  shift 2
  local groups=("$@")

  local query='
fields @timestamp, @logStream, @message
| filter @message like /'"${PR}"'/ 
    or @message like /'"${REPO_ID}"'/
    or @message like /'"${USER_LOGIN}"'/
    or @message like /'"${ORG}"'/
    or @message like /'"${REPO}"'/
    or @message like /legacy_internal_trigger/
    or @message like /trigger-change-request/
    or @message like /active_pr/
    or @message like /UpdateGitHubChangeRequest/
    or @message like /updateChangeRequestLegacyCompat/
    or @message like /GetPullRequestCommitAuthors/
    or @message like /GetCommitAuthorsSignedStatuses/
    or @message like /UpdatePullRequest/
    or @message like /Created success status/
| sort @timestamp asc
| limit 300
'

  echo
  echo "===== ${title} (${region}) ====="
  echo "Log groups:"
  printf '  %s\n' "${groups[@]}"

  query_id="$(
    aws --profile "${PROFILE}" --region "${region}" logs start-query \
      --log-group-names "${groups[@]}" \
      --start-time "${START_TS}" \
      --end-time "${END_TS}" \
      --query-string "${query}" \
      --query 'queryId' \
      --output text
  )"

  while true; do
    status="$(
      aws --profile "${PROFILE}" --region "${region}" logs get-query-results \
        --query-id "${query_id}" \
        --query 'status' \
        --output text
    )"
    case "${status}" in
      Complete|Failed|Cancelled|Timeout) break ;;
      *) sleep 1 ;;
    esac
  done

  echo "Query status: ${status}"
  aws --profile "${PROFILE}" --region "${region}" logs get-query-results \
    --query-id "${query_id}" \
    --output json \
  | jq -r '
      .results[]
      | map({(.field): .value}) | add
      | "\(.["@timestamp"] // "") \(.["@logStream"] // "")\n\(.["@message"] // "")\n"
    '
}

run_query "us-east-1" "legacy/v1/v2/v3" \
  "/aws/lambda/cla-backend-${STAGE}-githubactivity" \
  "/aws/lambda/cla-backend-${STAGE}-apiv2" \
  "/aws/lambda/cla-backend-${STAGE}-api-v3-lambda"

run_query "us-east-2" "v4" \
  "/aws/lambda/cla-backend-go-api-v4-lambda"
