#!/bin/bash

# Check V3 API coverage against cla.v1.compiled.yaml
# This script parses the Swagger file and checks if corresponding shell scripts exist

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SWAGGER_FILE="${PROJECT_ROOT}/cla-backend-go/swagger/cla.v1.compiled.yaml"

if [[ ! -f "$SWAGGER_FILE" ]]; then
    echo "Error: Swagger file not found at $SWAGGER_FILE"
    exit 1
fi

echo "=== V3 API Coverage Analysis ==="
echo "Swagger file: $SWAGGER_FILE"
echo "Utils directory: ${SCRIPT_DIR}"
echo ""

# Use Python to parse YAML and extract API information
python3 -c "
import yaml
import sys
import os
import re
from collections import defaultdict

with open('$SWAGGER_FILE', 'r') as f:
    swagger = yaml.safe_load(f)

apis = []
tag_counts = defaultdict(int)
tag_apis = defaultdict(list)

def camel_to_snake(name):
    s1 = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    return re.sub('([a-z0-9])([A-Z])', r'\1_\2', s1).lower()

def generate_script_name(operation_id, path, method):
    if operation_id:
        name = camel_to_snake(operation_id)
    else:
        clean_path = re.sub(r'/{[^}]+}', '', path.strip('/'))
        if not clean_path:
            clean_path = path.strip('/').replace('/', '_')
        name = re.sub(r'[^a-z0-9_]', '_', clean_path.lower())
        name = re.sub(r'_+', '_', name)
    
    method_lower = method.lower()
    if method_lower == 'get':
        if not name.startswith('get_') and not name.startswith('list_'):
            if 'list' in (operation_id.lower() if operation_id else ''):
                name = f'list_{name}'
            else:
                name = f'get_{name}'
    elif method_lower == 'post':
        if not name.startswith('post_') and not name.startswith('create_'):
            if 'create' in (operation_id.lower() if operation_id else ''):
                name = f'create_{name}'
            else:
                name = f'post_{name}'
    elif method_lower == 'put':
        if not name.startswith('put_') and not name.startswith('update_'):
            if 'update' in (operation_id.lower() if operation_id else ''):
                name = f'update_{name}'
            else:
                name = f'put_{name}'
    elif method_lower == 'delete':
        if not name.startswith('delete_'):
            name = f'delete_{name}'
    
    return f'{name}.sh'

for path, methods in swagger.get('paths', {}).items():
    for method, details in methods.items():
        if method.lower() in ['get', 'post', 'put', 'delete', 'patch']:
            tags = details.get('tags', ['untagged'])
            for tag in tags:
                tag_clean = tag.lower().replace(' ', '-').replace('_', '-')
                
                operation_id = details.get('operationId', '')
                script_name = generate_script_name(operation_id, path, method)
                
                apis.append({
                    'tag': tag_clean,
                    'method': method.upper(),
                    'path': path,
                    'script_name': script_name,
                    'operation_id': operation_id
                })
                
                tag_counts[tag_clean] += 1
                tag_apis[tag_clean].append(script_name)

print(f'Total APIs found: {len(apis)}')
print(f'Total tags: {len(tag_counts)}')
print('')

# Scan existing scripts
existing_scripts = 0
missing_scripts = []
existing_tags = set()
found_scripts = defaultdict(list)

# Scan all script files in tag directories
for tag in tag_counts.keys():
    tag_dir = f\"$SCRIPT_DIR/{tag}\"
    if os.path.exists(tag_dir):
        for file in os.listdir(tag_dir):
            if file.endswith('.sh') and not file.startswith('test_all_'):
                found_scripts[tag].append(file)

# Check coverage
for api in apis:
    tag = api['tag']
    script_name = api['script_name']
    
    if script_name in found_scripts[tag]:
        existing_scripts += 1
        existing_tags.add(tag)
    else:
        missing_scripts.append({
            'script': f\"{tag}/{script_name}\",
            'api': f\"{api['method']} {api['path']}\",
            'operation_id': api['operation_id']
        })

coverage_percent = (existing_scripts / len(apis)) * 100

print(f'Existing scripts: {existing_scripts}/{len(apis)} ({coverage_percent:.1f}%)')
print(f'Tags with scripts: {len(existing_tags)}/{len(tag_counts)}')
print('')

# Show tag summary ordered by number of APIs (descending)
print('=== Tag Summary (ordered by API count) ===')
sorted_tags = sorted(tag_counts.items(), key=lambda x: x[1], reverse=True)

for tag, api_count in sorted_tags:
    tag_dir = f\"$SCRIPT_DIR/{tag}\"
    script_count = len(found_scripts[tag])
    tag_coverage = (script_count / api_count * 100) if api_count > 0 else 0
    status = '✓' if tag_coverage == 100 else '✗' if tag_coverage == 0 else '~'
    print(f'{status} {tag}: {script_count}/{api_count} scripts ({tag_coverage:.0f}%) - {api_count} APIs')

if missing_scripts:
    print('')
    print('=== Missing Scripts by Tag ===')
    missing_by_tag = defaultdict(list)
    for missing in missing_scripts:
        tag = missing['script'].split('/')[0]
        missing_by_tag[tag].append(missing)
    
    for tag in sorted(missing_by_tag.keys()):
        print(f'\\n{tag}: {len(missing_by_tag[tag])} missing')
        for missing in missing_by_tag[tag][:5]:  # Show first 5 per tag
            print(f'  Missing: {missing[\"script\"]} for {missing[\"api\"]}')
        if len(missing_by_tag[tag]) > 5:
            print(f'  ... and {len(missing_by_tag[tag]) - 5} more')

print('')
print(f'Coverage: {coverage_percent:.1f}%')

# List next tags to implement (uncovered tags with most APIs)
print('')
print('=== Next Tags to Implement (most APIs first) ===')
uncovered_tags = []
for tag, api_count in sorted_tags:
    if tag not in existing_tags:
        uncovered_tags.append((tag, api_count))

for tag, api_count in uncovered_tags[:5]:
    print(f'  {tag}: {api_count} APIs')

sys.exit(0 if coverage_percent == 100.0 else 1)
"