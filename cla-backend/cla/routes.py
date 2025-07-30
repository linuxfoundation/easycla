# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
The entry point for the CLA service. Lays out all routes and controller functions.
"""

import hug
import requests
from falcon import HTTP_401, HTTP_400, HTTP_OK, HTTP_500, Response
from hug.middleware import LogMiddleware

import cla
import cla.auth
import cla.controllers.company
import cla.controllers.event
import cla.controllers.gerrit
import cla.controllers.github
import cla.controllers.project
import cla.controllers.project_logo
import cla.controllers.repository
import cla.controllers.repository_service
import cla.controllers.signature
import cla.controllers.signing
import cla.controllers.user
import cla.hug_types
import cla.salesforce
from cla.controllers.github import get_github_activity_action
from cla.controllers.github_activity import v4_easycla_github_activity
from cla.controllers.project_cla_group import get_project_cla_group
from cla.models.dynamo_models import Repository, Gerrit
from cla.project_service import ProjectService
from cla.utils import (
    get_supported_repository_providers,
    get_supported_document_content_types,
    get_session_middleware,
    get_log_middleware
)


#
# Middleware
#

@hug.request_middleware()
def process_data(request, response):
    """
    This middleware is needed here to copy the stream so we can re-read it
    later on in the other handlers, currently only active on /github/activity
    endpoint because it's an expensive operation.
    """
    cla.log.info('LG:api-request-path:' + request.path)
    if "/github/activity" in request.path:
        body = request.bounded_stream.read()
        request.bounded_stream.read = lambda: body


# CORS Middleware
@hug.response_middleware()
def process_data(request, response, resource):
    # response.set_header('Access-Control-Allow-Origin', cla.conf['ALLOW_ORIGIN'])
    response.set_header("Access-Control-Allow-Origin", "*")
    response.set_header("Access-Control-Allow-Credentials", "true")
    response.set_header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
    response.set_header("Access-Control-Allow-Headers", "Content-Type, Authorization")


@hug.directive()
def check_auth(request=None, **kwargs):
    """Returns the authenticated user"""
    # LG:
    # return request and cla.auth.fake_authenticate_user(request.headers)
    return request and cla.auth.authenticate_user(request.headers)


@hug.exception(cla.auth.AuthError)
def handle_auth_error(exception, response=None, **kwargs):
    """Handles authentication errors"""
    response.status = HTTP_401
    return exception.response


#
# Health check route.
#
@hug.get("/health", versions=2)
def get_health(request):
    """
    GET: /health

    Returns a basic health check on the CLA system.
    """
    cla.salesforce.get_projects(request, "")
    request.context["session"]["health"] = "up"
    return request.headers


#
# User routes.
#


# LG: This is ported to golang and no longer used in dev (still used in prod)
@hug.get("/user/{user_id}", versions=2)
def get_user(user_id: hug.types.uuid):
    """
    GET: /user/{user_id}

    Returns the requested user data based on ID.
    """
    # try:
    #     auth_user = check_auth(request)
    #     cla.log.debug(f'validated request for: {auth_user}')
    # except cla.auth.AuthError as auth_err:
    #     if auth_err.response == "missing authorization header":
    #         cla.log.info("getting github user: {}".format(user_id))
    #     else:
    #         raise auth_err

    return cla.controllers.user.get_user(user_id=user_id)


@hug.post("/user/gerrit", versions=1)
def post_or_get_user_gerrit(auth_user: check_auth):
    """
    GET: /user/gerrit

    For a Gerrit user, there is a case where a user with an lfid may be a user in the db.
    An endpoint to get a userId for gerrit, or create and retrieve the userId if not existent.
    """
    return cla.controllers.user.get_or_create_user(auth_user).to_dict()


@hug.get("/user/{user_id}/signatures", versions=1)
def get_user_signatures(auth_user: check_auth, user_id: hug.types.uuid):
    """
    GET: /user/{user_id}/signatures

    Returns a list of signatures associated with a user.
    """
    return cla.controllers.user.get_user_signatures(user_id)


@hug.get("/users/company/{user_company_id}", versions=1)
def get_users_company(auth_user: check_auth, user_company_id: hug.types.uuid):
    """
    GET: /users/company/{user_company_id}

    Returns a list of users associated with an company.

    TODO: Should probably not simply be auth only - need some role check?
    """
    return cla.controllers.user.get_users_company(user_company_id)


@hug.post("/user/{user_id}/request-company-whitelist/{company_id}", versions=2)
def request_company_whitelist(
        user_id: hug.types.uuid,
        company_id: hug.types.uuid,
        user_name: hug.types.text,
        user_email: cla.hug_types.email,
        project_id: hug.types.uuid,
        message=None,
        recipient_name: hug.types.text = None,
        recipient_email: cla.hug_types.email = None,
):
    """
    POST: /user/{user_id}/request-company-whitelist/{company_id}

    DATA: {'user_email': <email-selection>, 'message': 'custom message to manager'}

    Performs the necessary actions (ie: send email to manager) when the specified user requests to
    be added the the specified company's whitelist.
    """
    return cla.controllers.user.request_company_whitelist(
        user_id, str(company_id), str(user_name), str(user_email), str(project_id), message,
        str(recipient_name), str(recipient_email),
    )


@hug.post("/user/{user_id}/invite-company-admin", versions=2)
def invite_company_admin(
        user_id: hug.types.uuid,
        contributor_name: hug.types.text,
        contributor_email: cla.hug_types.email,
        cla_manager_name: hug.types.text,
        cla_manager_email: cla.hug_types.email,
        project_name: hug.types.text,
        company_name: hug.types.text,
):
    """
    POST: /user/{user_id}/invite-company-admin

    DATA: {
            'contributor_name': 'Sally Field',
            'contributor_email': 'user@example.com',
            'cla_manager_name': 'John Doe',
            'cla_manager_email': 'admin@example.com',
            'project_name': 'Project Name',
            'company_name': 'Company Name'
        }

    Sends an Email to the prospective CLA Manager to sign up through the ccla console.
    """
    return cla.controllers.user.invite_cla_manager(
        str(user_id), str(contributor_name), str(contributor_email),
        str(cla_manager_name), str(cla_manager_email),
        str(project_name), str(company_name)
    )


@hug.post("/user/{user_id}/request-company-ccla", versions=2)
def request_company_ccla(
        user_id: hug.types.uuid, user_email: cla.hug_types.email, company_id: hug.types.uuid,
        project_id: hug.types.uuid,
):
    """
    POST: /user/{user_id}/request_company_ccla

    Sends an Email to an admin of an existing company to sign a CCLA.
    """
    return cla.controllers.user.request_company_ccla(str(user_id), str(user_email), str(company_id), str(project_id))


# Company Admin's don't have a role within EasyCLA v1
# @hug.post("/user/{user_id}/company/{company_id}/request-access", versions=2)
# def request_company_admin_access(user_id: hug.types.uuid, company_id: hug.types.uuid):
#    """
#     POST: /user/{user_id}/company/{company_id}/request-access
#
#     Sends an Email for a user requesting access to be on Company ACL.
#     """
#     return cla.controllers.user.request_company_admin_access(str(user_id), str(company_id))


# LG: This is ported to golang and no longer used in dev (still used in prod)
@hug.get("/user/{user_id}/active-signature", versions=2)
def get_user_active_signature(user_id: hug.types.uuid):
    """
    GET: /user/{user_id}/active-signature

    Returns all metadata associated with a user's active signature.

    {'user_id': <user-id>,
     'project_id': <project-id>,
     'repository_id': <repository-id>,
     'pull_request_id': <PR>,
     'return_url': <url-where-user-initiated-signature-from>'}

    Returns null if the user does not have an active signature.
    """
    return cla.controllers.user.get_active_signature(user_id)


@hug.get("/user/{user_id}/project/{project_id}/last-signature", versions=2)
def get_user_project_last_signature(user_id: hug.types.uuid, project_id: hug.types.uuid):
    """
    GET: /user/{user_id}/project/{project_id}/last-signature

    Returns the user's latest ICLA signature for the project specified.
    """
    return cla.controllers.user.get_user_project_last_signature(user_id, project_id)


@hug.get("/user/{user_id}/project/{project_id}/last-signature/{company_id}", versions=1)
def get_user_project_company_last_signature(
        user_id: hug.types.uuid, project_id: hug.types.uuid, company_id: hug.types.uuid
):
    """
    GET: /user/{user_id}/project/{project_id}/last-signature/{company_id}

    Returns the user's latest employee signature for the project and company specified.
    """
    return cla.controllers.user.get_user_project_company_last_signature(user_id, project_id, company_id)


@hug.get("/signature/{signature_id}", versions=1)
def get_signature(auth_user: check_auth, signature_id: hug.types.uuid):
    """
    GET: /signature/{signature_id}

    Returns the CLA signature requested by UUID.
    """
    return cla.controllers.signature.get_signature(signature_id)


@hug.post(
    "/signature",
    versions=1,
    examples=" - {'signature_type': 'cla', 'signature_signed': true, \
                        'signature_embargo_acked': true, \
                        'signature_approved': true, 'signature_sign_url': 'http://sign.com/here', \
                        'signature_return_url': 'http://cla-system.com/signed', \
                        'signature_project_id': '<project-id>', \
                        'signature_reference_id': '<ref-id>', \
                        'signature_reference_type': 'individual'}",
)
def post_signature(
        auth_user: check_auth,  # pylint: disable=too-many-arguments
        signature_project_id: hug.types.uuid,
        signature_reference_id: hug.types.text,
        signature_reference_type: hug.types.one_of(["company", "user"]),
        signature_type: hug.types.one_of(["cla", "dco"]),
        signature_signed: hug.types.smart_boolean,
        signature_approved: hug.types.smart_boolean,
        signature_embargo_acked: hug.types.smart_boolean,
        signature_return_url: cla.hug_types.url,
        signature_sign_url: cla.hug_types.url,
        signature_user_ccla_company_id=None,
):
    """
    POST: /signature

    DATA: {'signature_type': 'cla',
           'signature_signed': true,
           'signature_approved': true,
           'signature_embargo_acked': true,
           'signature_sign_url': 'http://sign.com/here',
           'signature_return_url': 'http://cla-system.com/signed',
           'signature_project_id': '<project-id>',
           'signature_user_ccla_company_id': '<company-id>',
           'signature_reference_id': '<ref-id>',
           'signature_reference_type': 'individual'}

    signature_reference_type is either 'individual' or 'corporate', depending on the CLA type.
    signature_reference_id needs to reflect the user or company tied to this signature.

    Returns a CLA signatures that was created.
    """
    return cla.controllers.signature.create_signature(
        signature_project_id,
        signature_reference_id,
        signature_reference_type,
        signature_type=signature_type,
        signature_user_ccla_company_id=signature_user_ccla_company_id,
        signature_signed=signature_signed,
        signature_approved=signature_approved,
        signature_embargo_acked=signature_embargo_acked,
        signature_return_url=signature_return_url,
        signature_sign_url=signature_sign_url,
    )


@hug.put(
    "/signature",
    versions=1,
    examples=" - {'signature_id': '01620259-d202-4350-8264-ef42a861922d', \
                       'signature_type': 'cla', 'signature_signed': true, 'signature_embargo_acked': true}",
)
def put_signature(
        auth_user: check_auth,  # pylint: disable=too-many-arguments
        signature_id: hug.types.uuid,
        signature_project_id=None,
        signature_reference_id=None,
        signature_reference_type=None,
        signature_type=None,
        signature_signed=None,
        signature_approved=None,
        signature_embargo_acked=None,
        signature_return_url=None,
        signature_sign_url=None,
        domain_whitelist=None,
        email_whitelist=None,
        github_whitelist=None,
        github_org_whitelist=None,
):
    """
    PUT: /signature

    DATA: {'signature_id': '<signature-id>',
           'signature_type': 'cla', 'signature_signed': true, 'signature_embargo_acked': true}

    Supports all the fields as the POST equivalent.

    Returns the CLA signature that was just updated.
    """
    return cla.controllers.signature.update_signature(
        signature_id,
        auth_user=auth_user,
        signature_project_id=signature_project_id,
        signature_reference_id=signature_reference_id,
        signature_reference_type=signature_reference_type,
        signature_type=signature_type,
        signature_signed=signature_signed,
        signature_approved=signature_approved,
        signature_embargo_acked=signature_embargo_acked,
        signature_return_url=signature_return_url,
        signature_sign_url=signature_sign_url,
        domain_whitelist=domain_whitelist,
        email_whitelist=email_whitelist,
        github_whitelist=github_whitelist,
        github_org_whitelist=github_org_whitelist,
    )


@hug.delete("/signature/{signature_id}", versions=1)
def delete_signature(auth_user: check_auth, signature_id: hug.types.uuid):
    """
    DELETE: /signature/{signature_id}

    Deletes the specified signature.
    """
    # staff_verify(user)
    return cla.controllers.signature.delete_signature(signature_id)


@hug.get("/signatures/user/{user_id}", versions=1)
def get_signatures_user(auth_user: check_auth, user_id: hug.types.uuid):
    """
    GET: /signatures/user/{user_id}

    Get all signatures for user specified.
    """
    return cla.controllers.signature.get_user_signatures(user_id)


@hug.get("/signatures/user/{user_id}/project/{project_id}", versions=1)
def get_signatures_user_project(auth_user: check_auth, user_id: hug.types.uuid, project_id: hug.types.uuid):
    """
    GET: /signatures/user/{user_id}/project/{project_id}

    Get all signatures for user, filtered by project_id specified.
    """
    return cla.controllers.signature.get_user_project_signatures(user_id, project_id)


@hug.get("/signatures/user/{user_id}/project/{project_id}/type/{signature_type}", versions=1)
def get_signatures_user_project(
        auth_user: check_auth,
        user_id: hug.types.uuid,
        project_id: hug.types.uuid,
        signature_type: hug.types.one_of(["individual", "employee"]),
):
    """
    GET: /signatures/user/{user_id}/project/{project_id}/type/[individual|corporate|employee]

    Get all signatures for user, filtered by project_id and signature type specified.
    """
    return cla.controllers.signature.get_user_project_signatures(user_id, project_id, signature_type)


@hug.get("/signatures/company/{company_id}", versions=1)
def get_signatures_company(auth_user: check_auth, company_id: hug.types.uuid):
    """
    GET: /signatures/company/{company_id}

    Get all signatures for company specified.
    """
    return cla.controllers.signature.get_company_signatures_by_acl(auth_user.username, company_id)


@hug.get("/signatures/project/{project_id}", versions=1)
def get_signatures_project(auth_user: check_auth, project_id: hug.types.uuid):
    """
    GET: /signatures/project/{project_id}

    Get all signatures for project specified.
    """
    return cla.controllers.signature.get_project_signatures(project_id)


@hug.get("/signatures/company/{company_id}/project/{project_id}", versions=1)
def get_signatures_project_company(company_id: hug.types.uuid, project_id: hug.types.uuid):
    """
     GET: /signatures/company/{company_id}/project/{project_id}

     Get all signatures for project specified and a company specified
     """
    return cla.controllers.signature.get_project_company_signatures(company_id, project_id)


@hug.get("/signatures/company/{company_id}/project/{project_id}/employee", versions=1)
def get_project_employee_signatures(company_id: hug.types.uuid, project_id: hug.types.uuid):
    """
     GET: /signatures/company/{company_id}/project/{project_id}

     Get all employee signatures for project specified and a company specified
     """
    return cla.controllers.signature.get_project_employee_signatures(company_id, project_id)


@hug.get("/signature/{signature_id}/manager", versions=1)
def get_cla_managers(auth_user: check_auth, signature_id: hug.types.uuid):
    """
    GET: /project/{project_id}/managers

    Returns the CLA Managers from a CCLA's signature ACL.
    """
    return cla.controllers.signature.get_cla_managers(auth_user.username, signature_id)


@hug.post("/signature/{signature_id}/manager", versions=1)
def add_cla_manager(auth_user: check_auth, signature_id: hug.types.uuid, lfid: hug.types.text):
    """
    POST: /project/{project_id}/manager

    Adds CLA Manager to a CCLA's signature ACL and returns the new list of CLA managers.
    """
    return cla.controllers.signature.add_cla_manager(auth_user, str(signature_id), str(lfid))


@hug.delete("/signature/{signature_id}/manager/{lfid}", versions=1)
def remove_cla_manager(auth_user: check_auth, signature_id: hug.types.uuid, lfid: hug.types.text):
    """
    DELETE: /signature/{signature_id}/manager/{lfid}

    Removes a CLA Manager from a CCLA's signature ACL and returns the modified list of CLA Managers.
    """
    return cla.controllers.signature.remove_cla_manager(auth_user.username, signature_id, lfid)


@hug.get("/repository/{repository_id}", versions=1)
def get_repository(auth_user: check_auth, repository_id: hug.types.text):
    """
    GET: /repository/{repository_id}

    Returns the CLA repository requested by UUID.
    """
    return cla.controllers.repository.get_repository(repository_id)


@hug.post(
    "/repository",
    versions=1,
    examples=" - {'repository_project_id': '<project-id>', \
                        'repository_external_id': 'repo1', \
                        'repository_name': 'Repo Name', \
                        'repository_organization_name': 'Organization Name', \
                        'repository_type': 'github', \
                        'repository_url': 'http://url-to-repo.com'}",
)
def post_repository(
        auth_user: check_auth,  # pylint: disable=too-many-arguments
        repository_project_id: hug.types.uuid,
        repository_name: hug.types.text,
        repository_organization_name: hug.types.text,
        repository_type: hug.types.one_of(get_supported_repository_providers().keys()),
        repository_url: cla.hug_types.url,
        repository_external_id=None,
):
    """
    POST: /repository

    DATA: {'repository_project_id': '<project-id>',
           'repository_external_id': 'repo1',
           'repository_name': 'Repo Name',
           'repository_organization_name': 'Organization Name',
           'repository_type': 'github',
           'repository_url': 'http://url-to-repo.com'}

    repository_external_id is the ID of the repository given by the repository service provider.
    It is used to redirect the user back to the appropriate location once signing is complete.

    Returns the CLA repository that was just created.
    """
    return cla.controllers.repository.create_repository(
        auth_user,
        repository_project_id,
        repository_name,
        repository_organization_name,
        repository_type,
        repository_url,
        repository_external_id,
    )


@hug.put(
    "/repository",
    versions=1,
    examples=" - {'repository_id': '<repo-id>', \
                       'repository_id': 'http://new-url-to-repository.com'}",
)
def put_repository(
        auth_user: check_auth,  # pylint: disable=too-many-arguments
        repository_id: hug.types.text,
        repository_project_id=None,
        repository_name=None,
        repository_type=None,
        repository_url=None,
        repository_external_id=None,
):
    """
    PUT: /repository

    DATA: {'repository_id': '<repo-id>',
           'repository_url': 'http://new-url-to-repository.com'}

    Returns the CLA repository that was just updated.
    """
    return cla.controllers.repository.update_repository(
        repository_id,
        repository_project_id=repository_project_id,
        repository_name=repository_name,
        repository_type=repository_type,
        repository_url=repository_url,
        repository_external_id=repository_external_id,
    )


@hug.delete("/repository/{repository_id}", versions=1)
def delete_repository(auth_user: check_auth, repository_id: hug.types.text):
    """
    DELETE: /repository/{repository_id}

    Deletes the specified repository.
    """
    # staff_verify(user)
    return cla.controllers.repository.delete_repository(repository_id)


# #
# # Company Routes.
# #
@hug.get("/company", versions=1)
def get_companies(auth_user: check_auth):
    """
    GET: /company

    Returns all CLA companies associated with user.
    """
    cla.controllers.user.get_or_create_user(auth_user)  # Find or Create user -- For first login
    return cla.controllers.company.get_companies_by_user(auth_user.username)


@hug.get("/company", versions=2)
def get_all_companies():
    """
    GET: /company

    Returns all CLA companies.
    """
    return cla.controllers.company.get_companies()


@hug.get("/company/{company_id}", versions=2)
def get_company(company_id: hug.types.text):
    """
    GET: /company/{company_id}

    Returns the CLA company requested by UUID.
    """
    return cla.controllers.company.get_company(company_id)


@hug.get("/company/{company_id}/project/unsigned", versions=1)
def get_unsigned_projects_for_company(company_id: hug.types.text):
    """
    GET: /company/{company_id}/project/unsigned

    Returns a list of projects that the company has not signed CCLAs for.
    """
    return cla.controllers.project.get_unsigned_projects_for_company(company_id)


@hug.post(
    "/company",
    versions=1,
    examples=" - {'company_name': 'Company Name', \
                        'company_manager_id': 'user-id'}",
)
def post_company(
        auth_user: check_auth,
        company_name: hug.types.text,
        company_manager_user_name=None,
        company_manager_user_email=None,
        company_manager_id=None,
        is_sanctioned=None,
        response=None
):
    """
    POST: /company

    DATA: {'company_name': 'Org Name',
           'company_manager_id': <user-id>}

    Returns the CLA company that was just created.
    """

    create_resp = cla.controllers.company.create_company(
        auth_user,
        company_name=company_name,
        company_manager_id=company_manager_id,
        signing_entity_name=company_name,
        company_manager_user_name=company_manager_user_name,
        company_manager_user_email=company_manager_user_email,
        is_sanctioned=is_sanctioned,
        response=response,
    )

    response.status = create_resp.get("status_code")

    return create_resp.get("data")


@hug.put(
    "/company",
    versions=1,
    examples=" - {'company_id': '<company-id>', \
                       'company_name': 'New Company Name'}",
)
def put_company(
        auth_user: check_auth,  # pylint: disable=too-many-arguments
        company_id: hug.types.uuid,
        company_name=None,
        is_sanctioned=None,
        company_manager_id=None,
):
    """
    PUT: /company

    DATA: {'company_id': '<company-id>',
           'company_name': 'New Company Name'}

    Returns the CLA company that was just updated.
    """

    return cla.controllers.company.update_company(
        str(company_id),
        company_name=company_name,
        company_manager_id=company_manager_id,
        username=auth_user.username,
        is_sanctioned=is_sanctioned,
    )


@hug.delete("/company/{company_id}", versions=1)
def delete_company(auth_user: check_auth, company_id: hug.types.text):
    """
    DELETE: /company/{company_id}

    Deletes the specified company.
    """
    # staff_verify(user)
    return cla.controllers.company.delete_company(company_id, username=auth_user.username)


@hug.put("/company/{company_id}/import/whitelist/csv", versions=1)
def put_company_whitelist_csv(body, auth_user: check_auth, company_id: hug.types.uuid):
    """
    PUT: /company/{company_id}/import/whitelist/csv

    Imports a CSV file of whitelisted user emails.
    Expects the first column to have a header in the first row and contain email addresses.
    """
    # staff_verify(user) or company_manager_verify(user, company_id)
    content = body.read().decode()
    return cla.controllers.company.update_company_whitelist_csv(content, company_id, username=auth_user.username)


@hug.get("/companies/{manager_id}", version=1)
def get_manager_companies(manager_id: hug.types.uuid):
    """
    GET: /companies/{manager_id}

    Returns a list of companies a manager is associated with
    """
    return cla.controllers.company.get_manager_companies(manager_id)


# #
# # Project Routes.
# #
@hug.get("/project", versions=1)
def get_projects(auth_user: check_auth):
    """
    GET: /project

    Returns all CLA projects.
    """
    # staff_verify(user)
    projects = cla.controllers.project.get_projects()
    # For public endpoint, don't show the project_external_id.
    for project in projects:
        if "project_external_id" in project:
            del project["project_external_id"]
    return projects

# LG: This is ported to golang and no longer used in dev (still used in prod).
@hug.get("/project/{project_id}", versions=2)
def get_project(project_id: hug.types.uuid):
    """
    GET: /project/{project_id}

    Returns the CLA project requested by ID.
    """
    # returns value as a dict
    fn = '/v2/project/{project_id} handler'
    cla.log.debug(f'{fn} - loading cla group by cla group id: {project_id}')
    project = cla.controllers.project.get_project(project_id)
    if project.get("errors"):
        cla.log.warning(f'{fn} - problem loading cla group by cla group id: {project_id}')
        return project

    # For public endpoint, don't show the project_external_id.
    if "project_external_id" in project:
        del project["project_external_id"]

    # Remove all the document anchors, height, alignment details, not needed and it is a lot of JSON
    if "project_corporate_documents" in project:
        for doc in project['project_corporate_documents']:
            del doc['document_tabs']
    if "project_individual_documents" in project:
        for doc in project['project_individual_documents']:
            del doc['document_tabs']
    if "project_member_documents" in project:
        for doc in project['project_member_documents']:
            del doc['document_tabs']

    # Add the Project CLA Group Mappings to the response model
    sf_projects = []

    # Need a reference to the platform project service
    ps = ProjectService

    # Lookup the project to CLA Group mappings using the CLA Group ID (which is called the project_id in this case)
    cla.log.debug(f'{fn} - loading project cla group mapping by cla group id: {project_id}')
    project_cla_group_list = get_project_cla_group(project["project_id"])
    # Will have zero or more SF projects associated with this CLA Group
    signed_at_foundation = False
    for project_cla_group in project_cla_group_list:

        project_sfid = project_cla_group.get_project_sfid()
        foundation_sfid = project_cla_group.get_foundation_sfid()

        # Determine if this is a standalone project and if we are signed at the foundation level
        mapping_record = project_cla_group.to_dict()
        cla.log.debug(f'{fn} - determining if project {project_id} is a standalone project')
        mapping_record['standalone_project'] = ps.is_standalone(project_sfid)
        cla.log.debug(f"{fn} - project {project_id} is a standalone project: {mapping_record['standalone_project']}")

        mapping_record['lf_supported'] = ps.is_lf_supported(project_sfid)
        if project_sfid is not None and foundation_sfid is not None and project_sfid == foundation_sfid:
            cla.log.debug(f'{fn} - determined that the cla group is signed at the foundation')
            signed_at_foundation = True

        cla.log.debug(f'{fn} - querying github repositories for cla group: {project.get("project_name", None)} '
                      f'with id: {project_id}...')
        repositories = Repository().get_repository_by_project_sfid(project_sfid)
        mapping_record['github_repos'] = []
        mapping_record["gitlab_repos"] = []
        if repositories:
            mapping_record['github_repos'] = [repo.to_dict() for repo in repositories if repo.get_repository_type() == "github"]
            mapping_record["gitlab_repos"] = [repo.to_dict() for repo in repositories if repo.get_repository_type() == "gitlab"]
        mapping_record['gerrit_repos'] = []
        try:
            cla.log.debug(f'{fn} - querying gerrit repositories for cla group: {project.get("project_name", None)} '
                          f'with id: {project_id}...')
            gerrits = Gerrit().get_gerrit_by_project_sfid(project_sfid)
            if gerrits:
                # Convert the object to a generic dict to be marshalled
                array_dicts = []
                for gerrit in gerrits:
                    array_dicts.append(gerrit.to_dict())
                mapping_record['gerrit_repos'] = array_dicts
        except cla.models.DoesNotExist:
            cla.log.debug(f'{fn} - no gerrit repos configured for cla group: {project.get("project_name", None)} '
                          f'with id: {project_id}...')

        # Add this mapping record to list
        sf_projects.append(mapping_record)

    # Add the list of SF projects to our response model
    project["projects"] = sf_projects

    # Set the signed at foundation flag - default is false, common for v1 not to have any mappings
    project["signed_at_foundation_level"] = signed_at_foundation

    return project


@hug.get("/project/{project_id}/manager", versions=1)
def get_project_managers(auth_user: check_auth, project_id: hug.types.uuid):
    """
    GET: /project/{project_id}/managers
    Returns the CLA project managers.
    """
    return cla.controllers.project.get_project_managers(auth_user.username, project_id, enable_auth=True)


@hug.post("/project/{project_id}/manager", versions=1)
def add_project_manager(auth_user: check_auth, project_id: hug.types.text, lfid: hug.types.text):
    """
    POST: /project/{project_id}/manager
    Returns the new list of project managers
    """
    return cla.controllers.project.add_project_manager(auth_user.username, project_id, lfid)


@hug.delete("/project/{project_id}/manager/{lfid}", versions=1)
def remove_project_manager(auth_user: check_auth, project_id: hug.types.text, lfid: hug.types.text):
    """
    DELETE: /project/{project_id}/project/{lfid}
    Returns a success message if it was deleted
    """
    return cla.controllers.project.remove_project_manager(auth_user.username, project_id, lfid)


@hug.get("/project/external/{project_external_id}", version=1)
def get_external_project(auth_user: check_auth, project_external_id: hug.types.text):
    """
    GET: /project/external/{project_external_id}

    Returns the list of CLA projects marching the requested external ID.
    """
    return cla.controllers.project.get_projects_by_external_id(project_external_id, auth_user.username)


@hug.post("/project", versions=1, examples=" - {'project_name': 'Project Name'}")
def post_project(
        auth_user: check_auth,
        project_external_id: hug.types.text,
        project_name: hug.types.text,
        project_icla_enabled: hug.types.boolean,
        project_ccla_enabled: hug.types.boolean,
        project_ccla_requires_icla_signature: hug.types.boolean,
):
    """
    POST: /project

    DATA: {'project_external_id': '<proj-external-id>', 'project_name': 'Project Name',
           'project_icla_enabled': True, 'project_ccla_enabled': True,
           'project_ccla_requires_icla_signature': True}

    Returns the CLA project that was just created.
    """
    # staff_verify(user) or pm_verify_external_id(user, project_external_id)

    return cla.controllers.project.create_project(
        project_external_id,
        project_name,
        project_icla_enabled,
        project_ccla_enabled,
        project_ccla_requires_icla_signature,
        auth_user.username,
    )


@hug.put(
    "/project",
    versions=1,
    examples=" - {'project_id': '<proj-id>', \
                       'project_name': 'New Project Name'}",
)
def put_project(
        auth_user: check_auth,
        project_id: hug.types.uuid,
        project_name=None,
        project_icla_enabled=None,
        project_ccla_enabled=None,
        project_ccla_requires_icla_signature=None,
):
    """
    PUT: /project

    DATA: {'project_id': '<project-id>',
           'project_name': 'New Project Name'}

    Returns the CLA project that was just updated.
    """
    # staff_verify(user) or pm_verify(user, project_id)
    return cla.controllers.project.update_project(
        project_id,
        project_name=project_name,
        project_icla_enabled=project_icla_enabled,
        project_ccla_enabled=project_ccla_enabled,
        project_ccla_requires_icla_signature=project_ccla_requires_icla_signature,
        username=auth_user.username,
    )


@hug.delete("/project/{project_id}", versions=1)
def delete_project(auth_user: check_auth, project_id: hug.types.uuid):
    """
    DELETE: /project/{project_id}

    Deletes the specified project.
    """
    # staff_verify(user)
    return cla.controllers.project.delete_project(project_id, username=auth_user.username)


@hug.get("/project/{project_id}/repositories", versions=1)
def get_project_repositories(auth_user: check_auth, project_id: hug.types.uuid):
    """
    GET: /project/{project_id}/repositories

    Gets the specified project's repositories.
    """
    return cla.controllers.project.get_project_repositories(auth_user, project_id)


@hug.get("/project/{project_id}/repositories_group_by_organization", versions=1)
def get_project_repositories_group_by_organization(auth_user: check_auth, project_id: hug.types.uuid):
    """
    GET: /project/{project_id}/repositories_by_org

    Gets the specified project's repositories. grouped by organization name
    """
    return cla.controllers.project.get_project_repositories_group_by_organization(auth_user, project_id)


@hug.get("/project/{project_id}/configuration_orgs_and_repos", versions=1)
def get_project_configuration_orgs_and_repos(auth_user: check_auth, project_id: hug.types.uuid):
    """
    GET: /project/{project_id}/configuration_orgs_and_repos

    Gets the repositories from github api
    Gets all repositories for from an sfdc project ID
    """
    return cla.controllers.project.get_project_configuration_orgs_and_repos(auth_user, project_id)


@hug.get("/project/{project_id}/document/{document_type}", versions=2)
def get_project_document(
        project_id: hug.types.uuid, document_type: hug.types.one_of(["individual", "corporate"]),
):
    """
    GET: /project/{project_id}/document/{document_type}

    Fetch a project's signature document.
    """
    return cla.controllers.project.get_project_document(project_id, document_type)


@hug.get("/project/{project_id}/document/{document_type}/pdf", version=2)
def get_project_document_raw(
        response,
        auth_user: check_auth,
        project_id: hug.types.uuid,
        document_type: hug.types.one_of(["individual", "corporate"]),
):
    """
    GET: /project/{project_id}/document/{document_type}/pdf

    Returns the PDF document matching the latest individual or corporate contract for that project.
    """
    response.set_header("Content-Type", "application/pdf")
    return cla.controllers.project.get_project_document_raw(project_id, document_type)


@hug.get(
    "/project/{project_id}/document/{document_type}/pdf/{document_major_version}/{document_minor_version}", version=1,
)
def get_project_document_matching_version(
        response,
        auth_user: check_auth,
        project_id: hug.types.uuid,
        document_type: hug.types.one_of(["individual", "corporate"]),
        document_major_version: hug.types.number,
        document_minor_version: hug.types.number,
):
    """
    GET: /project/{project_id}/document/{document_type}/pdf/{document_major_version}/{document_minor_version}

    Returns the PDF document version matching the individual or corporate contract for that project.
    """
    response.set_header("Content-Type", "application/pdf")
    return cla.controllers.project.get_project_document_raw(
        project_id,
        document_type,
        document_major_version=document_major_version,
        document_minor_version=document_minor_version,
    )


@hug.get("/project/{project_id}/companies", versions=2)
def get_project_companies(project_id: hug.types.uuid):
    """
    GET: /project/{project_id}/companies
s
    Check if project exists and retrieves all companies
    """
    return cla.controllers.project.get_project_companies(project_id)


@hug.post(
    "/project/{project_id}/document/{document_type}",
    versions=1,
    examples=" - {'document_name': 'doc_name.pdf', \
                        'document_content_type': 'url+pdf', \
                        'document_content': 'http://url.com/doc.pdf', \
                        'new_major_version': true}",
)
def post_project_document(
        auth_user: check_auth,
        project_id: hug.types.uuid,
        document_type: hug.types.one_of(["individual", "corporate"]),
        document_name: hug.types.text,
        document_content_type: hug.types.one_of(get_supported_document_content_types()),
        document_content: hug.types.text,
        document_preamble=None,
        document_legal_entity_name=None,
        new_major_version=None,
):
    """
    POST: /project/{project_id}/document/{document_type}

    DATA: {'document_name': 'doc_name.pdf',
           'document_content_type': 'url+pdf',
           'document_content': 'http://url.com/doc.pdf',
           'document_preamble': 'Preamble here',
           'document_legal_entity_name': 'Legal entity name',
           'new_major_version': false}

    Creates a new CLA document for a specified project.

    Will create a new revision of the individual or corporate document. if new_major_version is set,
    the document will have a new major version and this will force users to re-sign.

    If document_content_type starts with 'storage+', the document_content is assumed to be base64
    encoded binary data that will be saved in the CLA system's configured storage service.
    """
    # staff_verify(user) or pm_verify(user, project_id)
    return cla.controllers.project.post_project_document(
        project_id=project_id,
        document_type=document_type,
        document_name=document_name,
        document_content_type=document_content_type,
        document_content=document_content,
        document_preamble=document_preamble,
        document_legal_entity_name=document_legal_entity_name,
        new_major_version=new_major_version,
        username=auth_user.username,
    )


@hug.post(
    "/project/{project_id}/document/template/{document_type}",
    versions=1,
    examples=" - {'document_name': 'doc_name.pdf', \
                        'document_preamble': 'Preamble here', \
                        'document_legal_entity_name': 'Legal entity name', \
                        'template_name': 'CNCFTemplate', \
                        'new_major_version': true}",
)
def post_project_document_template(
        auth_user: check_auth,
        project_id: hug.types.uuid,
        document_type: hug.types.one_of(["individual", "corporate"]),
        document_name: hug.types.text,
        document_preamble: hug.types.text,
        document_legal_entity_name: hug.types.text,
        template_name: hug.types.one_of(
            [
                "CNCFTemplate",
                "OpenBMCTemplate",
                "TungstenFabricTemplate",
                "OpenColorIOTemplate",
                "OpenVDBTemplate",
                "ONAPTemplate",
                "TektonTemplate",
            ]
        ),
        new_major_version=None,
):
    """
    POST: /project/{project_id}/document/template/{document_type}

#     DATA: {'document_name': 'doc_name.pdf',
#            'document_preamble': 'Preamble here',
#            'document_legal_entity_name': 'Legal entity name',
#            'template_name': 'CNCFTemplate',
#            'new_major_version': false}

#     Creates a new CLA document from a template for a specified project.

#     Will create a new revision of the individual or corporate document. if new_major_version is set,
#     the document will have a new major version and this will force users to re-sign.

#     The document_content_type is assumed to be 'storage+pdf', which means the document content will
#     be saved in the CLA system's configured storage service.
#     """
    # staff_verify(user) or pm_verify(user, project_id)
    return cla.controllers.project.post_project_document_template(
        project_id=project_id,
        document_type=document_type,
        document_name=document_name,
        document_preamble=document_preamble,
        document_legal_entity_name=document_legal_entity_name,
        template_name=template_name,
        new_major_version=new_major_version,
        username=auth_user.username,
    )


@hug.delete(
    "/project/{project_id}/document/{document_type}/{major_version}/{minor_version}", versions=1,
)
def delete_project_document(
        auth_user: check_auth,
        project_id: hug.types.uuid,
        document_type: hug.types.one_of(["individual", "corporate"]),
        major_version: hug.types.number,
        minor_version: hug.types.number,
):
    #     """
    #     DELETE: /project/{project_id}/document/{document_type}/{revision}

    #     Delete a project's signature document by revision.
    #     """
    #     # staff_verify(user)
    return cla.controllers.project.delete_project_document(
        project_id, document_type, major_version, minor_version, username=auth_user.username,
    )


# #
# # Document Signing Routes.
# #
@hug.post(
    "/request-individual-signature",
    versions=2,
    examples=" - {'project_id': 'some-proj-id', \
                        'user_id': 'some-user-uuid'}",
)
def request_individual_signature(
        request, project_id: hug.types.uuid, user_id: hug.types.uuid, return_url_type=None, return_url=None,
):
    """
    POST: /request-individual-signature

    DATA: {'project_id': 'some-project-id',
           'user_id': 'some-user-id',
           'return_url_type': Gerrit/Github/GitLab. Optional depending on presence of return_url
           'return_url': <optional>}

    Creates a new signature given project and user IDs. The user will be redirected to the
    return_url once signature is complete.

    Returns a dict of the format:

        {'user_id': <user_id>,
         'signature_id': <signature_id>,
         'project_id': <project_id>,
         'sign_url': <sign_url>}

    User should hit the provided URL to initiate the signing process through the
    signing service provider.
    """
    return cla.controllers.signing.request_individual_signature(project_id, user_id, return_url_type, return_url,
                                                                request=request)


@hug.post(
    "/request-corporate-signature",
    versions=1,
    examples=" - {'project_id': 'some-proj-id', \
                        'company_id': 'some-company-uuid'}",
)
def request_corporate_signature(
        auth_user: check_auth,
        project_id: hug.types.uuid,
        company_id: hug.types.uuid,
        signing_entity_name=None,
        send_as_email=False,
        authority_name=None,
        authority_email=None,
        return_url_type=None,
        return_url=None,
):
    """
    POST: /request-corporate-signature

    DATA: {'project_id': 'some-project-id',
           'company_id': 'some-company-id',
           'signing_entity_name': 'string',
           'send_as_email': 'boolean',
           'authority_name': 'string',
           'authority_email': 'string',
           'return_url': <optional>}

    {
      "project_id": "d8cead54-92b7-48c5-a2c8-b1e295e8f7f1",
      "company_id": "83f61e34-4457-45a6-a7ab-449ad6efcfbb",
      "return_url": "https://corporate.v1.easycla.lfx.linuxfoundation.org/#/company/83f61e34-4457-45a6-a7ab-449ad6efcfbb"
    }

    Creates a new signature given project and company IDs. The manager will be redirected to the
    return_url once signature is complete.

    The send_as_email flag determines whether to send the signing document because the CLA signatory/signer
    may not necessarily be a corporate/company manager/authority with signing privileges (e.g. may be the
    company manager, but not responsible for signing the CLAs).

    Returns a dict of the format:

        {'company_id': <user_id>,
         'signature_id': <signature_id>,
         'project_id': <project_id>,
         'sign_url': <sign_url>}

    Manager should hit the provided URL to initiate the signing process through the
    signing service provider.
    """
    # staff_verify(user) or company_manager_verify(user, company_id)
    return cla.controllers.signing.request_corporate_signature(
        auth_user=auth_user,
        project_id=str(project_id),
        company_id=str(company_id),
        signing_entity_name=signing_entity_name,
        send_as_email=send_as_email,
        authority_name=str(authority_name),
        authority_email=str(authority_email),
        return_url_type=str(return_url_type),
        return_url=str(return_url),
    )


@hug.post("/request-employee-signature", versions=2)
def request_employee_signature(
        project_id: hug.types.uuid,
        company_id: hug.types.uuid,
        user_id: hug.types.uuid,
        return_url_type: hug.types.text,
        return_url=None,
):
    """
    POST: /request-employee-signature

    DATA: {'project_id': <project-id>,
           'company_id': <company-id>,
           'user_id': <user-id>,
           'return_url': <optional>}

    Creates a placeholder signature object that represents an employee of a company having confirmed
    that they indeed work for company X which already has a CCLA with the project. This does not
    require a full DocuSign signature process, which means the sign/callback URLs and document
    versions may not be populated or reliable.
    """
    return cla.controllers.signing.request_employee_signature(
        project_id, company_id, user_id, return_url_type, return_url
    )


@hug.post("/check-prepare-employee-signature", versions=2)
def check_and_prepare_employee_signature(
        project_id: hug.types.uuid, company_id: hug.types.uuid, user_id: hug.types.uuid
):
    """
    POST: /check-prepare-employee-signature

    DATA: {'project_id': <project-id>,
           'company_id': <company-id>,
           'user_id': <user-id>
           }

    Checks if an employee is ready to sign a CCLA for a company.
    """
    return cla.controllers.signing.check_and_prepare_employee_signature(project_id, company_id, user_id)


@hug.post(
    "/signed/individual/{installation_id}/{github_repository_id}/{change_request_id}", versions=2,
)
def post_individual_signed(
        body,
        installation_id: hug.types.number,
        github_repository_id: hug.types.number,
        change_request_id: hug.types.number,
):
    """
    POST: /signed/individual/{installation_id}/{github_repository_id}/{change_request_id}

    TODO: Need to protect this endpoint somehow - at the very least ensure it's coming from
    DocuSign and the data hasn't been tampered with.

    Callback URL from signing service upon ICLA signature.
    """
    content = body.read()
    return cla.controllers.signing.post_individual_signed(
        content, installation_id, github_repository_id, change_request_id
    )

@hug.post(
    "/signed/gitlab/individual/{user_id}/{organization_id}/{gitlab_repository_id}/{merge_request_id}", versions=2,
)
def post_individual_signed_gitlab(
        body,
        user_id: hug.types.uuid,
        organization_id: hug.types.text,
        gitlab_repository_id: hug.types.number,
        merge_request_id: hug.types.number,
):
    """
    POST: /signed/gitlab/individual/{user_id}/{organization_id}/{gitlab_repository_id}/{merge_request_id}

    Callback URL from signing service upon ICLA signature for a Gitlab user.
    """
    content = body.read()
    return cla.controllers.signing.post_individual_signed_gitlab(
        content, user_id, organization_id, gitlab_repository_id, merge_request_id
    )


@hug.post("/signed/gerrit/individual/{user_id}", versions=2)
def post_individual_signed_gerrit(body, user_id: hug.types.uuid):
    """
    POST: /signed/gerritindividual/{user_id}

    Callback URL from signing service upon ICLA signature for a Gerrit user.
    """
    content = body.read()
    return cla.controllers.signing.post_individual_signed_gerrit(content, user_id)


@hug.post("/signed/corporate/{project_id}/{company_id}", versions=2)
def post_corporate_signed(body, project_id: hug.types.uuid, company_id: hug.types.uuid):
    """
    POST: /signed/corporate/{project_id}/{company_id}

    TODO: Need to protect this endpoint somehow - at the very least ensure it's coming from
    DocuSign and the data hasn't been tampered with.

    Callback URL from signing service upon CCLA signature.
    """
    content = body.read()
    return cla.controllers.signing.post_corporate_signed(content, str(project_id), str(company_id))


@hug.get("/return-url/{signature_id}", versions=2, output=hug.output_format.html)
def get_return_url(response: Response, signature_id: hug.types.uuid, event=None):
    """
    GET: /return-url/{signature_id}

    The endpoint the user will be redirected to upon completing signature. Will utilize the
    signature's "signature_return_url" field to redirect the user to the appropriate location.

    Will also capture the signing service provider's return GET parameters, such as DocuSign's
    'event' flag that describes the redirect reason.
    """
    result = cla.controllers.signing.return_url(signature_id, event)
    # if it's html we need to set the headers properly for it
    if result and isinstance(result, str):
        response.status = HTTP_OK
        response.content_type = 'text/html'
    return result


@hug.post("/send-authority-email", versions=2)
def send_authority_email(
        auth_user: check_auth,
        company_name: hug.types.text,
        project_name: hug.types.text,
        authority_name: hug.types.text,
        authority_email: cla.hug_types.email,
):
    """
    POST: /send-authority-email

    DATA: {
            'authority_name': John Doe,
            'authority_email': authority@example.com,
            'company_id': <company_id>
            'project_id': <project_id>
        }
    """
    return cla.controllers.signing.send_authority_email(company_name, project_name, authority_name, authority_email)


# #
# # Repository Provider Routes.
# #
@hug.get(
    "/repository-provider/{provider}/sign/{installation_id}/{github_repository_id}/{change_request_id}", versions=2,
)
def sign_request(
        provider: hug.types.one_of(get_supported_repository_providers().keys()),
        installation_id: hug.types.text,
        github_repository_id: hug.types.text,
        change_request_id: hug.types.text,
        request,
):
    """
    GET: /repository-provider/{provider}/sign/{installation_id}/{repository_id}/{change_request_id}

    Example: https://api.dev.lfcla.com/v2/repository-provider/github/sign/1973025/198682561/35

    The endpoint that will initiate a CLA signature for the user.
    """
    return cla.controllers.repository_service.sign_request(
        provider, installation_id, github_repository_id, change_request_id, request
    )


@hug.get("/repository-provider/{provider}/oauth2_redirect", versions=2)
def oauth2_redirect(
        auth_user: check_auth,  # pylint: disable=too-many-arguments
        provider: hug.types.one_of(get_supported_repository_providers().keys()),
        state: hug.types.text,
        code: hug.types.text,
        repository_id: hug.types.text,
        change_request_id: hug.types.text,
        request=None,
):
    """
    GET: /repository-provider/{provider}/oauth2_redirect

    TODO: This has been deprecated in favor of GET:/github/installation for GitHub Apps.

    Handles the redirect from an OAuth2 provider when initiating a signature.
    """
    # staff_verify(user)
    return cla.controllers.repository_service.oauth2_redirect(
        provider, state, code, repository_id, change_request_id, request
    )


@hug.post("/repository-provider/{provider}/activity", versions=2)
def received_activity(body, provider: hug.types.one_of(get_supported_repository_providers().keys())):
    """
    POST: /repository-provider/{provider}/activity

    TODO: Need to secure this endpoint somehow - maybe use GitHub's Webhook secret option.

    Acts upon a code repository provider's activity.
    """
    return cla.controllers.repository_service.received_activity(provider, body)


#
# GitHub Routes.
#
@hug.get("/github/organizations", versions=1)
def get_github_organizations(auth_user: check_auth):
    """
    GET: /github/organizations

    Returns all CLA Github Organizations.
    """
    return cla.controllers.github.get_organizations()


@hug.get("/github/organizations/{organization_name}", versions=1)
def get_github_organization(auth_user: check_auth, organization_name: hug.types.text):
    """
    GET: /github/organizations/{organization_name}

    Returns the CLA Github Organization requested by Name.
    """
    return cla.controllers.github.get_organization(organization_name)


@hug.get("/github/organizations/{organization_name}/repositories", versions=1)
def get_github_organization_repos(auth_user: check_auth, organization_name: hug.types.text):
    """
    GET: /github/organizations/{organization_name}/repositories

    Returns a list of Repositories selected under this organization.
    """
    return cla.controllers.github.get_organization_repositories(organization_name)


@hug.get("/sfdc/{sfid}/github/organizations", versions=1)
def get_github_organization_by_sfid(auth_user: check_auth, sfid: hug.types.text):
    """
    GET: /github/organizations/sfdc/{sfid}

    Returns a list of Github Organizations under this SFDC ID.
    """
    return cla.controllers.github.get_organization_by_sfid(auth_user, sfid)


@hug.post(
    "/github/organizations",
    versions=1,
    examples=" - {'organization_sfid': '<organization-sfid>', \
                        'organization_name': 'org-name'}",
)
def post_github_organization(
        auth_user: check_auth,  # pylint: disable=too-many-arguments
        organization_name: hug.types.text,
        organization_sfid: hug.types.text,
):
    """
    POST: /github/organizations

    DATA: { 'auth_user' : AuthUser to verify user permissions
            'organization_sfid': '<sfid-id>',
            'organization_name': 'org-name'}

    Returns the CLA GitHub Organization that was just created.
    """
    return cla.controllers.github.create_organization(auth_user, organization_name, organization_sfid)


@hug.delete("/github/organizations/{organization_name}", versions=1)
def delete_organization(auth_user: check_auth, organization_name: hug.types.text):
    """
    DELETE: /github/organizations/{organization_name}

    Deletes the specified Github Organization.
    """
    # staff_verify(user)
    return cla.controllers.github.delete_organization(auth_user, organization_name)


@hug.get("/github/installation", versions=2)
def github_oauth2_callback(code, state, request):
    """
    GET: /github/installation

    TODO: Need to secure this endpoint - possibly with GitHub's Webhook secrets.

    GitHub will send the user to this endpoint when new OAuth2 handshake occurs.
    This needs to match the callback used when users install the app as well (below).
    """
    return cla.controllers.github.user_oauth2_callback(code, state, request)


@hug.post("/github/installation", versions=2)
def github_app_installation(body, request, response):
    """
    POST: /github/installation

    TODO: Need to secure this endpoint - possibly with GitHub's Webhook secret.

    GitHub will fire off this webhook when new installation of our CLA app occurs.
    """
    return cla.controllers.github.user_authorization_callback(body)


@hug.post("/github/activity", versions=2)
def github_app_activity(body, request, response):
    """
    POST: /github/activity
    Acts upon any events triggered by our app installed in someone's organization.
    """
    # Verify that Webhook Signature is valid
    fn = 'routes.github.activity'
    event_type = request.headers.get('X-GITHUB-EVENT')
    action = get_github_activity_action(body)
    cla.log.debug(f'{fn} - received github activity with event type: \'{event_type}\' with action: \'{action}\'')

    # if not any of the events above we handle it via python
    valid_request = cla.controllers.github.webhook_secret_validation(request.headers.get('X-HUB-SIGNATURE'),
                                                                     request.bounded_stream.read())
    if not valid_request:
        cla.log.error(f'{fn} - webhook secret validation failed, sending email')
        maintainers = cla.config.PLATFORM_MAINTAINERS.split(',') if cla.config.PLATFORM_MAINTAINERS else []

        cla.log.error(f'{fn} - loaded maintainer list: {maintainers}')
        cla.controllers.github.webhook_secret_failed_email(event_type, body, maintainers)

        response.status = HTTP_401
        return {'status': "Invalid Secret Token"}
    cla.log.debug(f"{fn} - the webhook secret validation passed")

    cla.log.debug(f'{fn} - evaluating event type: \'{event_type}\' with action: \'{action}\' - '
                  'deciding if we need to forward the request to a separate handler...')
    if event_type == "installation_repositories" or \
            event_type == "integration_installation_repositories" or \
            event_type == "repository" or \
            (event_type == "push" and action and action == "created"):
        try:
            cla.log.debug(f'{fn} - redirecting event type: \'{event_type}\' with action: \'{action}\' to v4 golang api')
            v4_easycla_github_activity(cla.config.PLATFORM_GATEWAY_URL, request)
            response.status = HTTP_OK
            return {"status": "OK"}
        except requests.exceptions.HTTPError as ex:
            cla.log.error(f'{fn} - v4 golang api failed with : {ex.response.status_code} : {ex.response.json()}')
            response.status = HTTP_OK
            return {"status": "OK"}
        except Exception as ex:
            cla.log.error(f'{fn} - v4 golang api failed with : 500 : {ex}')
            response.status = HTTP_500
            return {"status": f'v4_easycla_github_activity failed {ex}'}
    cla.log.debug(f'{fn} - not forwarding event type: \'{event_type}\' with action: \'{action}\'.')

    if event_type is None:
        cla.log.error(f"{fn} - unable to determine the event type from request headers: {request.headers}")
        response.status = HTTP_400
        return {'status': 'Invalid request'}

    cla.log.debug(f'{fn} - routing github activity with event type: \'{event_type}\' with action: \'{action}\'...')
    return cla.controllers.github.activity(action, event_type, body)


@hug.post("/github/validate", versions=1)
def github_organization_validation(body):
    """
    POST: /github/validate

    TODO: Need to secure this endpoint with GitHub's Webhook secret.
    """
    return cla.controllers.github.validate_organization(body)


@hug.get("/github/check/namespace/{namespace}", versions=1)
def github_check_namespace(namespace):
    """
    GET: /github/check/namespace/{namespace}

    Returns True if the namespace provided is a valid GitHub account.
    """
    return cla.controllers.github.check_namespace(namespace)


@hug.get("/github/get/namespace/{namespace}", versions=1)
def github_get_namespace(namespace):
    """
    GET: /github/get/namespace/{namespace}

    Returns info on the GitHub account provided.
    """
    return cla.controllers.github.get_namespace(namespace)


#
# Gerrit instance routes
#
@hug.get("/project/{project_id}/gerrits", versions=1)
def get_project_gerrit_instance(project_id: hug.types.uuid):
    """
    GET: /project/{project_id}/gerrits

    Returns all CLA Gerrit instances for this project.
    """
    return cla.controllers.gerrit.get_gerrit_by_project_id(project_id)


@hug.get("/gerrit/{gerrit_id}", versions=2)
def get_gerrit_instance(gerrit_id: hug.types.uuid):
    """
    GET: /gerrit/gerrit_id

    Returns Gerrit instance with the given gerrit id.
    """
    return cla.controllers.gerrit.get_gerrit(gerrit_id)


@hug.post("/gerrit", versions=1)
def create_gerrit_instance(
        project_id: hug.types.uuid,
        gerrit_name: hug.types.text,
        gerrit_url: cla.hug_types.url,
        group_id_icla=None,
        group_id_ccla=None,
):
    """
    POST: /gerrit

    Creates a gerrit instance
    """
    return cla.controllers.gerrit.create_gerrit(project_id, gerrit_name, gerrit_url, group_id_icla, group_id_ccla)


@hug.delete("/gerrit/{gerrit_id}", versions=1)
def delete_gerrit_instance(gerrit_id: hug.types.uuid):
    """
    DELETE: /gerrit/{gerrit_id}

    Deletes the specified gerrit instance.
    """
    return cla.controllers.gerrit.delete_gerrit(gerrit_id)


@hug.get(
    "/gerrit/{gerrit_id}/{contract_type}/agreementUrl.html", versions=2, output=hug.output_format.html,
)
def get_agreement_html(gerrit_id: hug.types.uuid, contract_type: hug.types.text):
    """
    GET: /gerrit/{gerrit_id}/{contract_type}/agreementUrl.html
    Example: https://api.easycla.lfx.linuxfoundation.org/v2/gerrit/00022789-00fe-4658-a24b-9d05d3ee57e8/individual/agreementUrl.html

    Generates an appropriate HTML file for display in the Gerrit console.
    """
    return cla.controllers.gerrit.get_agreement_html(gerrit_id, contract_type)


# The following routes are only provided for project and cla manager
# permission management, and are not to be called by the UI Consoles.
@hug.get("/project/logo/{project_sfdc_id}", versions=1)
def upload_logo(auth_user: check_auth, project_sfdc_id: hug.types.text):
    return cla.controllers.project_logo.create_signed_logo_url(auth_user, project_sfdc_id)


@hug.post("/project/permission", versions=1)
def add_project_permission(auth_user: check_auth, username: hug.types.text, project_sfdc_id: hug.types.text):
    return cla.controllers.project.add_permission(auth_user, username, project_sfdc_id)


@hug.delete("/project/permission", versions=1)
def remove_project_permission(auth_user: check_auth, username: hug.types.text, project_sfdc_id: hug.types.text):
    return cla.controllers.project.remove_permission(auth_user, username, project_sfdc_id)


@hug.post("/company/permission", versions=1)
def add_company_permission(auth_user: check_auth, username: hug.types.text, company_id: hug.types.text):
    return cla.controllers.company.add_permission(auth_user, username, company_id)


@hug.delete("/company/permission", versions=1)
def remove_company_permission(auth_user: check_auth, username: hug.types.text, company_id: hug.types.text):
    return cla.controllers.company.remove_permission(auth_user, username, company_id)


@hug.get("/events", versions=1)
def search_events(request, response):
    return cla.controllers.event.events(request, response)


@hug.get("/events/{event_id}", versions=1)
def get_event(event_id: hug.types.text, response):
    """
    GET: /event/{event_id}
    Returns the requested event data based on ID
    """
    return cla.controllers.event.get_event(event_id=event_id, response=response)

@hug.get("/user-from-session", versions=2)
def user_from_session(request, response):
    """
    GET: /user-from-session
    Example: https://api.dev.lfcla.com/v2/user-from-session
    Example: https://api.dev.lfcla.com/v2/user-from-session?get_redirect_url=1
    Returns user object from OAuth2 session
    Example user returned:
    {
      "date_created": "2025-05-21T08:05:19.221609+0000",
      "date_modified": "2025-05-21T08:09:19.943455+0000",
      "lf_email": null,
      "lf_sub": null,
      "lf_username": null,
      "note": null,
      "user_company_id": null,
      "user_emails": [
        "test@user.com"
      ],
      "user_external_id": null,
      "user_github_id": "123",
      "user_github_username": "testuser",
      "user_gitlab_id": null,
      "user_gitlab_username": null,
      "user_id": "78bb15eb-8cbf-44e6-8b64-c713db6ee51b",
      "user_ldap_id": null,
      "user_name": "Test User",
      "version": "v1"
    }
    Will 302 redirect to /v2/github/installation if there is no session and that callback will return user data then
    WIll return 202 redirect to the same in reponse's JSON 'redirect_url' property if get_redirect_url=1 (param)
    Will return 200 and user data if there is an active GitHub session
    Can return 404 on OAuth2 errors
    """
    # https://github.com/sun-test-org/repo1/pull/215
    # from cla.models.github_models import GitHub
    # GitHub().process_opened_pull_request({"pull_request":{"number":215}, "repository":{"id":614349032}, "installation":{"id":35275118}})
    raw_redirect = request.params.get('get_redirect_url', 'false').lower()
    get_redirect_url = raw_redirect in ('1', 'true', 'yes')
    return cla.controllers.repository_service.user_from_session(get_redirect_url, request, response)

@hug.get("/user-from-token", versions=2)
def user_from_token(auth_user: check_auth, request, response):
    """
    GET: /user-from-token
    Example: https://api.dev.lfcla.com/v2/user-from-token
    Returns user object from Bearer token
    Example user returned:
    {
      "date_created": "2025-02-11T08:16:01.000000+0000",
      "date_modified": "2025-02-11T08:16:01.000000+0000",
      "lf_email": "lukaszgryglicki@o2.pl",
      "lf_sub": null,
      "lf_username": "lgryglicki",
      "note": null,
      "user_company_id": "0ca30016-6457-466c-bc41-a09560c1f9bf",
      "user_emails": null,
      "user_external_id": "0014100000Te0yqAAB",
      "user_github_id": null,
      "user_github_username": null,
      "user_gitlab_id": null,
      "user_gitlab_username": null,
      "user_id": "6e1fd921-e850-11ef-b5df-92cef1e60fc3",
      "user_ldap_id": null,
      "user_name": "Lukasz Gryglicki",
      "version": "v1"
    }
    Will return 200 and user data if token is valid
    Can return 404 on token errors
    """
    return cla.controllers.user.get_or_create_user(auth_user).to_dict()


@hug.post("/events", versions=1)
def create_event(
        event_data: hug.types.text,
        event_type: hug.types.text = None,
        user_id: hug.types.text = None,
        event_project_id: hug.types.text = None,
        event_company_id: hug.types.text = None,
        response=None,
):
    return cla.controllers.event.create_event(
        response=response,
        event_type=event_type,
        event_company_id=event_company_id,
        event_data=event_data,
        event_project_id=event_project_id,
        user_id=user_id,
    )


# Session Middleware
__hug__.http.add_middleware(get_session_middleware())
__hug__.http.add_middleware(get_log_middleware())
