# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Controller related to the signed callback.
"""
import time

import falcon
from jinja2 import Template

import cla
from cla.models import DoesNotExist
from cla.models.dynamo_models import Signature, User
from cla.user_service import UserService
from cla.utils import get_signing_service, get_signature_instance, get_email_service, \
    get_repository_service, get_project_instance, get_company_instance

CLA_MANAGER_ROLE = 'cla-manager'


def request_individual_signature(project_id, user_id, return_url_type, return_url=None, request=None):
    """
    Handle POST request to send ICLA signature request to user.
    :param project_id: The project to sign for.
    :type project_id: string
    :param user_id: The ID of the user that will sign.
    :type user_id: string
    :param return_url_type: Refers to the return url provider type: Gerrit or Github
    :type return_url_type: string
    :param return_url: The URL to return the user to after signing is complete.
    :type return_url: string
    :param request: The Falcon Request object.
    :type request: object
    """
    signing_service = get_signing_service()
    if return_url_type is not None and return_url_type.lower() == "gerrit":
        return signing_service.request_individual_signature_gerrit(str(project_id), str(user_id), return_url)
    elif return_url_type is not None and (return_url_type.lower() == "github" or return_url_type.lower() == "gitlab"):
        if return_url_type == "github":
            # fetching the primary for the account
            github = get_repository_service("github")
            primary_user_email = github.get_primary_user_email(request)
        elif return_url_type.lower() == "gitlab":
            try:
                cla.log.debug(f"Fetching user details for: {user_id}")
                user = User()
                user.load(user_id)
            except DoesNotExist as err:
                cla.log.warning('Individual Signature - user ID was NOT found for: {}'.format(user_id))
                return {'errors': {'user_id': str(err)}}
            primary_user_email = user.get_user_email()
        return signing_service.request_individual_signature(str(project_id), str(user_id), return_url, return_url_type,
                                                            preferred_email=primary_user_email)


def request_corporate_signature(auth_user,
                                project_id: str,
                                company_id: str,
                                signing_entity_name: str = None,
                                send_as_email: bool = False,
                                authority_name: str = None,
                                authority_email: str = None,
                                return_url_type: str = None,
                                return_url: str = None):
    """
    Creates CCLA signature object that represents a company signing a CCLA.

    :param auth_user: the authenticated user
    :type auth_user: an auth user object
    :param project_id: The ID of the project the company is signing a CCLA for.
    :type project_id: string
    :param company_id: The ID of the company that is signing the CCLA.
    :type company_id: string
    :param signing_entity_name: The CLA signing entity name for the DocuSign form
    :type signing_entity_name: string
    :param send_as_email: the send as email flag
    :type send_as_email: bool
    :param authority_name: the company manager/authority who is responsible for whitelisting/managing the company, but
    may not be a CLA signatory
    :type authority_name: str
    :param authority_email: the company manager/authority email
    :type authority_email: str
    :param return_url_type:
    :type return_url_type: str
    :param return_url:
    :type return_url: str
    :param return_url: The URL to return the user to after signing is complete.
    :type return_url: string
    """
    return get_signing_service().request_corporate_signature(
        auth_user=auth_user,
        project_id=project_id,
        company_id=company_id,
        signing_entity_name=signing_entity_name,
        send_as_email=send_as_email,
        signatory_name=authority_name,
        signatory_email=authority_email,
        return_url_type=return_url_type,
        return_url=return_url)


def request_employee_signature(project_id, company_id, user_id, return_url_type, return_url=None):
    """
    Creates placeholder signature object that represents a user signing a CCLA as an employee.

    :param project_id: The ID of the project the user is signing a CCLA for.
    :type project_id: string
    :param company_id: The ID of the company the employee belongs to.
    :type company_id: string
    :param user_id: The ID of the user.
    :type user_id: string
    :param return_url_type: Refers to the return url provider type: Gerrit or Github
    :type return_url_type: string
    :param return_url: The URL to return the user to after signing is complete.
    """
    fn = 'cla.controllers.signing.request_employee_signature'

    signing_service = get_signing_service()
    if return_url_type is not None and return_url_type.lower() == "gerrit":
        cla.log.error(f'{fn} - return type is gerrit - invoking: request_employee_signature_gerrit')
        return signing_service.request_employee_signature_gerrit(str(project_id), str(company_id), str(user_id),
                                                                 return_url)
    elif return_url_type is not None and (return_url_type.lower() == "github" or return_url_type.lower() == "gitlab"):
        cla.log.error(f'{fn} - return type is github - invoking: request_employee_signature')
        return signing_service.request_employee_signature(str(project_id), str(company_id), str(user_id), return_url)

    else:
        msg = (f'{fn} - unsupported return type {return_url_type} for '
               f'cla group: {project_id}, '
               f'company: {company_id}, '
               f'user: {user_id}')
        cla.log.error(msg)
        raise falcon.HTTPBadRequest(title=msg)


def check_and_prepare_employee_signature(project_id, company_id, user_id):
    """
    Checks that 
    1. The given project, company, and user exists 
    2. The company signatory has signed the CCLA for their company. 
    3. The user is included as part of the whitelist of the CCLA that the company signed. 

    :param project_id: The ID of the CLA Group (project) the user is signing a CCLA for.
    :type project_id: string
    :param company_id: The ID of the company the employee belongs to.
    :type company_id: string
    :param user_id: The ID of the user.
    :type user_id: string
    """
    return get_signing_service().check_and_prepare_employee_signature(str(project_id), str(company_id), str(user_id))


# Deprecated in favor of sending the email through DocuSign
def send_authority_email(company_name, project_name, authority_name, authority_email):
    """
    Sends email to the specified corporate authority to sign the CCLA Docusign file. 
    """

    subject = 'CLA: Invitation to Sign a Corporate Contributor License Agreement'
    body = '''Hello %s, 
    
Your organization: %s, 
    
has requested a Corporate Contributor License Agreement Form to be signed for the following project:

%s

Please read the agreement carefully and sign the attached file. 
    

- Linux Foundation CLA System
''' % (authority_name, company_name, project_name)
    recipient = authority_email
    email_service = get_email_service()
    email_service.send(subject, body, recipient)


def post_individual_signed(content, installation_id, github_repository_id, change_request_id):
    """
    Handle the posted callback from the signing service after ICLA signature.

    :param content: The POST body from the signing service callback.
    :type content: string
    :param repository_id: The ID of the repository that this signature was requested for.
    :type repository_id: string
    :param change_request_id: The ID of the change request or pull request that
        initiated this signature.
    :type change_request_id: string
    """
    get_signing_service().signed_individual_callback(content, installation_id, github_repository_id, change_request_id)

def post_individual_signed_gitlab(content, user_id, organization_id, gitlab_repository_id, merge_request_id):
    """
    Handle the posted callback from the signing service after ICLA signature.
    
    :param content: The POST body from the signing service callback.
    :type content: string
    :param user_id: The ID of the user that signed. 
    :type user_id: string
    """
    get_signing_service().signed_individual_callback_gitlab(content,user_id, organization_id, gitlab_repository_id, merge_request_id)


def post_individual_signed_gerrit(content, user_id):
    """
    Handle the posted callback from the signing service after ICLA signature for Gerrit.

    :param content: The POST body from the signing service callback.
    :type content: string
    :param user_id: The ID of the user that signed. 
    :type user_id: string
    """
    get_signing_service().signed_individual_callback_gerrit(content, user_id)


def post_corporate_signed(content, project_id, company_id):
    """
    Handle the posted callback from the signing service after CCLA signature.

    :param content: The POST body from the signing service callback.
    :type content: string
    :param project_id: The ID of the project that was signed.
    :type project_id: string
    :param company_id: The ID of the company that signed.
    :type company_id: string
    """
    get_signing_service().signed_corporate_callback(content, project_id, company_id)


def return_url(signature_id, event=None):  # pylint: disable=unused-argument
    """
    Handle the GET request from the user once they have successfully signed.

    :param signature_id: The ID of the signature they have just signed.
    :type signature_id: string
    :param event: The event GET flag sent back from the signing service provider.
    :type event: string | None
    """
    fn = 'return_url'
    try:  # Load the signature based on ID.
        signature = get_signature_instance()
        signature.load(str(signature_id))
    except DoesNotExist as err:
        cla.log.error('%s - Invalid signature_id provided when trying to send user back to their ' + \
                      'return_url after signing: %s', fn, signature_id)
        return {'errors': {'signature_id': str(err)}}
    # Ensure everything went well on the signing service provider's side.
    if event is not None:
        # Expired signing URL - the user was redirected back immediately but still needs to sign.
        if event == 'ttl_expired' and not signature.get_signature_signed():
            # Need to re-generate a sign_url and try again.
            cla.log.info('DocuSign URL used was expired, re-generating sign_url')
            callback_url = signature.get_signature_callback_url()
            get_signing_service().populate_sign_url(signature, callback_url)
            signature.save()
            raise falcon.HTTPFound(signature.get_signature_sign_url())
        if event == 'cancel':
            return canceled_signature_html(signature=signature)
    ret_url = signature.get_signature_return_url()
    if ret_url is not None:
        cla.log.info('%s- Signature success - sending user to return_url: %s', fn, ret_url)
        try:
            project = get_project_instance()
            project.load(str(signature.get_signature_project_id()))
        except DoesNotExist as err:
            cla.log.error('%s - Invalid project_id provided when trying to send user back to' \
                          'their return_url : %s', fn, signature.get_signature_project_id())

        if project.get_version() == 'v2':
            if signature.get_signature_reference_type() == 'company':
                cla.log.info('%s - Getting company instance : %s ', fn, signature.get_signature_reference_id())
                try:
                    company = get_company_instance()
                    company.load(str(signature.get_signature_reference_id()))
                except DoesNotExist as err:
                    cla.log.error('%s - Invalid company_id provided : err: %s', fn,
                                  signature.get_signature_reference_id)
                user_service = UserService
                cla.log.info('%s - Checking if cla managers have cla-manager role permission', fn)
                num_tries = 10
                i = 1
                cla.log.info(
                    f'{fn} - checking if managers:{signature.get_signature_acl()} have roles with {num_tries} tries')
                while i <= num_tries:
                    cla.log.info(f'{fn} - check try #: {i}')
                    assigned = {}
                    for manager in signature.get_signature_acl():
                        cla.log.info(
                            f'{fn}- Checking {manager} for {CLA_MANAGER_ROLE} for company: {company.get_company_external_id()}, cla_group_id: {signature.get_signature_project_id()}')
                        assigned[manager] = user_service.has_role(manager, CLA_MANAGER_ROLE,
                                                                  company.get_company_external_id(),
                                                                  signature.get_signature_project_id())
                    cla.log.info(f'{fn} - Assigned status : {assigned}')
                    # Ensure that assigned list doesnt have any False values -> All Managers have role assigned
                    if all(list(assigned.values())):
                        cla.log.info(
                            f'All managers have cla-manager role for company: {company.get_company_external_id()} and cla_group_id: {signature.get_signature_project_id()}')
                        break
                    time.sleep(0.5)
                    i += 1

        raise falcon.HTTPFound(ret_url)
    cla.log.info('No return_url set for signature - returning success message')
    return {'success': 'Thank you for signing'}


def canceled_signature_html(signature: Signature) -> str:
    """
    generates html for the signature when user clicks Finish Later or operation is
    canceled for some other reason.
    :param signature:
    :return:
    """
    msg = """
<html lang="en">
<head>
<title>The Linux Foundation – EasyCLA Signature Failure</title>
<!-- Required meta tags -->
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
<link rel="shortcut icon" href="https://www.linuxfoundation.org/wp-content/uploads/2017/08/favicon.png">
<link rel="stylesheet"
      href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/css/bootstrap.min.css"
      integrity="sha384-Gn5384xqQ1aoWXA+058RXPxPg6fy4IWvTNh0E263XmFcJlSAwiGgFAW/dAiS6JXm"
      crossorigin="anonymous"/>
<script src="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/js/bootstrap.min.js"
        integrity="sha384-JZR6Spejh4U02d8jOt6vLEHfe/JQGiRRSQQxSfFWpi1MquVdAyjUar5+76PVCmYl"
        crossorigin="anonymous"></script>
</head>
<body style='margin-top:20;margin-left:0;margin-right:0;'>
    <div class="text-center">
        <img width=300px"
         src="https://cla-project-logo-prod.s3.amazonaws.com/lf-horizontal-color.svg"
         alt="community bridge logo"/>
    </div>
    <h2 class="text-center">EasyCLA Account Authorization</h2>
    <p class="text-center">
    The authorization process was canceled and your account is not authorized under a signed CLA.  Click the button to authorize your account for
    {% if signature.get_signature_type() is not none and signature.get_signature_type()|length %}{{signature.get_signature_type().title()}}{% endif %} CLA.
    </p>
    <p class="text-center">
    <a href="{{signature.get_signature_sign_url()}}" class="btn btn-primary" role="button">
        Retry Docusign Authorization</a>
        {% if signature.get_signature_return_url() is not none and signature.get_signature_return_url()|length %}
    <a href="{{signature.get_signature_return_url()}}" class="btn btn-primary" role="button">
        Restart Authorization</a>
        {% endif %}
    </p>
</body>
</html>
        """
    t = Template(msg)
    return t.render(
        signature=signature,
    )
