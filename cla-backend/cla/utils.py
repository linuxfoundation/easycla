# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Utility functions for the CLA project.
"""

import inspect
import json
import os
import re
import secrets
import base64
import urllib.parse
import urllib.parse as urlparse
from datetime import datetime
from typing import List, Optional
from urllib.parse import urlencode

import cla
import falcon
import requests
from cla.middleware import CLALogMiddleware
from cla.models import DoesNotExist
from cla.models.dynamo_models import (CCLAWhitelistRequest, CLAManagerRequest,
                                      Company, CompanyInvite, Document, Event,
                                      Gerrit, GitHubOrg, GitlabOrg, Project,
                                      ProjectCLAGroup, Repository, Signature,
                                      User, UserPermissions)
from cla.models.event_types import EventType
from cla.user import UserCommitSummary
from hug.middleware import SessionMiddleware
from requests_oauthlib import OAuth2Session

API_BASE_URL = os.environ.get("CLA_API_BASE", "")
CLA_LOGO_URL = os.environ.get("CLA_BUCKET_LOGO_URL", "")
CORPORATE_BASE = os.environ.get("CLA_CORPORATE_BASE", "")
CORPORATE_V2_BASE = os.environ.get("CLA_CORPORATE_V2_BASE", "")


def get_cla_path():
    """Returns the CLA code root directory on the current system."""
    cla_folder_dir = os.path.dirname(os.path.abspath(inspect.getfile(inspect.currentframe())))
    cla_root_dir = os.path.dirname(cla_folder_dir)
    return cla_root_dir


def get_log_middleware():
    """Prepare the hug middleware to manage logging."""
    return CLALogMiddleware(logger=cla.log)


def get_session_middleware():
    """Prepares the hug middleware to manage key-value session data."""
    store = get_key_value_store_service()
    return SessionMiddleware(
        store,
        context_name="session",
        cookie_name="cla-sid",
        cookie_max_age=300,
        cookie_domain=None,
        cookie_path="/",
        cookie_secure=False,
    )


def create_database(conf=None):
    """
    Helper function to create the CLA database. Will utilize the appropriate database
    provider based on configuration.

    :param conf: Configuration dictionary/object - typically parsed from the CLA config file.
    :type conf: dict
    """
    if conf is None:
        conf = cla.conf
    cla.log.info("Creating CLA database in %s", conf["DATABASE"])
    if conf["DATABASE"] == "DynamoDB":
        from cla.models.dynamo_models import create_database as cd
    else:
        raise Exception("Invalid database selection in configuration: %s" % conf["DATABASE"])
    cd()


def delete_database(conf=None):
    """
    Helper function to delete the CLA database. Will utilize the appropriate database
    provider based on configuration.

    :WARNING: Use with caution.

    :param conf: Configuration dictionary/object - typically parsed from the CLA config file.
    :type conf: dict
    """
    if conf is None:
        conf = cla.conf
    cla.log.warning("Deleting CLA database in %s", conf["DATABASE"])
    if conf["DATABASE"] == "DynamoDB":
        from cla.models.dynamo_models import delete_database as dd
    else:
        raise Exception("Invalid database selection in configuration: %s" % conf["DATABASE"])
    dd()


def get_database_models(conf=None):
    """
    Returns the database models based on the configuration dict provided.

    :param conf: Configuration dictionary/object - typically parsed from the CLA config file.
    :type conf: dict
    :return: Dictionary of all the supported database object classes (User, Signature, Repository,
        company, Project, Document) - keyed by name:

            {'User': cla.models.model_interfaces.User,
             'Signature': cla.models.model_interfaces.Signature,...}

    :rtype: dict
    """
    if conf is None:
        conf = cla.conf
    if conf["DATABASE"] == "DynamoDB":
        return {
            "User": User,
            "Signature": Signature,
            "Repository": Repository,
            "Company": Company,
            "Project": Project,
            "Document": Document,
            "GitHubOrg": GitHubOrg,
            "Gerrit": Gerrit,
            "UserPermissions": UserPermissions,
            "Event": Event,
            "CompanyInvites": CompanyInvite,
            "ProjectCLAGroup": ProjectCLAGroup,
            "CCLAWhitelistRequest": CCLAWhitelistRequest,
            "CLAManagerRequest": CLAManagerRequest,
        }
    else:
        raise Exception("Invalid database selection in configuration: %s" % conf["DATABASE"])


def get_user_instance(conf=None) -> User:
    """
    Helper function to get a database User model instance based on CLA configuration.

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: A User model instance based on configuration specified.
    :rtype: cla.models.model_interfaces.User
    """
    return get_database_models(conf)["User"]()


def get_cla_manager_requests_instance(conf=None) -> CLAManagerRequest:
    """
    Helper function to get a database CLAManagerRequest model instance based on CLA configuration.

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: A CLAManagerRequest model instance based on configuration specified.
    :rtype: cla.models.model_interfaces.CLAManagerRequest
    """
    return get_database_models(conf)["CLAManagerRequest"]()


def get_user_permissions_instance(conf=None) -> UserPermissions:
    """
    Helper function to get a database UserPermissions model instance based on CLA configuration

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: A UserPermissions model instance based on configuration specified
    :rtype: cla.models.model_interfaces.UserPermissions
    """
    return get_database_models(conf)["UserPermissions"]()


def get_company_invites_instance(conf=None):
    """
    Helper function to get a database CompanyInvites model instance based on CLA configuration

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: A CompanyInvites model instance based on configuration specified
    :rtype: cla.models.model_interfaces.CompanyInvite
    """
    return get_database_models(conf)["CompanyInvites"]()


def get_signature_instance(conf=None) -> Signature:
    """
    Helper function to get a database Signature model instance based on CLA configuration.

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: An Signature model instance based on configuration.
    :rtype: cla.models.model_interfaces.Signature
    """
    return get_database_models(conf)["Signature"]()


def get_repository_instance(conf=None):
    """
    Helper function to get a database Repository model instance based on CLA configuration.

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: A Repository model instance based on configuration specified.
    :rtype: cla.models.model_interfaces.Repository
    """
    return get_database_models(conf)["Repository"]()


def get_github_organization_instance(conf=None):
    """
    Helper function to get a database GitHubOrg model instance based on CLA configuration.

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: A Repository model instance based on configuration specified.
    :rtype: cla.models.model_interfaces.GitHubOrg
    """
    return get_database_models(conf)["GitHubOrg"]()


def get_gerrit_instance(conf=None):
    """
    Helper function to get a database Gerrit model based on CLA configuration.

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: A Gerrit model instance based on configuration specified.
    :rtype: cla.models.model_interfaces.Gerrit
    """
    return get_database_models(conf)["Gerrit"]()


def get_company_instance(conf=None) -> Company:
    """
    Helper function to get a database company model instance based on CLA configuration.

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: A company model instance based on configuration specified.
    :rtype: cla.models.model_interfaces.Company
    """
    return get_database_models(conf)["Company"]()


def get_project_instance(conf=None) -> Project:
    """
    Helper function to get a database Project model instance based on CLA configuration.

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: A Project model instance based on configuration specified.
    :rtype: cla.models.model_interfaces.Project
    """
    return get_database_models(conf)["Project"]()


def get_document_instance(conf=None):
    """
    Helper function to get a database Document model instance based on CLA configuration.

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: A Document model instance based on configuration specified.
    :rtype: cla.models.model_interfaces.Document
    """
    return get_database_models(conf)["Document"]()


def get_event_instance(conf=None) -> Event:
    """
    Helper function to get a database Event model

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: A Event model instance based on configuration
    :rtype: cla.models.model_interfaces.Event
    """
    return get_database_models(conf)["Event"]()


def get_project_cla_group_instance(conf=None) -> ProjectCLAGroup:
    """
    Helper function to get a database ProjectCLAGroup model

    :param conf: the configuration model
    :type conf: dict
    :return: A ProjectCLAGroup model instance based on configuration
    :rtype: cla.models.model_interfaces.ProjectCLAGroup
    """

    return get_database_models(conf)["ProjectCLAGroup"]()


def get_ccla_whitelist_request_instance(conf=None) -> CCLAWhitelistRequest:
    """
    Helper function to get a database CCLAWhitelistRequest model

    :param conf: the configuration model
    :type conf: dict
    :return: A CCLAWhitelistRequest model instance based on configuration
    :rtype: cla.models.model_interfaces.CCLAWhitelistRequest
    """

    return get_database_models(conf)["CCLAWhitelistRequest"]()


def get_email_service(conf=None, initialize=True):
    """
    Helper function to get the configured email service instance.

    :param conf: Same as get_database_models().
    :type conf: dict
    :param initialize: Whether or not to run the initialize method on the instance.
    :type initialize: boolean
    :return: The email service model instance based on configuration specified.
    :rtype: EmailService
    """
    if conf is None:
        conf = cla.conf
    email_service = conf["EMAIL_SERVICE"]
    if email_service == "SMTP":
        from cla.models.smtp_models import SMTP as email
    elif email_service == "MockSMTP":
        from cla.models.smtp_models import MockSMTP as email
    elif email_service == "SES":
        from cla.models.ses_models import SES as email
    elif email_service == "SNS":
        from cla.models.sns_email_models import SNS as email
    elif email_service == "MockSES":
        from cla.models.ses_models import MockSES as email
    else:
        raise Exception("Invalid email service selected in configuration: %s" % email_service)
    email_instance = email()
    if initialize:
        email_instance.initialize(conf)
    return email_instance


def get_signing_service(conf=None, initialize=True):
    """
    Helper function to get the configured signing service instance.

    :param conf: Same as get_database_models().
    :type conf: dict
    :param initialize: Whether or not to run the initialize method on the instance.
    :type initialize: boolean
    :return: The signing service instance based on configuration specified.
    :rtype: SigningService
    """
    if conf is None:
        conf = cla.conf
    signing_service = conf["SIGNING_SERVICE"]
    if signing_service == "DocuSign":
        from cla.models.docusign_models import DocuSign as signing
    elif signing_service == "MockDocuSign":
        from cla.models.docusign_models import MockDocuSign as signing
    else:
        raise Exception("Invalid signing service selected in configuration: %s" % signing_service)
    signing_service_instance = signing()
    if initialize:
        signing_service_instance.initialize(conf)
    return signing_service_instance


def get_storage_service(conf=None, initialize=True):
    """
    Helper function to get the configured storage service instance.

    :param conf: Same as get_database_models().
    :type conf: dict
    :param initialize: Whether or not to run the initialize method on the instance.
    :type initialize: boolean
    :return: The storage service instance based on configuration specified.
    :rtype: StorageService
    """
    if conf is None:
        conf = cla.conf
    storage_service = conf["STORAGE_SERVICE"]
    if storage_service == "LocalStorage":
        from cla.models.local_storage import LocalStorage as storage
    elif storage_service == "S3Storage":
        from cla.models.s3_storage import S3Storage as storage
    elif storage_service == "MockS3Storage":
        from cla.models.s3_storage import MockS3Storage as storage
    else:
        raise Exception("Invalid storage service selected in configuration: %s" % storage_service)
    storage_instance = storage()
    if initialize:
        storage_instance.initialize(conf)
    return storage_instance


def get_pdf_service(conf=None, initialize=True):
    """
    Helper function to get the configured PDF service instance.

    :param conf: Same as get_database_models().
    :type conf: dict
    :param initialize: Whether or not to run the initialize method on the instance.
    :type initialize: boolean
    :return: The PDF service instance based on configuration specified.
    :rtype: PDFService
    """
    if conf is None:
        conf = cla.conf
    pdf_service = conf["PDF_SERVICE"]
    if pdf_service == "DocRaptor":
        from cla.models.docraptor_models import DocRaptor as pdf
    elif pdf_service == "MockDocRaptor":
        from cla.models.docraptor_models import MockDocRaptor as pdf
    else:
        raise Exception("Invalid PDF service selected in configuration: %s" % pdf_service)
    pdf_instance = pdf()
    if initialize:
        pdf_instance.initialize(conf)
    return pdf_instance


def get_key_value_store_service(conf=None):
    """
    Helper function to get the configured key-value store service instance.

    :param conf: Same as get_database_models().
    :type conf: dict
    :return: The key-value store service instance based on configuration specified.
    :rtype: KeyValueStore
    """
    if conf is None:
        conf = cla.conf
    keyvalue = cla.conf["KEYVALUE"]
    if keyvalue == "Memory":
        from hug.store import InMemoryStore as Store
    elif keyvalue == "DynamoDB":
        from cla.models.dynamo_models import Store
    else:
        raise Exception("Invalid key-value store selected in configuration: %s" % keyvalue)
    return Store()


def get_supported_repository_providers():
    """
    Returns a dict of supported repository service providers.

    :return: Dictionary of supported repository service providers in the following
        format: {'<provider_name>': <provider_class>}
    :rtype: dict
    """
    from cla.models.github_models import GitHub, MockGitHub

    # from cla.models.gitlab_models import GitLab, MockGitLab
    # return {'github': GitHub, 'mock_github': MockGitHub,
    # 'gitlab': GitLab, 'mock_gitlab': MockGitLab}
    return {"github": GitHub, "mock_github": MockGitHub}


def get_repository_service(provider, initialize=True):
    """
    Get a repository service instance by provider name.

    :param provider: The provider to load.
    :type provider: string
    :param initialize: Whether or not to call the initialize() method on the object.
    :type initialize: boolean
    :return: A repository provider instance (GitHub, Gerrit, etc).
    :rtype: RepositoryService
    """
    providers = get_supported_repository_providers()
    if provider not in providers:
        raise NotImplementedError("Provider not supported")
    instance = providers[provider]()
    if initialize:
        instance.initialize(cla.conf)
    return instance


def get_repository_service_by_repository(repository, initialize=True):
    """
    Helper function to get a repository service provider instance based
    on a repository.

    :param repository: The repository object or repository_id.
    :type repository: cla.models.model_interfaces.Repository | string
    :param initialize: Whether or not to call the initialize() method on the object.
    :type initialize: boolean
    :return: A repository provider instance (GitHub, Gerrit, etc).
    :rtype: RepositoryService
    """
    repository_model = get_database_models()["Repository"]
    if isinstance(repository, repository_model):
        repo = repository
    else:
        repo = repository_model()
        repo.load(repository)
    provider = repo.get_repository_type()
    return get_repository_service(provider, initialize)


def get_supported_document_content_types():  # pylint: disable=invalid-name
    """
    Returns a list of supported document content types.

    :return: List of supported document content types.
    :rtype: dict
    """
    return ["pdf", "url+pdf", "storage+pdf"]


def get_project_document(project, document_type, major_version, minor_version):
    """
    Helper function to get the specified document from a project.

    :param project: The project model object to look in.
    :type project: cla.models.model_interfaces.Project
    :param document_type: The type of document (individual or corporate).
    :type document_type: string
    :param major_version: The major version number to look for.
    :type major_version: integer
    :param minor_version: The minor version number to look for.
    :type minor_version: integer
    :return: The document model if found.
    :rtype: cla.models.model_interfaces.Document
    """
    if document_type == "individual":
        documents = project.get_project_individual_documents()
    else:
        documents = project.get_project_corporate_documents()
    for document in documents:
        if document.get_document_major_version() == major_version and document.get_document_minor_version() == minor_version:
            return document
    return None


def get_project_latest_individual_document(project_id):
    """
    Helper function to return the latest individual document belonging to a project.

    :param project_id: The project ID in question.
    :type project_id: string
    :return: Latest ICLA document object for this project.
    :rtype: cla.models.model_instances.Document
    """
    project = get_project_instance()
    project.load(str(project_id))
    document_models = project.get_project_individual_documents()
    major, minor = get_last_version(document_models)
    return project.get_project_individual_document(major, minor)


# TODO Heller remove
def get_project_latest_corporate_document(project_id):
    """
    Helper function to return the latest corporate document belonging to a project.

    :param project_id: The project ID in question.
    :type project_id: string
    :return: Latest CCLA document object for this project.
    :rtype: cla.models.model_instances.Document
    """
    project = get_project_instance()
    project.load(str(project_id))
    document_models = project.get_project_corporate_documents()
    major, minor = get_last_version(document_models)
    return project.get_project_corporate_document(major, minor)


def get_last_version(documents):
    """
    Helper function to get the last version of the list of documents provided.

    :param documents: List of documents to check.
    :type documents: [cla.models.model_interfaces.Document]
    :return: 2-item tuple containing (major, minor) version number.
    :rtype: tuple
    """
    last_major = 0  # 0 will be returned if no document was found.
    last_minor = -1  # -1 will be returned if no document was found.
    for document in documents:
        current_major = document.get_document_major_version()
        current_minor = document.get_document_minor_version()
        if current_major > last_major:
            last_major = current_major
            last_minor = current_minor
            continue
        if current_major == last_major and current_minor > last_minor:
            last_minor = current_minor
    return last_major, last_minor


def user_icla_check(user: User, project: Project, signature: Signature, latest_major_version=False) -> bool:
    cla.log.debug(
        f"ICLA signature found for user: {user} on project: {project}, " f"signature_id: {signature.get_signature_id()}"
    )

    # Here's our logic to determine if the signature is valid
    if latest_major_version:  # Ensure it's latest signature.
        document_models = project.get_project_individual_documents()
        major, _ = get_last_version(document_models)
        if signature.get_signature_document_major_version() != major:
            cla.log.debug(
                f"User: {user} only has an old document version signed "
                f"(v{signature.get_signature_document_major_version()}) - needs a new version"
            )
            return False

    if signature.get_signature_signed() and signature.get_signature_approved():
        # Signature found and signed/approved.
        cla.log.debug(
            f"User: {user} has ICLA signed and approved signature_id: {signature.get_signature_id()} "
            f"for project: {project}"
        )
        return True
    elif signature.get_signature_signed():  # Not approved yet.
        cla.log.debug(
            f"User: {user} has ICLA signed with signature_id: {signature.get_signature_id()}, "
            f"project: {project}, but has not been approved yet"
        )
        return False
    else:  # Not signed or approved yet.
        cla.log.debug(
            f"User: {user} has ICLA with signature_id: {signature.get_signature_id()}, "
            f"project: {project}, but has not been signed or approved yet"
        )
        return False


def user_ccla_check(user: User, project: Project, signature: Signature) -> bool:
    cla.log.debug(
        f"CCLA signature found for user: {user} on project: {project}, " f"signature_id: {signature.get_signature_id()}"
    )

    if signature.get_signature_signed() and signature.get_signature_approved():
        cla.log.debug(f"User: {user} has a signed and approved CCLA for project: {project}")
        return True

    if signature.get_signature_signed():
        cla.log.debug(
            f"User: {user} has CCLA signed with signature_id: {signature.get_signature_id()}, "
            f"project: {project}, but has not been approved yet"
        )
        return False
    else:  # Not signed or approved yet.
        cla.log.debug(
            f"User: {user} has CCLA with signature_id: {signature.get_signature_id()}, "
            f"project: {project}, but has not been signed or approved yet"
        )
        return False


def user_signed_project_signature(user: User, project: Project) -> bool:
    """
    Helper function to check if a user has signed a project signature tied to a repository.
    Will consider both ICLA and employee signatures.

    :param user: The user object to check for.
    :type user: cla.models.model_interfaces.User
    :param project: the project model
    :type project: cla.models.model_interfaces.Project
    :return: Whether or not the user has an signature that's signed and approved
        for this project.
    :rtype: boolean
    """

    fn = "utils.user_signed_project_signature"
    # Check if we have an ICLA for this user
    cla.log.debug(f"{fn} - checking to see if user has signed an ICLA, user: {user}, project: {project}")

    signature = user.get_latest_signature(project.get_project_id(), signature_signed=True, signature_approved=True)
    icla_pass = False
    if signature is not None:
        icla_pass = True
    else:
        cla.log.debug(f"{fn} - ICLA signature NOT found for User: {user} on project: {project}")

    # If we passed the ICLA check - good, return true, no need to check CCLA
    if icla_pass:
        cla.log.debug(f"{fn} - ICLA signature check passed for User: {user} on project: {project} - skipping CCLA check")
        return True
    else:
        cla.log.debug(f"{fn} - ICLA signature check failed for User: {user} on project: {project} - will now check CCLA")

    # Check if we have an CCLA for this user
    company_id = user.get_user_company_id()

    ccla_pass = False
    if company_id is not None:
        cla.log.debug(
            f"{fn} - CCLA signature check - user has a company: {company_id} - "
            "looking up user's employee acknowledgement..."
        )

        # Get employee signature
        employee_signature = user.get_latest_signature(
            project.get_project_id(), company_id=company_id, signature_signed=True, signature_approved=True
        )

        if employee_signature is not None:
            cla.log.debug(
                f"{fn} - CCLA signature check - located employee acknowledgement - "
                f"signature id: {employee_signature.get_signature_id()}"
            )

            company = get_company_instance()
            try:
                cla.log.debug(f"{fn} - CCLA signature check - loading company record by id: {company_id}...")
                company.load(company_id)
            except DoesNotExist as err:
                cla.log.debug(
                    f"{fn} - CCLA signature check failed - user is NOT associated with a valid company - "
                    f"company with id does not exist: {company_id}."
                )
                return False

            # Get CCLA signature of company to access whitelist
            cla.log.debug(
                f"{fn} - CCLA signature check - loading signed CCLA for project|company, "
                f"user: {user}, project_id: {project}, company_id: {company_id}"
            )
            signature = company.get_latest_signature(
                project.get_project_id(), signature_signed=True, signature_approved=True
            )

            # Don't check the version for employee signatures.
            if signature is not None:
                cla.log.debug(
                    f"{fn} - CCLA signature check - loaded signed CCLA for project|company, "
                    f"user: {user}, project_id: {project}, company_id: {company_id}, "
                    f"signature_id: {signature.get_signature_id()}"
                )

                # Verify if user has been approved: https://github.com/linuxfoundation/easycla/issues/332
                cla.log.debug(
                    f"{fn} - CCLA signature check - " "checking to see if the user is in one of the approval lists..."
                )
                # if project.get_project_ccla_requires_icla_signature() is True:
                #     cla.log.debug(f'{fn} - CCLA signature check - '
                #                   'project requires ICLA signature as well as CCLA signature ')
                if user.is_approved(signature):
                    ccla_pass = True
                else:
                    # Set user signatures approved = false due to user failing whitelist checks
                    cla.log.debug(
                        f"{fn} - user not in one of the approval lists - "
                        "marking signature approved = false for "
                        f"user: {user}, project_id: {project}, company_id: {company_id}"
                    )
                    user_signatures = user.get_user_signatures(
                        project_id=project.get_project_id(),
                        company_id=company_id,
                        signature_approved=True,
                        signature_signed=True,
                    )
                    for signature in user_signatures:
                        cla.log.debug(
                            f"{fn} - user not in one of the approval lists - "
                            "marking signature approved = false for "
                            f"user: {user}, project_id: {project}, company_id: {company_id}, "
                            f"signature: {signature.get_signature_id()}"
                        )
                        signature.set_signature_approved(False)
                        signature.save()
                        event_data = (
                            f"The employee signature of user {user.get_user_name()} was "
                            f"disapproved the during CCLA check for project {project.get_project_name()} "
                            f"and company {company.get_company_name()}"
                        )
                        Event.create_event(
                            event_type=EventType.EmployeeSignatureDisapproved,
                            event_cla_group_id=project.get_project_id(),
                            event_company_id=company.get_company_id(),
                            event_user_id=user.get_user_id(),
                            event_data=event_data,
                            event_summary=event_data,
                            contains_pii=True,
                        )
            else:
                cla.log.debug(
                    f"{fn} - CCLA signature check - unable to load signed CCLA for project|company, "
                    f"user: {user}, project_id: {project}, company_id: {company_id} - "
                    "signatory needs to sign the CCLA before the user can be authorized"
                )
        else:
            cla.log.debug(
                f"{fn} - CCLA signature check - unable to load employee acknowledgement for project|company, "
                f"user: {user}, project_id: {project}, company_id: {company_id}, "
                "signed=true, approved=true - user needs to be associated with an organization before "
                "they can be authorized."
            )
    else:
        cla.log.debug(
            f"{fn} - CCLA signature check failed - user is NOT associated with a company - "
            f"unable to check for a CCLA, user info: {user}."
        )

    if ccla_pass:
        cla.log.debug(f"{fn} - CCLA signature check passed for user: {user} on project: {project}")
        return True
    else:
        cla.log.debug(f"{fn} - CCLA signature check failed for user: {user} on project: {project}")

    cla.log.debug(f"{fn} - User: {user} failed both ICLA and CCLA checks")
    return False


def get_redirect_uri(repository_service, installation_id, github_repository_id, change_request_id):
    """
    Function to generate the redirect_uri parameter for a repository service's OAuth2 process.

    :param repository_service: The repository service provider we're currently initiating the
        OAuth2 process with. Currently only supports 'github' and 'gitlab'.
    :type repository_service: string
    :param installation_id: The EasyCLA GitHub application ID
    :type installation_id: string
    :param github_repository_id: The ID of the repository object that applies for this OAuth2 process.
    :type github_repository_id: string
    :param change_request_id: The ID of the change request in question. Is a PR number if
        repository_service is 'github'. Is a merge request if the repository_service is 'gitlab'.
    :type change_request_id: string
    :return: The redirect_uri parameter expected by the OAuth2 process.
    :rtype: string
    """
    params = {
        "installation_id": installation_id,
        "github_repository_id": github_repository_id,
        "change_request_id": change_request_id,
    }
    params = urllib.parse.urlencode(params)
    return "{}/v2/repository-provider/{}/oauth2_redirect?{}".format(cla.conf["API_BASE_URL"], repository_service, params)


def get_full_sign_url(repository_service, installation_id, github_repository_id, change_request_id, project_version):
    """
    Helper function to get the full sign URL that the user should click to initiate the signing
    workflow.
    :param repository_service: The repository service provider we're getting the sign url for.
        Should be one of the supported repository providers ('github', 'gitlab', etc).
    :type repository_service: string
    :param installation_id: The EasyCLA GitHub application ID
    :type installation_id: string
    :param github_repository_id: The ID of the repository for this signature (used in order to figure out
        where to send the user once signing is complete.
    :type github_repository_id: int
    :param change_request_id: The change request ID for this signature (used in order to figure out
        where to send the user once signing is complete. Should be a pull request number when
        repository_service is 'github'. Should be a merge request ID when repository_service is
        'gitlab'.
    :type change_request_id: int
    :param project_version: Project version associated with PR
    :type project_version: string
    """

    base_url = "{}/v2/repository-provider/{}/sign/{}/{}/{}/#/".format(
        cla.conf["API_BASE_URL"], repository_service, str(installation_id), str(github_repository_id), str(change_request_id)
    )

    return append_project_version_to_url(address=base_url, project_version=project_version)


def append_project_version_to_url(address: str, project_version: str) -> str:
    """
    appends the project version to given url if not already exists
    :param address:
    :param project_version:
    :return: returns the final url
    """
    version = "1"
    if project_version and project_version == "v2":
        version = "2"

    # seem if the url has # in it (https://dev.lfcla.com/#/version=1) the underlying urllib is being confused
    # In[21]: list(urlparse.urlparse(address))
    # Out[21]: ['https', 'dev.lfcla.com', '/', '', '', '/#/?version=1']

    query = {}
    if "?" in address:
        query = dict(urlparse.parse_qsl(address.split("?")[1]))

    # we don't alter for now
    if "version" in query:
        return address

    query["version"] = version
    query_params_str = urlencode(query)

    if "?" in address:
        return "?".join([address.split("?")[0], query_params_str])
    return "?".join([address, query_params_str])


def get_comment_badge(
    repository_type, all_signed, sign_url, project_version, missing_user_id=False, is_approved_by_manager=False
):
    """
    Returns the CLA badge that will appear on the change request comment (PR for 'github', merge
    request for 'gitlab', etc)

    :param repository_type: The repository service provider we're getting the badge for.
        Should be one of the supported repository providers ('github', 'gitlab', etc).
    :type repository_type: string
    :param all_signed: Whether or not all committers have signed the change request.
    :type all_signed: boolean
    :param sign_url: The URL for the user to click in order to initiate signing.
    :type sign_url: string
    :param missing_user_id: Flag to check if github id is missing
    :type missing_user_id: bool
    :param is_approved_by_manager; Flag checking if unregistered CLA user has been approved by a CLA Manager
    :type is_approved_by_manager: bool
    """

    alt = "CLA"
    if all_signed:
        badge_url = f"{CLA_LOGO_URL}/cla-signed.svg"
        badge_hyperlink = cla.conf["CLA_LANDING_PAGE"]
        badge_hyperlink = os.path.join(badge_hyperlink, "#/")
        badge_hyperlink = append_project_version_to_url(address=badge_hyperlink, project_version=project_version)
        alt = "CLA Signed"
        return (
            f'<a href="{badge_hyperlink}">'
            f'<img src="{badge_url}" alt="{alt}" align="left" height="28" width="328" >'
            "</a><br/>"
        )
    else:
        badge_hyperlink = sign_url
        text = ""
        if missing_user_id:
            badge_url = f"{CLA_LOGO_URL}/cla-missing-id.svg"
            alt = "CLA Missing ID"
            text = (
                f'{text} <a href="{badge_hyperlink}">'
                f'<img src="{badge_url}" alt="{alt}" align="left" height="28" width="328">'
                "</a>"
            )

        if is_approved_by_manager:
            badge_url = f"{CLA_LOGO_URL}/cla-confirmation-needed.svg"
            alt = "CLA Confirmation Needed"
            text = (
                f'{text} <a href="{badge_hyperlink}">'
                f'<img src="{badge_url}" alt="{alt}" align="left" height="28" width="328">'
                "</a>"
            )
        else:
            badge_url = f"{CLA_LOGO_URL}/cla-not-signed.svg"
            alt = "CLA Not Signed"
            text = (
                f'{text} <a href="{badge_hyperlink}">'
                f'<img src="{badge_url}" alt="{alt}" align="left" height="28" width="328">'
                "</a>"
            )

        return f"{text}<br/>"


def assemble_cla_status(author_name, signed=False):
    """
    Helper function to return the text that will display on a change request status.

    For GitLab there isn't much space here - we rely on the user hovering their mouse over the icon.
    For GitHub there is a 140 character limit.

    :param author_name: The name of the author of this commit.
    :type author_name: string
    :param signed: Whether or not the author has signed an signature.
    :type signed: boolean
    """
    if author_name is None:
        author_name = "Unknown"
    if signed:
        return author_name, "EasyCLA check passed. You are authorized to contribute."
    return author_name, "Missing CLA Authorization."


def assemble_cla_comment(
    repository_type,
    installation_id,
    github_repository_id,
    change_request_id,
    signed: List[UserCommitSummary],
    missing: List[UserCommitSummary],
    project_version,
):
    """
    Helper function to generate a CLA comment based on a a change request.


    :TODO: Update comments

    :param repository_type: The type of repository this comment will be posted on ('github',
        'gitlab', etc).
    :type repository_type: string
    :param installation_id: The EasyCLA GitHub application ID
    :type installation_id: string
    :param github_repository_id: The ID of the repository for this change request.
    :type github_repository_id: int
    :param change_request_id: The repository service's ID of this change request.
    :type change_request_id: id
    :param signed: The list of user commit summary objects indicating which authors that have signed an signature for
        this change request.
    :type signed: List[UserCommitSummary]
    :param missing: The list of user commit summary objects indicating which authors have not signed for this
        change request.
    :type missing: List[UserCommitSummary]
    :param project_version: Project version associated with PR comment
    :type project_version: string
    """

    # missing_ids = list(filter(lambda x: (x[1] is None or len(x[1]) == 0), missing))

    # Test to see if any of the users in the missing category are missing their user id
    no_user_id = len(list(filter(lambda x: (x.author_id is None), missing))) > 0

    # check if an unsigned committer has been approved by a CLA Manager, but not associated with a company
    # Logic not supported as we removed the DB query in the caller
    # approved_ids = list(filter(lambda x: len(x[1]) == 4 and x[1][3] is True, missing))
    # approved_by_manager = len(approved_ids) > 0
    sign_url = get_full_sign_url(repository_type, installation_id, github_repository_id, change_request_id, project_version)
    comment = get_comment_body(repository_type, sign_url, signed, missing)
    all_signed = len(missing) == 0
    badge = get_comment_badge(
        repository_type=repository_type,
        all_signed=all_signed,
        sign_url=sign_url,
        project_version=project_version,
        missing_user_id=no_user_id,
    )
    return badge + "<br />" + comment


def get_comment_body(repository_type, sign_url, signed: List[UserCommitSummary], missing: List[UserCommitSummary]):
    """
    Returns the CLA comment that will appear on the repository provider's change request item.

    :param: repository_type: The repository type where this comment will be posted ('github', 'gitlab', etc).
    :type: repository_type: string
    :param: sign_url: The URL for the user to click in order to initiate signing.
    :type: sign_url: string
    :param: signed: List of user commit summary objects containing the commit and author name of signers.
    :type: signed: List[UserCommitSummary]
    :param: missing: List of user commit summary objects containing the commit and author name of not-signed users.
    :type: missing: List[UserCommitSummary]
    """
    fn = "utils.get_comment_body"
    cla.log.info(f"{fn} - Getting comment body for repository type: %s", repository_type)
    failed = ":x:"
    success = ":white_check_mark:"
    committers_comment = ""
    num_signed = len(signed)
    num_missing = len(missing)
    text = ""

    # Start of the HTML to render the list of committers
    if len(signed) > 0 or len(missing) > 0:
        committers_comment += "<ul>"

    if num_signed > 0:
        # Group commits by author.
        committers = {}
        for user_commit_summary in signed:
            if user_commit_summary.is_valid_user():
                author_info = user_commit_summary.get_user_info(tag_user=False)
            else:
                author_info = "Unknown"

            if author_info not in committers:
                committers[author_info] = []

            # user commit summary includes the author information and the corresponding commit hash
            committers[author_info].append(user_commit_summary)

        # Print author commit information.
        for author_info, user_commit_summaries in committers.items():
            # build a quick list of just the commit hash values
            commit_shas = [user_commit_summary.commit_sha for user_commit_summary in user_commit_summaries]
            cla.log.info(f"{fn} SHAs for signed users: {commit_shas}")
            committers_comment += f'<li>{success} {author_info} ({", ".join(commit_shas)})</li>'

    if num_missing > 0:
        support_url = "https://jira.linuxfoundation.org/servicedesk/customer/portal/4"
        missing_id_help_url = "https://linuxfoundation.atlassian.net/wiki/spaces/LP/pages/160923756/Missing+ID+on+Commit+but+I+have+an+agreement+on+file"

        # Build a lookup table to group all the commits by author.
        committers = {}
        for user_commit_summary in missing:
            if user_commit_summary.is_valid_user():
                author_info = user_commit_summary.get_user_info(tag_user=True)
            else:
                author_info = "Unknown"

            if author_info not in committers:
                committers[author_info] = []

            # user commit summary includes the author information and the corresponding commit hash
            committers[author_info].append(user_commit_summary)

        # Print the author commit information.
        github_help_url = "https://help.github.com/en/github/committing-changes-to-your-project/why-are-my-commits-linked-to-the-wrong-user"
        for author_info, user_commit_summaries in committers.items():
            if author_info == "Unknown":
                # build a quick list of just the commit hash values
                commit_shas = [user_commit_summary.commit_sha for user_commit_summary in user_commit_summaries]
                committers_comment += (
                    f"<li> {failed} The email address for the commit ({', '.join(commit_shas)}) "
                    "is not linked to the GitHub account, preventing the EasyCLA check. Consult "
                    f"<a href='{missing_id_help_url}' target='_blank'>this Help Article</a> and "
                    f"<a href='{github_help_url}' target='_blank'>GitHub Help</a> to resolve. "
                    "(To view the commit's email address, add .patch at the end of this PR page's URL.) "
                    "For further assistance with EasyCLA, "
                    f"<a href='{support_url}' target='_blank'>please submit a support request ticket</a>."
                    "</li>"
                )
            else:
                missing_affiliations = [
                    user_commit_summary
                    for user_commit_summary in user_commit_summaries
                    if not user_commit_summary.affiliated and user_commit_summary.authorized
                ]
                if len(missing_affiliations) > 0:
                    # build a quick list of just the commit hash values for users missing company affiliations
                    commit_shas = [
                        user_commit_summary.commit_sha
                        for user_commit_summary in user_commit_summaries
                        if not user_commit_summary.affiliated
                    ]
                    cla.log.info(f"{fn} SHAs for users with missing company affiliations: {commit_shas}")
                    committers_comment += (
                        f'<li>{failed} {author_info} ({", ".join(commit_shas)}). '
                        f"This user is authorized, but they must confirm their affiliation with their company. "
                        f"Start the authorization process "
                        f"<a href='{sign_url}' target='_blank'> by clicking here</a>, click \"Corporate\", "
                        f"select the appropriate company from the list, then confirm "
                        f"your affiliation on the page that appears. "
                        f"For further assistance with EasyCLA, "
                        f"<a href='{support_url}' target='_blank'>please submit a support request ticket</a>."
                        "</li>"
                    )
                else:
                    # build a quick list of just the commit hash values
                    commit_shas = [user_commit_summary.commit_sha for user_commit_summary in user_commit_summaries]
                    committers_comment += (
                        f"<li>"
                        f"<a href='{sign_url}' target='_blank'>{failed}</a> - "
                        f"{author_info}. The commit ({', '.join(commit_shas)}) "
                        "is not authorized under a signed CLA. "
                        f"<a href='{sign_url}' target='_blank'>Please click here to be authorized</a>. "
                        f"For further assistance with EasyCLA, "
                        f"<a href='{support_url}' target='_blank'>please submit a support request ticket</a>."
                        "</li>"
                    )

    if len(signed) > 0 or len(missing) > 0:
        committers_comment += "</ul>"

    committers_comment += '<!-- Date Modified: ' + str(datetime.now()) + ' -->'

    if len(signed) > 0 and len(missing) == 0:
        text = "The committers listed above are authorized under a signed CLA."

    return text + committers_comment


def get_authorization_url_and_state(client_id, redirect_uri, scope, authorize_url, state=None):
    """
    Helper function to get an OAuth2 session authorization URL and state.

    :param client_id: The client ID for this OAuth2 session.
    :type client_id: string
    :param redirect_uri: The redirect URI to specify in this OAuth2 session.
    :type redirect_uri: string
    :param scope: The list of scope items to use for this OAuth2 session.
    :type scope: [string]
    :param authorize_url: The URL to submit the OAuth2 request.
    :type authorize_url: string
    """
    fn = "utils.get_authorization_url_and_state"
    if state is None:
        oauth = OAuth2Session(client_id, redirect_uri=redirect_uri, scope=scope)
        authorization_url, state = oauth.authorization_url(authorize_url)
        cla.log.debug(
            f"{fn} - initialized a new oauth session "
            f"using the github oauth client id: {client_id[0:5]}... "
            f"with the redirect_uri: {redirect_uri} "
            f"using scope of: {scope}. Obtained the "
            f"state: {state} and the "
            f"generated authorization_url: {authorize_url}"
        )
        return authorization_url, state
    else:
        csrf_token = secrets.token_urlsafe(16)
        state_payload = {"csrf": csrf_token, "state": state }
        state_json = json.dumps(state_payload)
        encoded_state = base64.urlsafe_b64encode(state_json.encode()).decode()
        oauth = OAuth2Session(client_id, redirect_uri=redirect_uri, scope=scope)
        authorization_url, _ = oauth.authorization_url(authorize_url, state=encoded_state)

        # Logging
        cla.log.debug(
            f"{fn} - initialized a new oauth session "
            f"using the github oauth client id: {client_id[0:5]}... "
            f"with the redirect_uri: {redirect_uri}. "
            f"using scope of: {scope}. "
            f"CSRF token: {csrf_token}. "
            f"custom value: {state}. "
            f"encoded state: {encoded_state}."
            f"Generated authorization_url: {authorization_url}"
        )
        return authorization_url, csrf_token


def fetch_token(client_id, state, token_url, client_secret, code, redirect_uri=None):  # pylint: disable=too-many-arguments
    """
    Helper function to fetch a OAuth2 session token.

    :param client_id: The client ID for this OAuth2 session.
    :type client_id: string
    :param state: The OAuth2 session state.
    :type state: string
    :param token_url: The token URL for this OAuth2 session.
    :type token_url: string
    :param client_secret: the client secret
    :type client_secret: string
    :param code: The OAuth2 session code.
    :type code: string
    :param redirect_uri: The redirect URI for this OAuth2 session.
    :type redirect_uri: string
    """
    fn = "utils.fetch_token"
    if redirect_uri is not None:
        oauth2 = OAuth2Session(client_id, state=state, scope=["user:email"], redirect_uri=redirect_uri)
    else:
        oauth2 = OAuth2Session(client_id, state=state, scope=["user:email"])
    #cla.log.debug(
    #    f"{fn} - oauth2.fetch_token - "
    #    f"token_url: {token_url}, "
    #    f"client_id: {client_id}, "
    #    f"client_secret: {client_secret}, "
    #    f"code: {code}"
    #)
    return oauth2.fetch_token(token_url, client_secret=client_secret, code=code)


def redirect_user_by_signature(user, signature):
    """
    Helper method to redirect a user based on their signature status and return_url.

    :param user: The user object for this redirect.
    :type user: cla.models.model_interfaces.User
    :param signature: The signature object for this user.
    :type signature: cla.models.model_interfaces.Signature
    """
    return_url = signature.get_signature_return_url()
    if signature.get_signature_signed() and signature.get_signature_approved():
        # Signature already signed and approved.
        # TODO: Notify user of signed and approved signature somehow.
        cla.log.info(
            "Signature already signed and approved for user: %s, %s", user.get_user_emails(), signature.get_signature_id()
        )
        if return_url is None:
            cla.log.info("No return_url set in signature object - serving success message")
            return {"status": "signed and approved"}
        else:
            cla.log.info("Redirecting user back to %s", return_url)
            raise falcon.HTTPFound(return_url)
    elif signature.get_signature_signed():
        # Awaiting approval.
        # TODO: Notify user of pending approval somehow.
        cla.log.info("Signature signed but not approved yet: %s", signature.get_signature_id())
        if return_url is None:
            cla.log.info("No return_url set in signature object - serving pending message")
            return {"status": "pending approval"}
        else:
            cla.log.info("Redirecting user back to %s", return_url)
            raise falcon.HTTPFound(return_url)
    else:
        # Signature awaiting signature.
        sign_url = signature.get_signature_sign_url()
        signature_id = signature.get_signature_id()
        cla.log.info("Signature exists, sending user to sign: %s (%s)", signature_id, sign_url)
        raise falcon.HTTPFound(sign_url)


def get_active_signature_metadata(user_id):
    """
    When a user initiates the signing process, the CLA system must store information on this
    signature - such as where the user came from, what repository it was initiated on, etc.
    This information is temporary while the signature is in progress. See the Signature object
    for information on this signature once the signing is complete.

    :param user_id: The ID of the user in question.
    :type user_id: string
    :return: Dict of data on the signature request from this user.
    :rtype: dict
    """
    store = get_key_value_store_service()
    key = "active_signature:" + str(user_id)
    if store.exists(key):
        return json.loads(store.get(key))
    return None


def set_active_signature_metadata(user_id, project_id, repository_id, pull_request_id):
    """
    When a user initiates the signing process, the CLA system must store information on this
    signature - such as where the user came from, what repository it was initiated on, etc.
    This is a helper function to perform the storage of this information.

    :param user_id: The ID of the user beginning the signing process.
    :type user_id: string
    :param project_id: The ID of the project this signature is for.
    :type project_id: string
    :param repository_id: The repository where the signature is coming from.
    :type repository_id: string
    :param pull_request_id: The PR where this signature request is coming from (where the user
        clicked on the 'Sign CLA' badge).
    :type pull_request_id: string
    """
    store = get_key_value_store_service()
    key = "active_signature:" + str(user_id)  # Should have been set when user initiated the signature.
    value = json.dumps(
        {"user_id": user_id, "project_id": project_id, "repository_id": repository_id, "pull_request_id": pull_request_id}
    )
    store.set(key, value)
    cla.log.info("Stored active signature details for user %s: Key - %s  Value - %s", user_id, key, value)


def delete_active_signature_metadata(user_id):
    """
    Helper function to delete all metadata regarding the active signature request for the user.

    :param user_id: The ID of the user in question.
    :type user_id: string
    """
    store = get_key_value_store_service()
    key = "active_signature:" + str(user_id)
    store.delete(key)
    cla.log.info("Deleted stored active signature details for user %s", user_id)


def set_active_pr_metadata(
    github_author_username: str, github_author_email: str, cla_group_id: str, repository_id: str, pull_request_id: str
):
    """
    When we receive a GitHub PR callback, we want to store a bit if information/metadata
    about the repository, PR, commit authors, and associated CLA Group so that we can later
    update the GitHub status check if a CLA manager asynchronously adds one or more commit
    authors to the approval list.
    This is a helper function to perform the storage of this information.

    :param github_author_username: The GitHub username/logic of the commit author
    :type github_author_username: string
    :param github_author_email: The GitHub user email of the commit author (if available)
    :type github_author_email: string
    :param cla_group_id: The ID of the CLA Group
    :type cla_group_id: string
    :param repository_id: The repository where the PR is coming from.
    :type repository_id: str
    :param pull_request_id: The PR identifier
    :type pull_request_id: str
    """
    store = get_key_value_store_service()

    # the same value is stored twice, indexed separately by username and email to allow lookups by either
    value = json.dumps(
        {
            "github_author_username": github_author_username,
            "github_author_email": github_author_email,
            "cla_group_id": cla_group_id,
            "repository_id": repository_id,
            "pull_request_id": pull_request_id,
        }
    )

    key_github_author_username = "active_pr:u:" + github_author_username
    store.set(key_github_author_username, value)
    cla.log.info(f"stored active pull request details by user email: %s", key_github_author_username)

    if github_author_email is not None:
        key_github_author_email = "active_pr:e:" + github_author_email
        store.set(key_github_author_email, value)
        cla.log.info(f"stored active pull request details by user email: %s", key_github_author_email)


def get_active_signature_return_url(user_id, metadata=None):
    """
    Helper function to get a user's active signature return URL.

    :param user_id: The user ID in question.
    :type user_id: string
    :param metadata: The signature metadata
    :type metadata: dict
    :return: The URL the user will be redirected to upon successful signature.
    :rtype: string
    """
    if metadata is None:
        metadata = get_active_signature_metadata(user_id)
    if metadata is None:
        cla.log.warning("Could not find active signature for user {}, return URL request failed".format(user_id))
        return None

    # Factor in Gitlab flow process
    if "merge_request_id" in metadata.keys():
        return metadata["return_url"]

    # Get Github ID from metadata
    github_repository_id = metadata["repository_id"]

    # Get installation id through a helper function
    installation_id = get_installation_id_from_github_repository(github_repository_id)
    if installation_id is None:
        cla.log.error("Could not find installation ID that is configured for this repository ID: %s", github_repository_id)
        return None

    github = cla.utils.get_repository_service("github")
    return github.get_return_url(metadata["repository_id"], metadata["pull_request_id"], installation_id)


def get_installation_id_from_github_repository(github_repository_id):
    # Get repository ID that references the github ID.
    try:
        repository = Repository().get_repository_by_external_id(github_repository_id, "github")
    except DoesNotExist:
        return None

    # Get Organization from this repository
    organization = GitHubOrg()
    try:
        organization.load(repository.get_repository_organization_name())
    except DoesNotExist:
        return None

    # Get this organization's installation ID
    return organization.get_organization_installation_id()


def get_organization_id_from_gitlab_repository(gitlab_repository_id):
    # Get repository ID that references the gitlab ID.
    try:
        repository = Repository().get_repository_by_external_id(gitlab_repository_id, "gitlab")
    except DoesNotExist:
        return None
    # Get GitLabGroup from this repository
    gitLabOrg = None
    try:
        gitLabOrg = GitlabOrg().search_organization_by_lower_name(repository.get_repository_organization_name().lower())
    except DoesNotExist:
        cla.log.debug(f"unable to get gitlab org by name: {repository.get_repository_organization_name()}")
        return None

    # return GitLab organization ID
    return gitLabOrg.get_organization_id()


def get_project_id_from_github_repository(github_repository_id):
    # Get repository ID that references the github ID.
    try:
        repository = Repository().get_repository_by_external_id(github_repository_id, "github")
    except DoesNotExist:
        return None

    # Get project ID (contract group ID) of this repository
    return repository.get_repository_project_id()


def get_individual_signature_callback_url(user_id, metadata=None):
    """
    Helper function to get a user's active signature callback URL.

    :param user_id: The user ID in question.
    :type user_id: string
    :param metadata: The signature metadata
    :type metadata: dict
    :return: The callback URL that will be hit by the signing service provider.
    :rtype: string
    """
    if metadata is None:
        metadata = get_active_signature_metadata(user_id)
    if metadata is None:
        cla.log.warning("Could not find active signature for user {}, callback URL request failed".format(user_id))
        return None

    # Get Github ID from metadata
    github_repository_id = metadata["repository_id"]

    # Get installation id through a helper function
    installation_id = get_installation_id_from_github_repository(github_repository_id)
    if installation_id is None:
        cla.log.error("Could not find installation ID that is configured for this repository ID: %s", github_repository_id)
        return None

    return os.path.join(
        API_BASE_URL,
        "v2/signed/individual",
        str(installation_id),
        str(metadata["repository_id"]),
        str(metadata["pull_request_id"]),
    )


def get_individual_signature_callback_url_gitlab(user_id, metadata=None):
    """
    Helper function to get a user's active signature callback URL.

    :param user_id: The user ID in question.
    :type user_id: string
    :param metadata: The signature metadata
    :type metadata: dict
    :return: The callback URL that will be hit by the signing service provider.
    :rtype: string
    """
    if metadata is None:
        metadata = get_active_signature_metadata(user_id)
    if metadata is None:
        cla.log.warning("Could not find active signature for user {}, callback URL request failed".format(user_id))
        return None

    # Get GitLab ID from metadata
    gitlab_repository_id = metadata["repository_id"]

    # Get organization id
    organization_id = get_organization_id_from_gitlab_repository(gitlab_repository_id)

    if organization_id is None:
        cla.log.error(
            "Could not find GitLab organization ID that is configured for this repository ID: %s", gitlab_repository_id
        )
        return None

    return os.path.join(
        API_BASE_URL,
        "v2/signed/gitlab/individual",
        str(user_id),
        str(organization_id),
        str(metadata["repository_id"]),
        str(metadata["merge_request_id"]),
    )


def request_individual_signature(installation_id, github_repository_id, user, change_request_id, callback_url=None):
    """
    Helper function send the user off to sign an signature based on the repository.

    :TODO: Update comments.

    :param installation_id: The GitHub installation ID
    :type installation_id: int
    :param github_repository_id: The GitHub repository ID ID
    :type github_repository_id: int
    :param user: The user in question.
    :type user: cla.models.model_interfaces.User
    :param change_request_id: The change request ID (used to redirect the user after signing).
    :type change_request_id: string
    :param callback_url: Optionally provided a callback_url. Will default to
        <SIGNED_CALLBACK_URL>/<repo_id>/<change_request_id>.
    :type callback_url: string
    """
    project_id = get_project_id_from_github_repository(github_repository_id)
    repo_service = get_repository_service("github")
    return_url = repo_service.get_return_url(github_repository_id, change_request_id, installation_id)
    if callback_url is None:
        callback_url = os.path.join(API_BASE_URL, "v2/signed/individual", str(installation_id), str(change_request_id))

    signing_service = get_signing_service()
    return_url_type = "Github"
    signature_data = signing_service.request_individual_signature(
        project_id, user.get_user_id(), return_url_type, return_url, callback_url
    )
    if "sign_url" in signature_data:
        raise falcon.HTTPFound(signature_data["sign_url"])
    cla.log.error("Could not get sign_url from signing service provider - sending user " "to return_url instead")
    raise falcon.HTTPFound(return_url)


def lookup_user_gitlab_username(user_gitlab_id: int) -> Optional[str]:
    """
    Given a user gitlab ID, looks up the user's gitlab login/username.
    :param user_gitlab_id: the gitlab id
    :return: the user's gitlab login/username
    """
    try:
        r = requests.get(f"https://gitlab.com/api/v4/users/{user_gitlab_id}")
        r.raise_for_status()
    except requests.exceptions.HTTPError as err:
        msg = f"Could not get user github user from id: {user_gitlab_id}: error: {err}"
        cla.log.warning(msg)
        return None

    gitlab_user = r.json()
    if "id" in gitlab_user:
        return gitlab_user["id"]
    else:
        cla.log.warning('Malformed HTTP response from GitLab - expecting "id" attribute ' f"- response: {gitlab_user}")
        return None


def lookup_user_gitlab_id(user_gitlab_username: str) -> Optional[str]:
    """
    Given a user gitlab username, looks up the user's gitlab id.
    :param user_gitlab_username: the gitlab username
    :return: the user's gitlab id
    """
    try:
        r = requests.get(f"https://gitlab.com/api/v4/users?username={user_gitlab_username}")
        r.raise_for_status()
    except requests.exceptions.HTTPError as err:
        msg = f"Could not get user github user from username: {user_gitlab_username}: error: {err}"
        cla.log.warning(msg)
        return None

    gitlab_user = r.json()
    if "username" in gitlab_user:
        return gitlab_user["username"]
    else:
        cla.log.warning('Malformed HTTP response from GitLab - expecting "username" attribute ' f"- response: {gitlab_user}")
        return None


def lookup_user_github_username(user_github_id: int) -> Optional[str]:
    """
    Given a user github ID, looks up the user's github login/username.
    :param user_github_id: the github id
    :return: the user's github login/username
    """
    try:
        headers = {
            "Authorization": "Bearer {}".format(cla.conf["GITHUB_OAUTH_TOKEN"]),
            "Accept": "application/json",
        }

        r = requests.get(f"https://api.github.com/user/{user_github_id}", headers=headers)
        r.raise_for_status()
    except requests.exceptions.HTTPError as err:
        msg = f"Could not get user github user from id: {user_github_id}: error: {err}"
        cla.log.warning(msg)
        return None

    github_user = r.json()
    if "message" in github_user:
        cla.log.warning(f"Unable to lookup user from id: {user_github_id} " f'- message: {github_user["message"]}')
        return None
    else:
        if "login" in github_user:
            return github_user["login"]
        else:
            cla.log.warning(
                'Malformed HTTP response from GitHub - expecting "login" attribute ' f"- response: {github_user}"
            )
            return None


def lookup_user_github_id(user_github_username: str) -> Optional[int]:
    """
    Given a user github username, looks up the user's github id.
    :param user_github_username: the github username
    :return: the user's github id
    """
    try:
        headers = {
            "Authorization": "Bearer {}".format(cla.conf["GITHUB_OAUTH_TOKEN"]),
            "Accept": "application/json",
        }

        r = requests.get(f"https://api.github.com/users/{user_github_username}", headers=headers)
        r.raise_for_status()
    except requests.exceptions.HTTPError as err:
        msg = f"Could not get user github id from username: {user_github_username}: error: {err}"
        cla.log.warning(msg)
        return None

    github_user = r.json()
    if "message" in github_user:
        cla.log.warning(f"Unable to lookup user from id: {user_github_username} " f'- message: {github_user["message"]}')
        return None
    else:
        if "id" in github_user:
            return github_user["id"]
        else:
            cla.log.warning('Malformed HTTP response from GitHub - expecting "id" attribute ' f"- response: {github_user}")
            return None


def lookup_github_organizations(github_username: str):
    # Use the Github API to retrieve github orgs that the user is a member of (user must be a public member).
    try:
        headers = {
            "Authorization": "Bearer {}".format(cla.conf["GITHUB_OAUTH_TOKEN"]),
            "Accept": "application/json",
        }

        r = requests.get(f"https://api.github.com/users/{github_username}/orgs", headers=headers)
        r.raise_for_status()
    except requests.exceptions.HTTPError as err:
        cla.log.warning("Could not get user github org: {}".format(err))
        return {"error": "Could not get user github org: {}".format(err)}
    return [github_org["login"] for github_org in r.json()]


def lookup_gitlab_org_members(organization_id):
    # Use the v2 Endpoint thats a wrapper for Gitlab Group member query
    try:
        r = requests.get(f"{cla.config.PLATFORM_GATEWAY_URL}/cla-service/v4/gitlab/group/{organization_id}/members")
        r.raise_for_status()
    except requests.exceptions.HTTPError as err:
        cla.log.warning(f"Could not fetch gitlab org users: {err}")
        return {f"error: Could not get user gitlab group id: {organization_id} members: {err}"}
    return r.json()["list"]


def update_github_username(github_user: dict, user: User):
    """
    When provided a GitHub user model from the GitHub service, updates the CLA
    user record with the github username.
    :param github_user:  the github user model as a dict from GitHub
    :param user:  the user DB object
    :return: None
    """
    # set the github username if available
    if "login" in github_user:
        if user.get_user_github_username() is None:
            cla.log.debug(f'Updating user record - adding github username: {github_user["login"]}')
            user.set_user_github_username(github_user["login"])
        if user.get_user_github_username() != github_user["login"]:
            cla.log.warning(
                f'Note: github user with id: {github_user["id"]}'
                f' has a mismatched username (gh: {github_user["id"]} '
                f"vs db user record: {user.get_user_github_username}) - "
                f'setting the value to: {github_user["login"]}'
            )
            user.set_user_github_username(github_user["login"])


def is_approved(ccla_signature: Signature, email=None, github_username=None, github_id=None):
    """
    Given either email, github username or github id a check is made against ccla signature to
    check whether a given parameter is whitelisted . This check is vital for a first time user
    who could have been whitelisted and has not confirmed affiliation

    :param ccla_signature: given signature used to check for ccla whitelists
    :param email: email that is checked against ccla signature email whitelist
    :param github_username: A given github username checked against ccla signature github/github-org whitelists
    :param github_id: A given github id checked against ccla signature github/github-org whitelists
    """
    fn = "utils.is_approved"

    if email:
        # Checking email whitelist
        whitelist = ccla_signature.get_email_whitelist()
        cla.log.debug(f"{fn} - testing email: {email} with CCLA approval list emails: {whitelist}")
        if whitelist is not None:
            if email.lower() in (s.lower() for s in whitelist):
                cla.log.debug(f"{fn} found user email in email approval list")
                return True

        # Checking domain whitelist
        patterns = ccla_signature.get_domain_whitelist()
        cla.log.debug(
            f"{fn} - testing user email domain: {email} with " f"domain approval list values in database: {patterns}"
        )
        if patterns is not None:
            if get_user_instance().preprocess_pattern([email], patterns):
                return True
            else:
                cla.log.debug(f"{fn} - did not match email: {email} with domain: {patterns}")
        else:
            cla.log.debug(f"{fn} - no domain approval list patterns defined - skipping domain approval list check")

    if github_id:
        github_username = lookup_user_github_username(github_id)

    # Github username whitelist
    if github_username is not None:
        # remove leading and trailing whitespace from github username
        github_username = github_username.strip()
        github_approval_list = ccla_signature.get_github_whitelist()
        cla.log.debug(
            f"{fn} - testing user github username: {github_username} with "
            f"CCLA github approval list: {github_approval_list}"
        )

        if github_approval_list is not None:
            # case insensitive search
            if github_username.lower() in (s.lower() for s in github_approval_list):
                cla.log.debug(f"{fn} - found github username in github approval list")
                return True
    else:
        cla.log.debug(f"{fn} - users github_username is not defined - skipping github username approval list check")

    # Check github org approval list
    if github_username is not None:
        github_orgs = cla.utils.lookup_github_organizations(github_username)
        if "error" not in github_orgs:
            # Fetch the list of orgs this user is part of
            github_org_approval_list = ccla_signature.get_github_org_whitelist()
            cla.log.debug(
                f"{fn} - testing user github orgs: {github_orgs} with "
                f"CCLA github org approval list values: {github_org_approval_list}"
            )

            if github_org_approval_list is not None:
                for dynamo_github_org in github_org_approval_list:
                    # case insensitive search
                    if dynamo_github_org.lower() in (s.lower() for s in github_orgs):
                        cla.log.debug(f"{fn} - found matching github org for user")
                        return True
    else:
        cla.log.debug(f"{fn} - users github_username is not defined - skipping github org approval list check")

    cla.log.debug(f"{fn} - unable to find user in any approval list")
    return False


def audit_event(func):
    """Decorator that audits events"""

    def wrapper(**kwargs):
        response = func(**kwargs)
        if response.get("status_code") == falcon.HTTP_200:
            cla.log.debug("Created event {} ".format(kwargs["event_type"]))
        else:
            cla.log.debug("Failed to add event")
        return response

    return wrapper


def get_oauth_client():
    return OAuth2Session(os.environ["GH_OAUTH_CLIENT_ID"])


def fmt_project(project: Project):
    return "{} ({})".format(project.get_project_name(), project.get_project_id())


def fmt_company(company: Company):
    return "{} ({}) - acl: {}".format(company.get_company_name(), company.get_company_id(), company.get_company_acl())


def fmt_user(user: User):
    return "{} ({}) {}".format(user.get_user_name(), user.get_user_id(), user.get_lf_email())


def fmt_users(users: List[User]):
    response = ""
    for user in users:
        response += fmt_user(user) + " "

    return response


def get_email_help_content(show_v2_help_link: bool) -> str:
    # v1 help link
    help_link = "https://docs.linuxfoundation.org/lfx/easycla"
    if show_v2_help_link:
        # v2 help link
        help_link = "https://docs.linuxfoundation.org/lfx/easycla"

    return f'<p>If you need help or have questions about EasyCLA, you can <a href="{help_link}" target="_blank">read the documentation</a> or <a href="https://jira.linuxfoundation.org/servicedesk/customer/portal/4/create/143" target="_blank">reach out to us for support</a>.</p>'


def get_email_sign_off_content() -> str:
    return "<p>Thanks,</p><p>EasyCLA Support Team</p>"


def get_corporate_url(project_version: str) -> str:
    """
    helper method that returns appropriate corporate link based on EasyCLA version
    :param project_version: cla_group version(v1|v2)
    :return: default is v1 corporate console
    """
    return CORPORATE_V2_BASE if project_version == "v2" else CORPORATE_BASE


def append_email_help_sign_off_content(body: str, project_version: str) -> str:
    """
    helper method which appends the help and sign off content to the body of the email
    :param body:
    :param project_version:
    :return:
    """
    return "".join(
        [
            body,
            get_email_help_content(project_version == "v2"),
            get_email_sign_off_content(),
        ]
    )


def append_email_help_sign_off_content_plain(body: str, project_version: str) -> str:
    """
    Wrapper method that appends the help and sign off content to the email body with no HTML formating
    :param body:
    :param project_version:
    :return:
    """
    return append_email_help_sign_off_content(body, project_version).replace("<p>", "").replace("</p>", "\n")


def get_current_time() -> str:
    """
    Helper function to return the current UTC datetime in an ISO standard format with timezone
    :return:
    """
    now = datetime.utcnow()
    return now.strftime("%Y-%m-%dT%H:%M:%S.%f%z") + "+0000"


def get_formatted_time(the_time: datetime) -> str:
    """
    Helper function to return the specified datetime object in an ISO standard format with timezone
    :return:
    """
    return the_time.strftime("%Y-%m-%dT%H:%M:%S.%f%z") + "+0000"


def get_time_from_string(date_string: str) -> Optional[datetime]:
    """
    Helper function to return the specified datetime object from an ISO standard format string
    :return:
    """
    # Try these formats
    formats = ["%Y-%m-%d %H:%M:%S.%f%z", "%Y-%m-%dT%H:%M:%S%z", "%Y-%m-%dT%H:%M:%S.%f%z", "%Y-%m-%dT%H:%M:%S.%f"]
    for fmt in formats:
        try:
            return datetime.strptime(date_string, fmt)
        except (ValueError, TypeError) as e:
            pass
            # print(f'unable to parse time {date_string} using {fmt}, error: {e}')
    return None


def get_public_email(user):
    """
    Helper function to return public user email to send emails
    """
    if len(user.get_all_user_emails()) > 0:
        return next((email for email in user.get_all_user_emails() if "noreply.github.com" not in email), None)


def get_co_authors_from_commit(commit):
    """
    Helper function to return co-authors from commit
    """
    fn = "get_co_authors_from_commit"
    co_authors = []
    if commit.commit:
        commit_message = commit.commit.message
        cla.log.debug(f"{fn} - commit message: {commit_message}")
        if commit_message:
            co_authors = re.findall(r"Co-authored-by: (.*) <(.*)>", commit_message)
    return co_authors


def extract_pull_request_number(pull_request_message):
    """
    Helper function to return pull request number from pull request message
    Extracts the pull request number from the first line of a message.
    It picks the last #number in the first line (GitHub appends it automatically).
    :param pull_request_message: message in merge_group payload
    :return:
    """
    fn = "extract_pull_request_number"
    pull_request_number = None
    try:
        first_line = pull_request_message.splitlines()[0]
        cla.log.debug(f"{fn} - checking line '{first_line}")
        # Case 1: "Merge pull request #N"
        matches = re.match(r'^Merge pull request #(\d+)', first_line)
        if matches:
            pull_request_number = int(matches.group(1))
            cla.log.debug(f"{fn} - extracted PR number {pull_request_number} from merge_queue data: {pull_request_message} by matching 'Merge pull request #N...'")
            return pull_request_number
        # Case 2: PR number in last (#N) group on first line, like: "Some text (#whatever) (#N)"
        matches = re.findall(r'\(#(\d+)\)', first_line)
        if matches:
            pull_request_number = int(matches[-1])  # last match
            cla.log.debug(f"{fn} - extracted PR number {pull_request_number} from merge_queue data: {pull_request_message} by matching '...(#N)'")
            return pull_request_number
        # Case 3: PR number in last #N on first line, like: "Some text #N"
        matches = re.findall(r"\s+#(\d+)", first_line)
        if matches:
            pull_request_number = int(matches[-1])  # last match
            cla.log.debug(f"{fn} - extracted PR number {pull_request_number} from merge_queue data: {pull_request_message} by matching '... #N'")
            return pull_request_number
        # Case 4: PR number in first #N in the entire commit message
        matches = re.findall(r"#(\d+)", pull_request_message)
        if matches:
            pull_request_number = int(matches[0])  # first match
            cla.log.debug(f"{fn} - extracted PR number {pull_request_number} from merge_queue data: {pull_request_message} by matching first '#N'")
            return pull_request_number
        else:
            cla.log.warning(f"{fn} - error - unable to extract pull request number from message: {pull_request_message}")
    except Exception as e:
        cla.log.warning(f"{fn} - error - unable to extract pull request number from message: {pull_request_message}, error: {e}")
    return pull_request_number
