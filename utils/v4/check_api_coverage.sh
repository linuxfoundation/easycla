#!/bin/bash
# Script to check V4 API coverage by parsing swagger and comparing with available shell scripts
# Usage: ./utils/v4/check_api_coverage.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SWAGGER_FILE="${PROJECT_ROOT}/cla-backend-go/swagger/cla.v2.compiled.yaml"
UTILS_V4_DIR="${PROJECT_ROOT}/utils/v4"

echo "======================================="
echo "V4 API Coverage Check"
echo "======================================="
echo "Project Root: ${PROJECT_ROOT}"
echo "Swagger File: ${SWAGGER_FILE}"
echo "Utils V4 Dir: ${UTILS_V4_DIR}"
echo ""

if [ ! -f "$SWAGGER_FILE" ]; then
    echo "ERROR: Swagger file not found: $SWAGGER_FILE"
    exit 1
fi

# Extract API paths and methods from swagger
echo "Parsing swagger file..."
TEMP_FILE=$(mktemp)

# Use Python to parse YAML and extract paths/methods/tags
SWAGGER_FILE="$SWAGGER_FILE" python3 << 'EOF' > "$TEMP_FILE"
import yaml
import sys
import os

swagger_file = os.environ['SWAGGER_FILE']
try:
    with open(swagger_file, 'r') as f:
        data = yaml.safe_load(f)
    
    paths = data.get('paths', {})
    apis = []
    
    for path, methods in paths.items():
        for method, details in methods.items():
            if method.upper() in ['GET', 'POST', 'PUT', 'DELETE', 'PATCH']:
                tags = details.get('tags', ['unknown'])
                tag = tags[0] if tags else 'unknown'
                
                # Clean up path for script naming
                path_clean = path.replace('/', '_').replace('{', '').replace('}', '').strip('_')
                if path_clean == '':
                    path_clean = 'root'
                
                # Create expected script name
                script_name = f"{path_clean}_{method.lower()}.sh"
                apis.append(f"{tag}:{path}:{method.upper()}:{script_name}")
    
    for api in sorted(apis):
        print(api)

except Exception as e:
    print(f"Error parsing swagger: {e}", file=sys.stderr)
    sys.exit(1)
EOF

if [ ! -s "$TEMP_FILE" ]; then
    echo "ERROR: Failed to parse swagger file"
    rm -f "$TEMP_FILE"
    exit 1
fi

echo "Found $(wc -l < "$TEMP_FILE") APIs in swagger file"
echo ""

# Check coverage
total_apis=0
covered_apis=0
missing_apis=()

echo "Checking API coverage..."
echo "Format: [✓/✗] TAG PATH METHOD -> SCRIPT"
echo ""

while IFS=: read -r tag path method script_name; do
    total_apis=$((total_apis + 1))
    
    # Look for script in appropriate tag directory
    script_path="${UTILS_V4_DIR}/${tag}/${script_name}"
    
    if [ -f "$script_path" ]; then
        echo "✓ ${tag} ${path} ${method} -> ${tag}/${script_name}"
        covered_apis=$((covered_apis + 1))
    else
        echo "✗ ${tag} ${path} ${method} -> ${tag}/${script_name} (MISSING)"
        missing_apis+=("${tag}:${path}:${method}:${script_name}")
    fi
done < "$TEMP_FILE"

rm -f "$TEMP_FILE"

echo ""
echo "======================================="
echo "Coverage Summary"
echo "======================================="
echo "Total APIs in swagger: $total_apis"
echo "Covered APIs: $covered_apis"
echo "Missing APIs: $((total_apis - covered_apis))"

if [ $total_apis -gt 0 ]; then
    coverage_percent=$(( (covered_apis * 100) / total_apis ))
    echo "Coverage: ${coverage_percent}%"
else
    coverage_percent=0
fi

if [ ${#missing_apis[@]} -gt 0 ]; then
    echo ""
    echo "Missing API Scripts:"
    echo "==================="
    for missing in "${missing_apis[@]}"; do
        IFS=: read -r tag path method script_name <<< "$missing"
        echo "  ${tag}/${script_name} for ${method} ${path}"
    done
fi

echo ""
if [ $coverage_percent -eq 100 ]; then
    echo "🎉 Perfect! 100% API coverage achieved!"
    exit 0
else
    echo "⚠️  Coverage is ${coverage_percent}% - $(( 100 - coverage_percent ))% missing"
    exit 1
fi