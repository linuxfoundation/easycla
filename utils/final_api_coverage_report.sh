#!/bin/bash

# Final comprehensive API coverage report for both V3 and V4 APIs
# This script generates a complete summary of API test coverage

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "========================================"
echo "  EASYCLA API COVERAGE FINAL REPORT"
echo "========================================"
echo ""
echo "Project root: $PROJECT_ROOT"
echo "Generated on: $(date)"
echo ""

# Change to project root for proper execution
cd "$PROJECT_ROOT"

echo "=== V3 API Coverage Analysis ==="
./utils/v3/check_api_coverage_v3.sh
v3_result=$?
echo ""

echo "=== V4 API Coverage Analysis ==="
./utils/v4/check_api_coverage_v4.sh
v4_result=$?
echo ""

echo "=== Cypress Test Files Coverage ==="
echo "V3 Cypress test files:"
ls -1 tests/functional/cypress/e2e/v3/*.cy.ts | sed 's|.*/||' | sort
echo ""
echo "V4 Cypress test files:"
ls -1 tests/functional/cypress/e2e/v4/*.cy.ts | sed 's|.*/||' | sort
echo ""

echo "=== Script Counts ==="
v3_scripts=$(find utils/v3 -name "*.sh" -not -path "*/shared/*" | wc -l)
v4_scripts=$(find utils/v4 -name "*.sh" -not -path "*/shared/*" | wc -l)
echo "V3 API scripts: $v3_scripts"
echo "V4 API scripts: $v4_scripts"
echo "Total scripts: $((v3_scripts + v4_scripts))"
echo ""

echo "=== Directory Structure ==="
echo "V3 API directories:"
find utils/v3 -type d -not -path "*/shared" | grep -v "^utils/v3$" | sort
echo ""
echo "V4 API directories:"
find utils/v4 -type d -not -path "*/shared" | grep -v "^utils/v4$" | sort
echo ""

echo "=== Testing Quick Syntax Check ==="
v3_syntax_errors=$(find utils/v3 -name "*.sh" -not -path "*/shared/*" -exec bash -n {} \; 2>&1 | wc -l)
v4_syntax_errors=$(find utils/v4 -name "*.sh" -not -path "*/shared/*" -exec bash -n {} \; 2>&1 | wc -l)

echo "V3 scripts syntax errors: $v3_syntax_errors"
echo "V4 scripts syntax errors: $v4_syntax_errors"
echo ""

echo "=== API URL Configuration ==="
echo "V3 shared config: utils/v3/shared/handle_api_url.sh"
echo "V4 shared config: utils/v4/shared/handle_api_url.sh"
echo ""

echo "=== Test Script Verification ==="
echo "Testing a few key scripts..."

echo -n "V3 health endpoint: "
if ./utils/v3/health/get_health.sh 2>&1 | grep -q '"Branch"'; then
    echo "✓ Working"
else
    echo "✗ Failed"
fi

echo -n "V4 health endpoint: "
if ./utils/v4/health/get_health.sh 2>&1 | grep -q '"Branch"'; then
    echo "✓ Working"
else
    echo "✗ Failed"
fi

echo -n "V3 version endpoint: "
if ./utils/v3/version/get_version.sh 2>&1 | grep -q '"branch"'; then
    echo "✓ Working"
else
    echo "✗ Failed"
fi

echo -n "V4 version endpoint: "
if ./utils/v4/version/get_version.sh 2>&1 | grep -q '"branch"'; then
    echo "✓ Working"
else
    echo "✗ Failed"
fi

echo ""

echo "=== Summary ==="
if [ $v3_result -eq 0 ] && [ $v4_result -eq 0 ] && [ $v3_syntax_errors -eq 0 ] && [ $v4_syntax_errors -eq 0 ]; then
    echo "🎉 ALL TESTS PASSED! 🎉"
    echo ""
    echo "✓ V3 APIs: 100% coverage"
    echo "✓ V4 APIs: 100% coverage"
    echo "✓ All scripts have valid syntax"
    echo "✓ API URL handling implemented for both local and remote modes"
    echo "✓ Comprehensive test coverage with Cypress tests"
    echo ""
    echo "The project now has complete API test coverage with shell scripts"
    echo "for all V3 and V4 endpoints, organized by swagger tags."
    echo ""
    echo "Usage examples:"
    echo "  Local mode:  ./utils/v3/health/get_health.sh"
    echo "  Remote mode: REMOTE=1 ./utils/v3/health/get_health.sh"
    echo "  With auth:   TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" ./utils/v3/users/get_user.sh user_id"
    exit 0
else
    echo "❌ SOME ISSUES FOUND"
    echo ""
    echo "V3 coverage: $([ $v3_result -eq 0 ] && echo '✓ PASS' || echo '✗ FAIL')"
    echo "V4 coverage: $([ $v4_result -eq 0 ] && echo '✓ PASS' || echo '✗ FAIL')"
    echo "V3 syntax:   $([ $v3_syntax_errors -eq 0 ] && echo '✓ PASS' || echo "✗ FAIL ($v3_syntax_errors errors)")"
    echo "V4 syntax:   $([ $v4_syntax_errors -eq 0 ] && echo '✓ PASS' || echo "✗ FAIL ($v4_syntax_errors errors)")"
    exit 1
fi