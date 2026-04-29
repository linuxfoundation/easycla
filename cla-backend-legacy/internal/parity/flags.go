// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package parity

import (
	"os"
	"strings"
)

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}

var (
	// EASYCLA_PARITY_FLAG: when true, send company CCLA emails instead of preserving the legacy Python crash path.
	FixRequestCompanyCclaV2 = envBool("EASYCLA_FIX_REQUEST_COMPANY_CCLA_V2")

	// EASYCLA_PARITY_FLAG: when true, allow PUT /v1/signature to update document versions as DynamoDB numbers.
	EnablePutSignatureDocumentVersionUpdates = envBool("EASYCLA_ENABLE_PUT_SIGNATURE_DOCUMENT_VERSION_UPDATES")

	// EASYCLA_PARITY_FLAG: when true, allow PUT /v1/signature to accept newer *_allowlist alias params in addition to legacy *_whitelist params.
	EnablePutSignatureAllowlistAliasParams = envBool("EASYCLA_ENABLE_PUT_SIGNATURE_ALLOWLIST_ALIAS_PARAMS")

	// EASYCLA_PARITY_FLAG: when true, allow PUT /v1/signature to update fields that were not exposed by the legacy Python route.
	EnablePutSignatureAdditionalFieldUpdates = envBool("EASYCLA_ENABLE_PUT_SIGNATURE_ADDITIONAL_FIELD_UPDATES")

	// EASYCLA_PARITY_FLAG: when true, return filtered user/project/type signatures instead of preserving the legacy AttributeError.
	FixGetSignaturesUserProjectTypeV1 = envBool("EASYCLA_FIX_GET_SIGNATURES_USER_PROJECT_TYPE_V1")

	// EASYCLA_PARITY_FLAG: when true, honor the requested major/minor document version instead of always serving the latest.
	FixGetProjectDocumentMatchingVersionV1 = envBool("EASYCLA_FIX_GET_PROJECT_DOCUMENT_MATCHING_VERSION_V1")

	// EASYCLA_PARITY_FLAG: when true, preserve the current major version on minor template bumps instead of resetting to the default 1.
	FixPostProjectDocumentTemplateV1Versioning = envBool("EASYCLA_FIX_POST_PROJECT_DOCUMENT_TEMPLATE_V1_VERSIONING")

	// EASYCLA_PARITY_FLAG: when true, require role scope across every project mapping in the CLA group.
	FixUserServiceHasRoleAllMappings = envBool("EASYCLA_FIX_USER_SERVICE_HAS_ROLE_ALL_MAPPINGS")

	// EASYCLA_PARITY_FLAG: when true, skip waiting in GET /v2/return-url/{signature_id} until v2 company CLA managers
	DisableReturnURLCompanyManagerWait = envBool("EASYCLA_DISABLE_RETURN_URL_COMPANY_MANAGER_WAIT")

	// EASYCLA_PARITY_FLAG: when true, return signature_id instead of preserving the legacy project_id error key bug.
	FixAddClaManagerV1NotFoundErrorKey = envBool("EASYCLA_FIX_ADD_CLA_MANAGER_V1_NOT_FOUND_ERROR_KEY")
)
