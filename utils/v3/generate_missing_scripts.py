#!/usr/bin/env python3

import yaml
import os
import re
import json
from collections import defaultdict

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

def extract_path_parameters(path):
    """Extract path parameters like {id}, {companyID} from path"""
    return re.findall(r'\{([^}]+)\}', path)

def needs_auth(tags, path):
    """Determine if endpoint needs authentication"""
    # Public endpoints that don't need auth
    public_paths = ['/ops/health', '/ops/version', '/api-docs', '/swagger.json', '/user-compat']
    return not any(pub_path in path for pub_path in public_paths)

def generate_script_content(api_info):
    tag = api_info['tag']
    method = api_info['method']
    path = api_info['path']
    script_name = api_info['script_name']
    operation_id = api_info['operation_id']
    
    path_params = extract_path_parameters(path)
    requires_auth = needs_auth([tag], path)
    
    method_lower = method.lower()
    method_upper = method.upper()
    
    # Generate parameter validation
    param_checks = []
    param_usage = []
    param_exports = []
    param_example = []
    
    for i, param in enumerate(path_params, 1):
        param_snake = camel_to_snake(param)
        param_checks.append(f'[ -z "${i}" ]')
        param_usage.append(f'<{param_snake}>')
        param_exports.append(f'export {param_snake}="${i}"')
        param_example.append(f'param{i}')
    
    # Build script content
    content = [
        '#!/bin/bash',
        f'# {method_upper} {path}',
    ]
    
    # Add description based on operation ID or path
    if operation_id:
        desc = operation_id.replace('_', ' ').replace('-', ' ').title()
        content.append(f'# {desc}' + (' (authenticated)' if requires_auth else ' (public endpoint, no auth required)'))
    else:
        content.append(f'# API endpoint' + (' (authenticated)' if requires_auth else ' (public endpoint, no auth required)'))
    
    # Add usage
    usage_line = f"# Usage: ./{script_name}"
    if param_usage:
        usage_line += f" {' '.join(param_usage)}"
        
        # Add example
        example_line = f"# Example: ./{script_name}"
        if param_example:
            example_line += f" {' '.join(param_example)}"
        content.append(example_line)
        
        # Add authenticated usage example
        if requires_auth:
            auth_example = f'# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./{script_name}'
            if param_usage:
                auth_example += f" {' '.join(param_usage)}"
            content.append(auth_example)
    else:
        # Simple usage examples
        content.append(usage_line)
        content.append(f'# API_URL=http://localhost:5001 ./{script_name}')
        content.append(f'# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./{script_name}')
    
    content.append('')
    
    # Add parameter validation
    if param_checks:
        content.append(f"if {' || '.join(param_checks)}")
        content.append("then")
        content.append(f'  echo "$0: you need to specify {", ".join([camel_to_snake(p) for p in path_params])} as parameter{"s" if len(path_params) > 1 else ""}"')
        content.append(f'  echo "Usage: $0 {" ".join(param_usage)}"')
        if param_example:
            content.append(f'  echo "Example: $0 {" ".join(param_example)}"')
        content.append('  exit 1')
        content.append('fi')
        content.append('')
        
        # Export parameters
        content.extend(param_exports)
        content.append('')
    
    # Standard boilerplate
    content.extend([
        'SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"',
        ''
    ])
    
    # Add authentication if needed
    if requires_auth:
        content.extend([
            '# Handle authentication',
            '. ./utils/shared/handle_auth.sh',
            ''
        ])
    
    # Add API URL handling
    content.extend([
        '# Handle API URL',
        '. ${SCRIPT_DIR}/../shared/handle_api_url.sh',
        ''
    ])
    
    # Build API path with parameter substitution
    api_path = path
    for param in path_params:
        param_snake = camel_to_snake(param)
        api_path = api_path.replace(f'{{{param}}}', f'${{{param_snake}}}')
    
    # Add payload for POST/PUT requests
    needs_payload = method_lower in ['post', 'put', 'patch']
    if needs_payload:
        content.extend([
            '# Build JSON payload',
            'PAYLOAD=$(cat <<EOF',
            '{',
            '  "example": "value"',
            '}',
            'EOF',
            ')',
            ''
        ])
    
    # Set up curl execution
    content.append('# Set up curl execution')
    content.append(f'API="${{API_URL}}/v3{api_path}"')
    
    if requires_auth:
        if needs_payload:
            curl_cmd = f'curl -s -X{method_upper} -H \\"Content-Type: application/json\\" -H \\"X-ACL: ${{XACL}}\\" -H \\"Authorization: Bearer ${{TOKEN}}\\" -d \'${{PAYLOAD}}\''
        else:
            curl_cmd = f'curl -s -X{method_upper} -H \\"Content-Type: application/json\\" -H \\"X-ACL: ${{XACL}}\\" -H \\"Authorization: Bearer ${{TOKEN}}\\"'
    else:
        if needs_payload:
            curl_cmd = f'curl -s -X{method_upper} -H \\"Content-Type: application/json\\" -d \'${{PAYLOAD}}\''
        else:
            curl_cmd = f'curl -s -X{method_upper} -H \\"Content-Type: application/json\\"'
    
    content.append(f'CURL_CMD="{curl_cmd}"')
    
    # Use jq for JSON responses except for docs endpoints
    use_jq = 'false' if 'docs' in tag and ('api-docs' in path or 'swagger' in path) else 'true'
    content.append(f'USE_JQ={use_jq}')
    content.append('. ./utils/shared/handle_curl_execution.sh')
    
    return '\n'.join(content) + '\n'

def main():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.join(script_dir, '../..')
    swagger_file = os.path.join(project_root, 'cla-backend-go/swagger/cla.v1.compiled.yaml')
    
    with open(swagger_file, 'r') as f:
        swagger = yaml.safe_load(f)
    
    apis = []
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
    
    # Check what exists and create missing scripts
    created_count = 0
    for api in apis:
        tag_dir = os.path.join(script_dir, api['tag'])
        script_path = os.path.join(tag_dir, api['script_name'])
        
        if not os.path.exists(script_path):
            os.makedirs(tag_dir, exist_ok=True)
            
            content = generate_script_content(api)
            
            with open(script_path, 'w') as f:
                f.write(content)
            
            # Make executable
            os.chmod(script_path, 0o755)
            
            print(f"Created: {api['tag']}/{api['script_name']}")
            created_count += 1
    
    print(f"\nCreated {created_count} missing scripts")
    return created_count

if __name__ == '__main__':
    main()