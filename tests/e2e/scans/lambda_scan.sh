#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
# Read-only Lambda log scan (CloudWatch filter-log-events): error / panic / timeout counts per EasyCLA function
# for the window [since, now]: since = --since / E2E_SINCE when given, otherwise the start of this run (E2E_RUN_START,
# exported by run.sh) minus 15 minutes, so the scan covers the traffic the other modules generated. CloudWatch ingests
# Lambda logs with a delay, so the scan first waits E2E_SCAN_SETTLE_SECS (default 90, 0 disables) and the window ends after
# that wait. Panics, Go runtime errors, Runtime.ExitError and timeouts FAIL; error counts, INIT_REPORT failures and the
# CloudWatch Errors/Invocations/Throttles sums per function are INFO (top messages listed).
# Needs the AWS CLI and credentials for $AWS_PROFILE_E2E (read-only logs:FilterLogEvents, cloudwatch:GetMetricStatistics,
# optional ssm:GetServiceSetting); without either the scan is recorded as SKIP.
set -u
. "$(cd "$(dirname "$0")/../lib" && pwd)/common.sh"
if ! command -v aws >/dev/null 2>&1; then
  e2e_result LS-AWS SKIP "aws CLI not installed - lambda scan skipped"
  e2e_summary_line; exit 0
fi
if ! aws --profile "$AWS_PROFILE_E2E" sts get-caller-identity --no-cli-pager >/dev/null 2>&1; then
  e2e_result LS-AWS SKIP "no AWS credentials for profile $AWS_PROFILE_E2E - lambda scan skipped"
  e2e_summary_line; exit 0
fi
settle="${E2E_SCAN_SETTLE_SECS:-90}"
case "$settle" in ''|*[!0-9]*) settle=90 ;; esac
if [ "$settle" -gt 0 ]; then
  e2e_log "waiting ${settle}s for CloudWatch log ingestion before scanning (E2E_SCAN_SETTLE_SECS)"
  sleep "$settle"
fi
# epoch ms without GNU `date -d` (works on BSD/macOS too)
since_ms=$(python3 -I - "${E2E_SINCE:-}" "${E2E_RUN_START:-}" <<'PY'
import sys, time, datetime
def parse(s):
    s = s.strip()
    if not s: return None
    if s.isdigit(): return int(s) * 1000
    s = s.replace("Z", "+00:00")
    try: dt = datetime.datetime.fromisoformat(s)
    except ValueError: return None
    if dt.tzinfo is None: dt = dt.replace(tzinfo=datetime.timezone.utc)
    return int(dt.timestamp() * 1000)
since = parse(sys.argv[1]) or ((parse(sys.argv[2]) or int(time.time() * 1000)) - 15 * 60 * 1000)
print(since)
PY
)
end_ms=$(python3 -c 'import time; print(int(time.time()*1000))')
e2e_result LS-WINDOW INFO "window $(python3 -c "import datetime,sys; print(datetime.datetime.fromtimestamp(int(sys.argv[1])/1000, datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'))" "$since_ms") .. $(e2e_now) (settled ${settle}s for log ingestion) profile=$AWS_PROFILE_E2E"

scan() { # scan <region> <function> <tag> <filter-pattern> <fail-if-any 0|1>
  local region=$1 fn=$2 tag=$3 pat=$4 fatal=$5 n out
  out="$E2E_EVIDENCE_DIR/$fn-$tag.private.json"
  if ! aws --profile "$AWS_PROFILE_E2E" --region "$region" --no-cli-pager logs filter-log-events --log-group-name "/aws/lambda/$fn" \
      --start-time "$since_ms" --end-time "$end_ms" --filter-pattern "$pat" --query 'events[].message' --output json >"$out" 2>"$out.err"; then
    if grep -q ResourceNotFoundException "$out.err"; then e2e_result "LS-$fn-$tag" SKIP "log group /aws/lambda/$fn not found in $region"
    else e2e_result "LS-$fn-$tag" FAIL "filter-log-events failed: $(head -c 160 "$out.err" | tr '\n' ' ')"; fi
    return
  fi
  chmod 600 "$out" 2>/dev/null || true
  local summary top
  summary=$(python3 -I - "$out" <<'PY'
import json, sys, collections, re
m = json.load(open(sys.argv[1])); c = collections.Counter()
for x in m:
    c[re.sub(r'"(x-request-id|XREQUESTID|time|requestId)":"[^"]*"', '', x.strip())[:120]] += 1
print(len(m)); print(" | ".join(f"{v}x {k}" for k, v in c.most_common(3)))
PY
)
  n=${summary%%$'\n'*}; top=${summary#*$'\n'}
  if [ "$n" = "0" ]; then e2e_result "LS-$fn-$tag" PASS "$fn [$tag] 0 events"
  elif [ "$fatal" = "1" ]; then e2e_result "LS-$fn-$tag" FAIL "$fn [$tag] $n events: $(e2e_redact 300 <<<"$top")"
  else e2e_result "LS-$fn-$tag" INFO "$fn [$tag] $n events: $(e2e_redact 300 <<<"$top")"; fi
}
# metrics <region> <function>: CloudWatch Errors/Invocations sums for the window (INFO; this is how the SSM cold-start
# throttling crashes were found - a crashed init leaves no application log line, only the platform error counter)
metrics() {
  local region=$1 fn=$2 out sums
  out="$E2E_EVIDENCE_DIR/$fn-metrics.private.json"
  local start end period
  start=$(python3 -c "import datetime,sys; print(datetime.datetime.fromtimestamp(int(sys.argv[1])/1000, datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'))" "$since_ms")
  end=$(python3 -c "import datetime,sys; print(datetime.datetime.fromtimestamp(int(sys.argv[1])/1000, datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'))" "$end_ms")
  period=$(( ( (end_ms - since_ms) / 1000 / 60 + 1 ) * 60 ))
  sums=""
  for metric in Errors Invocations Throttles; do
    if aws --profile "$AWS_PROFILE_E2E" --region "$region" --no-cli-pager cloudwatch get-metric-statistics --namespace AWS/Lambda \
        --metric-name "$metric" --dimensions Name=FunctionName,Value="$fn" --start-time "$start" --end-time "$end" \
        --period "$period" --statistics Sum --query 'Datapoints[].Sum' --output json >"$out" 2>/dev/null; then
      sums="$sums $metric=$(python3 -c "import json,sys; print(int(sum(json.load(open(sys.argv[1])) or [0])))" "$out")"
    else sums="$sums $metric=?"; fi
  done
  e2e_result "LS-$fn-metrics" INFO "$fn platform sums for the window:${sums}"
}
# scan_all <region> <function> <json 0|1>: JSON-logging lambdas (logrus) get field matches, text ones the classic signatures
scan_all() {
  local region=$1 fn=$2
  scan "$region" "$fn" json-error '{ $.level = "error" }' 0
  scan "$region" "$fn" json-fatal '{ $.level = "fatal" || $.level = "panic" }' 1
  # bare "panic" matches usernames and schema dumps; the runtime signatures below do not
  scan "$region" "$fn" panic '?"panic:" ?"goroutine " ?"runtime error"' 1
  scan "$region" "$fn" timeout '"Task timed out"' 1
  scan "$region" "$fn" exit-error '"Runtime.ExitError"' 1
  scan "$region" "$fn" init-error '"INIT_REPORT" "Status: error"' 0
  scan "$region" "$fn" init-timeout '"INIT_REPORT" "Status: timeout"' 0
  metrics "$region" "$fn"
}
scan_all "$LAMBDA_V4_REGION" "$LAMBDA_V4_NAME"
for suffix in api-v3-lambda apiv2 apiv1 authorizer githubactivity dynamo-signatures-events-lambda dynamo-events-events-lambda dynamo-projects-cla-groups-events-lambda user-event-handler-lambda; do
  scan_all "$LAMBDA_REGION" "$LAMBDA_PREFIX-$suffix"
done
# SSM Parameter Store throughput setting (INFO): off = 40 TPS, cold starts of many lambdas at once get throttled
if setting=$(aws --profile "$AWS_PROFILE_E2E" --region "$LAMBDA_REGION" --no-cli-pager ssm get-service-setting \
      --setting-id /ssm/parameter-store/high-throughput-enabled --query 'ServiceSetting.SettingValue' --output text 2>/dev/null); then
  e2e_result LS-SSM-HIGH-THROUGHPUT INFO "ssm parameter-store high-throughput-enabled=$setting"
else
  e2e_result LS-SSM-HIGH-THROUGHPUT SKIP "ssm get-service-setting not permitted for profile $AWS_PROFILE_E2E"
fi
e2e_summary_line
[ "$E2E_FAIL" = "0" ]
