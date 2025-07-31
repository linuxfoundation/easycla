# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Easily access CLA models backed by DynamoDB using pynamodb.
"""

import base64
import datetime
import os
import re
import time
import uuid
from datetime import timezone
from typing import Optional, List
from dateutil.parser import parse as parsedatestring

import dateutil.parser
from pynamodb import attributes
from pynamodb.attributes import (
    UTCDateTimeAttribute,
    UnicodeSetAttribute,
    UnicodeAttribute,
    BooleanAttribute,
    NumberAttribute,
    ListAttribute,
    JSONAttribute,
    MapAttribute,
)
from pynamodb.expressions.condition import Condition
from pynamodb.indexes import GlobalSecondaryIndex, AllProjection
from pynamodb.models import Model

import cla
from cla.models import model_interfaces, key_value_store_interface, DoesNotExist
from cla.models.event_types import EventType
from cla.models.model_interfaces import User, Signature, ProjectCLAGroup, Repository, Gerrit
from cla.models.model_utils import is_uuidv4
from cla.project_service import ProjectService

stage = os.environ.get("STAGE", "")
cla_logo_url = os.environ.get("CLA_BUCKET_LOGO_URL", "")


def create_database():
    """
    Named "create_database" instead of "create_tables" because create_database
    is expected to exist in all database storage wrappers.
    """
    tables = [
        RepositoryModel,
        ProjectModel,
        SignatureModel,
        CompanyModel,
        UserModel,
        StoreModel,
        GitHubOrgModel,
        GerritModel,
        EventModel,
        CCLAWhitelistRequestModel,

    ]
    # Create all required tables.
    for table in tables:
        # Wait blocks until table is created.
        table.create_table(wait=True)


def delete_database():
    """
    Named "delete_database" instead of "delete_tables" because delete_database
    is expected to exist in all database storage wrappers.

    WARNING: This will delete all existing table data.
    """
    tables = [
        RepositoryModel,
        ProjectModel,
        SignatureModel,
        CompanyModel,
        UserModel,
        StoreModel,
        GitHubOrgModel,
        GerritModel,
        CCLAWhitelistRequestModel,
    ]
    # Delete all existing tables.
    for table in tables:
        if table.exists():
            table.delete_table()


class GitHubUserIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying users by GitHub ID.
    """

    class Meta:
        """Meta class for GitHub User index."""

        index_name = "github-id-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    user_github_id = NumberAttribute(hash_key=True)


class SignatureProjectExternalIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying signatures by project external ID
    """

    class Meta:
        """ Meta class for Signature Project External Index """

        index_name = "project-signature-external-id-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index
    signature_project_external_id = UnicodeAttribute(hash_key=True)


class SignatureCompanySignatoryIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying signatures by signature company signatory ID
    """

    class Meta:
        """ Meta class for Signature Company Signatory Index """

        index_name = "signature-company-signatory-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    signature_company_signatory_id = UnicodeAttribute(hash_key=True)


class SignatureProjectReferenceIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying signatures by project reference ID
    """

    class Meta:
        """ Meta class for Signature Project Reference Index """

        index_name = "signature-project-reference-index"
        write_capacity_units = 10
        read_capacity_units = 10
        projection = AllProjection()

    signature_project_id = UnicodeAttribute(hash_key=True)
    signature_reference_id = UnicodeAttribute(range_key=True)

class SignatureCompanyInitialManagerIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying signatures by signature company initial manager ID
    """

    class Meta:
        """ Meta class for Signature Company Initial Manager Index """

        index_name = "signature-company-initial-manager-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    signature_company_initial_manager_id = UnicodeAttribute(hash_key=True)


class GitHubUsernameIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying users by github username.
    """

    class Meta:
        index_name = "github-username-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    # This attribute is the hash key for the index.
    user_github_username = UnicodeAttribute(hash_key=True)


class GitLabIDIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying users by github username.
    """

    class Meta:
        index_name = "gitlab-id-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    # This attribute is the hash key for the index.
    user_gitlab_id = UnicodeAttribute(hash_key=True)


class GitLabUsernameIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying users by github username.
    """

    class Meta:
        index_name = "gitlab-username-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    # This attribute is the hash key for the index.
    user_gitlab_username = UnicodeAttribute(hash_key=True)


class LFUsernameIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying users by LF Username.
    """

    class Meta:
        """Meta class for LF Username index."""

        index_name = "lf-username-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    lf_username = UnicodeAttribute(hash_key=True)


class ProjectRepositoryIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying repositories by project ID.
    """

    class Meta:
        """Meta class for project repository index."""

        index_name = "project-repository-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    repository_project_id = UnicodeAttribute(hash_key=True)


class ProjectSFIDRepositoryIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying repositories by project ID.
    """

    class Meta:
        """Meta class for project repository index."""

        index_name = "project-sfid-repository-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    project_sfid = UnicodeAttribute(hash_key=True)


class ExternalRepositoryIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying repositories by external ID.
    """

    class Meta:
        """Meta class for external ID repository index."""

        index_name = "external-repository-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    repository_external_id = UnicodeAttribute(hash_key=True)


class SFDCRepositoryIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying repositories by external ID.
    """

    class Meta:
        """Meta class for external ID repository index."""

        index_name = "sfdc-repository-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    repository_sfdc_id = UnicodeAttribute(hash_key=True)


class ExternalProjectIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying projects by external ID.
    """

    class Meta:
        """Meta class for external ID project index."""

        index_name = "external-project-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    project_external_id = UnicodeAttribute(hash_key=True)


class ProjectNameIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying projects by name.
    """

    class Meta:
        """Meta class for external ID project index."""

        index_name = "project-name-search-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    project_name = UnicodeAttribute(hash_key=True)


class ProjectNameLowerIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying projects by name.
    """

    class Meta:
        """Meta class for external ID project index."""

        index_name = "project-name-lower-search-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    project_name_lower = UnicodeAttribute(hash_key=True)


class ProjectFoundationIDIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying projects by name.
    """

    class Meta:
        """Meta class for external ID project index."""

        index_name = "foundation-sfid-project-name-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    foundation_sfid = UnicodeAttribute(hash_key=True)
    project_name = UnicodeAttribute(range_key=True)


class CompanyNameIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying companies by name.
    """

    class Meta:
        """Meta class for company name index."""

        index_name = "company-name-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    company_name = UnicodeAttribute(hash_key=True)


class SigningEntityNameIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying companies by the signing entity name.
    """

    class Meta:
        """Meta class for company name index."""

        index_name = "company-signing-entity-name-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    signing_entity_name = UnicodeAttribute(hash_key=True)


class ExternalCompanyIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying companies by external ID.
    """

    class Meta:
        """Meta class for external ID company index."""

        index_name = "external-company-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    company_external_id = UnicodeAttribute(hash_key=True)


class GithubOrgSFIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying github organizations by a Salesforce ID.
    """

    class Meta:
        """Meta class for external ID github org index."""

        index_name = "github-org-sfid-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    organization_sfid = UnicodeAttribute(hash_key=True)


class GitlabOrgSFIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying gitlab organizations by a Salesforce ID.
    """

    class Meta:
        """Meta class for external ID github org index."""

        index_name = "gitlab-org-sfid-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    organization_sfid = UnicodeAttribute(hash_key=True)


class GitlabOrgProjectSfidOrganizationNameIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying gitlab organizations by a Project sfid and
    Organization Name.
    """

    class Meta:
        """Meta class for external ID github org index."""

        index_name = "gitlab-project-sfid-organization-name-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    project_sfid = UnicodeAttribute(hash_key=True)
    organization_name = UnicodeAttribute(range_key=True)


class GitlabOrganizationNameLowerIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying gitlab organizations by Organization Name.
    """

    class Meta:
        """Meta class for external ID github org index."""

        index_name = "gitlab-organization-name-lower-search-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    organization_name_lower = UnicodeAttribute(hash_key=True)

class OrganizationNameLowerSearchIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying organizations by Organization Name.
    """

    class Meta:
        """Meta class for external ID github org index."""

        index_name = "organization-name-lower-search-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    organization_name_lower = UnicodeAttribute(hash_key=True)

class GitlabExternalGroupIDIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying gitlab organizations by group ID
    """

    class Meta:
        """Meta class for external ID for gitlab group id index"""

        index_name = "gitlab-external-group-id-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    external_gitlab_group_id = NumberAttribute(hash_key=True)


class GerritProjectIDIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying gerrit's by the project ID
    """

    class Meta:
        """Meta class for external ID github org index."""

        index_name = "gerrit-project-id-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    project_id = UnicodeAttribute(hash_key=True)


class GerritProjectSFIDIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying gerrit's by the project SFID
    """

    class Meta:
        """Meta class for external ID github org index."""

        index_name = "gerrit-project-sfid-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    project_sfid = UnicodeAttribute(hash_key=True)


class ProjectSignatureIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying signatures by project ID.
    """

    class Meta:
        """Meta class for reference Signature index."""

        index_name = "project-signature-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    signature_project_id = UnicodeAttribute(hash_key=True)


class ReferenceSignatureIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying signatures by reference.
    """

    class Meta:
        """Meta class for reference Signature index."""

        index_name = "reference-signature-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        # All attributes are projected - not sure if this is necessary.
        projection = AllProjection()

    # This attribute is the hash key for the index.
    signature_reference_id = UnicodeAttribute(hash_key=True)


class RequestedCompanyIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying company invites with a company ID.
    """

    class Meta:
        """Meta class for external ID company index."""

        index_name = "requested-company-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    requested_company_id = UnicodeAttribute(hash_key=True)


class EventTypeIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying events with an event type
    """

    class Meta:
        """Meta class for event type index."""

        index_name = "event-type-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    event_type = UnicodeAttribute(hash_key=True)


class EventUserIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying events by user ID.
    """

    class Meta:
        """Meta class for user ID index"""

        index_name = "user-id-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    user_id_index = UnicodeAttribute(hash_key=True)


class GithubUserExternalIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying users by a user external ID.
    """

    class Meta:
        """Meta class for github user external ID index"""

        index_name = "github-user-external-id-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    user_external_id = UnicodeAttribute(hash_key=True)


class FoundationSfidIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying mapping of cla-groups and projects by foundation_sfid
    """

    class Meta:
        """Meta class for project-cla-groups foundation_sfid index"""
        index_name = "foundation-sfid-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    foundation_sfid = UnicodeAttribute(hash_key=True)


class CLAGroupIDIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying by cla-group-id
    """

    class Meta:
        """Meta class for cla-groups-projects cla-group-id index"""
        index_name = "cla-group-id-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    cla_group_id = UnicodeAttribute(hash_key=True)


class CompanyIDProjectIDIndex(GlobalSecondaryIndex):
    """
    This class represents a global secondary index for querying by company-id
    """

    class Meta:
        """ Meta class for ccla-whitelist-requests company-id-project-id-index """
        index_name = "company-id-project-id-index"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])
        projection = AllProjection()

    company_id = UnicodeAttribute(hash_key=True)
    project_id = UnicodeAttribute(range_key=True)

# LG: patched class
class DateTimeAttribute(UTCDateTimeAttribute):
    """
    We need to patch deserialize, see https://pynamodb.readthedocs.io/en/stable/upgrading.html#no-longer-parsing-date-time-strings-leniently
    This fails for ProjectModel.date_created having '2022-11-21T10:31:31Z' instead of strictly expected '2022-08-25T16:26:04.000000+0000'
    """
    def deserialize(self, value):
        try:
            return self._fast_parse_utc_date_string(value)
        except (TypeError, ValueError, AttributeError):
            return parsedatestring(value)

# LG: patched class
class PatchedUnicodeSetAttribute(UnicodeSetAttribute):
    """
    In attribute value we can have:
    - set of strings "SS": {"SS":["id1","id2"]} - this is expected by pynamodb
    - list of strings "LS": {"L":[{"S": "id"},{"S":"id2"}] - this is what golang saves
    - NULL: {"NULL":true}
    """
    def get_value(self, value):
        # if self.attr_type not in value:
        if not value:
            return set()
        if self.attr_type == 'SS' and 'L' in value:
            value = {'SS':list(map(lambda x: x['S'], value['L']))}
        return super(PatchedUnicodeSetAttribute, self).get_value(value)

    def deserialize(self, value):
        if not value:
            return set()
        return set(value)

class BaseModel(Model):
    """
    Base pynamodb model used for all CLA models.
    """

    date_created = DateTimeAttribute(default=datetime.datetime.utcnow())
    date_modified = DateTimeAttribute(default=datetime.datetime.utcnow())
    version = UnicodeAttribute(default="v1")  # Schema version.

    def __iter__(self):
        """Used to convert model to dict for JSON-serialized string."""
        for name, attr in self.get_attributes().items():
            if isinstance(attr, ListAttribute):
                if attr is None or getattr(self, name) is None:
                    yield name, None
                else:
                    values = attr.serialize(getattr(self, name))
                    if len(values) < 1:
                        yield name, []
                    else:
                        key = list(values[0].keys())[0]
                        yield name, [value[key] for value in values]
            else:
                yield name, attr.serialize(getattr(self, name))

    def get_version(self):
        return self.version

    def get_date_created(self):
        return self.date_created

    def get_date_modified(self):
        return self.date_modified

    def set_version(self, version):
        self.version = version

    def set_date_created(self, date_created):
        self.date_created = date_created

    def set_date_modified(self, date_modified):
        self.date_modified = date_modified


class DocumentTabModel(MapAttribute):
    """
    Represents a document tab in the document model.
    """

    document_tab_type = UnicodeAttribute(default="text")
    document_tab_id = UnicodeAttribute(null=True)
    document_tab_name = UnicodeAttribute(null=True)
    document_tab_page = NumberAttribute(default=1)
    document_tab_position_x = NumberAttribute(null=True)
    document_tab_position_y = NumberAttribute(null=True)
    document_tab_width = NumberAttribute(default=200)
    document_tab_height = NumberAttribute(default=20)
    document_tab_is_locked = BooleanAttribute(default=False)
    document_tab_is_required = BooleanAttribute(default=True)
    document_tab_anchor_string = UnicodeAttribute(default=None, null=True)
    document_tab_anchor_ignore_if_not_present = BooleanAttribute(default=True)
    document_tab_anchor_x_offset = NumberAttribute(null=True)
    document_tab_anchor_y_offset = NumberAttribute(null=True)


class DocumentTab(model_interfaces.DocumentTab):
    """
    ORM-agnostic wrapper for the DynamoDB DocumentTab model.
    """

    def __init__(
            self,  # pylint: disable=too-many-arguments
            document_tab_type=None,
            document_tab_id=None,
            document_tab_name=None,
            document_tab_page=None,
            document_tab_position_x=None,
            document_tab_position_y=None,
            document_tab_width=None,
            document_tab_height=None,
            document_tab_is_locked=False,
            document_tab_is_required=True,
            document_tab_anchor_string=None,
            document_tab_anchor_ignore_if_not_present=True,
            document_tab_anchor_x_offset=None,
            document_tab_anchor_y_offset=None,
    ):
        super().__init__()
        self.model = DocumentTabModel()
        self.model.document_tab_id = document_tab_id
        self.model.document_tab_name = document_tab_name
        # x,y coordinates are None when anchor x,y offsets are supplied.
        if document_tab_position_x is not None:
            self.model.document_tab_position_x = document_tab_position_x
        if document_tab_position_y is not None:
            self.model.document_tab_position_y = document_tab_position_y
        # Use defaults if None is provided for the following attributes.
        if document_tab_type is not None:
            self.model.document_tab_type = document_tab_type
        if document_tab_page is not None:
            self.model.document_major_version = document_tab_page
        if document_tab_width is not None:
            self.model.document_tab_width = document_tab_width
        if document_tab_height is not None:
            self.model.document_tab_height = document_tab_height
        self.model.document_tab_is_locked = document_tab_is_locked
        self.model.document_tab_is_required = document_tab_is_required
        # Anchor string properties
        if document_tab_anchor_string is not None:
            self.model.document_tab_anchor_string = document_tab_anchor_string
        self.model.document_tab_anchor_ignore_if_not_present = document_tab_anchor_ignore_if_not_present
        if document_tab_anchor_x_offset is not None:
            self.model.document_tab_anchor_x_offset = document_tab_anchor_x_offset
        if document_tab_anchor_y_offset is not None:
            self.model.document_tab_anchor_y_offset = document_tab_anchor_y_offset

    def to_dict(self):
        return {
            "document_tab_type": self.model.document_tab_type,
            "document_tab_id": self.model.document_tab_id,
            "document_tab_name": self.model.document_tab_name,
            "document_tab_page": self.model.document_tab_page,
            "document_tab_position_x": self.model.document_tab_position_x,
            "document_tab_position_y": self.model.document_tab_position_y,
            "document_tab_width": self.model.document_tab_width,
            "document_tab_height": self.model.document_tab_height,
            "document_tab_is_locked": self.model.document_tab_is_locked,
            "document_tab_is_required": self.model.document_tab_is_required,
            "document_tab_anchor_string": self.model.document_tab_anchor_string,
            "document_tab_anchor_ignore_if_not_present": self.model.document_tab_anchor_ignore_if_not_present,
            "document_tab_anchor_x_offset": self.model.document_tab_anchor_x_offset,
            "document_tab_anchor_y_offset": self.model.document_tab_anchor_y_offset,
        }

    def get_document_tab_type(self):
        return self.model.document_tab_type

    def get_document_tab_id(self):
        return self.model.document_tab_id

    def get_document_tab_name(self):
        return self.model.document_tab_name

    def get_document_tab_page(self):
        return self.model.document_tab_page

    def get_document_tab_position_x(self):
        return self.model.document_tab_position_x

    def get_document_tab_position_y(self):
        return self.model.document_tab_position_y

    def get_document_tab_width(self):
        return self.model.document_tab_width

    def get_document_tab_height(self):
        return self.model.document_tab_height

    def get_document_tab_is_locked(self):
        return self.model.document_tab_is_locked

    def get_document_tab_anchor_string(self):
        return self.model.document_tab_anchor_string

    def get_document_tab_anchor_ignore_if_not_present(self):
        return self.model.document_tab_anchor_ignore_if_not_present

    def get_document_tab_anchor_x_offset(self):
        return self.model.document_tab_anchor_x_offset

    def get_document_tab_anchor_y_offset(self):
        return self.model.document_tab_anchor_y_offset

    def set_document_tab_type(self, tab_type):
        self.model.document_tab_type = tab_type

    def set_document_tab_id(self, tab_id):
        self.model.document_tab_id = tab_id

    def set_document_tab_name(self, tab_name):
        self.model.document_tab_name = tab_name

    def set_document_tab_page(self, tab_page):
        self.model.document_tab_page = tab_page

    def set_document_tab_position_x(self, tab_position_x):
        self.model.document_tab_position_x = tab_position_x

    def set_document_tab_position_y(self, tab_position_y):
        self.model.document_tab_position_y = tab_position_y

    def set_document_tab_width(self, tab_width):
        self.model.document_tab_width = tab_width

    def set_document_tab_height(self, tab_height):
        self.model.document_tab_height = tab_height

    def set_document_tab_is_locked(self, is_locked):
        self.model.document_tab_is_locked = is_locked

    def set_document_tab_anchor_string(self, document_tab_anchor_string):
        self.model.document_tab_anchor_string = document_tab_anchor_string

    def set_document_tab_anchor_ignore_if_not_present(self, document_tab_anchor_ignore_if_not_present):
        self.model.document_tab_anchor_ignore_if_not_present = document_tab_anchor_ignore_if_not_present

    def set_document_tab_anchor_x_offset(self, document_tab_anchor_x_offset):
        self.model.document_tab_anchor_x_offset = document_tab_anchor_x_offset

    def set_document_tab_anchor_y_offset(self, document_tab_anchor_y_offset):
        self.model.document_tab_anchor_y_offset = document_tab_anchor_y_offset


class DocumentModel(MapAttribute):
    """
    Represents a document in the project model.
    """

    document_name = UnicodeAttribute(null=True)
    document_file_id = UnicodeAttribute(null=True)
    document_content_type = UnicodeAttribute(null=True)  # pdf, url+pdf, storage+pdf, etc
    document_content = UnicodeAttribute(null=True)  # None if using storage service.
    document_major_version = NumberAttribute(default=1)
    document_minor_version = NumberAttribute(default=0)
    document_author_name = UnicodeAttribute(null=True)
    # LG: now we can use DateTimeAttribute - because pynamodb was updated
    # document_creation_date = UnicodeAttribute(null=True)
    document_creation_date = DateTimeAttribute(null=True)
    document_preamble = UnicodeAttribute(null=True)
    document_legal_entity_name = UnicodeAttribute(null=True)
    document_s3_url = UnicodeAttribute(null=True)
    document_tabs = ListAttribute(of=DocumentTabModel, default=list)


class Document(model_interfaces.Document):
    """
    ORM-agnostic wrapper for the DynamoDB Document model.
    """

    def __init__(
            self,  # pylint: disable=too-many-arguments
            document_name=None,
            document_file_id=None,
            document_content_type=None,
            document_content=None,
            document_major_version=None,
            document_minor_version=None,
            document_author_name=None,
            document_creation_date=None,
            document_preamble=None,
            document_legal_entity_name=None,
            document_s3_url=None,
    ):
        super().__init__()
        self.model = DocumentModel()
        self.model.document_name = document_name
        self.model.document_file_id = document_file_id
        self.model.document_author_name = document_author_name
        self.model.document_content_type = document_content_type
        if self.model.document_content is not None:
            self.model.document_content = self.set_document_content(document_content)
        self.model.document_preamble = document_preamble
        self.model.document_legal_entity_name = document_legal_entity_name
        self.model.document_s3_url = document_s3_url
        # Use defaults if None is provided for the following attributes.
        if document_major_version is not None:
            self.model.document_major_version = document_major_version
        if document_minor_version is not None:
            self.model.document_minor_version = document_minor_version
        if document_creation_date is not None:
            self.set_document_creation_date(document_creation_date)
        else:
            self.set_document_creation_date(datetime.datetime.now())

    def to_dict(self):
        return {
            "document_name": self.model.document_name,
            "document_file_id": self.model.document_file_id,
            "document_content_type": self.model.document_content_type,
            "document_content": self.model.document_content,
            "document_author_name": self.model.document_author_name,
            "document_major_version": self.model.document_major_version,
            "document_minor_version": self.model.document_minor_version,
            "document_creation_date": self.model.document_creation_date,
            "document_preamble": self.model.document_preamble,
            "document_legal_entity_name": self.model.document_legal_entity_name,
            "document_s3_url": self.model.document_s3_url,
            "document_tabs": self.model.document_tabs,
        }

    def get_document_name(self):
        return self.model.document_name

    def get_document_file_id(self):
        return self.model.document_file_id

    def get_document_content_type(self):
        return self.model.document_content_type

    def get_document_author_name(self):
        return self.model.document_author_name

    def get_document_content(self):
        content_type = self.get_document_content_type()
        if content_type is None:
            cla.log.warning("Empty content type for document - not sure how to retrieve content")
        else:
            if content_type.startswith("storage+"):
                filename = self.get_document_file_id()
                return cla.utils.get_storage_service().retrieve(filename)
        return self.model.document_content

    def get_document_major_version(self):
        return self.model.document_major_version

    def get_document_minor_version(self):
        return self.model.document_minor_version

    def get_document_creation_date(self):
        # LG: we now can use datetime because pynamodb was updated
        # return dateutil.parser.parse(self.model.document_creation_date)
        return self.model.document_creation_date

    def get_document_preamble(self):
        return self.model.document_preamble

    def get_document_legal_entity_name(self):
        return self.model.document_legal_entity_name

    def get_document_s3_url(self):
        return self.model.document_s3_url

    def get_document_tabs(self):
        tabs = []
        for tab in self.model.document_tabs:
            tab_obj = DocumentTab()
            tab_obj.model = tab
            tabs.append(tab_obj)
        return tabs

    def set_document_author_name(self, document_author_name):
        self.model.document_author_name = document_author_name

    def set_document_name(self, document_name):
        self.model.document_name = document_name

    def set_document_file_id(self, document_file_id):
        self.model.document_file_id = document_file_id

    def set_document_content_type(self, document_content_type):
        self.model.document_content_type = document_content_type

    def set_document_content(self, document_content, b64_encoded=True):
        content_type = self.get_document_content_type()
        if content_type is not None and content_type.startswith("storage+"):
            if b64_encoded:
                document_content = base64.b64decode(document_content)
            filename = self.get_document_file_id()
            if filename is None:
                filename = str(uuid.uuid4())
                self.set_document_file_id(filename)
            cla.log.info(
                "Saving document content for %s to %s", self.get_document_name(), filename,
            )
            cla.utils.get_storage_service().store(filename, document_content)
        else:
            self.model.document_content = document_content

    def set_document_major_version(self, version):
        self.model.document_major_version = version

    def set_document_minor_version(self, version):
        self.model.document_minor_version = version

    def set_document_creation_date(self, document_creation_date):
        # LG: we now can use datetime because pynamodb was updated
        # self.model.document_creation_date = document_creation_date.isoformat()
        self.model.document_creation_date = document_creation_date

    def set_document_preamble(self, document_preamble):
        self.model.document_preamble = document_preamble

    def set_document_legal_entity_name(self, entity_name):
        self.model.document_legal_entity_name = entity_name

    def set_document_s3_url(self, document_s3_url):
        self.model.document_s3_url = document_s3_url

    def set_document_tabs(self, tabs):
        self.model.document_tabs = tabs

    def add_document_tab(self, tab):
        self.model.document_tabs.append(tab.model)

    def set_raw_document_tabs(self, tabs_data):
        self.model.document_tabs = []
        for tab_data in tabs_data:
            self.add_raw_document_tab(tab_data)

    def add_raw_document_tab(self, tab_data):
        tab = DocumentTab()
        tab.set_document_tab_type(tab_data["type"])
        tab.set_document_tab_id(tab_data["id"])
        tab.set_document_tab_name(tab_data["name"])
        if "position_x" in tab_data:
            tab.set_document_tab_position_x(tab_data["position_x"])
        if "position_y" in tab_data:
            tab.set_document_tab_position_y(tab_data["position_y"])
        tab.set_document_tab_width(tab_data["width"])
        tab.set_document_tab_height(tab_data["height"])
        tab.set_document_tab_page(tab_data["page"])
        if "anchor_string" in tab_data:
            tab.set_document_tab_anchor_string(tab_data["anchor_string"])
        if "anchor_ignore_if_not_present" in tab_data:
            tab.set_document_tab_anchor_ignore_if_not_present(tab_data["anchor_ignore_if_not_present"])
        if "anchor_x_offset" in tab_data:
            tab.set_document_tab_anchor_x_offset(tab_data["anchor_x_offset"])
        if "anchor_y_offset" in tab_data:
            tab.set_document_tab_anchor_y_offset(tab_data["anchor_y_offset"])
        self.add_document_tab(tab)


class ProjectModel(BaseModel):
    """
    Represents a project in the database.
    """

    class Meta:
        """Meta class for Project."""

        table_name = "cla-{}-projects".format(stage)
        if stage == "local":
            host = "http://localhost:8000"

    project_id = UnicodeAttribute(hash_key=True)
    project_external_id = UnicodeAttribute(null=True)
    project_name = UnicodeAttribute(null=True)
    project_name_lower = UnicodeAttribute(null=True)
    project_individual_documents = ListAttribute(of=DocumentModel, default=list)
    project_corporate_documents = ListAttribute(of=DocumentModel, default=list)
    project_member_documents = ListAttribute(of=DocumentModel, default=list)
    project_icla_enabled = BooleanAttribute(default=True)
    project_ccla_enabled = BooleanAttribute(default=True)
    project_ccla_requires_icla_signature = BooleanAttribute(default=False)
    project_live = BooleanAttribute(default=False)
    foundation_sfid = UnicodeAttribute(null=True)
    root_project_repositories_count = NumberAttribute(null=True)
    note = UnicodeAttribute(null=True)
    # Indexes
    project_external_id_index = ExternalProjectIndex()
    project_name_search_index = ProjectNameIndex()
    project_name_lower_search_index = ProjectNameLowerIndex()
    foundation_sfid_project_name_index = ProjectFoundationIDIndex()

    project_acl = PatchedUnicodeSetAttribute(default=set)
    # Default is v1 for all of our models - override for this model so that we can redirect to new UI when ready
    # version = UnicodeAttribute(default="v2")  # Schema version is v2 for Project Models


class Project(model_interfaces.Project):  # pylint: disable=too-many-public-methods
    """
    ORM-agnostic wrapper for the DynamoDB Project model.
    """

    def __init__(
            self,
            project_id=None,
            project_external_id=None,
            project_name=None,
            project_name_lower=None,
            project_icla_enabled=True,
            project_ccla_enabled=True,
            project_ccla_requires_icla_signature=False,
            project_acl=set(),
            project_live=False,
            note=None
    ):
        super(Project).__init__()
        self.model = ProjectModel()
        self.model.project_id = project_id
        self.model.project_external_id = project_external_id
        self.model.project_name = project_name
        self.model.project_name_lower = project_name_lower
        self.model.project_icla_enabled = project_icla_enabled
        self.model.project_ccla_enabled = project_ccla_enabled
        self.model.project_ccla_requires_icla_signature = project_ccla_requires_icla_signature
        self.model.project_acl = project_acl
        self.model.project_live = project_live
        self.model.note = note

    def __str__(self):
        return (
            f"id:{self.model.project_id}, "
            f"project_name:{self.model.project_name}, "
            f"project_name_lower:{self.model.project_name_lower}, "
            f"project_external_id:{self.model.project_external_id}, "
            f"foundation_sfid:{self.model.foundation_sfid}, "
            f"project_icla_enabled: {self.model.project_icla_enabled}, "
            f"project_ccla_enabled: {self.model.project_ccla_enabled}, "
            f"project_ccla_requires_icla_signature: {self.model.project_ccla_requires_icla_signature}, "
            f"project_live: {self.model.project_live}, "
            f"project_acl: {self.model.project_acl}, "
            f"root_project_repositories_count: {self.model.root_project_repositories_count}, "
            f"date_created: {self.model.date_created}, "
            f"date_modified: {self.model.date_modified}, "
            f"version: {self.model.version}"
        )

    def to_dict(self):
        individual_documents = []
        corporate_documents = []
        member_documents = []
        for doc in self.model.project_individual_documents:
            document = Document()
            document.model = doc
            individual_documents.append(document.to_dict())
        for doc in self.model.project_corporate_documents:
            document = Document()
            document.model = doc
            corporate_documents.append(document.to_dict())
        for doc in self.model.project_member_documents:
            document = Document()
            document.model = doc
            member_documents.append(document.to_dict())
        project_dict = dict(self.model)
        project_dict["project_individual_documents"] = individual_documents
        project_dict["project_corporate_documents"] = corporate_documents
        project_dict["project_member_documents"] = member_documents

        project_dict["logoUrl"] = "{}/{}.png".format(cla_logo_url, self.model.project_external_id)

        return project_dict

    def save(self) -> None:
        self.model.date_modified = datetime.datetime.utcnow()
        self.model.save()

    def load(self, project_id):
        try:
            project = self.model.get(project_id)
        except ProjectModel.DoesNotExist:
            raise cla.models.DoesNotExist("Project not found")
        self.model = project

    def load_project_by_name(self, project_name):
        try:
            project_generator = self.model.project_name_lower_search_index.query(project_name.lower())
            for project_model in project_generator:
                self.model = project_model
                return
            # Didn't find a result - throw an error
            raise cla.models.DoesNotExist(f'Project with name {project_name} not found')
        except ProjectModel.DoesNotExist:
            raise cla.models.DoesNotExist(f'Project with name {project_name} not found')

    def delete(self):
        self.model.delete()

    def get_project_id(self):
        return self.model.project_id

    def get_foundation_sfid(self):
        return self.model.foundation_sfid

    def get_root_project_repositories_count(self):
        return self.model.root_project_repositories_count

    def get_project_external_id(self):
        return self.model.project_external_id

    def get_project_name(self):
        return self.model.project_name

    def get_project_name_lower(self):
        return self.model.project_name_lower

    def get_project_icla_enabled(self):
        return self.model.project_icla_enabled

    def get_project_ccla_enabled(self):
        return self.model.project_ccla_enabled

    def get_project_live(self):
        return self.model.project_live

    def get_project_individual_documents(self):
        documents = []
        for doc in self.model.project_individual_documents:
            document = Document()
            document.model = doc
            documents.append(document)
        return documents

    def get_project_corporate_documents(self):
        documents = []
        for doc in self.model.project_corporate_documents:
            document = Document()
            document.model = doc
            documents.append(document)
        return documents

    def get_project_individual_document(self, major_version=None, minor_version=None):
        fn = 'models.dynamodb_models.get_project_individual_document'
        document_models = self.get_project_individual_documents()
        num_documents = len(document_models)

        if num_documents < 1:
            raise cla.models.DoesNotExist("No individual document exists for this project")

        version = self._get_latest_version(document_models)
        cla.log.debug(f'{fn} - latest version is : {version}')
        document = version[2]
        return document

    def get_latest_individual_document(self):
        fn = 'models.dynamodb_models.get_latest_individual_document'
        document_models = self.get_project_individual_documents()
        version = self._get_latest_version(document_models)
        cla.log.debug(f'{fn} - latest version is : {version}')
        document = version[2]
        return document

    def get_project_corporate_document(self, major_version=None, minor_version=None):
        fn = 'models.dynamodb_models.get_project_corporate_document'
        document_models = self.get_project_corporate_documents()
        num_documents = len(document_models)
        if num_documents < 1:
            raise cla.models.DoesNotExist("No corporate document exists for this project")
        version = self._get_latest_version(document_models)
        cla.log.debug(f'{fn} - latest version is : {version}')
        document = version[2]
        return document

    def get_latest_corporate_document(self):
        """
        Helper function to return the latest corporate document belonging to a project.

        :return: Latest CCLA document object for this project.
        :rtype: cla.models.model_instances.Document
        """
        fn = 'models.dynamodb_models.get_latest_corporate_document'
        document_models = self.get_project_corporate_documents()
        version = self._get_latest_version(document_models)
        cla.log.debug(f'{fn} - latest version is : {version}')
        document = version[2]

        return document

    def _get_latest_version(self, documents):
        """
        Helper function to get the last version of the list of documents provided.

        :param documents: List of documents to check.
        :type documents: [cla.models.model_interfaces.Document]
        :return: 2-item tuple containing (major, minor) version number.
        :rtype: tuple
        """
        last_major = 0  # 0 will be returned if no document was found.
        last_minor = -1  # -1 will be returned if no document was found.
        latest_date = None
        current_document = None
        for document in documents:
            current_major = document.get_document_major_version()
            current_minor = document.get_document_minor_version()
            if current_major > last_major:
                last_major = current_major
                last_minor = current_minor
                latest_date = document.get_document_creation_date()
                current_document = document
                continue
            if current_major == last_major and current_minor > last_minor:
                last_minor = current_minor
                latest_date = document.get_document_creation_date()
                current_document = document
                continue
            # Retrieve document that has the latest date
            if current_major == last_major and current_minor == last_minor and (not latest_date or document.get_document_creation_date() > latest_date):
                latest_date = document.get_document_creation_date()
                current_document = document
        return (last_major, last_minor, current_document)

    def get_project_ccla_requires_icla_signature(self):
        return self.model.project_ccla_requires_icla_signature

    def get_project_latest_major_version(self):
        pass
        # @todo: Loop through documents for this project, return the highest version of them all.

    def get_project_acl(self):
        return self.model.project_acl

    def get_version(self):
        return self.model.version

    def get_date_created(self):
        return self.model.date_created

    def get_date_modified(self):
        return self.model.date_modified

    def get_note(self) -> Optional[str]:
        return self.model.note

    def set_project_id(self, project_id):
        self.model.project_id = str(project_id)

    def set_foundation_sfid(self, foundation_sfid):
        self.model.foundation_sfid = str(foundation_sfid)

    def set_root_project_repositories_count(self, root_project_repositories_count):
        self.model.root_project_repositories_count = root_project_repositories_count

    def set_project_external_id(self, project_external_id):
        self.model.project_external_id = str(project_external_id)

    def set_project_name(self, project_name):
        self.model.project_name = project_name

    def set_project_name_lower(self, project_name_lower):
        self.model.project_name_lower = project_name_lower

    def set_project_icla_enabled(self, project_icla_enabled):
        self.model.project_icla_enabled = project_icla_enabled

    def set_project_ccla_enabled(self, project_ccla_enabled):
        self.model.project_ccla_enabled = project_ccla_enabled

    def set_project_live(self, project_live):
        self.model.project_live = project_live

    def set_note(self, note: str) -> None:
        self.model.note = note

    def add_project_individual_document(self, document):
        self.model.project_individual_documents.append(document.model)

    def add_project_corporate_document(self, document):
        self.model.project_corporate_documents.append(document.model)

    def remove_project_individual_document(self, document):
        new_documents = _remove_project_document(
            self.model.project_individual_documents,
            document.get_document_major_version(),
            document.get_document_minor_version(),
        )
        self.model.project_individual_documents = new_documents

    def remove_project_corporate_document(self, document):
        new_documents = _remove_project_document(
            self.model.project_corporate_documents,
            document.get_document_major_version(),
            document.get_document_minor_version(),
        )
        self.model.project_corporate_documents = new_documents

    def set_project_individual_documents(self, documents):
        self.model.project_individual_documents = documents

    def set_project_corporate_documents(self, documents):
        self.model.project_corporate_documents = documents

    def set_project_ccla_requires_icla_signature(self, ccla_requires_icla_signature):
        self.model.project_ccla_requires_icla_signature = ccla_requires_icla_signature

    def set_project_acl(self, project_acl_username):
        self.model.project_acl = set([project_acl_username])

    def add_project_acl(self, username):
        self.model.project_acl.add(username)

    def remove_project_acl(self, username):
        if username in self.model.project_acl:
            self.model.project_acl.remove(username)

    def get_project_repositories(self):
        repository_generator = RepositoryModel.repository_project_index.query(self.get_project_id())
        repositories = []
        for repository_model in repository_generator:
            repository = Repository()
            repository.model = repository_model
            repositories.append(repository)
        return repositories

    def get_project_signatures(self, signature_signed=None, signature_approved=None):
        return Signature().get_signatures_by_project(
            self.get_project_id(), signature_approved=signature_approved, signature_signed=signature_signed,
        )

    def get_projects_by_external_id(self, project_external_id, username):
        project_generator = self.model.project_external_id_index.query(project_external_id)
        projects = []
        for project_model in project_generator:
            project = Project()
            project.model = project_model
            projects.append(project)
        return projects

    def get_managers(self):
        return self.get_managers_by_project_acl(self.get_project_acl())

    def get_managers_by_project_acl(self, project_acl):
        managers = []
        user_model = User()
        for username in project_acl:
            users = user_model.get_user_by_username(str(username))
            if users is not None:
                if len(users) > 1:
                    cla.log.warning(
                        f"More than one user record was returned ({len(users)}) from user "
                        f"username: {username} query"
                    )
                managers.append(users[0])
        return managers

    def set_version(self, version):
        self.model.version = version

    def set_date_modified(self, date_modified):
        self.model.date_modified = date_modified

    def all(self, project_ids=None):
        if project_ids is None:
            projects = self.model.scan()
        else:
            projects = ProjectModel.batch_get(project_ids)
        ret = []
        for project in projects:
            proj = Project()
            proj.model = project
            ret.append(proj)
        return ret


def _remove_project_document(documents, major_version, minor_version):
    # TODO Need to optimize this on the DB side - delete directly from list of records.
    new_documents = []
    found = False
    for document in documents:
        if document.document_major_version == major_version and document.document_minor_version == minor_version:
            found = True
            if document.document_content_type.startswith("storage+"):
                cla.utils.get_storage_service().delete(document.document_file_id)
            continue
        new_documents.append(document)
    if not found:
        raise cla.models.DoesNotExist("Document revision not found")
    return new_documents


class UserModel(BaseModel):
    """
    Represents a user in the database.
    """

    class Meta:
        """Meta class for User."""

        table_name = "cla-{}-users".format(stage)
        if stage == "local":
            host = "http://localhost:8000"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])

    user_id = UnicodeAttribute(hash_key=True)
    # User Emails are specifically GitHub Emails
    user_external_id = UnicodeAttribute(null=True)
    user_emails = PatchedUnicodeSetAttribute(default=set)
    user_name = UnicodeAttribute(null=True)
    user_company_id = UnicodeAttribute(null=True)
    user_github_id = NumberAttribute(null=True)
    user_github_username = UnicodeAttribute(null=True)
    user_github_username_index = GitHubUsernameIndex()
    user_gitlab_id = NumberAttribute(null=True)
    user_gitlab_username = UnicodeAttribute(null=True)
    user_gitlab_id_index = GitLabIDIndex()
    user_gitlab_username_index = GitLabUsernameIndex()
    user_ldap_id = UnicodeAttribute(null=True)
    user_github_id_index = GitHubUserIndex()
    github_user_external_id_index = GithubUserExternalIndex()
    note = UnicodeAttribute(null=True)
    lf_email = UnicodeAttribute(null=True)
    lf_username = UnicodeAttribute(null=True)
    lf_username_index = LFUsernameIndex()
    lf_sub = UnicodeAttribute(null=True)


class User(model_interfaces.User):  # pylint: disable=too-many-public-methods
    """
    ORM-agnostic wrapper for the DynamoDB User model.
    """

    def __init__(
            self,
            user_email=None,
            user_external_id=None,
            user_github_id=None,
            user_github_username=None,
            user_gitlab_id=None,
            user_gitlab_username=None,
            user_ldap_id=None,
            lf_username=None,
            lf_sub=None,
            user_company_id=None,
            note=None,
            # this is for cases when the user has more than one email (eg. github) and the get_email
            # function is used in all over the places in legacy code. It's just not possible to introduce
            # new functionality and not forget to update any of those references
            preferred_email=None,
    ):
        super(User).__init__()
        self.model = UserModel()
        if user_email is not None:
            self.set_user_email(user_email)
        self.model.user_external_id = user_external_id
        self.model.user_github_id = user_github_id
        self.model.user_github_username = user_github_username
        self.model.user_ldap_id = user_ldap_id
        self.model.lf_username = lf_username
        self.model.lf_sub = lf_sub
        self.model.user_company_id = user_company_id
        self.model.note = note
        self._preferred_email = preferred_email
        self.model.user_gitlab_id = user_gitlab_id
        self.model.user_gitlab_username = user_gitlab_username

    def __str__(self):
        return (
            "id: {}, username: {}, gh id: {}, gh username: {}, "
            "lf email: {}, emails: {}, ldap id: {}, lf username: {}, "
            "user company id: {}, note: {}, user external id: {}, user gitlab id: {}, user gitlab username: {}"
        ).format(
            self.model.user_id,
            self.model.user_github_username,
            self.model.user_github_id,
            self.model.user_github_username,
            self.model.lf_email,
            self.model.user_emails,
            self.model.user_ldap_id,
            self.model.lf_username,
            self.model.user_company_id,
            self.model.note,
            self.model.user_external_id,
            self.model.user_gitlab_id,
            self.model.user_gitlab_username,
        )

    def to_dict(self):
        ret = dict(self.model)
        if ret["user_github_id"] == "null":
            ret["user_github_id"] = None
        if ret["user_ldap_id"] == "null":
            ret["user_ldap_id"] = None
        if ret["user_gitlab_id"] == "null":
            ret["user_gitlab_id"] = None
        return ret

    def log_info(self, msg):
        """
        Helper logger function to write the info message and the user details.
        :param msg: the log message
        :return: None
        """
        cla.log.info("{} for user: {}".format(msg, self))

    def log_debug(self, msg):
        """
        Helper logger function to write the debug message and the user details.
        :param msg: the log message
        :return: None
        """
        cla.log.debug("{} for user: {}".format(msg, self))

    def log_warning(self, msg):
        """
        Helper logger function to write the debug message and the user details.
        :param msg: the log message
        :return: None
        """
        cla.log.warning("{} for user: {}".format(msg, self))

    def save(self) -> None:
        self.model.date_modified = datetime.datetime.utcnow()
        self.model.save()

    def load(self, user_id):
        try:
            repo = self.model.get(str(user_id))
        except UserModel.DoesNotExist:
            raise cla.models.DoesNotExist("User not found")
        self.model = repo

    def delete(self):
        self.model.delete()

    def get_user_id(self):
        return self.model.user_id

    def get_lf_username(self):
        return self.model.lf_username

    def get_user_external_id(self):
        return self.model.user_external_id

    def get_lf_email(self):
        return self.model.lf_email

    def get_lf_sub(self):
        return self.model.lf_sub

    def get_user_email(self, preferred_email=None):
        """
        :param preferred_email: if the preferred email is in list of registered emails
        it'd be returned, otherwise whatever email is present will be returned randomly
        :return:
        """
        if preferred_email and self.model.lf_email is None and preferred_email == self.model.lf_email:
            return preferred_email

        preferred_email = preferred_email or self._preferred_email
        if preferred_email and preferred_email in self.model.user_emails:
            return preferred_email

        if self.model.lf_email is not None:
            return self.model.lf_email
        elif len(self.model.user_emails) > 0:
            # Ordering not guaranteed, better to use get_user_emails.
            return next(iter(self.model.user_emails), None)
        return None

    def get_user_emails(self):
        return self.model.user_emails

    def get_all_user_emails(self):
        emails = self.model.user_emails
        if self.model.lf_email is not None:
            emails.add(self.model.lf_email)

        return emails

    def get_user_name(self):
        return self.model.user_name

    def get_user_company_id(self):
        return self.model.user_company_id

    def get_user_github_id(self):
        return self.model.user_github_id

    def get_github_username(self):
        return self.model.user_github_username

    def get_user_gitlab_id(self):
        return self.model.user_gitlab_id

    def get_user_gitlab_username(self):
        return self.model.user_gitlab_username

    def get_user_github_username(self):
        """
        Getter for the user's GitHub ID.

        :return: The user's GitHub ID.
        :rtype: integer
        """
        return self.model.user_github_username

    def get_note(self):
        """
        Getter for the user's note.
        :return: the note value for the user
        :rtype: str
        """
        return self.model.note

    def set_user_id(self, user_id):
        self.model.user_id = user_id

    def set_lf_username(self, lf_username):
        self.model.lf_username = lf_username

    def set_user_external_id(self, user_external_id):
        self.model.user_external_id = user_external_id

    def set_lf_email(self, lf_email):
        self.model.lf_email = lf_email

    def set_lf_sub(self, sub):
        self.model.sub = sub

    def set_user_email(self, user_email):
        # Standard set/list operations (add or append) don't work as expected.
        # Seems to apply the operations on the class attribute which means that
        # all future user objects have all the other user's emails as well.
        # Explicitly creating new list and casting to set seems to work as expected.
        email_list = list(self.model.user_emails) + [user_email]
        self.model.user_emails = set(email_list)

    def set_user_emails(self, user_emails):
        # LG: handle different possible types passed as argument
        if user_emails:
            if isinstance(user_emails, list):
                self.model.user_emails = set(user_emails)
            elif isinstance(user_emails, set):
                self.model.user_emails = user_emails
            else:
                self.model.user_emails = set([user_emails])
        else:
            self.model.user_emails = set()

    def set_user_name(self, user_name):
        self.model.user_name = user_name

    def set_user_company_id(self, company_id):
        self.model.user_company_id = company_id

    def set_user_github_id(self, user_github_id):
        self.model.user_github_id = user_github_id

    def set_user_github_username(self, user_github_username):
        self.model.user_github_username = user_github_username

    def set_user_gitlab_id(self, user_gitlab_id):
        self.model.user_gitlab_id = user_gitlab_id

    def set_user_gitlab_username(self, user_gitlab_username):
        self.model.user_gitlab_username = user_gitlab_username

    def set_note(self, note):
        self.model.note = note

    def get_user_by_email(self, user_email) -> Optional[List[User]]:
        if user_email is None:
            cla.log.warning("Unable to lookup user by user_email - email is empty")
            return None

        users = []
        for user_model in UserModel.scan(UserModel.user_emails.contains(user_email)):
            user = User()
            user.model = user_model
            users.append(user)
        if len(users) > 0:
            return users
        else:
            return None

    def get_user_by_github_id(self, user_github_id: int) -> Optional[List[User]]:
        if user_github_id is None:
            cla.log.warning("Unable to lookup user by github id - id is empty")
            return None

        users = []
        for user_model in self.model.user_github_id_index.query(int(user_github_id)):
            user = User()
            user.model = user_model
            users.append(user)
        if len(users) > 0:
            return users
        else:
            return None

    def get_user_by_username(self, username) -> Optional[List[User]]:
        if username is None:
            cla.log.warning("Unable to lookup user by username - username is empty")
            return None

        users = []
        for user_model in self.model.lf_username_index.query(username):
            user = User()
            user.model = user_model
            users.append(user)
        if len(users) > 0:
            return users
        else:
            return None

    def get_user_by_github_username(self, github_username) -> Optional[List[User]]:
        if github_username is None:
            cla.log.warning("Unable to lookup user by github_username - github_username is empty")
            return None

        users = []
        for user_model in self.model.user_github_username_index.query(github_username):
            user = User()
            user.model = user_model
            users.append(user)
        if len(users) > 0:
            return users
        else:
            return None

    def get_user_signatures(
            self, project_id=None, company_id=None, signature_signed=None, signature_approved=None,
    ):
        cla.log.debug(
            "get_user_signatures with params - "
            f"user_id: {self.get_user_id()}, "
            f"project_id: {project_id}, "
            f"company_id: {company_id}, "
            f"signature_signed: {signature_signed}, "
            f"signature_approved: {signature_approved}"
        )
        return Signature().get_signatures_by_reference(
            self.get_user_id(),
            "user",
            project_id=project_id,
            user_ccla_company_id=company_id,
            signature_approved=signature_approved,
            signature_signed=signature_signed,
        )

    def get_latest_signature(self, project_id, company_id=None, signature_signed=None, signature_approved=None) -> \
            Optional[Signature]:
        """
        Helper function to get a user's latest signature for a project.

        :param project_id: The ID of the project to check for.
        :type project_id: string
        :param company_id: The company ID if looking for an employee signature.
        :type company_id: string
        :param signature_signed: The signature signed flag
        :type signature_signed: bool
        :param signature_approved: The signature approved flag
        :type signature_approved: bool
        :return: The latest versioned signature object if it exists.
        :rtype: cla.models.model_interfaces.Signature or None
        """
        fn = 'dynamodb_models.get_latest_signature'
        cla.log.debug(
            f"{fn} - self.get_user_signatures with "
            f"user_id: {self.get_user_id()}, "
            f"project_id: {project_id}, "
            f"company_id: {company_id}"
        )
        signatures = self.get_user_signatures(project_id=project_id, company_id=company_id,
                                              signature_signed=signature_signed, signature_approved=signature_approved)
        latest = None
        for signature in signatures:
            if latest is None:
                latest = signature
            elif signature.get_signature_document_major_version() > latest.get_signature_document_major_version():
                latest = signature
            elif (
                    signature.get_signature_document_major_version() == latest.get_signature_document_major_version()
                    and signature.get_signature_document_minor_version() > latest.get_signature_document_minor_version()
            ):
                latest = signature

        if latest is None:
            cla.log.debug(
                f"{fn} - unable to find user signature using "
                f"user_id: {self.get_user_id()}, "
                f"project id: {project_id}, "
                f"company id: {company_id}"
            )
        else:
            cla.log.debug(
                f"{fn} - found user user signature using "
                f"user_id: {self.get_user_id()}, "
                f"project id: {project_id}, "
                f"company id: {company_id}"
            )

        return latest

    def preprocess_pattern(self, emails, patterns) -> bool:
        """
        Helper function that preprocesses given emails against patterns

        :param emails: User emails to be checked
        :type emails: list
        :return: True if at least one email is matched against pattern else False
        :rtype: bool
        """
        fn = 'dynamo_models.preprocess_pattern'
        for pattern in patterns:
            if pattern.startswith("*."):
                pattern = pattern.replace("*.", ".*")
            elif pattern.startswith("*"):
                pattern = pattern.replace("*", ".*")
            elif pattern.startswith("."):
                pattern = pattern.replace(".", ".*")

            preprocessed_pattern = "^.*@" + pattern + "$"
            pat = re.compile(preprocessed_pattern)
            for email in emails:
                if pat.match(email) is not None:
                    self.log_debug(f'{fn} - found user email in email approval pattern')
                    return True
        return False

    # Accepts a Signature object

    def is_approved(self, ccla_signature: Signature) -> bool:
        """
        Helper function to determine whether at least one of the user's email
        addresses are whitelisted for a particular ccla signature.

        :param ccla_signature: The ccla signature to check against.
        :type ccla_signature: cla.models.Signature
        :return: True if at least one email is whitelisted, False otherwise.
        :rtype: bool
        """
        fn = 'dynamo_models.is_approved'
        # Returns the union of lf_emails and emails (separate columns)
        emails = self.get_all_user_emails()
        if len(emails) > 0:
            # remove leading and trailing whitespace before checking emails
            emails = [email.strip() for email in emails]

        # First, we check email whitelist
        whitelist = ccla_signature.get_email_whitelist()
        cla.log.debug(f'{fn} - testing user emails: {emails} with '
                      f'CCLA approval emails: {whitelist}')

        if whitelist is not None:
            for email in emails:
                # Case insensitive match
                if email.lower() in (s.lower() for s in whitelist):
                    cla.log.debug(f'{fn} - found user email in email approval list')
                    return True
        else:
            cla.log.debug(f'{fn} - no email whitelist match for user: {self}')

        # Secondly, let's check domain whitelist
        # If a naked domain (e.g. google.com) is provided, we prefix it with '^.*@',
        # so that sub-domains are not allowed.
        # If a '*', '*.' or '.' prefix is provided, we replace the prefix with '.*\.',
        # which will allow subdomains.
        patterns = ccla_signature.get_domain_whitelist()
        cla.log.debug(f'{fn} - testing user email domains: {emails} with '
                      f'domain approval values: {patterns}')

        if patterns is not None:
            if self.preprocess_pattern(emails, patterns):
                return True
            else:
                self.log_debug(f'{fn} - did not match email: {emails} with domain: {patterns}')
        else:
            cla.log.debug(f'{fn} - no domain approval patterns defined - '
                          'skipping domain approval checks')

        # Third and Forth, check github whitelists
        github_username = self.get_user_github_username()
        github_id = self.get_user_github_id()

        # TODO: DAD -
        # Since usernames can be changed, if we have the github_id already - let's
        # lookup the username by id to see if they have changed their username
        # if the username is different, then we should reset the field to the
        # new value - this will potentially change the github username whitelist
        # since the old username is already in the list

        # Attempt to fetch the github username based on the github id
        if github_username is None and github_id is not None:
            github_username = cla.utils.lookup_user_github_username(github_id)
            if github_username is not None:
                cla.log.debug(f'{fn} - updating user record - adding github username: {github_username}')
                self.set_user_github_username(github_username)
                self.save()

        # Attempt to fetch the github id based on the github username
        if github_id is None and github_username is not None:
            github_username = github_username.strip()
            github_id = cla.utils.lookup_user_github_id(github_username)
            if github_id is not None:
                cla.log.debug(f'{fn} - updating user record - adding github id: {github_id}')
                self.set_user_github_id(github_id)
                self.save()

        # GitHub username approval list processing
        if github_username is not None:
            # remove leading and trailing whitespace from github username
            github_username = github_username.strip()
            github_whitelist = ccla_signature.get_github_whitelist()
            cla.log.debug(f'{fn} - testing user github username: {github_username} with '
                          f'CCLA github approval list: {github_whitelist}')

            if github_whitelist is not None:
                # case insensitive search
                if github_username.lower() in (s.lower() for s in github_whitelist):
                    cla.log.debug(f'{fn} - found github username in github approval list')
                    return True
        else:
            cla.log.debug(f'{fn} - users github_username is not defined - '
                          'skipping github username approval list check')

        # Check github org approval list
        if github_username is not None:
            # Load the github org approval list for this CCLA signature record
            github_org_approval_list = ccla_signature.get_github_org_whitelist()
            if github_org_approval_list is not None:
                # Fetch the list of orgs associated with this user
                cla.log.debug(f'{fn} - determining if github user {github_username} is associated '
                              f'with any of the github organizations: {github_org_approval_list}')
                github_orgs = cla.utils.lookup_github_organizations(github_username)
                if "error" not in github_orgs:
                    cla.log.debug(f'{fn} - testing user github org: {github_orgs} with '
                                  f'CCLA github org approval list: {github_org_approval_list}')

                    for dynamo_github_org in github_org_approval_list:
                        # case insensitive search
                        if dynamo_github_org.lower() in (s.lower() for s in github_orgs):
                            cla.log.debug(f'{fn} - found matching github organization for user')
                            return True
                        else:
                            cla.log.debug(f'{fn} - user {github_username} is not in the '
                                          f'organization: {dynamo_github_org}')
                else:
                    cla.log.warning(f'{fn} - unable to lookup github organizations for the user: {github_username}: '
                                    f'{github_orgs}')
            else:
                cla.log.debug(f'{fn} - no github organization approval list defined for this CCLA')
        else:
            cla.log.debug(f'{fn} - user\'s github_username is not defined - skipping github org approval list check')

        # Check GitLab username and id
        gitlab_username = self.get_user_gitlab_username()
        gitlab_id = self.get_user_gitlab_id()

        # Attempt to fetch the gitlab username based on the gitlab id
        if gitlab_username is None and gitlab_id is not None:
            github_username = cla.utils.lookup_user_gitlab_username(gitlab_id)
            if gitlab_username is not None:
                cla.log.debug(f'{fn} - updating user record - adding gitlab username: {gitlab_username}')
                self.set_user_gitlab_username(gitlab_username)
                self.save()

        # Attempt to fetch the gitlab id based on the gitlab username
        if gitlab_id is None and gitlab_username is not None:
            gitlab_username = gitlab_username.strip()
            gitlab_id = cla.utils.lookup_user_gitlab_id(gitlab_username)
            if gitlab_id is not None:
                cla.log.debug(f'{fn} - updating user record - adding gitlab id: {gitlab_id}')
                self.set_user_gitlab_id(gitlab_id)
                self.save()

        # GitLab username approval list processing
        if gitlab_username is not None:
            # remove leading and trailing whitespace from gitlab username
            gitlab_username = gitlab_username.strip()
            gitlab_whitelist = ccla_signature.get_gitlab_username_approval_list()
            cla.log.debug(f'{fn} - testing user github username: {gitlab_username} with '
                          f'CCLA github approval list: {gitlab_whitelist}')

            if gitlab_whitelist is not None:
                # case insensitive search
                if gitlab_username.lower() in (s.lower() for s in gitlab_whitelist):
                    cla.log.debug(f'{fn} - found gitlab username in gitlab approval list')
                    return True
        else:
            cla.log.debug(f'{fn} - users gitlab_username is not defined - '
                          'skipping gitlab username approval list check')

        if gitlab_username is not None:
            cla.log.debug(f'{fn} fetching gitlab org approval list items to search by username: {gitlab_username}')
            gitlab_org_approval_lists = ccla_signature.get_gitlab_org_approval_list()
            cla.log.debug(f'{fn} checking gitlab org approval list: {gitlab_org_approval_lists}')
            if gitlab_org_approval_lists:
                for gl_name in gitlab_org_approval_lists:
                    try:
                        gl_org = GitlabOrg().search_organization_by_group_url(gl_name)
                        cla.log.debug(
                            f"{fn} checking gitlab_username against approval list for gitlab group: {gl_name}")
                        gl_list = list(filter(lambda gl_user: gl_user.get('username') == gitlab_username,
                                              cla.utils.lookup_gitlab_org_members(gl_org.get_organization_id())))
                        if len(gl_list) > 0:
                            cla.log.debug(f'{fn} - found gitlab username in gitlab approval list')
                            return True
                    except DoesNotExist as err:
                        cla.log.debug(f'gitlab group with full path: {gl_name} does not exist: {err}')

        cla.log.debug(f'{fn} - unable to find user in any whitelist')
        return False

    def get_users_by_company(self, company_id):
        user_generator = self.model.scan(user_company_id__eq=str(company_id))
        users = []
        for user_model in user_generator:
            user = User()
            user.model = user_model
            users.append(user)
        return users

    def all(self, emails=None):
        if emails is None:
            users = self.model.scan()
        else:
            users = UserModel.batch_get(emails)
        ret = []
        for user in users:
            usr = User()
            usr.model = user
            ret.append(usr)
        return ret


class RepositoryModel(BaseModel):
    """
    Represents a repository in the database.
    """

    class Meta:
        """Meta class for Repository."""

        table_name = "cla-{}-repositories".format(stage)
        if stage == "local":
            host = "http://localhost:8000"

    repository_id = UnicodeAttribute(hash_key=True)
    repository_project_id = UnicodeAttribute(null=True)
    repository_name = UnicodeAttribute(null=True)
    repository_type = UnicodeAttribute(null=True)  # Gerrit, GitHub, etc.
    repository_url = UnicodeAttribute(null=True)
    repository_organization_name = UnicodeAttribute(null=True)
    repository_external_id = UnicodeAttribute(null=True)
    repository_sfdc_id = UnicodeAttribute(null=True)
    project_sfid = UnicodeAttribute(null=True)
    enabled = BooleanAttribute(default=False)
    note = UnicodeAttribute(null=True)
    repository_external_index = ExternalRepositoryIndex()
    repository_project_index = ProjectRepositoryIndex()
    project_sfid_repository_index = ProjectSFIDRepositoryIndex()
    repository_sfdc_index = SFDCRepositoryIndex()


class Repository(model_interfaces.Repository):
    """
    ORM-agnostic wrapper for the DynamoDB Repository model.
    """

    def __init__(
            self,
            repository_id=None,
            repository_project_id=None,  # pylint: disable=too-many-arguments
            repository_name=None,
            repository_type=None,
            repository_url=None,
            repository_organization_name=None,
            repository_external_id=None,
            repository_sfdc_id=None,
            note=None,
    ):
        super(Repository).__init__()
        self.model = RepositoryModel()
        self.model.repository_id = repository_id
        self.model.repository_project_id = repository_project_id
        self.model.repository_sfdc_id = repository_sfdc_id
        self.model.project_sfid = repository_sfdc_id
        self.model.repository_name = repository_name
        self.model.repository_type = repository_type
        self.model.repository_url = repository_url
        self.model.repository_organization_name = repository_organization_name
        self.model.repository_external_id = repository_external_id
        self.model.note = note

    def to_dict(self):
        return dict(self.model)

    def save(self) -> None:
        self.model.date_modified = datetime.datetime.utcnow()
        self.model.save()

    def load(self, repository_id):
        try:
            repo = self.model.get(repository_id)
        except RepositoryModel.DoesNotExist:
            raise cla.models.DoesNotExist("Repository not found")
        self.model = repo

    def get_repository_models_by_project_sfid(self, project_sfid) -> List[Repository]:
        repository_generator = self.model.project_sfid_repository_index.query(project_sfid)
        repositories = []
        for repository_model in repository_generator:
            repository = Repository()
            repository.model = repository_model
            repositories.append(repository)
        return repositories

    def get_repository_by_project_sfid(self, project_sfid) -> List[dict]:
        repository_generator = self.model.project_sfid_repository_index.query(project_sfid)
        repositories = []
        for repository_model in repository_generator:
            repository = Repository()
            repository.model = repository_model
            repositories.append(repository)
        return repositories

    def get_repository_models_by_repository_sfdc_id(self, project_sfid) -> List[Repository]:
        repository_generator = self.model.repository_sfdc_index.query(project_sfid)
        repositories = []
        for repository_model in repository_generator:
            repository = Repository()
            repository.model = repository_model
            repositories.append(repository)
        return repositories

    def get_repository_models_by_repository_cla_group_id(self, cla_group_id: str) -> List[Repository]:
        repository_generator = self.model.repository_project_index.query(cla_group_id)
        repositories = []
        for repository_model in repository_generator:
            repository = Repository()
            repository.model = repository_model
            repositories.append(repository)
        return repositories

    def delete(self):
        self.model.delete()

    def get_repository_id(self):
        return self.model.repository_id

    def get_repository_project_id(self):
        return self.model.repository_project_id

    def get_repository_name(self):
        return self.model.repository_name

    def get_repository_type(self):
        return self.model.repository_type

    def get_repository_url(self):
        return self.model.repository_url

    def get_repository_external_id(self):
        return self.model.repository_external_id

    def get_repository_sfdc_id(self):
        return self.model.repository_sfdc_id

    def get_project_sfid(self):
        return self.model.project_sfid

    def get_repository_organization_name(self):
        return self.model.repository_organization_name

    def get_enabled(self):
        return self.model.enabled

    def get_note(self):
        return self.model.note

    def set_repository_id(self, repo_id):
        self.model.repository_id = str(repo_id)

    def set_repository_project_id(self, project_id):
        self.model.repository_project_id = project_id

    def set_repository_name(self, name):
        self.model.repository_name = name

    def set_repository_type(self, repo_type):
        self.model.repository_type = repo_type

    def set_repository_url(self, repository_url):
        self.model.repository_url = repository_url

    def set_repository_external_id(self, repository_external_id):
        self.model.repository_external_id = str(repository_external_id)

    def set_repository_sfdc_id(self, repository_sfdc_id):
        self.model.repository_sfdc_id = str(repository_sfdc_id)
        self.set_project_sfid(str(repository_sfdc_id))

    def set_project_sfid(self, project_sfid):
        self.model.project_sfid = str(project_sfid)

    def set_repository_organization_name(self, organization_name):
        self.model.repository_organization_name = organization_name

    def set_enabled(self, enabled):
        self.model.enabled = enabled

    def set_note(self, note):
        self.model.note = note

    def add_note(self, note):
        if self.model.note is None:
            self.model.note = note
        else:
            self.model.note = self.model.note + ' ' + note

    def get_repositories_by_cla_group_id(self, cla_group_id):
        repository_generator = self.model.repository_project_index.query(str(cla_group_id))
        repositories = []
        for repository_model in repository_generator:
            repository = Repository()
            repository.model = repository_model
            repositories.append(repository)
        return repositories

    def get_repository_by_external_id(self, repository_external_id, repository_type):
        # TODO: Optimize this on the DB end.
        repository_generator = self.model.repository_external_index.query(str(repository_external_id))
        for repository_model in repository_generator:
            if repository_model.repository_type == repository_type:
                repository = Repository()
                repository.model = repository_model
                return repository
        return None

    def get_repository_by_sfdc_id(self, repository_sfdc_id):
        repositories = self.model.repository_sfdc_index.query(str(repository_sfdc_id))
        ret = []
        for repository in repositories:
            repo = Repository()
            repo.model = repository
            ret.append(repo)
        return ret

    def get_repositories_by_organization(self, organization_name):
        repository_generator = self.model.scan(repository_organization_name__eq=organization_name)
        repositories = []
        for repository_model in repository_generator:
            repository = Repository()
            repository.model = repository_model
            repositories.append(repository)
        return repositories

    def all(self, ids=None):
        if ids is None:
            repositories = self.model.scan()
        else:
            repositories = RepositoryModel.batch_get(ids)
        ret = []
        for repository in repositories:
            repo = Repository()
            repo.model = repository
            ret.append(repo)
        return ret


def create_filter(attributes, model):
    """
    Helper function that creates filter condition based on available attributes

    :param attributes: attributes consisting of model attributes and values
    :rtype attributes: dict
    :param model: Model instance that handles filtering
    :rtype model: pynamodb.models.Model
    """
    filter_condition = None
    for key, value in attributes.items():
        if not value:
            continue
        condition = getattr(model, key) == value
        filter_condition = (
            condition if not isinstance(filter_condition, Condition) else filter_condition & condition
        )
    return filter_condition


class SignatureModel(BaseModel):  # pylint: disable=too-many-instance-attributes
    """
    Represents an signature in the database.
    """

    class Meta:
        """Meta class for Signature."""

        table_name = "cla-{}-signatures".format(stage)
        if stage == "local":
            host = "http://localhost:8000"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])

    signature_id = UnicodeAttribute(hash_key=True)
    signature_external_id = UnicodeAttribute(null=True)
    signature_project_id = UnicodeAttribute(null=True)
    signature_document_minor_version = NumberAttribute(null=True)
    signature_document_major_version = NumberAttribute(null=True)
    signature_reference_id = UnicodeAttribute(range_key=True)
    signature_reference_name = UnicodeAttribute(null=True)
    signature_reference_name_lower = UnicodeAttribute(null=True)
    signature_reference_type = UnicodeAttribute(null=True)
    signature_type = UnicodeAttribute(default="cla")
    signature_signed = BooleanAttribute(default=False)
    # Signed on date/time
    signed_on = UnicodeAttribute(null=True)
    signatory_name = UnicodeAttribute(null=True)
    signing_entity_name = UnicodeAttribute(null=True)
    # Encoded string for searching
    # eg: icla#true#true#123abd-sadf0-458a-adba-a9393939393
    sigtype_signed_approved_id = UnicodeAttribute(null=True)
    signature_approved = BooleanAttribute(default=False)
    signature_sign_url = UnicodeAttribute(null=True)
    signature_return_url = UnicodeAttribute(null=True)
    signature_callback_url = UnicodeAttribute(null=True)
    signature_user_ccla_company_id = UnicodeAttribute(null=True)
    signature_acl = PatchedUnicodeSetAttribute(default=set)
    signature_project_index = ProjectSignatureIndex()
    signature_reference_index = ReferenceSignatureIndex()
    signature_envelope_id = UnicodeAttribute(null=True)
    signature_embargo_acked = BooleanAttribute(default=True, null=True)
    # Callback type refers to either Gerrit or GitHub
    signature_return_url_type = UnicodeAttribute(null=True)
    note = UnicodeAttribute(null=True)
    signature_project_external_id = UnicodeAttribute(null=True)
    signature_company_signatory_id = UnicodeAttribute(null=True)
    signature_company_signatory_name = UnicodeAttribute(null=True)
    signature_company_signatory_email = UnicodeAttribute(null=True)
    signature_company_initial_manager_id = UnicodeAttribute(null=True)
    signature_company_initial_manager_name = UnicodeAttribute(null=True)
    signature_company_initial_manager_email = UnicodeAttribute(null=True)
    signature_company_secondary_manager_list = JSONAttribute(null=True)
    signature_company_signatory_index = SignatureCompanySignatoryIndex()
    signature_company_initial_manager_index = SignatureCompanyInitialManagerIndex()
    project_signature_external_id_index = SignatureProjectExternalIndex()
    signature_project_reference_index = SignatureProjectReferenceIndex()

    # approval lists (previously called whitelists) are only used by CCLAs
    domain_whitelist = ListAttribute(null=True)
    email_whitelist = ListAttribute(null=True)
    github_whitelist = ListAttribute(null=True)
    github_org_whitelist = ListAttribute(null=True)
    gitlab_org_approval_list = ListAttribute(null=True)
    gitlab_username_approval_list = ListAttribute(null=True)

    # Additional attributes for ICLAs
    user_email = UnicodeAttribute(null=True)
    user_github_username = UnicodeAttribute(null=True)
    user_name = UnicodeAttribute(null=True)
    user_lf_username = UnicodeAttribute(null=True)
    user_docusign_name = UnicodeAttribute(null=True)
    user_docusign_date_signed = UnicodeAttribute(null=True)
    user_docusign_raw_xml = UnicodeAttribute(null=True)

    auto_create_ecla = BooleanAttribute(default=False)


class Signature(model_interfaces.Signature):  # pylint: disable=too-many-public-methods
    """
    ORM-agnostic wrapper for the DynamoDB Signature model.
    """

    def __init__(
            self,  # pylint: disable=too-many-arguments
            signature_id=None,
            signature_external_id=None,
            signature_project_id=None,
            signature_document_minor_version=None,
            signature_document_major_version=None,
            signature_reference_id=None,
            signature_reference_name=None,
            signature_reference_type="user",
            signature_type=None,
            signature_signed=False,
            signature_approved=False,
            signature_embargo_acked=True,
            signed_on=None,
            signatory_name=None,
            signing_entity_name=None,
            sigtype_signed_approved_id=None,
            signature_sign_url=None,
            signature_return_url=None,
            signature_callback_url=None,
            signature_user_ccla_company_id=None,
            signature_acl=set(),
            signature_return_url_type=None,
            signature_envelope_id=None,
            domain_whitelist=None,
            email_whitelist=None,
            github_whitelist=None,
            github_org_whitelist=None,
            note=None,
            signature_project_external_id=None,
            signature_company_signatory_id=None,
            signature_company_signatory_name=None,
            signature_company_signatory_email=None,
            signature_company_initial_manager_id=None,
            signature_company_initial_manager_name=None,
            signature_company_initial_manager_email=None,
            signature_company_secondary_manager_list=None,
            user_email=None,
            user_github_username=None,
            user_name=None,
            user_docusign_name=None,
            user_docusign_date_signed=None,
            auto_create_ecla: bool = False,
    ):
        super(Signature).__init__()

        self.model = SignatureModel()
        self.model.signature_id = signature_id
        self.model.signature_external_id = signature_external_id
        self.model.signature_project_id = signature_project_id
        self.model.signature_document_minor_version = signature_document_minor_version
        self.model.signature_document_major_version = signature_document_major_version
        self.model.signature_reference_id = signature_reference_id
        self.model.signature_reference_name = signature_reference_name
        if signature_reference_name:
            self.model.signature_reference_name_lower = signature_reference_name.lower()
        self.model.signature_reference_type = signature_reference_type
        self.model.signature_type = signature_type
        self.model.signature_signed = signature_signed
        self.model.signed_on = signed_on
        self.model.signatory_name = signatory_name
        self.model.signing_entity_name = signing_entity_name
        self.model.sigtype_signed_approved_id = sigtype_signed_approved_id
        self.model.signature_approved = signature_approved
        self.model.signature_embargo_acked = signature_embargo_acked
        self.model.signature_sign_url = signature_sign_url
        self.model.signature_return_url = signature_return_url
        self.model.signature_callback_url = signature_callback_url
        self.model.signature_user_ccla_company_id = signature_user_ccla_company_id
        self.model.signature_acl = signature_acl
        self.model.signature_return_url_type = signature_return_url_type
        self.model.signature_envelope_id = signature_envelope_id
        self.model.domain_whitelist = domain_whitelist
        self.model.email_whitelist = email_whitelist
        self.model.github_whitelist = github_whitelist
        self.model.github_org_whitelist = github_org_whitelist
        self.model.note = note
        self.model.signature_project_external_id = signature_project_external_id
        self.model.signature_company_signatory_id = signature_company_signatory_id
        self.model.signature_company_signatory_email = signature_company_signatory_email
        self.model.signature_company_initial_manager_id = signature_company_initial_manager_id
        self.model.signature_company_initial_manager_name = signature_company_initial_manager_name
        self.model.signature_company_initial_manager_email = signature_company_initial_manager_email
        self.model.signature_company_secondary_manager_list = signature_company_secondary_manager_list
        self.model.user_email = user_email
        self.model.user_github_username = user_github_username
        self.model.user_name = user_name
        self.model.user_docusign_name = user_docusign_name
        # in format of 2020-12-21T08:29:20.51
        self.model.user_docusign_date_signed = user_docusign_date_signed
        self.model.auto_create_ecla = auto_create_ecla

    def __str__(self):
        return (
            "id: {}, project id: {}, reference id: {}, reference name: {}, reference name lower: {}, "
            "reference type: {}, "
            "user cla company id: {}, signed: {}, signed_on: {}, signatory_name: {}, signing entity name: {},"
            "sigtype_signed_approved_id: {}, "
            "approved: {}, embargo_acked: {}, domain whitelist: {}, "
            "email whitelist: {}, github user whitelist: {}, github domain whitelist: {}, "
            "note: {},signature project external id: {}, signature company signatory id: {}, "
            "signature company signatory name: {}, signature company signatory email: {},"
            "signature company initial manager id: {}, signature company initial manager name: {},"
            "signature company initial manager email: {}, signature company secondary manager list: {},"
            "user_email: {}, user_github_username: {}, user_name: {}, "
            "user_docusign_name: {}, user_docusign_date_signed: {}, "
            "auto_create_ecla: {}, "
            "created_on: {}, updated_on: {}"
        ).format(
            self.model.signature_id,
            self.model.signature_project_id,
            self.model.signature_reference_id,
            self.model.signature_reference_name,
            self.model.signature_reference_name_lower,
            self.model.signature_reference_type,
            self.model.signature_user_ccla_company_id,
            self.model.signature_signed,
            self.model.signed_on,
            self.model.signatory_name,
            self.model.signing_entity_name,
            self.model.sigtype_signed_approved_id,
            self.model.signature_approved,
            self.model.signature_embargo_acked,
            self.model.domain_whitelist,
            self.model.email_whitelist,
            self.model.github_whitelist,
            self.model.github_org_whitelist,
            self.model.note,
            self.model.signature_project_external_id,
            self.model.signature_company_signatory_id,
            self.model.signature_company_signatory_name,
            self.model.signature_company_signatory_email,
            self.model.signature_company_initial_manager_id,
            self.model.signature_company_initial_manager_name,
            self.model.signature_company_initial_manager_email,
            self.model.signature_company_secondary_manager_list,
            self.model.user_email,
            self.model.user_github_username,
            self.model.user_name,
            self.model.user_docusign_name,
            self.model.user_docusign_date_signed,
            self.model.auto_create_ecla,
            self.model.get_date_created(),
            self.model.get_date_modified(),
        )

    def to_dict(self):
        """
        to_dict returns dictionary  representation of the model, this is what's sent back as
        API result to the users, this is the place we need to filter out some sensitive data
        (eg. user_docusign_raw_xml)
        :return:
        """
        d = dict(self.model)
        keys_to_filter = ["user_docusign_raw_xml"]

        for k in keys_to_filter:
            if k in d:
                del d[k]
        return d

    def save(self) -> None:
        self.model.date_modified = datetime.datetime.now(timezone.utc)
        cla.log.info(f'saving datetime: {self.model.date_modified}')
        self.model.save()

    def load(self, signature_id):
        try:
            signature = self.model.get(signature_id)
        except SignatureModel.DoesNotExist:
            raise cla.models.DoesNotExist("Signature not found")
        self.model = signature

    def delete(self):
        self.model.delete()

    def get_signature_id(self):
        return self.model.signature_id

    def get_signature_external_id(self):
        return self.model.signature_external_id

    def get_signature_project_id(self):
        return self.model.signature_project_id

    def get_signature_document_minor_version(self):
        return self.model.signature_document_minor_version

    def get_signature_document_major_version(self):
        return self.model.signature_document_major_version

    def get_signature_type(self):
        return self.model.signature_type

    def get_signature_signed(self):
        return self.model.signature_signed

    def get_signed_on(self):
        return self.model.signed_on

    def get_signatory_name(self):
        return self.model.signatory_name

    def get_signing_entity_name(self):
        return self.model.signing_entity_name

    def get_sigtype_signed_approved_id(self):
        return self.model.sigtype_signed_approved_id

    def get_signature_approved(self):
        return self.model.signature_approved

    def get_signature_embargo_acked(self):
        return self.model.signature_embargo_acked

    def get_signature_sign_url(self):
        return self.model.signature_sign_url

    def get_signature_return_url(self):
        return self.model.signature_return_url

    def get_signature_callback_url(self):
        return self.model.signature_callback_url

    def get_signature_reference_id(self):
        return self.model.signature_reference_id

    def get_signature_reference_name(self):
        return self.model.signature_reference_name

    def get_signature_reference_name_lower(self):
        return self.model.signature_reference_name_lower

    def get_signature_reference_type(self):
        return self.model.signature_reference_type

    def get_signature_user_ccla_company_id(self):
        return self.model.signature_user_ccla_company_id

    def get_signature_acl(self):
        return self.model.signature_acl or set()

    def get_signature_return_url_type(self):
        # Refers to either Gerrit or GitHub
        return self.model.signature_return_url_type

    def get_signature_envelope_id(self):
        return self.model.signature_envelope_id

    def get_domain_whitelist(self):
        return self.model.domain_whitelist

    def get_email_whitelist(self):
        return self.model.email_whitelist

    def get_github_whitelist(self):
        return self.model.github_whitelist

    def get_github_org_whitelist(self):
        return self.model.github_org_whitelist

    def get_gitlab_org_approval_list(self):
        return self.model.gitlab_org_approval_list

    def get_gitlab_username_approval_list(self):
        return self.model.gitlab_username_approval_list

    def get_note(self):
        return self.model.note

    def get_signature_company_signatory_id(self):
        return self.model.signature_company_signatory_id

    def get_signature_company_signatory_name(self):
        return self.model.signature_company_signatory_name

    def get_signature_company_signatory_email(self):
        return self.model.signature_company_signatory_email

    def get_signature_company_initial_manager_id(self):
        return self.model.signature_company_initial_manager_id

    def get_signature_company_initial_manager_name(self):
        return self.model.signature_company_initial_manager_name

    def get_signature_company_initial_manager_email(self):
        return self.model.signature_company_initial_manager_email

    def get_signature_company_secondary_manager_list(self):
        return self.model.signature_company_secondary_manager_list

    def get_signature_project_external_id(self):
        return self.model.signature_project_external_id

    def get_user_email(self):
        return self.model.user_email

    def get_user_github_username(self):
        return self.model.user_github_username

    def get_user_name(self):
        return self.model.user_name

    def get_user_lf_username(self):
        return self.model.user_lf_username

    def get_user_docusign_name(self):
        return self.model.user_docusign_name

    def get_user_docusign_date_signed(self):
        return self.model.user_docusign_date_signed

    def get_user_docusign_raw_xml(self):
        return self.model.user_docusign_raw_xml

    def get_auto_create_ecla(self) -> bool:
        return self.model.auto_create_ecla

    def set_signature_id(self, signature_id) -> None:
        self.model.signature_id = str(signature_id)

    def set_signature_external_id(self, signature_external_id) -> None:
        self.model.signature_external_id = str(signature_external_id)

    def set_signature_project_id(self, project_id) -> None:
        self.model.signature_project_id = str(project_id)

    def set_signature_document_minor_version(self, document_minor_version) -> None:
        self.model.signature_document_minor_version = int(document_minor_version)

    def set_signature_document_major_version(self, document_major_version) -> None:
        self.model.signature_document_major_version = int(document_major_version)

    def set_signature_type(self, signature_type) -> None:
        self.model.signature_type = signature_type

    def set_signature_signed(self, signed) -> None:
        self.model.signature_signed = bool(signed)

    def set_signed_on(self, signed_on) -> None:
        self.model.signed_on = signed_on

    def set_signatory_name(self, signatory_name) -> None:
        self.model.signatory_name = signatory_name

    def set_signing_entity_name(self, signing_entity_name) -> None:
        self.model.signing_entity_name = signing_entity_name

    def set_sigtype_signed_approved_id(self, sigtype_signed_approved_id) -> None:
        self.model.sigtype_signed_approved_id = sigtype_signed_approved_id

    def set_signature_approved(self, approved) -> None:
        self.model.signature_approved = bool(approved)

    def set_signature_embargo_acked(self, embargo_acked) -> None:
        self.model.signature_embargo_acked = bool(embargo_acked)

    def set_signature_sign_url(self, sign_url) -> None:
        self.model.signature_sign_url = sign_url

    def set_signature_return_url(self, return_url) -> None:
        self.model.signature_return_url = return_url

    def set_signature_callback_url(self, callback_url) -> None:
        self.model.signature_callback_url = callback_url

    def set_signature_reference_id(self, reference_id) -> None:
        self.model.signature_reference_id = reference_id

    def set_signature_reference_name(self, reference_name) -> None:
        self.model.signature_reference_name = reference_name
        self.model.signature_reference_name_lower = reference_name.lower()

    def set_signature_reference_type(self, reference_type) -> None:
        self.model.signature_reference_type = reference_type

    def set_signature_user_ccla_company_id(self, company_id) -> None:
        self.model.signature_user_ccla_company_id = company_id

    def set_signature_acl(self, signature_acl_username) -> None:
        self.model.signature_acl = set([signature_acl_username])

    def set_signature_return_url_type(self, signature_return_url_type) -> None:
        self.model.signature_return_url_type = signature_return_url_type

    def set_signature_envelope_id(self, signature_envelope_id) -> None:
        self.model.signature_envelope_id = signature_envelope_id

    def set_signature_company_signatory_id(self, signature_company_signatory_id) -> None:
        self.model.signature_company_signatory_id = signature_company_signatory_id

    def set_signature_company_signatory_name(self, signature_company_signatory_name) -> None:
        self.model.signature_company_signatory_name = signature_company_signatory_name

    def set_signature_company_signatory_email(self, signature_company_signatory_email) -> None:
        self.model.signature_company_signatory_email = signature_company_signatory_email

    def set_signature_company_initial_manager_id(self, signature_company_initial_manager_id) -> None:
        self.model.signature_company_initial_manager_id = signature_company_initial_manager_id

    def set_signature_company_initial_manager_name(self, signature_company_initial_manager_name) -> None:
        self.model.signature_company_initial_manager_name = signature_company_initial_manager_name

    def set_signature_company_initial_manager_email(self, signature_company_initial_manager_email) -> None:
        self.model.signature_company_initial_manager_email = signature_company_initial_manager_email

    def set_signature_company_secondary_manager_list(self, signature_company_secondary_manager_list) -> None:
        self.model.signature_company_secondary_manager_list = signature_company_secondary_manager_list

    # Remove leading and trailing whitespace for all items before setting whitelist

    def set_domain_whitelist(self, domain_whitelist) -> None:
        self.model.domain_whitelist = [domain.strip() for domain in domain_whitelist]

    def set_email_whitelist(self, email_whitelist) -> None:
        self.model.email_whitelist = [email.strip() for email in email_whitelist]

    def set_github_whitelist(self, github_whitelist) -> None:
        self.model.github_whitelist = [github_user.strip() for github_user in github_whitelist]

    def set_github_org_whitelist(self, github_org_whitelist) -> None:
        self.model.github_org_whitelist = [github_org.strip() for github_org in github_org_whitelist]

    def set_gitlab_username_approval_list(self, gitlab_username_approval_list) -> None:
        self.model.gitlab_username_approval_list = [gitlab_user.strip() for gitlab_user in
                                                    gitlab_username_approval_list]

    def set_gitlab_org_approval_list(self, gitlab_org_approval_list) -> None:
        self.model.gitlab_org_approval_list = [gitlab_org.strip() for gitlab_org in gitlab_org_approval_list]

    def set_note(self, note) -> None:
        self.model.note = note

    def set_signature_project_external_id(self, signature_project_external_id) -> None:
        self.model.signature_project_external_id = signature_project_external_id

    def add_signature_acl(self, username) -> None:
        if not self.model.signature_acl:
            self.model.signature_acl = set()
        self.model.signature_acl.add(username)

    def remove_signature_acl(self, username) -> None:
        current_acl = self.model.signature_acl or set()
        if username not in current_acl:
            return
        self.model.signature_acl.remove(username)

    def set_user_email(self, user_email) -> None:
        self.model.user_email = user_email

    def set_user_github_username(self, user_github_username) -> None:
        self.model.user_github_username = user_github_username

    def set_user_name(self, user_name) -> None:
        self.model.user_name = user_name

    def set_user_lf_username(self, user_lf_username) -> None:
        self.model.user_lf_username = user_lf_username

    def set_user_docusign_name(self, user_docusign_name) -> None:
        self.model.user_docusign_name = user_docusign_name

    def set_user_docusign_date_signed(self, user_docusign_date_signed) -> None:
        self.model.user_docusign_date_signed = user_docusign_date_signed

    def set_user_docusign_raw_xml(self, user_docusign_raw_xml) -> None:
        self.model.user_docusign_raw_xml = user_docusign_raw_xml

    def set_auto_create_ecla(self, auto_create_ecla: bool) -> None:
        self.model.auto_create_ecla = auto_create_ecla
    def get_signatures_by_reference(
            self,  # pylint: disable=too-many-arguments
            reference_id,
            reference_type,
            project_id=None,
            user_ccla_company_id=None,
            signature_signed=None,
            signature_approved=None,
    ):
        fn = 'cla.models.dynamo_models.signature.get_signatures_by_reference'
        cla.log.debug(f'{fn} - reference_id: {reference_id}, reference_type: {reference_type},'
                      f' project_id: {project_id}, user_ccla_company_id: {user_ccla_company_id},'
                      f' signature_signed: {signature_signed}, signature_approved: {signature_approved}')

        cla.log.debug(f'{fn} - performing signature_reference_id query using: {reference_id}')
        # TODO: Optimize this query to use filters properly.
        # signature_generator = self.model.signature_reference_index.query(str(reference_id))
        try:
            signature_generator = self.model.signature_project_reference_index.query(str(project_id), range_key_condition=SignatureModel.signature_reference_id == str(reference_id))
        except Exception as e:
            cla.log.error(f'{fn} - error performing signature_reference_id query using: {reference_id} - '
                          f'error: {e}')
            raise e

        signatures = []
        for signature_model in signature_generator:
            cla.log.debug(f'{fn} - processing signature {signature_model}')

            # Skip signatures that are not the same reference type: user/company
            if signature_model.signature_reference_type != reference_type:
                cla.log.debug(f"{fn} - skipping signature - "
                              f"reference types do not match: {signature_model.signature_reference_type} "
                              f"versus {reference_type}")
                continue
            cla.log.debug(f"{fn} - signature reference types match: {signature_model.signature_reference_type}")

            # Skip signatures that are not an employee CCLA if user_ccla_company_id is present.
            # if user_ccla_company_id and signature_user_ccla_company_id are both none
            # it loads the ICLA signatures for a user.
            if signature_model.signature_user_ccla_company_id != user_ccla_company_id:
                cla.log.debug(f"{fn} - skipping signature - "
                              f"user_ccla_company_id values do not match: "
                              f"{signature_model.signature_user_ccla_company_id} "
                              f"versus {user_ccla_company_id}")
                continue

            # # Skip signatures that are not of the same project
            # if project_id is not None and signature_model.signature_project_id != project_id:
            #     cla.log.debug(f"{fn} - skipping signature - "
            #                   f"project_id values do not match: {signature_model.signature_project_id} "
            #                   f"versus {project_id}")
            #     continue

            # Skip signatures that do not have the same signed flags
            # e.g. retrieving only signed / approved signatures
            if signature_signed is not None and signature_model.signature_signed != signature_signed:
                cla.log.debug(f"{fn} - skipping signature - "
                              f"signature_signed values do not match: {signature_model.signature_signed} "
                              f"versus {signature_signed}")
                continue

            if signature_approved is not None and signature_model.signature_approved != signature_approved:
                cla.log.debug(f"{fn} - skipping signature - "
                              f"signature_approved values do not match: {signature_model.signature_approved} "
                              f"versus {signature_approved}")
                continue

            signature = Signature()
            signature.model = signature_model
            signatures.append(signature)
            cla.log.debug(f'{fn} -  signature match - adding signature to signature list: {signature}')
        return signatures

    def get_signatures_by_project(
            self,
            project_id,
            signature_signed=None,
            signature_approved=None,
            signature_type=None,
            signature_reference_type=None,
            signature_reference_id=None,
            signature_user_ccla_company_id=None,
    ):

        signature_attributes = {
            "signature_signed": signature_signed,
            "signature_approved": signature_approved,
            "signature_type": signature_type,
            "signature_reference_type": signature_reference_type,
            "signature_reference_id": signature_reference_id,
            "signature_user_ccla_company_id": signature_user_ccla_company_id
        }
        filter_condition = create_filter(signature_attributes, SignatureModel)

        cla.log.info("Loading signature by project for project_id: %s", project_id)
        signature_generator = self.model.signature_project_index.query(
            project_id, filter_condition=filter_condition
        )
        cla.log.info('Loaded signature by project for project_id: %s', project_id)
        signatures = []

        for signature_model in signature_generator:
            signature = Signature()
            signature.model = signature_model
            signatures.append(signature)
        cla.log.info('Returning %d signatures for project_id: %s', len(signatures), project_id)
        return signatures

    def get_signatures_by_company_project(self, company_id, project_id):
        signature_generator = self.model.signature_reference_index.query(
            company_id, SignatureModel.signature_project_id == project_id
        )
        signatures = []
        for signature_model in signature_generator:
            signature = Signature()
            signature.model = signature_model
            signatures.append(signature)
        signatures_dict = [signature_model.to_dict() for signature_model in signatures]
        return signatures_dict

    def get_ccla_signatures_by_company_project(self, company_id, project_id):
        signature_attributes = {
            "signature_signed": True,
            "signature_approved": True,
            "signature_type": 'ccla',
            "signature_reference_type": 'company',
            "signature_project_id": project_id,
        }
        filter_condition = create_filter(signature_attributes, SignatureModel)
        signature_generator = self.model.signature_reference_index.query(
            company_id, filter_condition=filter_condition & (
                SignatureModel.signature_user_ccla_company_id.does_not_exist())
        )
        signatures = []
        for signature_model in signature_generator:
            signature = Signature()
            signature.model = signature_model
            signatures.append(signature)
        cla.log.info(f'Returning {len(signatures)} signatures for '
                     f'project_id: {project_id} and '
                     f'company_id: {company_id}')
        return signatures

    def get_employee_signatures_by_company_project(self, company_id, project_id):
        signature_generator = self.model.signature_project_index.query(
            project_id, SignatureModel.signature_user_ccla_company_id == company_id
        )
        signatures = []
        for signature_model in signature_generator:
            signature = Signature()
            signature.model = signature_model
            signatures.append(signature)
        return signatures

    def get_employee_signature_by_company_project(self, company_id, project_id, user_id) -> Optional[Signature]:
        """
        Returns the employee signature for the specified user associated with
        the project/company. Returns None if no employee signature exists for
        this set of query parameters.
        """
        signature_attributes = {
            "signature_signed": True,
            "signature_approved": True,
            "signature_type": 'cla',
            "signature_reference_type": 'user',
            "signature_project_id": project_id,
            "signature_user_ccla_company_id": company_id
        }
        filter_condition = create_filter(signature_attributes, SignatureModel)
        signature_generator = self.model.signature_reference_index.query(
            user_id, filter_condition=filter_condition
        )
        signatures = []
        for signature_model in signature_generator:
            signature = Signature()
            signature.model = signature_model
            signatures.append(signature)
        # No employee signatures were found that were signed/approved
        if len(signatures) == 0:
            return None
        # Oops, we found more than 1?? This isn't good - maybe we simply return the first one?
        if len(signatures) > 1:
            cla.log.warning(
                "Why do we have more than one employee signature for this user? - Will return the first one only.")
        return signatures[0]

    def get_employee_signature_by_company_project_list(self, company_id, project_id, user_id) -> Optional[
        List[Signature]]:
        """
        Returns the employee signature for the specified user associated with
        the project/company. Returns None if no employee signature exists for
        this set of query parameters.
        """
        signature_attributes = {
            "signature_signed": True,
            "signature_approved": True,
            "signature_type": 'cla',
            "signature_reference_type": 'user',
            "signature_project_id": project_id,
            "signature_user_ccla_company_id": company_id
        }
        filter_condition = create_filter(signature_attributes, SignatureModel)
        signature_generator = self.model.signature_reference_index.query(
            user_id, filter_condition=filter_condition
        )
        signatures = []
        for signature_model in signature_generator:
            signature = Signature()
            signature.model = signature_model
            signatures.append(signature)
        # No employee signatures were found that were signed/approved
        if len(signatures) == 0:
            return None
        return signatures

    def get_employee_signatures_by_company_project_model(self, company_id, project_id) -> List[Signature]:
        signature_attributes = {
            "signature_signed": True,
            "signature_approved": True,
            "signature_type": 'cla',
            "signature_reference_type": 'user',
            "signature_user_ccla_company_id": company_id
        }
        filter_condition = create_filter(signature_attributes, SignatureModel)
        signature_generator = self.model.signature_project_index.query(
            project_id, filter_condition=filter_condition
        )
        signatures = []
        for signature_model in signature_generator:
            signature = Signature()
            signature.model = signature_model
            signatures.append(signature)
        return signatures

    def get_projects_by_company_signed(self, company_id):
        # Query returns all the signatures that the company has an approved and signed a CCLA for.
        # Loop through the signatures and retrieve only the project IDs referenced by the signatures.
        # Company Signatures
        signature_attributes = {
            "signature_signed": True,
            "signature_approved": True,
            "signature_type": 'ccla',
            "signature_reference_type": 'company',
        }
        filter_condition = create_filter(signature_attributes, SignatureModel)
        signature_generator = self.model.signature_reference_index.query(
            company_id,
            filter_condition=filter_condition & (SignatureModel.signature_user_ccla_company_id.does_not_exist())
        )
        project_ids = []
        for signature in signature_generator:
            project_ids.append(signature.signature_project_id)
        return project_ids

    def get_managers_by_signature_acl(self, signature_acl):
        managers = []
        user_model = User()
        for username in signature_acl:
            users = user_model.get_user_by_username(str(username))
            if users is not None:
                managers.append(users[0])
        return managers

    def get_managers(self):
        return self.get_managers_by_signature_acl(self.get_signature_acl())

    def all(self, ids: str = None) -> List[Signature]:
        if ids is None:
            signatures = self.model.scan()
        else:
            signatures = SignatureModel.batch_get(ids)
        ret = []
        for signature in signatures:
            sig = Signature()
            sig.model = signature
            ret.append(sig)
        return ret

    def all_limit(self, limit: Optional[int] = None, last_evaluated_key: Optional[str] = None) -> \
            (List[Signature], str, int):
        result_iterator = self.model.scan(limit=limit, last_evaluated_key=last_evaluated_key)
        ret = []
        for signature in result_iterator:
            sig = Signature()
            sig.model = signature
            ret.append(sig)
        return ret, result_iterator.last_evaluated_key, result_iterator.total_count


class ProjectCLAGroupModel(BaseModel):
    """
    Represents the lookuptable for clagroup and salesforce projects
    """

    class Meta:
        """Meta class for ProjectCLAGroup. """

        table_name = "cla-{}-projects-cla-groups".format(stage)
        if stage == "local":
            host = "http://localhost:8000"

    project_sfid = UnicodeAttribute(hash_key=True)
    project_name = UnicodeAttribute(null=True)
    cla_group_id = UnicodeAttribute(null=True)
    cla_group_name = UnicodeAttribute(null=True)
    foundation_sfid = UnicodeAttribute(null=True)
    foundation_name = UnicodeAttribute(null=True)
    foundation_sfid_index = FoundationSfidIndex()
    repositories_count = NumberAttribute(null=True)
    note = UnicodeAttribute(null=True)
    cla_group_id_index = CLAGroupIDIndex()


class ProjectCLAGroup(model_interfaces.ProjectCLAGroup):
    """
    ORM-agnostic wrapper for the DynamoDB ProjectCLAGroup model.
    """

    def __init__(self, project_sfid=None, project_name=None,
                 foundation_sfid=None, foundation_name=None,
                 cla_group_id=None, cla_group_name=None,
                 repositories_count=0, note=None, version='v1'):
        super(ProjectCLAGroup).__init__()
        self.model = ProjectCLAGroupModel()
        self.model.project_sfid = project_sfid
        self.model.project_name = project_name
        self.model.foundation_sfid = foundation_sfid
        self.model.foundation_name = foundation_name
        self.model.cla_group_id = cla_group_id
        self.model.cla_group_name = cla_group_name
        self.model.repositories_count = repositories_count
        self.model.note = note
        self.model.version = version

    def __str__(self):
        return (
            f"cla_group_id: {self.model.cla_group_id}",
            f"cla_group_name: {self.model.cla_group_name}",
            f"project_sfid: {self.model.project_sfid}",
            f"project_name: {self.model.project_name}",
            f"foundation_sfid: {self.model.foundation_sfid}",
            f"foundation_name: {self.model.foundation_name}",
            f"repositories_count: {self.model.repositories_count}",
            f"note: {self.model.note}",
            f"date_created: {self.model.date_created}",
            f"date_modified: {self.model.date_modified}",
            f"version: {self.model.version}",
        )

    def to_dict(self):
        return dict(self.model)

    def save(self):
        self.model.date_modified = datetime.datetime.utcnow()
        return self.model.save()

    def load(self, project_sfid):
        try:
            project_cla_group = self.model.get(project_sfid)
        except ProjectCLAGroupModel.DoesNotExist:
            raise cla.models.DoesNotExist("projectCLAGroup does not exist")
        self.model = project_cla_group

    def delete(self):
        self.model.delete()

    @property
    def signed_at_foundation(self) -> bool:
        foundation_level_cla = False
        if self.model.foundation_sfid:
            # Get all records that have the same foundation ID (including this current record)
            for mapping in self.get_by_foundation_sfid(self.model.foundation_sfid):
                # Foundation level CLA means that we have an entry where the FoundationSFID == ProjectSFID
                if mapping.get_foundation_sfid() == mapping.get_project_sfid():
                    foundation_level_cla = True
                    break
                    # DD: The below logic is incorrect - does not matter if we have a standalone project or not
                    # First check if project is a standalone project
                    # ps = ProjectService
                    # if not ps.is_standalone(mapping.get_project_sfid()):
                    #    foundation_level_cla = True
                    # break

        return foundation_level_cla

    def get_project_sfid(self) -> str:
        return self.model.project_sfid

    def get_project_name(self) -> str:
        return self.model.project_name

    def get_foundation_sfid(self) -> str:
        return self.model.foundation_sfid

    def get_foundation_name(self) -> str:
        return self.model.foundation_name

    def get_cla_group_id(self) -> str:
        return self.model.cla_group_id

    def get_cla_group_name(self) -> str:
        return self.model.cla_group_name

    def get_repositories_count(self) -> int:
        return self.model.repositories_count

    def get_note(self) -> str:
        return self.model.note

    def get_version(self) -> str:
        return self.model.version

    def set_project_sfid(self, project_sfid):
        self.model.project_sfid = project_sfid

    def set_project_name(self, project_name):
        self.model.project_name = project_name

    def set_foundation_sfid(self, foundation_sfid):
        self.model.foundation_sfid = foundation_sfid

    def set_foundation_name(self, foundation_name):
        self.model.foundation_name = foundation_name

    def set_cla_group_id(self, cla_group_id):
        self.model.cla_group_id = cla_group_id

    def set_cla_group_name(self, cla_group_name):
        self.model.cla_group_name = cla_group_name

    def set_repositories_count(self, repositories_count):
        self.model.repositories_count = repositories_count

    def set_note(self, note):
        self.model.note = note

    def set_date_modified(self, date_modified):
        self.model.date_modified = date_modified

    def get_by_foundation_sfid(self, foundation_sfid) -> List[ProjectCLAGroup]:
        project_cla_groups = ProjectCLAGroupModel.foundation_sfid_index.query(foundation_sfid)
        ret = []
        for project_cla_group in project_cla_groups:
            proj_cla_group = ProjectCLAGroup()
            proj_cla_group.model = project_cla_group
            ret.append(proj_cla_group)
        return ret

    def get_by_cla_group_id(self, cla_group_id) -> List[ProjectCLAGroup]:
        project_cla_groups = ProjectCLAGroupModel.cla_group_id_index.query(cla_group_id)
        ret = []
        for project_cla_group in project_cla_groups:
            proj_cla_group = ProjectCLAGroup()
            proj_cla_group.model = project_cla_group
            ret.append(proj_cla_group)
        return ret

    def all(self, project_sfids=None) -> List[ProjectCLAGroup]:
        if project_sfids is None:
            project_cla_groups = self.model.scan()
        else:
            project_cla_groups = ProjectCLAGroupModel.batch_get(project_sfids)
        ret = []
        for project_cla_group in project_cla_groups:
            proj_cla_group = ProjectCLAGroup()
            proj_cla_group.model = project_cla_group
            ret.append(proj_cla_group)
        return ret


class CompanyModel(BaseModel):
    """
    Represents an company in the database.
    """

    class Meta:
        """Meta class for Company."""

        table_name = "cla-{}-companies".format(stage)
        if stage == "local":
            host = "http://localhost:8000"

    company_id = UnicodeAttribute(hash_key=True)
    company_external_id = UnicodeAttribute(null=True)
    company_manager_id = UnicodeAttribute(null=True)
    company_name = UnicodeAttribute(null=True)  # parent
    signing_entity_name = UnicodeAttribute(null=True)  # also the parent name or could be alternative name
    company_name_index = CompanyNameIndex()
    signing_entity_name_index = SigningEntityNameIndex()
    company_external_id_index = ExternalCompanyIndex()
    company_acl = PatchedUnicodeSetAttribute(default=set)
    note = UnicodeAttribute(null=True)
    is_sanctioned = BooleanAttribute(default=False, null=True)


class Company(model_interfaces.Company):  # pylint: disable=too-many-public-methods
    """
    ORM-agnostic wrapper for the DynamoDB Company model.
    """

    def __init__(
            self,  # pylint: disable=too-many-arguments
            company_id=None,
            company_external_id=None,
            company_manager_id=None,
            company_name=None,
            signing_entity_name=None,
            company_acl=set(),
            note=None,
            is_sanctioned=False,
    ):
        super(Company).__init__()

        self.model = CompanyModel()
        self.model.company_id = company_id
        self.model.company_external_id = company_external_id
        self.model.company_manager_id = company_manager_id
        self.model.company_name = company_name
        if signing_entity_name:
            self.model.signing_entity_name = signing_entity_name
        else:
            self.model.signing_entity_name = company_name
        self.model.company_acl = company_acl
        self.model.note = note
        self.model.is_sanctioned = is_sanctioned

    def __str__(self) -> str:
        return (
            f"id:{self.model.company_id}, "
            f"name: {self.model.company_name}, "
            f"signing_entity_name: {self.model.signing_entity_name}, "
            f"external id: {self.model.company_external_id}, "
            f"manager id: {self.model.company_manager_id}, "
            f"is_sanctioned: {self.model.is_sanctioned}, "
            f"acl: {self.model.company_acl}, "
            f"note: {self.model.note}"
        )

    def to_dict(self) -> dict:
        return dict(self.model)

    def save(self) -> None:
        self.model.date_modified = datetime.datetime.utcnow()
        self.model.save()

    def load(self, company_id: str) -> None:
        try:
            company = self.model.get(str(company_id))
        except CompanyModel.DoesNotExist:
            raise cla.models.DoesNotExist("Company not found")
        self.model = company

    def load_company_by_name(self, company_name: str) -> Optional[DoesNotExist]:
        try:
            company_generator = self.model.company_name_index.query(company_name)
            for company_model in company_generator:
                self.model = company_model
                return
            # Didn't find a result - throw an error
            raise cla.models.DoesNotExist(f'Company with name {company_name} not found')
        except CompanyModel.DoesNotExist:
            raise cla.models.DoesNotExist(f'Company with name {company_name} not found')

    def delete(self) -> None:
        self.model.delete()

    def get_company_id(self) -> str:
        return self.model.company_id

    def get_company_external_id(self) -> str:
        return self.model.company_external_id

    def get_company_manager_id(self) -> str:
        return self.model.company_manager_id

    def get_company_name(self) -> str:
        return self.model.company_name

    def get_signing_entity_name(self) -> str:
        # if self.model.signing_entity_name is None:
        #    return self.model.company_name
        return self.model.signing_entity_name

    def get_company_acl(self) -> Optional[List[str]]:
        return self.model.company_acl

    def get_note(self) -> str:
        return self.model.note

    def get_is_sanctioned(self):
        return self.model.is_sanctioned

    def set_company_id(self, company_id: str) -> None:
        self.model.company_id = company_id

    def set_company_external_id(self, company_external_id: str) -> None:
        self.model.company_external_id = company_external_id

    def set_company_manager_id(self, company_manager_id: str) -> None:
        self.model.company_manager_id = company_manager_id

    def set_company_name(self, company_name: str) -> None:
        self.model.company_name = str(company_name)

    def set_signing_entity_name(self, signing_entity_name: str) -> None:
        self.model.signing_entity_name = signing_entity_name

    def set_company_acl(self, company_acl_username: str) -> None:
        self.model.company_acl = set([company_acl_username])

    def set_note(self, note: str) -> None:
        self.model.note = note

    def set_is_sanctioned(self, is_sanctioned) -> None:
        self.model.is_sanctioned = bool(is_sanctioned)

    def update_note(self, note: str) -> None:
        if self.model.note:
            self.model.note = self.model.note + ' ' + note
        else:
            self.model.note = note

    def set_date_modified(self) -> None:
        """
        Updates the company modified date/time to the current time.
        """
        self.model.date_modified = datetime.datetime.now()

    def add_company_acl(self, username: str) -> None:
        self.model.company_acl.add(username)

    def remove_company_acl(self, username: str) -> None:
        if username in self.model.company_acl:
            self.model.company_acl.remove(username)

    def get_managers(self) -> List[User]:
        return self.get_managers_by_company_acl(self.get_company_acl())

    def get_company_signatures(self, project_id: str = None, signature_signed: bool = None,
                               signature_approved: bool = None) -> Optional[List[Signature]]:
        return Signature().get_signatures_by_reference(
            self.get_company_id(),
            "company",
            project_id=project_id,
            signature_approved=signature_approved,
            signature_signed=signature_signed,
        )

    def get_latest_signature(self, project_id: str, signature_signed: bool = None,
                             signature_approved: bool = None) -> Optional[Signature]:
        """
        Helper function to get a company's latest signature for a project.

        :param project_id: The ID of the project to check for.
        :type project_id: string
        :param signature_signed: The signature signed flag
        :type signature_signed: bool
        :param signature_approved: The signature approved flag
        :type signature_approved: bool
        :return: The latest versioned signature object if it exists.
        :rtype: cla.models.model_interfaces.Signature or None
        """
        cla.log.debug(f"locating latest signature - project_id={project_id}, "
                      f"signature_signed={signature_signed}, "
                      f"signature_approved={signature_approved}")
        signatures = self.get_company_signatures(
            project_id=project_id, signature_signed=signature_signed, signature_approved=signature_approved)
        latest = None
        cla.log.debug(f"retrieved {len(signatures)}")
        for signature in signatures:
            if latest is None:
                latest = signature
            elif signature.get_signature_document_major_version() > latest.get_signature_document_major_version():
                latest = signature
            elif (
                    signature.get_signature_document_major_version() == latest.get_signature_document_major_version()
                    and signature.get_signature_document_minor_version() > latest.get_signature_document_minor_version()
            ):
                latest = signature

        return latest

    def get_company_by_id(self, company_id: str):
        companies = self.model.scan()
        for company in companies:
            org = Company()
            org.model = company
            if org.model.company_id == company_id:
                return org
        return None

    def get_company_by_external_id(self, company_external_id: str):
        company_generator = self.model.company_external_id_index.query(company_external_id)
        companies = []
        for company_model in company_generator:
            company = Company()
            company.model = company_model
            companies.append(company)
        return companies

    def all(self, ids: List[str] = None):
        if ids is None:
            companies = self.model.scan()
        else:
            companies = CompanyModel.batch_get(ids)
        ret = []
        for company in companies:
            org = Company()
            org.model = company
            ret.append(org)
        return ret

    def get_companies_by_manager(self, manager_id: str):
        company_generator = self.model.scan(company_manager_id__eq=str(manager_id))
        companies = []
        for company_model in company_generator:
            company = Company()
            company.model = company_model
            companies.append(company)
        companies_dict = [company_model.to_dict() for company_model in companies]
        return companies_dict

    def get_managers_by_company_acl(self, company_acl: List[str]) -> Optional[List[User]]:
        managers = []
        user_model = User()
        for username in company_acl:
            users = user_model.get_user_by_username(str(username))
            if len(users) > 1:
                cla.log.warning(f"More than one user record returned for username: {username}")
            if users is not None:
                managers.append(users[0])
        return managers


class StoreModel(Model):
    """
    Represents a key-value store in a DynamoDB.
    """

    class Meta:
        """Meta class for Store."""

        table_name = "cla-{}-store".format(stage)
        if stage == "local":
            host = "http://localhost:8000"
        write_capacity_units = int(cla.conf["DYNAMO_WRITE_UNITS"])
        read_capacity_units = int(cla.conf["DYNAMO_READ_UNITS"])

    key = UnicodeAttribute(hash_key=True)
    value = JSONAttribute(null=True)
    expire = NumberAttribute(null=True)


class Store(key_value_store_interface.KeyValueStore):
    """
    ORM-agnostic wrapper for the DynamoDB key-value store model.
    """

    def __init__(self):
        super(Store).__init__()

    def set(self, key, value):
        model = StoreModel()
        model.key = key
        model.value = value
        model.expire = self.get_expire_timestamp()
        model.save()

    def get(self, key):
        import json
        model = StoreModel()
        try:
            val = model.get(key).value
            if isinstance(val, dict):
                val = json.dumps(val)
            return val
        except StoreModel.DoesNotExist:
            raise cla.models.DoesNotExist("Key not found")

    def delete(self, key):
        model = StoreModel()
        model.key = key
        model.delete()

    def exists(self, key):
        # May want to find a better way. Maybe using model.count()?
        try:
            self.get(key)
            return True
        except cla.models.DoesNotExist:
            return False

    def get_expire_timestamp(self):
        # helper function to set store item ttl: 45 minutes
        exp_datetime = datetime.datetime.now() + datetime.timedelta(minutes=45)
        return exp_datetime.timestamp()


class GitlabOrgModel(BaseModel):
    """
    Represents a Gitlab Organization in the database.
    """

    class Meta:
        table_name = "cla-{}-gitlab-orgs".format(stage)
        if stage == "local":
            host = "http://localhost:8000"

    organization_id = UnicodeAttribute(hash_key=True)
    organization_name = UnicodeAttribute(null=True)
    organization_url = UnicodeAttribute(null=True)
    organization_name_lower = UnicodeAttribute(null=True)
    organization_sfid = UnicodeAttribute(null=True)
    external_gitlab_group_id = NumberAttribute(null=True)
    project_sfid = UnicodeAttribute(null=True)
    auth_info = UnicodeAttribute(null=True)
    organization_sfid_index = GitlabOrgSFIndex()
    project_sfid_organization_name_index = GitlabOrgProjectSfidOrganizationNameIndex()
    organization_name_lower_index = GitlabOrganizationNameLowerIndex()
    gitlab_external_group_id_index = GitlabExternalGroupIDIndex()
    auto_enabled = BooleanAttribute(null=True)
    auto_enabled_cla_group_id = UnicodeAttribute(null=True)
    branch_protection_enabled = BooleanAttribute(null=True)
    enabled = BooleanAttribute(null=True)
    note = UnicodeAttribute(null=True)


class GitHubOrgModel(BaseModel):
    """
    Represents a Gitlab Organization in the database.
    """

    class Meta:
        """Meta class for User."""

        table_name = "cla-{}-github-orgs".format(stage)
        if stage == "local":
            host = "http://localhost:8000"

    organization_name = UnicodeAttribute(hash_key=True)
    organization_name_lower = UnicodeAttribute(null=True)
    organization_installation_id = NumberAttribute(null=True)
    organization_sfid = UnicodeAttribute(null=True)
    project_sfid = UnicodeAttribute(null=True)
    organization_sfid_index = GitlabOrgSFIndex()
    project_sfid_organization_name_index = GitlabOrgProjectSfidOrganizationNameIndex()
    organization_name_lower_index = GitlabOrganizationNameLowerIndex()
    organization_name_lower_search_index = OrganizationNameLowerSearchIndex()
    organization_project_id = UnicodeAttribute(null=True)
    organization_company_id = UnicodeAttribute(null=True)
    auto_enabled = BooleanAttribute(null=True)
    branch_protection_enabled = BooleanAttribute(null=True)
    enabled = BooleanAttribute(null=True)
    note = UnicodeAttribute(null=True)
    skip_cla = MapAttribute(of=UnicodeAttribute, null=True)


class GitHubOrg(model_interfaces.GitHubOrg):  # pylint: disable=too-many-public-methods
    """
    ORM-agnostic wrapper for the DynamoDB GitHubOrg model.
    """

    def __init__(
            self, organization_name=None, organization_installation_id=None, organization_sfid=None,
            auto_enabled=False, branch_protection_enabled=False, note=None, enabled=True, skip_cla=None,
    ):
        super(GitHubOrg).__init__()
        self.model = GitHubOrgModel()
        self.model.organization_name = organization_name
        if self.model.organization_name:
            self.model.organization_name_lower = self.model.organization_name.lower()
        self.model.organization_installation_id = organization_installation_id
        self.model.organization_sfid = organization_sfid
        self.model.auto_enabled = auto_enabled
        self.model.branch_protection_enabled = branch_protection_enabled
        self.model.note = note
        self.model.enabled = enabled
        self.model.skip_cla = skip_cla

    def __str__(self):
        return (
            f'organization id:{self.model.organization_name}, '
            f'organization installation id: {self.model.organization_installation_id}, '
            f'organization SFID: {self.model.organization_sfid}, '
            f'organization project id: {self.model.organization_project_id}, '
            f'organization company id: {self.model.organization_company_id}, '
            f'auto_enabled: {self.model.auto_enabled},'
            f'branch_protection_enabled: {self.model.branch_protection_enabled},'
            f'note: {self.model.note},'
            f'enabled: {self.model.enabled},'
            f'skip_cla: {self.model.skip_cla}'
        )

    def to_dict(self):
        ret = dict(self.model)
        if ret["organization_installation_id"] == "null":
            ret["organization_installation_id"] = None
        if ret["organization_sfid"] == "null":
            ret["organization_sfid"] = None
        return ret

    def save(self) -> None:
        self.model.date_modified = datetime.datetime.utcnow()
        self.model.save()

    def load(self, organization_name):
        try:
            organization = self.model.get(str(organization_name))
        except GitHubOrgModel.DoesNotExist:
            raise cla.models.DoesNotExist("GitHub Org not found")
        self.model = organization

    def delete(self):
        self.model.delete()

    def get_organization_name(self):
        return self.model.organization_name

    def get_organization_installation_id(self):
        return self.model.organization_installation_id

    def get_organization_sfid(self):
        return self.model.organization_sfid

    def get_project_sfid(self):
        return self.model.project_sfid

    def get_organization_name_lower(self):
        return self.model.organization_name_lower

    def get_auto_enabled(self):
        return self.model.auto_enabled

    def get_branch_protection_enabled(self):
        return self.model.branch_protection_enabled

    def get_skip_cla(self):
        return self.model.skip_cla

    def get_note(self):
        """
        Getter for the note.
        :return: the note value for the github organization record
        :rtype: str
        """
        return self.model.note

    def get_enabled(self):
        return self.model.enabled

    def set_organization_name(self, organization_name):
        self.model.organization_name = organization_name
        if self.model.organization_name:
            self.model.organization_name_lower = self.model.organization_name.lower()

    def set_organization_installation_id(self, organization_installation_id):
        self.model.organization_installation_id = organization_installation_id

    def set_organization_project_id(self, organization_project_id):
        self.model.organization_project_id = organization_project_id

    def set_organization_sfid(self, organization_sfid):
        self.model.organization_sfid = organization_sfid

    def set_project_sfid(self, project_sfid):
        self.model.project_sfid = project_sfid

    def set_organization_name_lower(self, organization_name_lower):
        self.model.organization_name_lower = organization_name_lower

    def set_auto_enabled(self, auto_enabled):
        self.model.auto_enabled = auto_enabled

    def set_branch_protection_enabled(self, branch_protection_enabled):
        self.model.branch_protection_enabled = branch_protection_enabled

    def set_skip_cla(self, skip_cla):
        self.model.skip_cla = skip_cla

    def set_note(self, note):
        self.model.note = note

    def set_enabled(self, enabled):
        self.model.enabled = enabled

    def get_organization_by_sfid(self, sfid) -> List:
        organization_generator = self.model.organization_sfid_index.query(sfid)
        organizations = []
        for org_model in organization_generator:
            org = GitHubOrg()
            org.model = org_model
            organizations.append(org)
        return organizations

    def get_organization_by_installation_id(self, installation_id):
        organization_generator = self.model.scan(organization_installation_id__eq=installation_id)
        for org_model in organization_generator:
            org = GitHubOrg()
            org.model = org_model
            return org
        return None

    def get_organization_by_lower_name(self, organization_name):
        org_generator = self.model.organization_name_lower_search_index.query(organization_name.lower())
        for org_model in org_generator:
            org = GitHubOrg()
            org.model = org_model
            return org
        return None

    def all(self):
        orgs = self.model.scan()
        ret = []
        for organization in orgs:
            org = GitHubOrg()
            org.model = organization
            ret.append(org)
        return ret


class GitlabOrg(model_interfaces.GitlabOrg):  # pylint: disable=too-many-public-methods
    """
    ORM-agnostic wrapper for the DynamoDB GitlabOrg model.
    """

    def __init__(
            self, organization_id=None, organization_name=None, organization_sfid=None, auth_info=None,
            project_sfid=None, auto_enabled=False, branch_protection_enabled=False, note=None, enabled=True
    ):
        super(GitlabOrg).__init__()
        self.model = GitlabOrgModel()
        if not organization_id:
            organization_id = str(uuid.uuid4())
        self.model.organization_id = organization_id

        self.model.organization_name = organization_name
        if self.model.organization_name:
            self.model.organization_name_lower = self.model.organization_name.lower()

        self.model.organization_sfid = organization_sfid
        self.model.project_sfid = project_sfid
        self.model.auto_enabled = auto_enabled
        self.model.branch_protection_enabled = branch_protection_enabled
        self.model.enabled = enabled
        self.model.note = note
        self.model.auth_info = auth_info

    def __str__(self):
        return (
            f'organization id:{self.model.organization_id}, '
            f'organization name:{self.model.organization_name}, '
            f'organization url : {self.model.organization_url}, '
            f'organization SFID: {self.model.organization_sfid}, '
            f'auto_enabled: {self.model.auto_enabled},'
            f'branch_protection_enabled: {self.model.branch_protection_enabled},'
            f'enabled: {self.model.enabled},'
            f'note: {self.model.note}',
            f'auth_info: {self.model.auth_info}'
            f'external_gitlab_group_id: {self.model.external_gitlab_group_id}'
        )

    def to_dict(self):
        ret = dict(self.model)
        if ret["organization_sfid"] == "null":
            ret["organization_sfid"] = None
        return ret

    def save(self) -> None:
        self.model.date_modified = datetime.datetime.utcnow()
        self.model.save()

    def load(self, organization_id: str):
        try:
            organization = self.model.get(organization_id)
        except GitlabOrgModel.DoesNotExist:
            raise cla.models.DoesNotExist("Gitlab Org not found")
        self.model = organization

    def delete(self):
        self.model.delete()

    def get_external_gitlab_group_id(self):
        return self.model.external_gitlab_group_id

    def get_organization_id(self):
        return self.model.organization_id

    def get_organization_url(self):
        return self.model.organization_url

    def get_organization_name(self):
        return self.model.organization_name

    def get_organization_sfid(self):
        return self.model.organization_sfid

    def get_project_sfid(self):
        return self.model.project_sfid

    def get_organization_name_lower(self):
        return self.model.organization_name_lower

    def get_auto_enabled(self):
        return self.model.auto_enabled

    def get_branch_protection_enabled(self):
        return self.model.branch_protection_enabled

    def get_note(self):
        """
        Getter for the note.
        :return: the note value for the github organization record
        :rtype: str
        """
        return self.model.note

    def get_auth_info(self):
        return self.model.auth_info

    def get_enabled(self):
        return self.model.enabled

    def set_external_gitlab_group_id(self, external_gitlab_group_id):
        self.model.external_gitlab_group_id = external_gitlab_group_id

    def set_organization_name(self, organization_name):
        self.model.organization_name = organization_name
        if self.model.organization_name:
            self.model.organization_name_lower = self.model.organization_name.lower()

    def set_organization_url(self, organization_url):
        self.model.organization_url = organization_url

    def set_organization_sfid(self, organization_sfid):
        self.model.organization_sfid = organization_sfid

    def set_project_sfid(self, project_sfid):
        self.model.project_sfid = project_sfid

    def set_organization_name_lower(self, organization_name_lower):
        self.model.organization_name_lower = organization_name_lower

    def set_auto_enabled(self, auto_enabled):
        self.model.auto_enabled = auto_enabled

    def set_branch_protection_enabled(self, branch_protection_enabled):
        self.model.branch_protection_enabled = branch_protection_enabled

    def set_note(self, note):
        self.model.note = note

    def set_enabled(self, enabled):
        self.model.enabled = enabled

    def set_auth_info(self, auth_info):
        self.model.auth_info = auth_info

    def get_organization_by_groupid(self, groupid):
        org_generator = self.model.gitlab_external_group_id_index.query(groupid)
        for org_model in org_generator:
            org = GitlabOrg()
            org.model = org_model
            return org
        return None

    def get_organization_by_sfid(self, sfid) -> List:
        organization_generator = self.model.organization_sfid_index.query(sfid)
        organizations = []
        for org_model in organization_generator:
            org = GitlabOrg()
            org.model = org_model
            organizations.append(org)
        return organizations

    def search_organization_by_lower_name(self, organization_name):
        organizations = list(
            filter(lambda org: org.get_organization_name_lower() == organization_name.lower(), self.all()))
        if organizations:
            return organizations[0]
        raise cla.models.DoesNotExist(f"Gitlab Org : {organization_name} does not exist")

    def search_organization_by_group_url(self, group_url):
        # first check for match.. could be in the format https://gitlab.com/groups/<group_name>
        groups = self.all()
        organizations = list(filter(lambda org: org.get_organization_url() == group_url.strip(), groups))
        if organizations:
            return organizations[0]
        # also cater for potentially missing groups in url
        pattern = re.compile(r"(?P<base>\bhttps://gitlab.com/\b)(?P<group>\bgroups\/\b)?(?P<name>\w+)")
        match = pattern.search(group_url)
        updated_url = ''
        if match and not match.group('group'):
            cla.log.debug(f'{group_url} missing groups in url. Inserting groups to url ')
            parse_url_list = list(match.groups())
            parse_url_list[1] = 'groups/'
            updated_url = ''.join(parse_url_list)
        if updated_url:
            cla.log.debug(f'Updated group_url to : {updated_url}')
            organizations = list(filter(lambda org: org.get_organization_url() == updated_url.strip(), groups))
            if organizations:
                return organizations[0]

        raise cla.models.DoesNotExist(f"Gitlab Org : {group_url} does not exist")

    def get_organization_by_lower_name(self, organization_name):
        organization_name = organization_name.lower()
        organization_generator = self.model.organization_name_lower_index.query(organization_name)
        organizations = []
        for org_model in organization_generator:
            org = GitlabOrg()
            org.model = org_model
            organizations.append(org)
        return organizations

    def all(self):
        orgs = self.model.scan()
        ret = []
        for organization in orgs:
            org = GitlabOrg()
            org.model = organization
            ret.append(org)
        return ret


class GerritModel(BaseModel):
    """
    Represents a Gerrit Instance in the database.
    """

    class Meta:
        """Meta class for User."""

        table_name = "cla-{}-gerrit-instances".format(stage)
        if stage == "local":
            host = "http://localhost:8000"

    gerrit_id = UnicodeAttribute(hash_key=True)
    project_id = UnicodeAttribute(null=True)
    gerrit_name = UnicodeAttribute(null=True)
    gerrit_url = UnicodeAttribute(null=True)
    group_id_icla = UnicodeAttribute(null=True)
    group_id_ccla = UnicodeAttribute(null=True)
    group_name_icla = UnicodeAttribute(null=True)
    group_name_ccla = UnicodeAttribute(null=True)
    project_sfid = UnicodeAttribute(null=True)
    project_id_index = GerritProjectIDIndex()
    project_sfid_index = GerritProjectSFIDIndex()


class Gerrit(model_interfaces.Gerrit):  # pylint: disable=too-many-public-methods
    """
    ORM-agnostic wrapper for the DynamoDB Gerrit model.
    """

    def __init__(
            self,
            gerrit_id=None,
            gerrit_name=None,
            project_id=None,
            gerrit_url=None,
            group_id_icla=None,
            group_id_ccla=None,
    ):
        super(Gerrit).__init__()
        self.model = GerritModel()
        self.model.gerrit_id = gerrit_id
        self.model.gerrit_name = gerrit_name
        self.model.project_id = project_id
        self.model.gerrit_url = gerrit_url
        self.model.group_id_icla = group_id_icla
        self.model.group_id_ccla = group_id_ccla

    def __str__(self):
        return (
            f"gerrit_id:{self.model.gerrit_id}, "
            f"gerrit_name:{self.model.gerrit_name}, "
            f"project_id:{self.model.project_id}, "
            f"gerrit_url:{self.model.gerrit_url}, "
            f"group_id_icla: {self.model.group_id_icla}, "
            f"group_id_ccla: {self.model.group_id_ccla}, "
            f"date_created: {self.model.date_created}, "
            f"date_modified: {self.model.date_modified}, "
            f"version: {self.model.version}"
        )

    def to_dict(self):
        ret = dict(self.model)
        return ret

    def load(self, gerrit_id):
        try:
            gerrit = self.model.get(str(gerrit_id))
        except GerritModel.DoesNotExist:
            raise cla.models.DoesNotExist("Gerrit Instance not found")
        self.model = gerrit

    def get_gerrit_id(self):
        return self.model.gerrit_id

    def get_project_sfid(self):
        return self.model.project_sfid

    def get_gerrit_name(self):
        return self.model.gerrit_name

    def get_project_id(self):
        return self.model.project_id

    def get_gerrit_url(self):
        return self.model.gerrit_url

    def get_group_id_icla(self):
        return self.model.group_id_icla

    def get_group_id_ccla(self):
        return self.model.group_id_ccla

    def set_project_sfid(self, project_sfid):
        self.model.project_sfid = str(project_sfid)

    def set_gerrit_id(self, gerrit_id):
        self.model.gerrit_id = gerrit_id

    def set_gerrit_name(self, gerrit_name):
        self.model.gerrit_name = gerrit_name

    def set_project_id(self, project_id):
        self.model.project_id = project_id

    def set_gerrit_url(self, gerrit_url):
        self.model.gerrit_url = gerrit_url

    def set_group_id_icla(self, group_id_icla):
        self.model.group_id_icla = group_id_icla

    def set_group_id_ccla(self, group_id_ccla):
        self.model.group_id_ccla = group_id_ccla

    def set_group_name_icla(self, group_name_icla):
        self.model.group_name_icla = group_name_icla

    def set_group_name_ccla(self, group_name_ccla):
        self.model.group_name_ccla = group_name_ccla

    def save(self) -> None:
        self.model.date_modified = datetime.datetime.utcnow()
        self.model.save()

    def delete(self):
        self.model.delete()

    def get_gerrit_by_project_id(self, project_id) -> List[Gerrit]:
        gerrit_generator = self.model.project_id_index.query(project_id)
        gerrits = []
        for gerrit_model in gerrit_generator:
            gerrit = Gerrit()
            gerrit.model = gerrit_model
            gerrits.append(gerrit)
        if len(gerrits) >= 1:
            return gerrits
        else:
            raise cla.models.DoesNotExist("Gerrit instance does not exist")

    def get_gerrit_by_project_sfid(self, project_sfid) -> List[Gerrit]:
        gerrit_generator = self.model.project_sfid_index.query(project_sfid)
        gerrits = []
        for gerrit_model in gerrit_generator:
            gerrit = Gerrit()
            gerrit.model = gerrit_model
            gerrits.append(gerrit)
        if len(gerrits) >= 1:
            return gerrits
        else:
            raise cla.models.DoesNotExist("Gerrit instance does not exist")

    def all(self):
        gerrits = self.model.scan()
        ret = []
        for gerrit_model in gerrits:
            gerrit = Gerrit()
            gerrit.model = gerrit_model
            ret.append(gerrit)
        return ret


class CLAManagerRequests(BaseModel):
    """
    Represents CLA Manager Requests in the database
    """

    class Meta:
        """
        Meta class for CLA Manager Requests.
        """
        table_name = "cla-{}-cla-manager-requests".format(stage)
        if stage == "local":
            host = "http://localhost:8000"

    request_id = UnicodeAttribute(hash_key=True)
    company_id = UnicodeAttribute(null=True)
    company_external_id = UnicodeAttribute(null=True)
    company_name = UnicodeAttribute(null=True)
    project_id = UnicodeAttribute(null=True)
    project_external_id = UnicodeAttribute(null=True)
    project_name = UnicodeAttribute(null=True)
    user_id = UnicodeAttribute(null=True)
    user_external_id = UnicodeAttribute(null=True)
    user_name = UnicodeAttribute(null=True)
    user_email = UnicodeAttribute(null=True)
    status = UnicodeAttribute(null=True)


class CLAManagerRequest(model_interfaces.CLAManagerRequest):  # pylint: disable=too-many-public-methods
    """
    ORM-agnostic wrapper for the DynamoDB CLAManagerRequest model.
    """

    def __init__(
            self,
            request_id=None,
            company_id=None,
            company_external_id=None,
            company_name=None,
            project_id=None,
            project_external_id=None,
            project_name=None,
            user_id=None,
            user_external_id=None,
            user_name=None,
            user_email=None,
            status=None,
    ):
        super(CLAManagerRequest).__init__()
        self.model = CLAManagerRequests()
        self.model.request_id = request_id
        self.model.company_id = company_id
        self.model.company_external_id = company_external_id
        self.model.company_name = company_name
        self.model.project_id = project_id
        self.model.project_external_id = project_external_id
        self.model.project_name = project_name
        self.model.user_id = user_id
        self.model.user_external_id = user_external_id
        self.model.user_name = user_name
        self.model.user_email = user_email
        self.model.status = status

    def __str__(self):
        return (
            f"request_id:{self.model.request_id}, "
            f"company_id:{self.model.company_id}, "
            f"company_external_id:{self.model.company_external_id}, "
            f"company_name:{self.model.company_name}, "
            f"project_id: {self.model.project_id}, "
            f"project_external_id: {self.model.project_external_id}, "
            f"project_name: {self.model.project_name}, "
            f"user_id: {self.model.user_id}, "
            f"user_external_id: {self.model.user_external_id},"
            f"user_name: {self.model.user_name},"
            f"user_email: {self.model.user_email},"
            f"status: {self.model.status}"
        )

    def to_dict(self):
        ret = dict(self.model)
        return ret

    def load(self, request_id):
        try:
            cla_manager_request = self.model.get(str(request_id))
        except CLAManagerRequests.DoesNotExist:
            raise cla.models.DoesNotExist("CLA Manager Request Instance not found")
        self.model = cla_manager_request

    def get_request_id(self):
        return self.model.request_id

    def get_company_id(self):
        return self.model.company_id

    def get_company_external_id(self):
        return self.model.company_external_id

    def get_company_name(self):
        return self.model.company_name

    def get_project_id(self):
        return self.model.project_id

    def get_project_external_id(self):
        return self.model.project_external_id

    def get_project_name(self):
        return self.model.project_name

    def get_user_id(self):
        return self.model.user_id

    def get_user_external_id(self):
        return self.model.user_external_id

    def get_user_name(self):
        return self.model.user_name

    def get_user_email(self):
        return self.model.user_email

    def get_status(self):
        return self.model.status

    def set_request_id(self, request_id):
        self.model.request_id = request_id

    def set_company_id(self, company_id):
        self.model.company_id = company_id

    def set_company_external_id(self, company_external_id):
        self.model.company_external_id = company_external_id

    def set_company_name(self, company_name):
        self.model.company_name = company_name

    def set_project_id(self, project_id):
        self.model.project_id = project_id

    def set_project_external_id(self, project_external_id):
        self.model.project_external_id = project_external_id

    def set_project_name(self, project_name):
        self.model.project_name = project_name

    def set_user_id(self, user_id):
        self.model.user_id = user_id

    def set_user_external_id(self, user_external_id):
        self.model.user_external_id = user_external_id

    def set_user_name(self, user_name):
        self.model.user_name = user_name

    def set_user_email(self, user_email):
        self.model.user_email = user_email

    def set_status(self, status):
        self.model.status = status

    def save(self) -> None:
        self.model.date_modified = datetime.datetime.utcnow()
        self.model.save()

    def delete(self):
        self.model.delete()

    def all(self):
        cla_manager_requests = self.model.scan()
        ret = []
        for cla_manager_request in cla_manager_requests:
            manager_request = CLAManagerRequest()
            manager_request.model = cla_manager_request
            ret.append(manager_request)
        return ret


class UserPermissionsModel(BaseModel):
    """
    Represents user permissions in the database.
    """

    class Meta:
        """Meta class for User Permissions."""

        table_name = "cla-{}-user-permissions".format(stage)
        if stage == "local":
            host = "http://localhost:8000"

    username = UnicodeAttribute(hash_key=True)
    projects = PatchedUnicodeSetAttribute(default=set)


class UserPermissions(model_interfaces.UserPermissions):  # pylint: disable=too-many-public-methods
    """
    ORM-agnostic wrapper for the DynamoDB UserPermissions model.
    """

    def __init__(self, username=None, projects=set()):
        super(UserPermissions).__init__()
        self.model = UserPermissionsModel()
        self.model.username = username
        if projects is not None:
            self.model.projects = set(projects)

    def add_project(self, project_id: str):
        if self.model is not None and self.model.projects is not None:
            self.model.projects.add(project_id)

    def remove_project(self, project_id: str):
        if project_id in self.model.projects:
            self.model.projects.remove(project_id)

    def has_permission(self, project_id: str):
        return project_id in self.model.projects

    def to_dict(self):
        ret = dict(self.model)
        return ret

    def save(self) -> None:
        self.model.date_modified = datetime.datetime.utcnow()
        self.model.save()

    def load(self, username):
        try:
            user_permissions = self.model.get(str(username))
        except UserPermissionsModel.DoesNotExist:
            raise cla.models.DoesNotExist("User Permissions not found")
        self.model = user_permissions

    def delete(self):
        self.model.delete()

    def all(self):
        user_permissions = self.model.scan()
        ret = []
        for user_permission in user_permissions:
            permission = UserPermissions()
            permission.model = user_permission
            ret.append(permission)
        return ret

    def get_username(self):
        return self.model.username


class CompanyInviteModel(BaseModel):
    """
    Represents company invites in the database.

    Note that this model is utilized in the Go backend from the 'accesslist' package.
    """

    class Meta:
        table_name = "cla-{}-company-invites".format(stage)
        if stage == "local":
            host = "http://localhost:8000"

    company_invite_id = UnicodeAttribute(hash_key=True)
    user_id = UnicodeAttribute(null=True)
    requested_company_id = UnicodeAttribute(null=True)
    requested_company_id_index = RequestedCompanyIndex()


class CompanyInvite(model_interfaces.CompanyInvite):
    def __init__(self, user_id=None, requested_company_id=None):
        super(CompanyInvite).__init__()
        self.model = CompanyInviteModel()
        self.model.user_id = user_id
        self.model.requested_company_id = requested_company_id

    def to_dict(self):
        ret = dict(self.model)
        return ret

    def load(self, company_invite_id):
        try:
            company_invite = self.model.get(str(company_invite_id))
        except CompanyInviteModel.DoesNotExist:
            raise cla.models.DoesNotExist("Company Invite not found")
        self.model = company_invite

    def set_company_invite_id(self, company_invite_id):
        self.model.company_invite_id = company_invite_id

    def get_company_invite_id(self):
        return self.model.company_invite_id

    def get_user_id(self):
        return self.model.user_id

    def get_requested_company_id(self):
        return self.model.requested_company_id

    def set_user_id(self, user_id):
        self.model.user_id = user_id

    def set_requested_company_id(self, requested_company_id):
        self.model.requested_company_id = requested_company_id

    def get_invites_by_company(self, requested_company_id):
        invites_generator = self.model.requested_company_id_index.query(requested_company_id)
        invites = []
        for invite_model in invites_generator:
            invite = CompanyInvite()
            invite.model = invite_model
            invites.append(invite)
        return invites

    def save(self) -> None:
        self.model.date_modified = datetime.datetime.utcnow()
        self.model.save()

    def delete(self):
        self.model.delete()


class EventModel(BaseModel):
    """
    Represents an event in the database
    """

    class Meta:
        """Meta class for event """

        table_name = "cla-{}-events".format(stage)
        if stage == "local":
            host = "http://localhost:8000"

    event_id = UnicodeAttribute(hash_key=True)
    event_user_id = UnicodeAttribute(null=True)
    event_type = UnicodeAttribute(null=True)

    event_cla_group_id = UnicodeAttribute(null=True)
    event_cla_group_name = UnicodeAttribute(null=True)
    event_cla_group_name_lower = UnicodeAttribute(null=True)

    event_project_id = UnicodeAttribute(null=True)
    event_project_sfid = UnicodeAttribute(null=True)
    event_project_name = UnicodeAttribute(null=True)
    event_project_name_lower = UnicodeAttribute(null=True)
    event_parent_project_sfid = UnicodeAttribute(null=True)
    event_parent_project_name = UnicodeAttribute(null=True)

    event_company_id = UnicodeAttribute(null=True)
    event_company_sfid = UnicodeAttribute(null=True)
    event_company_name = UnicodeAttribute(null=True)
    event_company_name_lower = UnicodeAttribute(null=True)

    event_user_name = UnicodeAttribute(null=True)
    event_user_name_lower = UnicodeAttribute(null=True)

    event_time = DateTimeAttribute(default=datetime.datetime.utcnow())
    event_time_epoch = NumberAttribute(default=int(time.time()))
    event_date = UnicodeAttribute(null=True)

    event_data = UnicodeAttribute(null=True)
    event_data_lower = UnicodeAttribute(null=True)
    event_summary = UnicodeAttribute(null=True)

    event_date_and_contains_pii = UnicodeAttribute(null=True)
    company_id_external_project_id = UnicodeAttribute(null=True)
    contains_pii = BooleanAttribute(null=True)
    user_id_index = EventUserIndex()
    event_type_index = EventTypeIndex()


class Event(model_interfaces.Event):
    """
    ORM-agnostic wrapper for the DynamoDB Event model.
    """

    def __init__(
            self,
            event_id=None,
            event_type=None,
            user_id=None,
            event_cla_group_id=None,
            event_cla_group_name=None,
            event_project_id=None,
            event_company_id=None,
            event_company_sfid=None,
            event_data=None,
            event_summary=None,
            event_company_name=None,
            event_user_name=None,
            event_project_name=None,
            contains_pii=False,
    ):

        super(Event).__init__()
        self.model = EventModel()
        self.model.event_id = event_id
        self.model.event_type = event_type

        self.model.event_user_id = user_id
        self.model.event_user_name = event_user_name
        if self.model.event_user_name:
            self.model.event_user_name_lower = self.model.event_user_name.lower()

        self.model.event_cla_group_id = event_cla_group_id
        self.model.event_cla_group_name = event_cla_group_name
        if self.model.event_cla_group_name:
            self.model.event_cla_group_name_lower = self.model.event_cla_group_name.lower()

        self.model.event_project_id = event_project_id
        self.model.event_project_name = event_project_name
        if self.model.event_project_name:
            self.model.event_project_name_lower = self.model.event_project_name.lower()

        self.model.event_company_id = event_company_id
        self.model.event_company_sfid = event_company_sfid
        self.model.event_company_name = event_company_name
        if self.model.event_company_name:
            self.model.event_company_name_lower = self.model.event_company_name.lower()

        self.model.event_data = event_data
        if self.model.event_data:
            self.model.event_data_lower = self.model.event_data.lower()
        self.model.event_summary = event_summary
        self.model.contains_pii = contains_pii

    def __str__(self):
        return (
            f"id:{self.model.event_id}, "
            f"event type:{self.model.event_type}, "

            f"event_user id:{self.model.event_user_id}, "
            f"event user name: {self.model.event_user_name},"

            f"event cla group id:{self.model.event_cla_group_id}, "
            f"event cla group name:{self.model.event_cla_group_name}, "

            f"event project id:{self.model.event_project_id}, "
            f"event project sfid: {self.model.event_project_sfid},"
            f"event project name: {self.model.event_project_name}, "
            f"event parent project sfid:{self.model.event_parent_project_sfid}, "
            f"event parent project name: {self.model.event_parent_project_name}, "

            f"event company id: {self.model.event_company_id}, "
            f"event company sfid: {self.model.event_company_sfid}, "
            f"event company name: {self.model.event_company_name}, "

            f"event time: {self.model.event_time}, "
            f"event time epoch: {self.model.event_time_epoch}, "
            f"event date: {self.model.event_date},"

            f"event data: {self.model.event_data}, "
            f"event summary: {self.model.event_summary}, "
            f"contains pii: {self.model.contains_pii}"
        )

    def to_dict(self):
        return dict(self.model)

    def save(self) -> None:
        self.model.date_modified = datetime.datetime.utcnow()
        self.model.save()

    def delete(self):
        self.model.delete()

    def load(self, event_id):
        try:
            event = self.model.get(str(event_id))
        except EventModel.DoesNotExist:
            raise cla.models.DoesNotExist("Event not found")
        self.model = event

    def get_event_date_created(self) -> str:
        return self.model.date_created

    def get_event_date_modified(self) -> str:
        return self.model.date_modified

    def get_event_user_id(self) -> str:
        return self.model.event_user_id

    def get_event_data(self) -> str:
        return self.model.event_data

    def get_event_data_lower(self) -> str:
        return self.model.event_data_lower

    def get_event_summary(self) -> str:
        return self.model.event_summary

    def get_event_date(self) -> str:
        return self.model.event_date

    def get_event_id(self) -> str:
        return self.model.event_id

    def get_event_cla_group_id(self) -> str:
        return self.model.event_cla_group_id

    def get_event_cla_group_name(self) -> str:
        return self.model.event_cla_group_name

    def get_event_cla_group_name_lower(self) -> str:
        return self.model.event_cla_group_name_lower

    def get_event_project_id(self) -> str:
        return self.model.event_project_id

    def get_event_project_sfid(self) -> str:
        return self.model.event_project_sfid

    def get_event_project_name(self) -> str:
        return self.model.event_project_name

    def get_event_project_name_lower(self) -> str:
        return self.model.event_project_name_lower

    def get_event_parent_project_sfid(self) -> str:
        return self.model.event_parent_project_sfid

    def get_event_parent_project_name(self) -> str:
        return self.model.event_parent_project_name

    def get_event_type(self) -> str:
        return self.model.event_type

    def get_event_time(self) -> str:
        return self.model.date_created

    def get_event_time_epoch(self) -> int:
        return self.model.event_time_epoch

    def get_event_company_id(self) -> str:
        return self.model.event_company_id

    def get_event_company_sfid(self) -> str:
        return self.model.event_company_sfid

    def get_event_company_name(self) -> str:
        return self.model.event_company_name

    def get_event_company_name_lower(self) -> str:
        return self.model.event_company_name_lower

    def get_event_user_name(self) -> str:
        return self.model.event_user_name

    def get_event_user_name_lower(self) -> str:
        return self.model.event_user_name_lower

    def get_company_id_external_project_id(self) -> str:
        return self.model.company_id_external_project_id

    def all(self, ids=None):
        if ids is None:
            events = self.model.scan()
        else:
            events = EventModel.batch_get(ids)
        ret = []
        for event in events:
            ev = Event()
            ev.model = event
            ret.append(ev)
        return ret

    def all_limit(self, limit: Optional[int] = None, last_evaluated_key: Optional[str] = None):
        result_iterator = self.model.scan(limit=limit, last_evaluated_key=last_evaluated_key)
        ret = []
        for signature in result_iterator:
            evt = Event()
            evt.model = signature
            ret.append(evt)
        return ret, result_iterator.last_evaluated_key, result_iterator.total_count

    def search_missing_event_data_lower(self, limit: Optional[int] = None, last_evaluated_key: Optional[str] = None):
        filter_condition = (EventModel.event_data_lower.does_not_exist())
        projection = ["event_id", "event_data", "event_data_lower"]
        result_iterator = self.model.scan(limit=limit,
                                          last_evaluated_key=last_evaluated_key,
                                          filter_condition=filter_condition,
                                          attributes_to_get=projection)
        ret = []
        for signature in result_iterator:
            evt = Event()
            evt.model = signature
            ret.append(evt)
        return ret, result_iterator.last_evaluated_key, result_iterator.total_count

    # def search_by_year(self, year: str, limit: Optional[int] = None, last_evaluated_key: Optional[str] = None):
    #     filter_condition = (EventModel.event_date.contains(year))
    #     projection = ["event_id", "event_date"]
    #     result_iterator = self.model.scan(limit=limit,
    #                                       last_evaluated_key=last_evaluated_key,
    #                                       filter_condition=filter_condition,
    #                                       attributes_to_get=projection)
    #     ret = []
    #     for signature in result_iterator:
    #         evt = Event()
    #         evt.model = signature
    #         ret.append(evt)
    #     return ret, result_iterator.last_evaluated_key, result_iterator.total_count

    def get_events_type_by_week(self, event_type: EventType) -> dict:
        filter_attributes = {
            "event_type": event_type.name,
        }
        filter_condition = create_filter(filter_attributes, EventModel)
        projection = ["event_id", "event_type", "date_created"]
        cla.log.debug(f'querying events using filter: {filter_condition}...')
        result_iterator = self.model.scan(filter_condition=filter_condition, attributes_to_get=projection)

        ret = {}

        for event_record in result_iterator:
            date_time_value = cla.utils.get_time_from_string(str(event_record.date_created))
            year = date_time_value.year
            week_number = date_time_value.isocalendar()[1]
            cla.log.debug(f'processing events - '
                          f'{event_record.event_id} - '
                          f'{event_record.event_type} - '
                          f'{event_record.date_created} - '
                          f'{year} - {week_number:02d}')
            key = f'{year} {week_number:02d}'
            if key in ret:
                ret[key] += 1
            else:
                ret[key] = 1
        return ret

    def set_event_data(self, event_data: str):
        self.model.event_data = event_data
        self.model.event_data_lower = event_data.lower()

    def set_event_data_lower(self, event_data: str):
        if event_data:
            self.model.event_data_lower = event_data.lower()

    def set_event_summary(self, event_summary: str):
        self.model.event_summary = event_summary

    def set_event_id(self, event_id: str):
        self.model.event_id = event_id

    def set_event_company_id(self, company_id: str):
        self.model.event_company_id = company_id

    def set_event_company_sfid(self, company_sfid: str):
        self.model.event_company_sfid = company_sfid

    def set_event_company_name(self, company_name: str):
        self.model.event_company_name = company_name
        if company_name:
            self.model.event_company_name_lower = company_name.lower()

    def set_event_user_id(self, user_id: str):
        self.model.event_user_id = user_id

    def set_event_cla_group_id(self, event_cla_group_id: str):
        self.model.event_cla_group_id = event_cla_group_id

    def set_event_cla_group_name(self, event_cla_group_name: str):
        self.model.event_cla_group_name = event_cla_group_name
        if event_cla_group_name:
            self.model.event_cla_group_name_lower = event_cla_group_name.lower()

    def set_event_project_id(self, event_project_id: str):
        self.model.event_project_id = event_project_id

    def set_event_project_sfid(self, event_project_sfid: str):
        self.model.event_project_sfid = event_project_sfid

    def set_event_project_name(self, event_project_name: str):
        self.model.event_project_name = event_project_name
        if event_project_name:
            self.model.event_project_name_lower = event_project_name.lower()

    def set_event_parent_project_sfid(self, event_parent_project_sfid: str):
        self.model.event_parent_project_sfid = event_parent_project_sfid

    def set_event_parent_project_name(self, event_parent_project_name: str):
        self.model.event_parent_project_name = event_parent_project_name

    def set_event_type(self, event_type: str):
        self.model.event_type = event_type

    def set_event_user_name(self, event_user_name: str):
        self.model.event_user_name = event_user_name
        self.model.event_user_name_lower = event_user_name.lower()

    def set_event_date_and_contains_pii(self, contains_pii: bool = False):
        dateDDMMYYYY = datetime.date.today().strftime("%d-%m-%Y")
        self.model.contains_pii = contains_pii
        self.model.event_date = dateDDMMYYYY
        self.model.event_date_and_contains_pii = '{}#{}'.format(dateDDMMYYYY, str(contains_pii).lower())

    def set_company_id_external_project_id(self):
        if self.model.event_project_sfid is not None and self.model.event_company_id is not None:
            self.model.company_id_external_project_id = (f'{self.model.event_company_id}'
                                                         f'#{self.model.event_project_sfid}')

    @staticmethod
    def set_cla_group_details(event, cla_group_id: str):
        try:
            project = Project()
            project.load(str(cla_group_id))
            event.set_event_cla_group_id(cla_group_id)
            event.set_event_cla_group_name(project.get_project_name())
            event.set_event_project_sfid(project.get_project_external_id())
            Event.set_project_details(event, project.get_project_external_id())
        except Exception as err:
            cla.log.warning(f'unable to set CLA Group name due to the following error: {err}')

    @staticmethod
    def set_project_details(event, event_project_id: str):
        try:
            sf_project = ProjectService.get_project_by_id(event_project_id)
            if sf_project is not None:
                event.set_event_project_name(sf_project.get("Name"))
                # Does this project have a parent?
                if sf_project.get("Parent") is not None:
                    # Load the parent to get the name
                    Event.set_project_parent_details(event, sf_project.get("Parent"))
        except Exception as err:
            cla.log.warning(f'unable to set project name and parent ID/name '
                            f'due to the following error: {err}')

    @staticmethod
    def set_project_parent_details(event, event_parent_project_id: str):
        sf_project = ProjectService.get_project_by_id(event_parent_project_id)
        if sf_project is not None:
            event.set_event_parent_project_sfid(sf_project.get("ID"))
            event.set_event_parent_project_name(sf_project.get("Name"))

    def search_events(self, **kwargs):
        """
        Function that filters events
        :param **kwargs: query options that is used to filter events
        """

        attributes = [
            'event_id',
            "event_company_id",
            "event_project_id",
            "event_type",
            "event_user_id",
            "event_project_name",
            "event_company_name",
            "event_project_name_lower",
            "event_company_name_lower",
            "event_time",
            "event_time_epoch",
        ]
        filter_condition = None
        for key, value in kwargs.items():
            if key not in attributes:
                continue
            condition = getattr(EventModel, key) == value
            filter_condition = (
                condition if not isinstance(filter_condition, Condition) else filter_condition & condition
            )

        if isinstance(filter_condition, Condition):
            events = self.model.scan(filter_condition)
        else:
            events = self.model.scan()

        ret = []
        for event in events:
            ev = Event()
            ev.model = event
            ret.append(ev)

        return ret

    @classmethod
    def create_event(
            cls,
            event_type: Optional[EventType] = None,
            event_cla_group_id: Optional[str] = None,
            event_project_id: Optional[str] = None,
            event_company_id: Optional[str] = None,
            event_project_name: Optional[str] = None,
            event_company_name: Optional[str] = None,
            event_data: Optional[str] = None,
            event_summary: Optional[str] = None,
            event_user_id: Optional[str] = None,
            event_user_name: Optional[str] = None,
            contains_pii: bool = False,
            dry_run: bool = False
    ):
        """
        Creates an event returns the newly created event in dict format.

        :param event_type: The type of event
        :type event_type: EventType
        :param event_project_id: The project associated with event
        :type event_project_id: string
        :param event_cla_group_id: The CLA Group ID associated with event
        :type event_cla_group_id: string
        :param event_project_name: The project name associated with event
        :type event_project_name: string
        :param event_company_id: The company associated with event
        :type event_company_id: string
        :param event_company_name: The company name associated with event
        :type event_company_name: string
        :param event_data: The event message/data
        :type event_data: string
        :param event_summary: The event summary message/data
        :type event_summary: string
        :param event_user_id: The user that is associated with the event
        :type event_user_id: string
        :param event_user_name: The user's name that is associated with the event
        :type event_user_name: string
        :param contains_pii: flag to indicate if the message contains personal information (deprecated)
        :type contains_pii: bool
        :param dry_run: flag to indicate this is for testing and the record should not be stored/created
        :type dry_run: bool
        """
        try:
            event = cls()
            if event_project_name is None:
                event_project_name = "undefined"
            if event_company_name is None:
                event_company_name = "undefined"

            # Handle case where teh event_project_id == CLA Group ID or SalesForce ID
            if event_project_id and is_uuidv4(event_project_id):  # cla group id in the project_id field
                Event.set_cla_group_details(event, event_project_id)
            elif event_project_id and not is_uuidv4(event_project_id):  # external SFID
                Event.set_project_details(event, event_project_id)

            # if the caller has given us a CLA Group ID
            if event_cla_group_id is not None:  # cla_group_id
                Event.set_cla_group_details(event, event_cla_group_id)

            if event_company_id:
                try:
                    company = Company()
                    company.load(str(event_company_id))
                    event_company_name = company.get_company_name()
                    event.set_event_company_id(event_company_id)
                except DoesNotExist as err:
                    return {"errors": {"event_company_id": str(err)}}

            if event_user_id:
                try:
                    user = User()
                    user.load(str(event_user_id))
                    event.set_event_user_id(event_user_id)
                    user_name = user.get_user_name()
                    if user_name is not None:
                        event.set_event_user_name(user_name)
                except DoesNotExist as err:
                    return {"errors": {"event_": str(err)}}

            if event_user_name:
                event.set_event_user_name(event_user_name)

            event.set_event_id(str(uuid.uuid4()))
            if event_type:
                event.set_event_type(event_type.name)
            event.set_event_project_name(event_project_name)  # potentially overrides the SF Name
            event.set_event_summary(event_summary)
            event.set_event_company_name(event_company_name)
            event.set_event_data(event_data)
            event.set_event_date_and_contains_pii(contains_pii)
            if not dry_run:
                event.save()
            return {"data": event.to_dict()}

        except Exception as err:
            return {"errors": {"event_id": str(err)}}


class CCLAWhitelistRequestModel(BaseModel):
    """
    Represents a CCLAWhitelistRequest in the database
    """

    class Meta:
        """ Meta class for cclawhitelistrequest """

        table_name = "cla-{}-ccla-whitelist-requests".format(stage)
        if stage == "local":
            host = "http://localhost:8000"

    request_id = UnicodeAttribute(hash_key=True)
    company_id = UnicodeAttribute(null=True)
    company_name = UnicodeAttribute(null=True)
    project_id = UnicodeAttribute(null=True)
    project_name = UnicodeAttribute(null=True)
    request_status = UnicodeAttribute(null=True)
    user_emails = PatchedUnicodeSetAttribute(default=set)
    user_id = UnicodeAttribute(null=True)
    user_github_id = UnicodeAttribute(null=True)
    user_github_username = UnicodeAttribute(null=True)
    user_name = UnicodeAttribute(null=True)
    project_external_id = UnicodeAttribute(null=True)
    company_id_project_id_index = CompanyIDProjectIDIndex()


class CCLAWhitelistRequest(model_interfaces.CCLAWhitelistRequest):
    """
    ORM-agnostic wrapper for the DynamoDB CCLAWhitelistRequestModel
    """

    def __init__(
            self,
            request_id=None,
            company_id=None,
            company_name=None,
            project_id=None,
            project_name=None,
            request_status=None,
            user_emails=set(),
            user_id=None,
            user_github_id=None,
            user_github_username=None,
            user_name=None,
            project_external_id=None,
    ):
        super(CCLAWhitelistRequest).__init__()
        self.model = CCLAWhitelistRequestModel()
        self.model.request_id = request_id
        self.model.company_id = company_id
        self.model.company_name = company_name
        self.model.project_id = project_id
        self.model.project_name = project_name
        self.model.request_status = request_status
        self.model.user_emails = user_emails
        self.model.user_id = user_id
        self.model.user_github_id = user_github_id
        self.model.user_github_username = user_github_username
        self.model.user_name = user_name
        self.model.project_external_id = project_external_id

    def __str__(self):
        return (
            f"request_id:{self.model.request_id}, "
            f"company_id:{self.model.company_id}, "
            f"company_name:{self.model.company_name}, "
            f"project_id:{self.model.project_id}, "
            f"project_name:{self.model.project_name}, "
            f"request_status:{self.model.request_status}, "
            f"user_emails:{self.model.user_emails}, "
            f"user_id:{self.model.user_id}, "
            f"user_github_id:{self.model.user_github_id}, "
            f"user_github_username:{self.model.user_github_username}, "
            f"user_name:{self.model.user_name}"
        )

    def to_dict(self):
        return dict(self.model)

    def save(self):
        self.model.date_modified = datetime.datetime.utcnow()
        return self.model.save()

    def load(self, request_id):
        try:
            ccla_whitelist_request = self.model.get(str(request_id))
        except CCLAWhitelistRequest.DoesNotExist:
            raise cla.models.DoesNotExist("CCLAWhitelistRequest not found")

    def delete(self):
        self.model.delete()

    def get_request_id(self):
        return self.model.request_id

    def get_company_id(self):
        return self.model.company_id

    def get_company_name(self):
        return self.model.company_name

    def get_project_id(self):
        return self.model.project_id

    def get_project_name(self):
        return self.model.project_name

    def get_request_status(self):
        return self.model.request_status

    def get_user_emails(self):
        return self.model.user_emails

    def get_user_id(self):
        return self.model.user_id

    def get_user_github_id(self):
        return self.model.user_github_id

    def get_user_github_username(self):
        return self.model.user_github_username

    def get_user_name(self):
        return self.model.user_name

    def get_project_external_id(self):
        return self.model.project_external_id

    def set_request_id(self, request_id):
        self.model.request_id = request_id

    def set_company_id(self, company_id):
        self.model.company_id = company_id

    def set_company_name(self, company_name):
        self.model.company_name = company_name

    def set_project_id(self, project_id):
        self.model.project_id = project_id

    def set_project_name(self, project_name):
        self.model.project_name = project_name

    def set_request_status(self, request_status):
        self.model.request_status = request_status

    def set_user_emails(self, user_emails):
        # LG: handle different possible types passed as argument
        if user_emails:
            if isinstance(user_emails, list):
                self.model.user_emails = set(user_emails)
            elif isinstance(user_emails, set):
                self.model.user_emails = user_emails
            else:
                self.model.user_emails = set([user_emails])
        else:
            self.model.user_emails = set()

    def set_user_id(self, user_id):
        self.model.user_id = user_id

    def set_user_github_id(self, user_github_id):
        self.model.user_github_id = user_github_id

    def set_user_github_username(self, user_github_username):
        self.model.user_github_username = user_github_username

    def set_user_name(self, user_name):
        self.model.user_name = user_name

    def set_project_external_id(self, project_external_id):
        self.model.project_external_id = project_external_id

    def all(self):
        ccla_whitelist_requests = self.model.scan()
        ret = []
        for request in ccla_whitelist_requests:
            ccla_whitelist_request = CCLAWhitelistRequest()
            ccla_whitelist_request.model = request
            ret.append(ccla_whitelist_request)
        return ret
