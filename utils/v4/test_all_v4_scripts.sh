#!/bin/bash

# Test all V4 API scripts to ensure they can execute without errors
# This script tests syntax and basic execution of all V4 API scripts

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "=== Testing All V4 API Scripts ==="
echo "Script directory: $SCRIPT_DIR"
echo "Project root: $PROJECT_ROOT"
echo ""

# Change to project root for proper relative path resolution
cd "$PROJECT_ROOT"

# Track results
TOTAL_SCRIPTS=0
SYNTAX_OK=0
EXECUTION_OK=0
ERRORS=()

# Function to test script syntax
test_script_syntax() {
    local script="$1"
    bash -n "$script" 2>/dev/null
    return $?
}

# Function to test script execution (help or parameter validation)
test_script_execution() {
    local script="$1"
    
    # Try to run script with --help or invalid params to test parameter validation
    timeout 10s bash "$script" --help 2>/dev/null >/dev/null
    local help_result=$?
    
    # If --help doesn't work, try with no params (should show usage)
    if [ $help_result -ne 0 ]; then
        timeout 10s bash "$script" 2>&1 | grep -q "Usage:\|you need to specify\|Error:"
        return $?
    fi
    
    return 0
}

echo "Testing scripts..."

# Find all shell scripts except test scripts and shared utilities
while IFS= read -r -d '' script; do
    # Skip test scripts, shared utilities, and generator scripts
    if [[ "$script" == *"/test_all_"* ]] || \
       [[ "$script" == *"/shared/"* ]] || \
       [[ "$script" == *"generate_missing_scripts.py"* ]] || \
       [[ "$script" == *"check_api_coverage"* ]] || \
       [[ "$script" == *"test_all_v4_scripts.sh"* ]] || \
       [[ "$script" == *"simple_test.sh"* ]] || \
       [[ "$script" == *"final_coverage_report.sh"* ]]; then
        continue
    fi
    
    TOTAL_SCRIPTS=$((TOTAL_SCRIPTS + 1))
    
    # Get relative path for display
    rel_script="${script#$PROJECT_ROOT/}"
    
    printf "Testing %-60s " "$rel_script"
    
    # Test syntax
    if test_script_syntax "$script"; then
        SYNTAX_OK=$((SYNTAX_OK + 1))
        
        # Test execution
        if test_script_execution "$script"; then
            EXECUTION_OK=$((EXECUTION_OK + 1))
            echo "✓"
        else
            echo "✗ (execution)"
            ERRORS+=("$rel_script: execution test failed")
        fi
    else
        echo "✗ (syntax)"
        ERRORS+=("$rel_script: syntax error")
    fi
    
done < <(find "$SCRIPT_DIR" -name "*.sh" -type f -print0)

echo ""
echo "=== Results ==="
echo "Total scripts tested: $TOTAL_SCRIPTS"
echo "Syntax OK: $SYNTAX_OK/$TOTAL_SCRIPTS"
echo "Execution OK: $EXECUTION_OK/$TOTAL_SCRIPTS"

if [ ${#ERRORS[@]} -gt 0 ]; then
    echo ""
    echo "=== Errors ==="
    for error in "${ERRORS[@]}"; do
        echo "  $error"
    done
fi

# Calculate success rate
if [ $TOTAL_SCRIPTS -gt 0 ]; then
    success_rate=$((EXECUTION_OK * 100 / TOTAL_SCRIPTS))
    echo ""
    echo "Success rate: $success_rate%"
    
    if [ $success_rate -eq 100 ]; then
        echo "🎉 All scripts passed!"
        exit 0
    else
        echo "❌ Some scripts failed"
        exit 1
    fi
else
    echo "No scripts found to test"
    exit 1
fi