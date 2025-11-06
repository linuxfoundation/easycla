#!/bin/bash
# Simple test of key V4 scripts to verify they work from project root
# Usage: ./utils/v4/simple_test.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "======================================="
echo "Simple V4 Scripts Test"
echo "======================================="
echo "Project Root: ${PROJECT_ROOT}"
echo ""

# Change to project root
cd "$PROJECT_ROOT"

# Test a few key scripts that don't require parameters
test_scripts=(
    "utils/v4/health/get_health.sh"
    "utils/v4/version/get_version.sh"
    "utils/v4/docs/get_swagger_json.sh"
    "utils/v4/current-user/get_user_from_token.sh"
)

success_count=0
total_count=${#test_scripts[@]}

for script in "${test_scripts[@]}"; do
    echo -n "Testing $script ... "
    if timeout 10s "./$script" > /dev/null 2>&1; then
        echo "✅ OK"
        success_count=$((success_count + 1))
    else
        echo "❌ FAILED"
    fi
done

echo ""
echo "Results: $success_count/$total_count scripts working"

# Count total scripts
total_scripts=$(find utils/v4 -name "*.sh" -type f ! -path "*/shared/*" ! -name "*test*" | wc -l)
echo "Total V4 scripts: $total_scripts"

if [ $success_count -eq $total_count ]; then
    echo "🎉 All tested scripts are working!"
else
    echo "⚠️  Some scripts need attention"
fi