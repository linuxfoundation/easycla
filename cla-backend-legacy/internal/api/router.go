// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/middleware"
)

func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.StripSlashes)
	// Legacy-style logging lines for request paths (used by existing log rollups).
	r.Use(middleware.RequestLog)
	r.Use(middleware.CORS)
	// Legacy Python uses server-side sessions via cookie "cla-sid" (hug SessionMiddleware).
	// Needed for GitHub OAuth flows:
	//   - GET /v2/user-from-session
	//   - GET /v2/github/installation
	//   - GET /v2/repository-provider/github/sign/...
	r.Use(middleware.SessionMiddleware(h.kv))

	// Return legacy-compatible 404/405 for unknown or method-mismatched routes.
	r.NotFound(h.NotFound)
	r.MethodNotAllowed(h.MethodNotAllowed)

	RegisterRoutes(r, h)
	return r
}

func RegisterRoutes(r chi.Router, h *Handlers) {
	// Versioned APIs
	r.Route("/v1", func(r chi.Router) {
		// Python: separate lambda handlers in serverless.yml
		r.HandleFunc("/salesforce/projects", h.SalesforceGetProjectsV1)
		r.HandleFunc("/salesforce/project", h.SalesforceGetProjectV1)

		r.Post("/user/gerrit", h.PostOrGetUserGerritV1)

		r.Get("/user/{user_id}/signatures", h.GetUserSignaturesV1)

		r.Get("/users/company/{user_company_id}", h.GetUsersCompanyV1)

		r.Get("/user/{user_id}/project/{project_id}/last-signature/{company_id}", h.GetUserProjectCompanyLastSignatureV1)

		r.Get("/signature/{signature_id}", h.GetSignatureV1)

		r.Post("/signature", h.PostSignatureV1)

		r.Put("/signature", h.PutSignatureV1)

		r.Delete("/signature/{signature_id}", h.DeleteSignatureV1)

		r.Get("/signatures/user/{user_id}", h.GetSignaturesUserV1)

		r.Get("/signatures/user/{user_id}/project/{project_id}", h.GetSignaturesUserProjectV1)

		r.Get("/signatures/user/{user_id}/project/{project_id}/type/{signature_type}", h.GetSignaturesUserProjectTypeV1)

		r.Get("/signatures/company/{company_id}", h.GetSignaturesCompanyV1)

		r.Get("/signatures/project/{project_id}", h.GetSignaturesProjectV1)

		r.Get("/signatures/company/{company_id}/project/{project_id}", h.GetSignaturesProjectCompanyV1)

		r.Get("/signatures/company/{company_id}/project/{project_id}/employee", h.GetProjectEmployeeSignaturesV1)

		r.Get("/signature/{signature_id}/manager", h.GetClaManagersV1)

		r.Post("/signature/{signature_id}/manager", h.AddClaManagerV1)

		r.Delete("/signature/{signature_id}/manager/{lfid}", h.RemoveClaManagerV1)

		r.Get("/repository/{repository_id}", h.GetRepositoryV1)

		r.Post("/repository", h.PostRepositoryV1)

		r.Put("/repository", h.PutRepositoryV1)

		r.Delete("/repository/{repository_id}", h.DeleteRepositoryV1)

		r.Get("/company", h.GetCompaniesV1)

		r.Get("/company/{company_id}/project/unsigned", h.GetUnsignedProjectsForCompanyV1)

		r.Post("/company", h.PostCompanyV1)

		r.Put("/company", h.PutCompanyV1)

		r.Delete("/company/{company_id}", h.DeleteCompanyV1)

		r.Put("/company/{company_id}/import/whitelist/csv", h.PutCompanyAllowlistCsvV1)

		r.Get("/companies/{manager_id}", h.GetManagerCompaniesV1)

		r.Get("/project", h.GetProjectsV1)

		r.Get("/project/{project_id}/manager", h.GetProjectManagersV1)

		r.Post("/project/{project_id}/manager", h.AddProjectManagerV1)

		r.Delete("/project/{project_id}/manager/{lfid}", h.RemoveProjectManagerV1)

		r.Get("/project/external/{project_external_id}", h.GetExternalProjectV1)

		r.Post("/project", h.PostProjectV1)

		r.Put("/project", h.PutProjectV1)

		r.Delete("/project/{project_id}", h.DeleteProjectV1)

		r.Get("/project/{project_id}/repositories", h.GetProjectRepositoriesV1)

		r.Get("/project/{project_id}/repositories_group_by_organization", h.GetProjectRepositoriesGroupByOrganizationV1)

		r.Get("/project/{project_id}/configuration_orgs_and_repos", h.GetProjectConfigurationOrgsAndReposV1)

		r.Get("/project/{project_id}/document/{document_type}/pdf/{document_major_version}/{document_minor_version}", h.GetProjectDocumentMatchingVersionV1)

		r.Post("/project/{project_id}/document/{document_type}", h.PostProjectDocumentV1)

		r.Post("/project/{project_id}/document/template/{document_type}", h.PostProjectDocumentTemplateV1)

		r.Delete("/project/{project_id}/document/{document_type}/{major_version}/{minor_version}", h.DeleteProjectDocumentV1)

		r.Post("/request-corporate-signature", h.RequestCorporateSignatureV1)

		r.Get("/github/organizations", h.GetGithubOrganizationsV1)

		r.Get("/github/organizations/{organization_name}", h.GetGithubOrganizationV1)

		r.Get("/github/organizations/{organization_name}/repositories", h.GetGithubOrganizationReposV1)

		r.Get("/sfdc/{sfid}/github/organizations", h.GetGithubOrganizationBySfidV1)

		r.Post("/github/organizations", h.PostGithubOrganizationV1)

		r.Delete("/github/organizations/{organization_name}", h.DeleteOrganizationV1)

		r.Post("/github/validate", h.GithubOrganizationValidationV1)

		r.Get("/github/check/namespace/{namespace}", h.GithubCheckNamespaceV1)

		r.Get("/github/get/namespace/{namespace}", h.GithubGetNamespaceV1)

		r.Get("/project/{project_id}/gerrits", h.GetProjectGerritInstanceV1)

		r.Post("/gerrit", h.CreateGerritInstanceV1)

		r.Delete("/gerrit/{gerrit_id}", h.DeleteGerritInstanceV1)

		r.Get("/project/logo/{project_sfdc_id}", h.UploadLogoV1)

		r.Post("/project/permission", h.AddProjectPermissionV1)

		r.Delete("/project/permission", h.RemoveProjectPermissionV1)

		r.Post("/company/permission", h.AddCompanyPermissionV1)

		r.Delete("/company/permission", h.RemoveCompanyPermissionV1)

		r.Get("/events", h.SearchEventsV1)

		r.Get("/events/{event_id}", h.GetEventV1)

		r.Post("/events", h.CreateEventV1)

	})

	r.Route("/v2", func(r chi.Router) {

		r.Get("/health", h.GetHealthV2)

		r.Get("/user/{user_id}", h.GetUserV2)

		r.Post("/user/{user_id}/request-company-whitelist/{company_id}", h.RequestCompanyAllowlistV2)

		r.Post("/user/{user_id}/invite-company-admin", h.InviteCompanyAdminV2)

		r.Post("/user/{user_id}/request-company-ccla", h.RequestCompanyCclaV2)

		r.Get("/user/{user_id}/active-signature", h.GetUserActiveSignatureV2)

		r.Get("/user/{user_id}/project/{project_id}/last-signature", h.GetUserProjectLastSignatureV2)

		r.Get("/company", h.GetAllCompaniesV2)

		r.Get("/company/{company_id}", h.GetCompanyV2)

		r.Get("/project/{project_id}", h.GetProjectV2)

		r.Get("/project/{project_id}/document/{document_type}", h.GetProjectDocumentV2)

		r.Get("/project/{project_id}/document/{document_type}/pdf", h.GetProjectDocumentRawV2)

		r.Get("/project/{project_id}/companies", h.GetProjectCompaniesV2)

		r.Post("/request-individual-signature", h.RequestIndividualSignatureV2)

		r.Post("/request-employee-signature", h.RequestEmployeeSignatureV2)

		r.Post("/check-prepare-employee-signature", h.CheckAndPrepareEmployeeSignatureV2)

		r.Post("/signed/individual/{installation_id}/{github_repository_id}/{change_request_id}", h.PostIndividualSignedV2)

		r.Post("/signed/gitlab/individual/{user_id}/{organization_id}/{gitlab_repository_id}/{merge_request_id}", h.PostIndividualSignedGitlabV2)

		r.Post("/signed/gerrit/individual/{user_id}", h.PostIndividualSignedGerritV2)

		r.Post("/signed/corporate/{project_id}/{company_id}", h.PostCorporateSignedV2)

		r.Get("/return-url/{signature_id}", h.GetReturnUrlV2)

		r.Post("/send-authority-email", h.SendAuthorityEmailV2)

		r.Get("/repository-provider/{provider}/sign/{installation_id}/{github_repository_id}/{change_request_id}", h.SignRequestV2)

		r.Get("/repository-provider/{provider}/oauth2_redirect", h.Oauth2RedirectV2)

		r.Post("/repository-provider/{provider}/activity", h.ReceivedActivityV2)

		r.Get("/github/installation", h.GithubOauth2CallbackV2)

		r.Post("/github/installation", h.GithubAppInstallationV2)

		r.Post("/github/activity", h.GithubAppActivityV2)

		r.Get("/gerrit/{gerrit_id}", h.GetGerritInstanceV2)

		r.Get("/gerrit/{gerrit_id}/{contract_type}/agreementUrl.html", h.GetAgreementHtmlV2)

		r.Get("/user-from-session", h.UserFromSessionV2)

		r.Get("/user-from-token", h.UserFromTokenV2)

		r.Post("/clear-cache", h.ClearCacheV2)

	})
}
