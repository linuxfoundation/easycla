#!/bin/bash
# Create a CLA group (authenticated)
# Usage: ./create_cla_group.sh <foundation_sfid> <project_sfid> <cla_group_name> <description>
# Example: ./create_cla_group.sh a01234567890123456 a09876543210987654 "My CLA Group" "CLA group description"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./create_cla_group.sh <params>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ]
then
  echo "$0: you need to specify foundation_sfid, project_sfid, cla_group_name, and description as parameters"
  echo "Usage: $0 <foundation_sfid> <project_sfid> <cla_group_name> <description>"
  echo "Example: $0 a01234567890123456 a09876543210987654 \"My CLA Group\" \"CLA group description\""
  exit 1
fi

export foundation_sfid="$1"
export project_sfid="$2"
export cla_group_name="$3"
export description="$4"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "icla_enabled": true,
  "ccla_enabled": true,
  "ccla_requires_icla": true,
  "cla_group_description": "${description}",
  "cla_group_name": "${cla_group_name}",
  "foundation_sfid": "${foundation_sfid}",
  "project_sfid_list": ["${project_sfid}"],
  "template_fields": {
    "TemplateID": "fb4cc144-a76c-4c17-8a52-c648f158fded",
    "MetaFields": [
      {
        "description": "Project's Full Name.",
        "name": "Project Name",
        "templateVariable": "PROJECT_NAME",
        "value": "${cla_group_name}"
      },
      {
        "description": "The Full Entity Name of the Project.",
        "name": "Project Entity Name",
        "templateVariable": "PROJECT_ENTITY_NAME",
        "value": "${cla_group_name}"
      },
      {
        "description": "The E-Mail Address of the Person managing the CLA.",
        "name": "Contact Email Address",
        "templateVariable": "CONTACT_EMAIL",
        "value": "admin@example.com"
      }
    ]
  }
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/cla-group"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh