#!/bin/bash
# Test all V4 scripts to ensure they work from project root
# Usage: ./utils/v4/test_all_scripts.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "======================================="
echo "Testing All V4 Scripts"
echo "======================================="
echo "Project Root: ${PROJECT_ROOT}"
echo "Testing from: $(pwd)"
echo ""

# Change to project root
cd "$PROJECT_ROOT"

total_scripts=0
working_scripts=0
failing_scripts=0
skipped_scripts=0

# Find all shell scripts in v4 directories (excluding shared and test scripts)
while IFS= read -r -d '' script_path; do
    total_scripts=$((total_scripts + 1))
    script_name=$(basename "$script_path")
    script_dir=$(dirname "$script_path" | sed "s|${PROJECT_ROOT}/||")
    
    # Skip certain scripts that don't follow standard API patterns
    if [[ "$script_name" == "test_all"* ]] || [[ "$script_name" == "check_api_coverage.sh" ]] || [[ "$script_name" == "shared"* ]]; then
        echo "⏭️  SKIP $script_dir/$script_name (utility script)"
        skipped_scripts=$((skipped_scripts + 1))
        continue
    fi
    
    # Test if script can be executed without parameters
    echo -n "🧪 TEST $script_dir/$script_name ... "
    
    # Try to run the script and capture its output
    output=$(timeout 5s bash -c "./$script_path 2>&1" | head -5)
    exit_code=$?
    
    # Check if the script produced output or failed gracefully
    if [[ $exit_code -eq 124 ]]; then
        echo "⏱️  TIMEOUT"
        failing_scripts=$((failing_scripts + 1))
    elif [[ -n "$output" ]]; then
        # Check if output contains typical response patterns
        if echo "$output" | grep -q -E "(\{|\[|<|Usage|usage|ERROR|Error|required|parameters|arguments|you need to specify)"; then
            echo "✅ OK"
            working_scripts=$((working_scripts + 1))
        else
            echo "❓ UNKNOWN ($output)"
            failing_scripts=$((failing_scripts + 1))
        fi
    else
        echo "❌ NO OUTPUT"
        failing_scripts=$((failing_scripts + 1))
    fi
    
done < <(find "${PROJECT_ROOT}/utils/v4" -name "*.sh" -type f ! -path "*/shared/*" -print0)

echo ""
echo "======================================="
echo "Test Results Summary"
echo "======================================="
echo "Total scripts found: $total_scripts"
echo "Working scripts: $working_scripts"
echo "Failing scripts: $failing_scripts"
echo "Skipped scripts: $skipped_scripts"
echo ""

if [ $failing_scripts -eq 0 ]; then
    echo "🎉 All scripts are working correctly!"
    success_rate=100
else
    success_rate=$(( (working_scripts * 100) / (total_scripts - skipped_scripts) ))
    echo "⚠️  $failing_scripts scripts need attention"
fi

echo "Success rate: ${success_rate}% (excluding skipped scripts)"
echo ""

exit $failing_scripts