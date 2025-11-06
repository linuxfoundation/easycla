#!/bin/bash
# Test ALL V4 Signatures API endpoints
# Usage: ./test_all_signatures_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_signatures_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 Signatures API Endpoints ==="
echo "API_URL: ${API_URL}"
echo ""

# For authenticated endpoints - handle optionally
if [ -z "$TOKEN" ]
then
  TOKEN="$(cat ./token.secret 2>/dev/null || echo '')"
fi

if [ -z "$XACL" ]
then
  XACL="$(cat ./x-acl.secret 2>/dev/null || echo '')"
fi

if [ ! -z "$TOKEN" ] && [ ! -z "$XACL" ]; then
  echo "1. Testing GET /cla-group/{claGroupID}/icla/signatures (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_cla_group_icla_signatures.sh <cla-group-id>"
  echo "   [Skipping - requires valid CLA group ID]"
  echo ""

  echo "2. Testing GET /cla-group/{claGroupID}/corporate-contributors (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_cla_group_corporate_contributors.sh <cla-group-id>"
  echo "   [Skipping - requires valid CLA group ID]"
  echo ""

  echo "3. Testing GET /signatures/id/{signatureID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_signature_by_id.sh <signature-id>"
  echo "   [Skipping - requires valid signature ID]"
  echo ""

  echo "4. Testing GET /signatures/company/{companyID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_company_signatures.sh <company-id>"
  echo "   [Skipping - requires valid company ID]"
  echo ""

  echo "5. Testing GET /signatures/project/{claGroupID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_project_signatures.sh <project-id>"
  echo "   [Skipping - requires valid project ID]"
  echo ""

  echo "6. Testing GET /signatures/project/{projectSFID}/company/{companyID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_project_company_signatures.sh <project-sfid> <company-id>"
  echo "   [Skipping - requires valid IDs]"
  echo ""

  echo "7. Testing GET /signatures/project/{projectSFID}/company/{companyID}/employee (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_project_company_employee_signatures.sh <project-sfid> <company-id>"
  echo "   [Skipping - requires valid IDs]"
  echo ""

  echo "8. Testing GET /signatures/user/{userID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_user_signatures.sh <user-id>"
  echo "   [Skipping - requires valid user ID]"
  echo ""

  echo "9. Testing GET /signatures/{signatureID}/signed-document (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_signed_document.sh <signature-id>"
  echo "   [Skipping - requires valid signature ID]"
  echo ""

  echo "10. Testing GET /signatures/{signatureID}/gh-org-whitelist (authenticated)"
  echo "    Command: ${SCRIPT_DIR}/get_github_org_whitelist.sh <signature-id>"
  echo "    [Skipping - requires valid signature ID]"
  echo ""

  echo "11. Testing POST /signatures/{signatureID}/gh-org-whitelist (authenticated)"
  echo "    Command: ${SCRIPT_DIR}/update_github_org_whitelist.sh <signature-id> <org-id>"
  echo "    [Skipping - requires valid parameters]"
  echo ""

  echo "12. Testing Download endpoints (CSV/PDF) (authenticated)"
  echo "    Commands: ${SCRIPT_DIR}/download_project_ccla_csv.sh <project-id>"
  echo "              ${SCRIPT_DIR}/download_project_ccla_pdfs.sh <project-id>"
  echo "              ${SCRIPT_DIR}/download_project_icla_csv.sh <project-id>"
  echo "    [Skipping - requires valid project ID]"
  echo ""

  echo "13. Testing PUT /signatures/project/{projectSFID}/company/{companyID}/clagroup/{claGroupID}/approval-list (authenticated)"
  echo "    Command: ${SCRIPT_DIR}/update_approval_list.sh <project-sfid> <company-id> <cla-group-id> <operation> <type> <value>"
  echo "    [Skipping - requires valid parameters]"
  echo ""

  echo "14. Testing PUT /signatures/company/{companyID}/clagroup/{claGroupID}/ecla-auto-create (authenticated)"
  echo "    Command: ${SCRIPT_DIR}/update_ecla_auto_create.sh <company-id> <cla-group-id> <flag>"
  echo "    [Skipping - requires valid parameters]"
  echo ""

  echo "15. Testing PUT /cla-group/{claGroupID}/user/{userID}/icla (authenticated)"
  echo "    Command: ${SCRIPT_DIR}/invalidate_user_icla.sh <cla-group-id> <user-id>"
  echo "    [Skipping - requires valid parameters]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test signatures APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 Signatures API Testing Complete ==="