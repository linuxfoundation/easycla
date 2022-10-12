# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Holds the model interfaces that all storage models must implement.
"""


class Project(object):  # pylint: disable=too-many-public-methods
    """
    Interface to the Project model.
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        :rtype: dict
        """
        raise NotImplementedError()

    def save(self):
        """
        Simple abstraction around the supported ORMs to save a model.

        Should also save the Documents tied to this model.
        """
        raise NotImplementedError()

    def load(self, project_id):
        """
        Simple abstraction around the supported ORMs to load a model.
        Should populate the current object and also load all the Documents.

        :param project_id: The project's ID.
        :type project_id: string
        """
        raise NotImplementedError()

    def delete(self):
        """
        Simple abstraction around the supported ORMs to delete a model.

        Should also delete the documents tied to this project.
        """
        raise NotImplementedError()

    def get_project_id(self):
        """
        Getter for the project's ID.

        :return: The project's ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_project_external_id(self):
        """
        Getter for the project's External ID.

        :return: The project's External ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_project_name(self):
        """
        Getter for the project's name.

        :return: The project's name.
        :rtype: string
        """
        raise NotImplementedError()

    def get_project_icla_enabled(self):
        """
        Getter to determine whether or not this project has an ICLA.

        :return: The project's ICLA state.
        :rtype: boolean
        """
        raise NotImplementedError()

    def get_project_ccla_enabled(self):
        """
        Getter to determine whether or not this project has an CCLA.

        :return: The project's CCLA state.
        :rtype: boolean
        """
        raise NotImplementedError()

    def get_project_individual_documents(self):
        """
        Getter for the project's individual signature documents.

        :return: The project ICLA documents.
        :rtype: [cla.models.model_interfaces.Document]
        """
        raise NotImplementedError()

    def get_project_corporate_documents(self):
        """
        Getter for the project's corporate signature documents.

        :return: The project CCLA documents.
        :rtype: [cla.models.model_interfaces.Document]
        """
        raise NotImplementedError()

    def get_project_ccla_requires_icla_signature(self):
        """
        Getter for the project's ccla_requires_icla_signature setting.

        :return: If the Project requires CCLAs employee to sign a iCLA.
        :rtype: bool
        """
        raise NotImplementedError()

    def get_project_current_major_version(self):
        """
        Getter for the project's current Major Document Version.

        :return: Version of the current Major Version.
        :rtype: int
        """
        raise NotImplementedError()

    def get_project_individual_document(self, major_version=None, minor_version=None):
        """
        Getter for the project's individual signature document given a version number.

        A version number of None for both major and minor should return the latest document.

        :param major_version: The major version requested. None for latest version.
        :type major_version: integer
        :param minor_version: The minor version requested. None for latest version.
        :type minor_version: integer
        :return: The project's ICLA document corresponding to the revision requested.
        :rtype: cla.models.model_interfaces.Document
        """
        raise NotImplementedError()

    def get_project_corporate_document(self, major_version=None, minor_version=None):
        """
        Getter for the project's corporate signature document by version number.

        A version number of None for major and minor should return the latest document.

        :param major_version: The major version requested. None for latest version.
        :type major_version: integer
        :param minor_version: The minor version requested. None for latest version.
        :type minor_version: integer
        :return: The project CCLA document requested.
        :rtype: cla.models.model_interfaces.Document
        """
        raise NotImplementedError()

    def set_project_id(self, project_id):
        """
        Setter for the project's ID.

        :param project_id: The project's ID.
        :type project_id: string
        """
        raise NotImplementedError()

    def set_project_external_id(self, project_external_id):
        """
        Setter for the project's External ID.

        :param project_external_id: The project's External ID.
        :type project_external_id: string
        """
        raise NotImplementedError()

    def set_project_name(self, project_name):
        """
        Setter for the project's name.

        :param project_name: The project's name.
        :type project_name: string
        """
        raise NotImplementedError()

    def set_project_individual_documents(self, documents):
        """
        Setter for the project's individual signature documents.

        :param documents: The project's individual documents.
        :type documents: [cla.models.model_interfaces.Document]
        """
        raise NotImplementedError()

    def set_project_corporate_documents(self, documents):
        """
        Setter for the project's corporate signature documents.

        :param document: The project's corporate documents.
        :type document: [cla.models.model_interfaces.Document]
        """
        raise NotImplementedError()

    def set_project_ccla_requires_icla_signature(self, ccla_requires_icla_signature):
        """
        Setter for the project's ccla_requires_icla_signature setting.

        :param ccla_requires_icla_signature
        :type bool
        """
        raise NotImplementedError()

    def add_project_individual_document(self, document):
        """
        Add a single individual document to this project.

        :param document: The document to add to this project as ICLA.
        :type document: cla.models.model_interfaces.Document
        """
        raise NotImplementedError()

    def add_project_corporate_document(self, document):
        """
        Add a single corporate document to this project.

        :param document: The document to add to this project as CCLA.
        :type document: cla.models.model_interfaces.Document
        """
        raise NotImplementedError()

    def remove_project_individual_document(self, document):
        """
        Removes a single individual document from this project.

        :param document: The ICLA document to remove from this project.
        :type document: cla.models.model_interfaces.Document
        """
        raise NotImplementedError()

    def remove_project_corporate_document(self, document):
        """
        Remove a single corporate document from this project.

        :param document: The CCLA document to remove from this project.
        :type document: cla.models.model_interfaces.Document
        """
        raise NotImplementedError()

    def get_project_repositories(self):
        """
        Getter for the project's repositories.

        :return: The project's repository objects.
        :rtype: [cla.models.model_interfaces.Repository]
        """
        raise NotImplementedError()

    def get_project_signatures(self, signature_signed=None, signature_approved=None):
        """
        Getter for the project's signatures.

        :param signature_signed: Whether or not to filter by signed signatures.
            None = no filter, True = only signed, False = only unsigned.
        :type signature_signed: boolean
        :param signature_approved: Whether or not to filter by approved signatures.
            None = no filter, True = only approved, False = only unapproved.
        :type signature_approved: boolean
        :return: The project's signature objects.
        :rtype: [cla.models.model_interfaces.Signature]
        """
        raise NotImplementedError()

    def get_projects_by_external_id(self, project_external_id):
        """
        Fetches the projects that matche the external ID provided.

        :param project_external_id: The project's external ID.
        :type project_external_id: string
        :return: List of projects that matches the external ID specified.
        :rtype: [cla.models.model_interfaces.Project]
        """
        raise NotImplementedError()

    def all(self, project_ids=None):
        """
        Fetches all projects in the CLA system.

        :param project_ids: List of project IDs to retrieve.
        :type project_ids: None or [string]
        :return: A list of project objects.
        :rtype: [cla.models.model_interfaces.Project]
        """
        raise NotImplementedError()


class User(object):
    """
    Interface to the User model.
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        :rtype: dict
        """
        raise NotImplementedError()

    def save(self):
        """
        Simple abstraction around the supported ORMs to save a model.
        """
        raise NotImplementedError()

    def load(self, user_id):
        """
        Simple abstraction around the supported ORMs to load a model.
        Should populate the current object.

        :param user_id: The user's ID.
        :type user_id: string
        """
        raise NotImplementedError()

    def delete(self):
        """
        Simple abstraction around the supported ORMs to delete a model.
        """
        raise NotImplementedError()

    def get_user_id(self):
        """
        Getter for the user's ID.

        :return: The user's ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_user_external_id(self):
        """
        Getter for the user's External ID.

        :return: The user's External ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_lf_email(self):
        raise NotImplementedError()

    def get_user_email(self):
        """
        Getter for the user's first email address.

        :return: The user's first email.
        :rtype: string
        """
        raise NotImplementedError()

    def get_user_emails(self):
        """
        Getter for a list of the user's email addresses.

        :return: List of emails for this user.
        :rtype: [string]
        """
        raise NotImplementedError()

    def get_user_name(self):
        """
        Getter for the user's name.

        :return: The user's name.
        :rtype: string
        """
        raise NotImplementedError()

    def get_user_github_id(self):
        """
        Getter for the user's GitHub ID.

        :return: The user's GitHub ID.
        :rtype: integer
        """
        raise NotImplementedError()

    def get_user_github_username(self):
        """
        Getter for the user's GitHub username.

        :return: The user's GitHub username.
        :rtype: string
        """
        raise NotImplementedError()

    def get_user_ldap_id(self):
        """
        Getter for the user's LDAP ID.

        :return: The user's LDAP ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_user_company_id(self):
        """
        Getter for the user's company ID.

        :return: The user's company ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_lf_username(self):
        """
        Getter for the user's Linux Foundation Username.
        """

        raise NotImplementedError()

    def get_lf_sub(self):
        raise NotImplementedError()

    def set_user_id(self, user_id):
        """
        Setter for the user's ID.

        :param user_id: The ID for this user.
        :type user_id: string
        """
        raise NotImplementedError()

    def set_user_external_id(self, user_external_id):
        """
        Setter for the user's External ID.

        :param user_external_id: The External ID for this user.
        :type user_external_id: string
        """
        raise NotImplementedError()

    def set_lf_email(self, lf_email):
        raise NotImplementedError()

    def set_user_email(self, user_email):
        """
        Will add a new email address for this user and ensure no duplicates.

        :param user_email: The new email for this user.
        :type user_email: string
        """
        raise NotImplementedError()

    def set_user_emails(self, user_emails):
        """
        Will explicitly set the user's email address list.

        :param user_emails: The list of emails to set for this user.
        :type user_emails: [string]
        """
        raise NotImplementedError()

    def set_user_name(self, user_name):
        """
        Setter for the user's name.

        :param user_name: The user's name.
        :type user_name: string
        """
        raise NotImplementedError()

    def set_user_company_id(self, company_id):
        """
        Setter for the user's company ID.

        :param company_id: The user's company ID.
        :type company_id: string
        """
        raise NotImplementedError()

    def set_user_github_id(self, user_github_id):
        """
        Setter for the user's GitHub ID.

        :param user_github_id: The user's GitHub ID.
        :type user_github_id: integer
        """
        raise NotImplementedError()

    def set_user_ldap_id(self, user_ldap_id):
        """
        Setter for the user's LDAP ID.

        :param user_ldap_id: The user's LDAP ID.
        :type user_ldap_id: integer
        """
        raise NotImplementedError()

    def set_lf_username(self, lf_username):
        """
        Setter for the user's Linux Foundation Username.
        :param lf_username: The user's LF Username.
        :type lf_username: string
        """

        raise NotImplementedError()

    def set_lf_sub(self, sub):
        raise NotImplementedError()

    def get_user_by_email(self, user_email):
        """
        Fetches the user object that matches the email specified.

        :param user_email: The user's email.
        :type user_email: string
        :return: The user object with the matching email address - None if not found.
        :rtype: cla.models.model_interfaces.User | None
        """
        raise NotImplementedError()

    def get_user_by_github_id(self, user_github_id):
        """
        Fetches the user object that matches the GitHub ID specified.

        :param user_github_id: The user's GitHub ID.
        :type user_github_id: integer
        :return: The user object with the GitHub ID, or None if not found.
        :rtype: cla.models.model_interfaces.User | None
        """
        raise NotImplementedError()

    def get_user_by_username(self, username):
        raise NotImplementedError()

    def get_user_signatures(self, project_id=None, company_id=None, signature_signed=None, signature_approved=None):
        """
        Fetches the signatures associated with this user.

        :param project_id: Filter for project IDs. None = no filter.
        :type project_id: string | None
        :param company_id: Filter employee signatures by company_id. If not provided, an ICLA will
            be retrieved instead of an employee signature.
        :type company_id: string
        :param signature_signed: Whether or not to filter by signed signatures.
            None = no filter, True = only signed, False = only unsigned.
        :type signature_signed: boolean
        :param signature_approved: Whether or not to filter by approved signatures.
            None = no filter, True = only approved, False = only unapproved.
        :type signature_approved: boolean
        :return: The signature objects associated with this user.
        :rtype: [cla.models.model_interfaces.Signature]
        """
        raise NotImplementedError()

    def get_users_by_company(self, company_id):
        """
        Fetches the users associated with an company.

        :param company_id: The company ID to filter users by.
        :type company_id: string
        :return: The signature objects associated with this user.
        :rtype: [cla.models.model_interfaces.Signature]
        """
        raise NotImplementedError()

    def all(self, emails=None):
        """
        Fetches all users in the CLA system.

        :param emails: List of user emails to retrieve.
        :type emails: None or [string]
        :return: A list of user objects.
        :rtype: [cla.models.model_interfaces.User]
        """
        raise NotImplementedError()


class Repository(object):
    """
    Interface to the Repository model.
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        :rtype: dict
        """
        raise NotImplementedError()

    def save(self):
        """
        Simple abstraction around the supported ORMs to save a model.
        """
        raise NotImplementedError()

    def load(self, repository_id):
        """
        Simple abstraction around the supported ORMs to load a model.
        Should populate the current object.

        :param repository_id: The repository ID of the repo to load.
        :type repository_id: string
        """
        raise NotImplementedError()

    def delete(self):
        """
        Simple abstraction around the supported ORMs to delete a model.
        """
        raise NotImplementedError()

    def get_repository_id(self):
        """
        Getter for a repository's ID.

        :return: The repository's ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_repository_project_id(self):
        """
        Getter for a repository's project ID.

        :return: The repository's project ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_repository_name(self):
        """
        Getter for a repository's name.

        :return: The repository's name.
        :rtype: string
        """
        raise NotImplementedError()

    def get_repository_type(self):
        """
        Getter for a repository's type ('github', 'gerrit', etc).

        :return: The repository type (github, gerrit, etc).
        :rtype: string
        """
        raise NotImplementedError()

    def get_repository_url(self):
        """
        Getter for a repository's accessible url.

        :return: The repository's accessible url.
        :rtype: string
        """
        raise NotImplementedError()

    def get_repository_external_id(self):
        """
        Getter for a repository's external ID. What the repository provider IDs
        this repository with.

        :return: The repository's external ID.
        :rtype: string
        """
        raise NotImplementedError()

    def set_repository_id(self, repo_id):
        """
        Setter for a repository ID.

        :param repo_id: The repo's ID.
        :type repo_id: string
        """
        raise NotImplementedError()

    def set_repository_project_id(self, project_id):
        """
        Setter for a repository's project ID.

        :param project_id: The repo's project ID.
        :type project_id: string
        """
        raise NotImplementedError()

    def set_repository_name(self, name):
        """
        Setter for a repository's name.

        :param name: The new repository name.
        :type name: string
        """
        raise NotImplementedError()

    def set_repository_type(self, repo_type):
        """
        Setter for a repository's type ('github', 'gerrit', etc).

        :param repo_type: The repository type ('github', 'gerrit', etc).
        :type repo_type: string
        """
        raise NotImplementedError()

    def set_repository_url(self, repository_url):
        """
        Setter for a repository's accessible url.

        :param repository_url: The repository url.
        :type repository_url: string
        """
        raise NotImplementedError()

    def set_repository_external_id(self, repository_external_id):
        """
        Setter for a repository's external ID. What the repository provider IDs
        this repository with.

        :param repository_external_id: The repository external ID.
        :type repository_external_id: string
        """
        raise NotImplementedError()

    def get_repository_by_external_id(self, repository_external_id, repository_type):
        """
        Loads the repository object based on the external ID specified.

        :param repository_external_id: The ID given to the repository by the external provider.
        :type repository_external_id: string
        :param repository_type: The type of repository (GitHub, Gerrit, etc).
        :type repository_type: string
        """
        raise NotImplementedError()

    def get_repositories_by_organization(self, organization_name):
        """
        Loads all repositories configured under this organization.

        :param organization_name: The organization name
        :type organization_name: string
        """
        raise NotImplementedError()

    def all(self, ids=None):
        """
        Fetches all repositories in the CLA system.

        :param ids: List of repository IDs to retrieve.
        :type ids: None or [string]
        :return: A list of repository objects.
        :rtype: [cla.models.model_interfaces.Repository]
        """
        raise NotImplementedError()


class Signature(object):  # pylint: disable=too-many-public-methods
    """
    Interface to the Signature model.
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        :rtype: dict
        """
        raise NotImplementedError()

    def save(self):
        """
        Simple abstraction around the supported ORMs to save a model.
        """
        raise NotImplementedError()

    def load(self, signature_id):
        """
        Simple abstraction around the supported ORMs to load a model.
        Populates the current object.

        :param signature_id: The signature ID of the repo to load.
        :type signature_id: string
        """
        raise NotImplementedError()

    def delete(self):
        """
        Simple abstraction around the supported ORMs to delete a model.
        """
        raise NotImplementedError()

    def get_signature_id(self):
        """
        Getter for an signature's ID.

        :return: The signature's ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_signature_external_id(self):
        """
        Getter for an signature's External ID.

        :return: The signature's External ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_signature_project_id(self):
        """
        Getter for an signature's project ID.

        :return: The signature's project ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_signature_document_minor_version(self):
        """
        Getter for an signature's document minor version.

        :return: The signature's document minor version.
        :rtype: integer
        """
        raise NotImplementedError()

    def get_signature_document_major_version(self):
        """
        Getter for an signature's document major version.

        :return: The signature's document major version.
        :rtype: integer
        """
        raise NotImplementedError()

    def get_signature_reference_id(self):
        """
        Getter for an signature's user or company ID, depending on the type
        of signature this is (individual or corporate).

        :return: The signature's user or company ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_signature_reference_type(self):
        """
        Getter for an signature's reference type - could be 'user' or 'company'.

        :return: The signature's reference type.
        :rtype: string
        """
        raise NotImplementedError()

    def get_signature_type(self):
        """
        Getter for an signature's type ('cla' or 'dco').

        :return: The signature type (cla or dco)
        :rtype: string
        """
        raise NotImplementedError()

    def get_signature_signed(self):
        """
        Getter for an signature's signed status. True for signed, False otherwise.

        :return: The signature's signed status. True if signed, False otherwise.
        :rtype: boolean
        """
        raise NotImplementedError()

    def get_signature_approved(self):
        """
        Getter for an signature's approval status. True is approved, False otherwise.

        :return: The signature's approval status. True is approved, False otherwise.
        :rtype: boolean
        """
        raise NotImplementedError()

    def get_signature_sign_url(self):
        """
        Getter for an signature's signing URL. The URL the user has to visit in
        order to sign the signature.

        :return: The signature's signing URL.
        :rtype: string
        """
        raise NotImplementedError()

    def get_signature_return_url(self):
        """
        Getter for an signature's return URL. The URL the user gets sent to after signing.

        :return: The signature's return URL.
        :rtype: string
        """
        raise NotImplementedError()

    def get_signature_callback_url(self):
        """
        Getter for an signature's callback URL. The URL that the signing service provider should
        hit once signature has been confirmed.

        :return: The signature's callback URL.
        :rtype: string
        """
        raise NotImplementedError()

    def get_signature_user_ccla_company_id(self):
        """
        Getter for the company ID of the user's CCLA. This is populated when a CCLA is signed by a
        user stating that they work for a particular company that has a CCLA with a project.

        This is not the same as a user signing a ICLA or a company signing a CCLA.

        :return: The company ID associated with the user's CCLA.
        :rtype: string
        """
        raise NotImplementedError()

    def get_domain_whitelist(self):
        raise NotImplementedError()

    def get_email_whitelist(self):
        raise NotImplementedError()

    def get_github_whitelist(self):
        raise NotImplementedError()

    def get_github_org_whitelist(self):
        raise NotImplementedError()

    def get_gitlab_org_approval_list(self):
        raise NotImplementedError

    def get_gitlab_username_approval_list(self):
        raise NotImplementedError

    def get_note(self):
        raise NotImplementedError()

    def get_auto_create_ecla(self):
        raise NotImplementedError()

    def set_signature_id(self, signature_id):
        """
        Setter for an signature ID.

        :param signature_id: The signature's ID.
        :type signature_id: string
        """
        raise NotImplementedError()

    def set_signature_external_id(self, signature_external_id):
        """
        Setter for an signature External ID.

        :param signature_external_id: The signature's External ID.
        :type signature_external_id: string
        """
        raise NotImplementedError()

    def set_signature_project_id(self, project_id):
        """
        Setter for an signature's project ID.

        :param project_id: The signature's project ID.
        :type project_id: string
        """
        raise NotImplementedError()

    def set_signature_document_minor_version(self, document_minor_version):
        """
        Setter for an signature's document minor version.

        :param document_minor_version: The signature's document minor version.
        :type document_minor_version: string
        """
        raise NotImplementedError()

    def set_signature_document_major_version(self, document_major_version):
        """
        Setter for an signature's document major version.

        :param document_major_version: The signature's document major version.
        :type document_major_version: string
        """
        raise NotImplementedError()

    def set_signature_reference_id(self, reference_id):
        """
        Setter for an signature's reference ID.

        :param reference_id: The signature's reference ID.
        :type reference_id: string
        """
        raise NotImplementedError()

    def set_signature_reference_type(self, reference_type):
        """
        Setter for an signature's reference type.

        :param reference_type: The signature's reference type ('user' or 'company').
        :type reference_type: string
        """
        raise NotImplementedError()

    def set_signature_type(self, signature_type):
        """
        Setter for an signature's type ('cla' or 'dco').

        :param signature_type: The signature type ('cla' or 'dco').
        :type signature_type: string
        """
        raise NotImplementedError()

    def set_signature_signed(self, signed):
        """
        Setter for an signature's signed status.

        :param signed: Signed status. True for signed, False otherwise.
        :type signed: bool
        """
        raise NotImplementedError()

    def set_signature_approved(self, approved):
        """
        Setter for an signature's approval status.

        :param approved: Approved status. True for approved, False otherwise.
        :type approved: bool
        """
        raise NotImplementedError()

    def set_signature_sign_url(self, sign_url):
        """
        Setter for an signature's signing URL. Optional on signature creation.
        The signing provider's request_individual_signatures() method will populate the field.

        :param sign_url: The URL the user must visit in order to sign the signature.
        :type sign_url: string
        """
        raise NotImplementedError()

    def set_signature_return_url(self, return_url):
        """
        Setter for an signature's return URL. Optional on signature creation.

        If this value is not set, the CLA system will do it's best to redirect the user to the
        appropriate location once signing is complete (project or repository page).

        :param return_url: The URL the user will be redirected to once signing is complete.
        :type return_url: string
        """
        raise NotImplementedError()

    def set_signature_callback_url(self, callback_url):
        """
        Setter for an signature's callback URL. Optional on signature creation.

        If this value is not set, the signing service provider will not fire a callback request
        when the user's signature has been confirmed.

        :param callback_url: The URL that will hit once the user has signed.
        :type callback_url: string
        """
        raise NotImplementedError()

    def set_signature_user_ccla_company_id(self, company_id):
        """
        Setter for the company ID of the user's CCLA. This is populated when a CCLA is signed by a
        user stating that they work for a particular company that has a CCLA with a project.

        This is not the same as a user signing a ICLA or a company signing a CCLA.

        :param company_id: The company ID associated with the user's CCLA.
        :type: string
        """
        raise NotImplementedError()

    def get_signatures_by_reference(
            self,
            reference_id,
            reference_type,  # pylint: disable=too-many-arguments
            project_id=None,
            signature_signed=None,
            signature_approved=None,
    ):
        """
        Simple abstraction around the supported ORMs to get a user's or
        orgnanization's signatures.

        :param reference_id: The reference ID (user_id or company_id) for
            whom we'll be fetching signatures.
        :type reference_id: string
        :param reference_type: The reference type ('user' or 'company') for
            whom we'll be fetching signatures.
        :type reference_id: string
        :param project_id: The project ID to filter by. None will not apply any filters.
        :type project_id: string or None
        :param signature_signed: Whether or not to return only signed/unsigned signatures.
            None will not apply any filters for signed.
        :type signature_signed: bool or None
        :param signature_approved: Whether or not to return only approved/unapproved signatures.
            None will not apply any filters for approved.
        :type signature_approved: bool or None
        :return: List of signatures.
        :rtype: [cla.models.model_interfaces.Signature]
        """
        raise NotImplementedError()

    def get_signatures_by_project(self, project_id, signature_signed=None, signature_approved=None):
        """
        Simple abstraction around the supported ORMs to get a project's signatures.

        :param project_id: The project ID we'll be fetching signatures for.
        :type project_id: string
        :param signature_signed: Whether or not to return only signed/unsigned signatures.
            None will not apply any filters for signed.
        :type signature_signed: bool or None
        :param signature_approved: Whether or not to return only approved/unapproved signatures.
            None will not apply any filters for approved.
        :type signature_approved: bool or None
        :return: List of signatures.
        :rtype: [cla.models.model_interfaces.Signature]
        """
        raise NotImplementedError()

    def get_signatures_by_company_project(self, company_id, project_id):
        """
        Simple abstraction around the supported ORMs to get signatures based on projects and company.

        :param: company_id: The company ID we'll be fetching signatures for.
        :param: project_id: The project ID we'll be fetching signatures for.
        :type:  company_id: string
        :type:  project_id: string
        :return: Dictionary of signatures.
        :rtype: [cla.models.model_interfaces.Signature]
        """
        raise NotImplementedError()

    def all(self, ids=None):
        """
        Fetches all signatures in the CLA system.

        :param ids: List of signature IDs to retrieve.
        :type ids: None or [string]
        :return: A list of signature objects.
        :rtype: [cla.models.model_interfaces.Signature]
        """
        raise NotImplementedError()


class Company(object):  # pylint: disable=too-many-public-methods
    """
    Interface to the Company model.
    """

    def to_dict(self) -> dict:
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        :rtype: dict
        """
        raise NotImplementedError()

    def save(self) -> None:
        """
        Simple abstraction around the supported ORMs to save a model.
        """
        raise NotImplementedError()

    def load(self, company_id) -> None:
        """
        Simple abstraction around the supported ORMs to load a model.
        Populates the current object.

        :param company_id: The ID of the company to load.
        :type company_id: string
        """
        raise NotImplementedError()

    def delete(self) -> None:
        """
        Simple abstraction around the supported ORMs to delete a model.
        """
        raise NotImplementedError()

    def get_company_id(self) -> str:
        """
        Getter for an company's ID.

        :return: The company's ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_company_external_id(self) -> str:
        """
        Getter for an company's External ID.

        :return: The company's External ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_company_manager_id(self) -> str:
        """
        Getter for the company's CLA manager user ID.

        :return: The company's CLA manager user ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_company_name(self) -> str:
        """
        Getter for an company's name.

        :return: The company's name.
        :rtype: string
        """
        raise NotImplementedError()

    def get_signing_entity_name(self) -> str:
        """
        Getter for an company's signing entity name.

        :return: The company's signing entity name.
        :rtype: string
        """
        raise NotImplementedError()

    def get_company_signatures(self, project_id: str = None, signature_signed: bool = None,
                               signature_approved: bool = None):
        """
        Simple abstraction around the supported ORMs to fetch signatures for
        an company.

        :param project_id: Filter signatures by project_id.
        :type project_id: string or None
        :param signature_signed: Whether or not to return only signed/unsigned signatures.
            None will return all signatures for this company.
        :type signature_signed: bool or None
        :param signature_approved: Whether or not to return only approved signatures.
        :type signature_approved: bool or None
        :return: The filtered signatures for this company.
        :rtype: [cla.models.model_interfaces.Signature]
        """
        raise NotImplementedError()

    def set_company_id(self, company_id: str) -> None:
        """
        Setter for an company ID.

        :param company_id: The company's ID.
        :type company_id: string
        """
        raise NotImplementedError()

    def set_company_external_id(self, company_external_id: str) -> None:
        """
        Setter for an company External ID.

        :param company_external_id: The company's External ID.
        :type company_external_id: string
        """
        raise NotImplementedError()

    def set_company_manager_id(self, company_manager_id: str) -> None:
        """
        Setter for the company manager ID.

        :param company_manager_id: The company manager ID.
        :type company_manager_id: string
        """
        raise NotImplementedError()

    def set_company_name(self, company_name: str) -> None:
        """
        Setter for an company's name.

        :param company_name: The name of the company.
        :type company_name: string
        """
        raise NotImplementedError()

    def set_signing_entity_name(self, signing_entity_name: str) -> None:
        """
        Setter for an company's name.

        :param signing_entity_name: The name of the company's signing entity name.
        :type signing_entity_name: string
        """
        raise NotImplementedError()

    def set_company_whitelist(self, whitelist):
        """
        Setter for an company's whitelisted domain names.

        :param whitelist: The list of domain names to mark as safe.
            Example: ['ibm.com', 'ibm.ca']
        :type whitelist: list of strings
        """
        raise NotImplementedError()

    def add_company_whitelist(self, whitelist_item):
        """
        Adds another entry in the list of whitelisted domain names.
        Does not query the DB - save() will take care of that.

        :param whitelist_item: A domain name to add to the whitelist of this company.
        :type whitelist_item: string
        """
        raise NotImplementedError()

    def remove_company_whitelist(self, whitelist_item):
        """
        Removes an entry from the list of whitelisted domain names.
        Does not query the DB - save() will take care of that.

        :param whitelist_item: A domain name to remove from the whitelist of this company.
        :type whitelist_item: string
        """
        raise NotImplementedError()

    def set_company_whitelist_patterns(self, whitelist_patterns):
        """
        Setter for an company's whitelist regex patterns.

        :param whitelist_patterns: The list of email patterns to exlude from signing.
            Example: ['.*@ibm.co.uk$', '^info.*']
        :type whitelist_patterns: list of strings

        :todo: Need to actually test out those examples.
        """
        raise NotImplementedError()

    def add_company_whitelist_pattern(self, whitelist_pattern):
        """
        Adds another entry in the list of whitelistd patterns.
        Does not query the DB - save() will take care of that.

        :param whitelist_pattern: A regex string to add to the excluded patterns of this company.
        :type whitelist_pattern: string
        """
        raise NotImplementedError()

    def remove_company_whitelist_pattern(self, whitelist_pattern):
        """
        Removes an entry from the list of whitelisted domain names.
        Does not query the DB - save() will take care of that.

        :param whitelist_pattern: A regex string to remove from the exluded patterns
            of this company.
        :type whitelist_pattern: string
        """
        raise NotImplementedError()

    def get_company_by_external_id(self, company_external_id):
        """
        Fetches the company that matches the external ID provided.

        :param company_external_id: The company's external ID.
        :type company_external_id: string
        :return: The company that matches the external ID specified.
        :rtype: cla.models.model_interfaces.Company
        """
        raise NotImplementedError()

    def get_companies_by_manager(self, manager_id):
        """
        Fetches the companies a manager is part of given manager_id.

        :param manager_id: The managers id.
        :type manager_id: string
        :return: The companies that match that manager_id specified.
        :rtype: cla.models.model_interfaces.Company
        """
        raise NotImplementedError()

    def all(self, ids=None):
        """
        Fetches all companies in the CLA system.

        :param ids: List of company IDs to retrieve.
        :type ids: None or [string]
        :return: A list of company objects.
        :rtype: [cla.models.model_interfaces.Company]
        """
        raise NotImplementedError()


class Document(object):
    """
    Interface to the Document model.

    Save/Load/Delete operations should be done through the Project model.
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        :rtype: dict
        """
        raise NotImplementedError()

    def get_document_name(self):
        """
        Getter for the document's name.

        :return: The document's name.
        :rtype: string
        """
        raise NotImplementedError()

    def get_document_file_id(self):
        """
        Getter for the document's file ID used as filename for storage.

        :return: The document's file ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_document_content_type(self):
        """
        Getter for the document's content type.

        :return: The document's content type.
        :rtype: string
        """
        raise NotImplementedError()

    def get_document_content(self):
        """
        Getter for the document's content.

        If content type starts with 'storage+', should utilize the storage service to fetch
        the document content. Otherwise, return the content of this field (assumes document
        content stored in DB).

        :return: The document's content.
        :rtype: string
        """
        raise NotImplementedError()

    def get_document_author_name(self):
        """
        Getter for the document's author name.

        :return: The document's author name.
        :rtype: string
        """
        raise NotImplementedError()

    def get_document_major_version(self):
        """
        Getter for the document's major version number.

        :return: The document's major version number.
        :rtype: integer
        """
        raise NotImplementedError()

    def get_document_minor_version(self):
        """
        Getter for the document's minor version number.

        :return: The document's minor version number.
        :rtype: integer
        """
        raise NotImplementedError()

    def get_document_creation_date(self):
        """
        Getter for the document's creation date.

        :return: The document's creation date.
        :rtype: datetime
        """
        raise NotImplementedError()

    def get_document_preamble(self):
        """
        Getter for the document's preamble text.

        :return: The document's preamble text.
        :rtype: string
        """
        raise NotImplementedError()

    def get_document_legal_entity_name(self):
        """
        Getter for the legal entity name on the document.

        :return: The legal entity name on this document.
        :rtype: string
        """
        raise NotImplementedError()

    def get_document_tabs(self):
        """
        Getter for the document's field metadata information.
        This information is used to generate documents with fields that will capture user data.

        :return: The list of tabs for this document.
        :rtype: [cla.models.model_interfaces.DocumentTab]
        """
        raise NotImplementedError()

    def set_document_name(self, document_name):
        """
        Setter for the document's name.

        :param document_name: The document's name.
        :type document_name: string
        """
        raise NotImplementedError()

    def set_document_file_id(self, document_file_id):
        """
        Setter for the document's file ID that's used as filename in storage.

        :param document_file_id: The document's file ID.
        :type document_file_id: string
        """
        raise NotImplementedError()

    def set_document_content_type(self, document_content_type):
        """
        Setter for the document's content type.

        :param document_content_type: The document's content type.
        :type document_content_type: string
        """
        raise NotImplementedError()

    def set_document_content(self, document_content, b64_encoded=True):
        """
        Setter for the document's content.

        If content type starts with 'storage+', should utilize the storage service to save
        the document content. Otherwise, simply store the value provided in the DB.
        The value provided could be a URL (for content type such as 'url+pdf') or
        the raw binary data of the document content.

        NOTE: document_file_id should be used as filename when storing with storage service.
        If document_file_id is None, one needs to be provided before saving (typically a UUID).

        :param document_content: The document's content.
        :type document_content: string
        :param b64_encoded: Whether or not the contents should be base64 decoded before saving.
        :type b64_encoded: boolean
        """
        raise NotImplementedError()

    def set_document_author_name(self, document_author_name):
        """
        Setter for the document's author name.

        :param document_author_name: The name of the author.
        :type document_author_name: string
        """
        raise NotImplementedError()

    def set_document_major_version(self, version):
        """
        Setter for the document's major version number.

        :param version: The document's major version number.
        :type version: integer
        """
        raise NotImplementedError()

    def set_document_minor_version(self, version):
        """
        Setter for the document's minor version number.

        :param version: The document's minor version number.
        :type version: integer
        """
        raise NotImplementedError()

    def set_document_creation_date(self, document_creation_date):
        """
        Setter for the document's creation date.

        :param document_creation_date: The document's creation date to set.
        :type document_creation_date: datetime
        """
        raise NotImplementedError()

    def set_document_preamble(self, document_preamble):
        """
        Setter for the document's preamble text.

        :param document_preamble: The preamble text for this document.
        :type document_preamble: string
        """
        raise NotImplementedError()

    def set_document_legal_entity_name(self, entity_name):
        """
        Setter for the legal entity name on the document.

        :param entity_name: The legal entity name on the document.
        :type entity_name: string
        """
        raise NotImplementedError()

    def set_document_tabs(self, tabs):
        """
        Setter for the document's field metadata information.

        :param tabs: List of tabs to set for this document.
        :type tabs: [cla.models.model_interfaces.DocumentTab]
        """
        raise NotImplementedError()

    def set_raw_document_tabs(self, tabs_data):
        """
        Same as set_document_tabs except it accepts a list of dict of values instead.

        :param tabs_data: List of dict of tab data to set for this document.
        :type tabs_data: [dict]
        """
        raise NotImplementedError()

    def add_document_tab(self, tab):
        """
        Adds another tab to the list of tabs in this document.

        :param tab: The tab to add.
        :type tab: cla.models.model_interfaces.DocumentTab
        """
        raise NotImplementedError()

    def add_raw_document_tab(self, tab_data):
        """
        Same as add_document_tab except it accepts a dict of values instead.

        :param tab_data: Data on the tab to add.
        :type tab_data: dict
        """
        raise NotImplementedError()


class DocumentTab(object):
    """
    Interface to a Document tab.
    """

    def to_dict(self):
        """
        Converts a DocumentTab into a python dict for json serialization.

        :return: A dict representation of the DocumentTab.
        :rtype: dict
        """
        raise NotImplementedError()

    def get_document_tab_type(self):
        """
        Getter for the document tab type.

        :return: The document tab type.
        :rtype: string
        """
        raise NotImplementedError()

    def get_document_tab_name(self):
        """
        Getter for the document tab name.

        :return: The document tab name.
        :rtype: string
        """
        raise NotImplementedError()

    def get_document_tab_page(self):
        """
        Getter for the document tab's page number.

        :return: The document tab's page number.
        :rtype: int
        """
        raise NotImplementedError()

    def get_document_tab_position_x(self):
        """
        Getter for the document tab's X position.

        :return: The document tab's X position.
        :rtype: int
        """
        raise NotImplementedError()

    def get_document_tab_position_y(self):
        """
        Getter for the document tab's Y position.

        :return: The document tab's Y position.
        :rtype: int
        """
        raise NotImplementedError()

    def get_document_tab_width(self):
        """
        Getter for the document tab's width.

        :return: The document tab's width.
        :rtype: int
        """
        raise NotImplementedError()

    def get_document_tab_height(self):
        """
        Getter for the document tab's height.

        :return: The document tab's height.
        :rtype: int
        """
        raise NotImplementedError()

    def set_document_tab_type(self, tab_type):
        """
        Setter for the document tab type.

        :param tab_type: The document tab type.
        :type tab_type: string
        """
        raise NotImplementedError()

    def set_document_tab_name(self, tab_name):
        """
        Setter for the document tab name.

        :param tab_name: The document tab name.
        :type tab_name: string
        """
        raise NotImplementedError()

    def set_document_tab_page(self, tab_page):
        """
        Setter for the document tab's page number.

        :param tab_page: The document tab's page number.
        :type tab_page: int
        """
        raise NotImplementedError()

    def set_document_tab_position_x(self, tab_position_x):
        """
        Setter for the document tab's X position.

        :param tab_position_x: The document tab's X position.
        :type tab_position_x: int
        """
        raise NotImplementedError()

    def set_document_tab_position_y(self, tab_position_y):
        """
        Setter for the document tab's Y position.

        :param tab_position_y: The document tab's Y position.
        :type tab_position_y: int
        """
        raise NotImplementedError()

    def set_document_tab_width(self, tab_width):
        """
        Setter for the document tab's width.

        :param tab_width: The document tab's width.
        :type tab_width: int
        """
        raise NotImplementedError()

    def set_document_tab_height(self, tab_height):
        """
        Setter for the document tab's height.

        :param tab_height: The document tab's height.
        :type tab_height: int
        """
        raise NotImplementedError()


class GitlabOrg(object):
    """
    Interface to the GitlabOrg model
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        :rtype: dict
        """
        raise NotImplementedError()

    def save(self):
        """
        Simple abstraction around the supported ORMs to save a model.
        """
        raise NotImplementedError()

    def load(self, organization_id):
        """
        Simple abstraction around the supported ORMs to load a model.
        Should populate the current object.

        :param organization_id: The gitlab organization's ID.
        :type organization_id: string
        """
        raise NotImplementedError()

    def delete(self):
        """
        Simple abstraction around the supported ORMs to delete a model.
        """
        raise NotImplementedError()

    def all(self):
        """
        Fetches all gitlab organizations in the CLA system.

        :return: A list of GitlabOrg objects.
        :rtype: [cla.models.model_interfaces.GitlabOrg]
        """
        raise NotImplementedError()


class GitHubOrg(object):
    """
    Interface to the GitHubOrg model.
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        :rtype: dict
        """
        raise NotImplementedError()

    def save(self):
        """
        Simple abstraction around the supported ORMs to save a model.
        """
        raise NotImplementedError()

    def load(self, organization_id):
        """
        Simple abstraction around the supported ORMs to load a model.
        Should populate the current object.

        :param organization_id: The github organization's ID.
        :type organization_id: string
        """
        raise NotImplementedError()

    def delete(self):
        """
        Simple abstraction around the supported ORMs to delete a model.
        """
        raise NotImplementedError()

    def get_organization_name(self):
        """
        Getter for the github organization's Name.

        :return: The github organization's Name.
        :rtype: string
        """
        raise NotImplementedError()

    def get_organization_company_id(self):
        """
        Getter for the github organization's company id.

        :return: The github organization's id.
        :rtype: string
        """
        raise NotImplementedError()

    def get_organization_installation_id(self):
        """
        Getter for the github organization's installation id.

        :return: The github organization's installation id.
        :rtype: string
        """
        raise NotImplementedError()

    def set_organization_name(self, organization_name):
        """
        Setter for the github organization's name.

        :param organization_name: The Name for this github organization.
        :type organization_name: string
        """
        raise NotImplementedError()

    def set_organization_installation_id(self, organization_installation_id):
        """
        Setter for the github organization's installation id.

        :param organization_installation_id: The github organization's installation id.
        :type organization_installation_id: string
        """
        raise NotImplementedError()

    def get_organization_by_sfid(self, sfid):
        """
        Fetches the github organizations associated with a sfid

        :param sfid: The SFDC ID to filter github organizations by.
        :type sfid: string
        :return: The organization associated with the project specified.
        :rtype: [cla.models.model_interfaces.GitHubOrg]
        """
        raise NotImplementedError()

    def all(self):
        """
        Fetches all github organizations in the CLA system.

        :return: A list of GitHubOrg objects.
        :rtype: [cla.models.model_interfaces.GitHubOrg]
        """
        raise NotImplementedError()


class CLAManagerRequest(object):
    """
    Interface to the CLAManagerRequest model.
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        :rtype: dict
        """
        raise NotImplementedError()

    def save(self):
        """
        Simple abstraction around the supported ORMs to save a model.
        """
        raise NotImplementedError()

    def load(self, request_id):
        """
        Simple abstraction around the supported ORMs to load a model.
        Should populate the current object.

        :param request_id: The Request ID.
        :type request_id: string
        """
        raise NotImplementedError()

    def delete(self):
        """
        Simple abstraction around the supported ORMs to delete a model.
        """
        raise NotImplementedError()

    def all(self):
        """
        Fetches all CLAManagerRequest instances in the CLA system.

        :return: A list of CLAManagerRequest Instance objects.
        :rtype: [cla.models.model_interfaces.CLAManagerRequest]
        """
        raise NotImplementedError()


class Gerrit(object):
    """
    Interface to the Gerrit model.
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        :rtype: dict
        """
        raise NotImplementedError()

    def save(self):
        """
        Simple abstraction around the supported ORMs to save a model.
        """
        raise NotImplementedError()

    def load(self, gerrit_id):
        """
        Simple abstraction around the supported ORMs to load a model.
        Should populate the current object.

        :param gerrit_id: The Gerrit instance's ID.
        :type gerrit_id: string
        """
        raise NotImplementedError()

    def delete(self):
        """
        Simple abstraction around the supported ORMs to delete a model.
        """
        raise NotImplementedError()

    def all(self):
        """
        Fetches all gerrit instances in the CLA system.

        :return: A list ofG Gerrit Instance objects.
        :rtype: [cla.models.model_interfaces.Gerrit]
        """
        raise NotImplementedError()


class UserPermissions(object):
    """
    Interface to the UserPermissions model.
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        :rtype: dict
        """
        raise NotImplementedError()

    def save(self):
        """
        Simple abstraction around the supported ORMs to save a model.
        """
        raise NotImplementedError()

    def load(self, user_id):
        """
        Simple abstraction around the supported ORMs to load a model.
        Should populate the current object.

        :param user_id: The user's ID.
        :type user_id: string
        """
        raise NotImplementedError()

    def delete(self):
        """
        Simple abstraction around the supported ORMs to delete a model.
        """
        raise NotImplementedError()

    def get_gerrit_by_project_id(self):
        """
        Gets all gerrit instances by a project ID.
        """
        raise NotImplementedError()

    def all(self):
        """
        Fetches all github organizations in the CLA system.

        :return: A list of UserPermission objects.
        :rtype: [cla.models.model_interfaces.UserPermission]
        """
        raise NotImplementedError()


class CompanyInvite(object):
    """
    Interface to the CompanyInvite model.
    """

    def to_dict(self):
        raise NotImplementedError()

    def save(self):
        raise NotImplementedError()

    def load(self, company_invite_id):
        raise NotImplementedError()

    def delete(self):
        raise NotImplementedError()


class Event(object):
    """
    Interface to the Event model
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        "rtype: dict
        """
        raise NotImplementedError()

    def save(self):
        """
        Simple abstraction around the supported ORMs to save a model
        """
        raise NotImplementedError()

    def load(self, event_id):
        """
        Simple abstraction around the supported ORMs to load a model
        Populates the current object.

        :param event_id: The ID of the event to load
        :type event_id: string
        """
        raise NotImplementedError()

    def get_event_id(self):
        """
        Getter for events' ID

        :return: The events ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_event_user_id(self):
        """
        Getter for the event's user ID

        :return: The users ID

        """

        raise NotImplementedError()

    def get_event_type(self):
        """
        Getter for event type

        :return: The events ID.
        :rtype: string
        """
        raise NotImplementedError()

    def get_event_project_id(self):
        """
        Getter for the event's project ID.

        :return: The events user ID.
        :rtype: string
        """

        raise NotImplementedError()

    def get_event_company_id(self):
        """
        Getter for the event's project ID

        :return: the events project ID.
        :rtype: string
        """

        raise NotImplementedError()

    def get_event_time(self):
        """
        Getter for the event time

        :return: the event time
        :rtype: string
        """

        raise NotImplementedError()

    def get_event_data(self):
        raise NotImplementedError()

    def get_events(self, event_id=None, event_type=None):
        raise NotImplementedError()

    def set_event_id(self, event_id):
        raise NotImplementedError()

    def set_event_user_id(self, user_id):
        raise NotImplementedError()

    def set_event_type(self, event_type):
        raise NotImplementedError()

    def set_event_project_id(self, event_project_id):
        raise NotImplementedError()

    def set_event_company_id(self, company_id):
        raise NotImplementedError()

    def set_event_data(self, event_data):
        raise NotImplementedError()

    def set_event_time(self, event_time):
        raise NotImplementedError()

    def all(self, ids=None):
        """
        Fetches all events in the CLA system

        :param ids: List of event IDs to retrieve
        :type ids: None or [string]
        :return: A list of event objects.
        """

        raise NotImplementedError()


class ProjectCLAGroup(object):
    """
    Interface to the ProjectCLA Model
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        "rtype: dict
        """
        raise NotImplementedError()

    def save(self):
        """
        Simple abstraction around the supported ORMs to save a model
        """
        raise NotImplementedError()

    def delete(self):
        """
        Simple abstraction around the supported ORMs to delete a model
        """
        raise NotImplementedError()

    def load(self, project_sfid):
        """
        Simple abstraction around the supported ORMs to load a model
        Populates the current object.

        :param project_sfid: The ID of the projectCLAGroup to load
        :type project_sfid: string
        """
        raise NotImplementedError()

    def all(self, project_sfids):
        """
        Fetches all projectCLAGroups in the CLA system.

        :param project_sfids: List of project_sfids to retrieve
        :return: A list of projectCLAGroup objects.
        :rtype: [cla.models.model_interfaces.ProjectCLAGroup]
        """
        raise NotImplementedError()


class CCLAWhitelistRequest(object):
    """
    Interface to the CCLAWhitelistRequest Model
    """

    def to_dict(self):
        """
        Converts models to dictionaries for JSON serialization.

        :return: A dict representation of the model.
        "rtype: dict
        """
        raise NotImplementedError()

    def save(self):
        """
        Simple abstraction around the supported ORMs to save a model
        """
        raise NotImplementedError()

    def delete(self):
        """
        Simple abstraction around the supported ORMs to delete a model
        """
        raise NotImplementedError()

    def load(self, request_id):
        """
        Simple abstraction around the supported ORMs to load a model
        Populates the current object.

        :param request_id: The id of the ccla whitelist request
        :type request_id: string
        """
        raise NotImplementedError()

    def all(self):
        """
        Fetches all CCLAWhitelistRequests in the CLA system.

        :return: A list of projectCLAGroup objects.
        :rtype: [cla.models.model_interfaces.ProjectCLAGroup]
        """
        raise NotImplementedError()
