#!/bin/bash
# Comprehensive test script for all V3 and V4 shell scripts
# Usage: ./test_all_scripts_comprehensive.sh [v3|v4|all]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

VERSION="${1:-all}"
ERRORS=0
TESTED=0

echo "=========================================="
echo "Comprehensive API Script Testing"
echo "=========================================="
echo "Version: ${VERSION}"
echo "Project Root: ${PROJECT_ROOT}"
echo ""

# Test function for a single script
test_script() {
    local script_path="$1"
    local script_name="$(basename "$script_path")"
    local relative_path="${script_path#$PROJECT_ROOT/}"
    
    echo -n "Testing: $relative_path ... "
    
    # Skip test runner scripts
    if [[ "$script_name" == test_* ]]; then
        echo "SKIPPED (test runner)"
        return 0
    fi
    
    # Test local mode
    if timeout 30 "$script_path" > /dev/null 2>&1; then
        echo -n "LOCAL:OK "
    else
        echo -n "LOCAL:FAIL "
        ((ERRORS++))
    fi
    
    # Test remote mode
    if timeout 30 REMOTE=1 "$script_path" > /dev/null 2>&1; then
        echo "REMOTE:OK"
    else
        echo "REMOTE:FAIL"
        ((ERRORS++))
    fi
    
    ((TESTED++))
}

# Test V3 scripts
if [[ "$VERSION" == "v3" || "$VERSION" == "all" ]]; then
    echo "=========================================="
    echo "Testing V3 Scripts"
    echo "=========================================="
    
    for script in $(find "${PROJECT_ROOT}/utils/v3" -name "*.sh" -type f | sort); do
        test_script "$script"
    done
    
    echo ""
fi

# Test V4 scripts  
if [[ "$VERSION" == "v4" || "$VERSION" == "all" ]]; then
    echo "=========================================="
    echo "Testing V4 Scripts" 
    echo "=========================================="
    
    for script in $(find "${PROJECT_ROOT}/utils/v4" -name "*.sh" -type f | sort); do
        test_script "$script"
    done
    
    echo ""
fi

echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo "Scripts tested: $TESTED"
echo "Errors: $ERRORS"

if [[ $ERRORS -eq 0 ]]; then
    echo "✓ All scripts passed!"
    exit 0
else
    echo "✗ $ERRORS scripts failed"
    exit 1
fi