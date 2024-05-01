# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

from unittest.mock import MagicMock, patch

import pytest
from cla.models.dynamo_models import (Company, CompanyModel, EventModel,
                                      Project, ProjectModel, Signature,
                                      SignatureModel, User, UserModel)
from cla.tests.unit.data import (COMPANY_TABLE_DATA, EVENT_TABLE_DESCRIPTION,
                                 PROJECT_TABLE_DESCRIPTION,
                                 SIGNATURE_TABLE_DATA, USER_TABLE_DATA)

PATCH_METHOD = "pynamodb.connection.Connection._make_api_call"


# @pytest.fixture()
# def signature_instance():
#     """
#     Mock signature instance
#     """
#     with patch(PATCH_METHOD) as req:
#         req.return_value = SIGNATURE_TABLE_DATA
#         instance = Signature()
#         instance.set_signature_id("sig_id")
#         instance.set_signature_project_id("proj_id")
#         instance.set_signature_reference_id("ref_id")
#         instance.set_signature_type("type")
#         instance.set_signature_project_external_id("proj_id")
#         instance.set_signature_company_signatory_id("comp_sig_id")
#         instance.set_signature_company_signatory_name("name")
#         instance.set_signature_company_signatory_email("email")
#         instance.set_signature_company_initial_manager_id("manager_id")
#         instance.set_signature_company_initial_manager_name("manager_name")
#         instance.set_signature_company_initial_manager_email("manager_email")
#         instance.set_signature_company_secondary_manager_list({"foo": "bar"})
#         instance.set_signature_document_major_version(1)
#         instance.set_signature_document_minor_version(2)
#         instance.save()
#         yield instance


@pytest.fixture()
def user_instance():
    """
    Mock user instance
    """
    with patch(PATCH_METHOD) as req:
        req.return_value = USER_TABLE_DATA
        instance = User()
        instance.set_user_id("foo")
        instance.set_user_name("username")
        instance.set_user_external_id("bar")
        instance.save()
        yield instance


@pytest.fixture()
def company_instance():
    """
    Mock Company instance
    """
    with patch(PATCH_METHOD) as req:
        req.return_value = COMPANY_TABLE_DATA
        instance = Company()
        instance.set_company_id("uuid")
        instance.set_company_name("co")
        instance.set_signing_entity_name("co entity name")
        instance.set_company_external_id("external id")
        instance.save()
        yield instance


@pytest.fixture()
def event_table():
    """ Fixture that creates the event table """

    def fake_dynamodb(*args):
        return EVENT_TABLE_DESCRIPTION

    fake_db = MagicMock()
    fake_db.side_effect = fake_dynamodb

    with patch(PATCH_METHOD, new=fake_db):
        EventModel.create_table(read_capacity_units=1, write_capacity_units=1)


@pytest.fixture()
def user_table():
    """ Fixture that creates the user table """

    def fake_dynamodb(*args):
        return USER_TABLE_DATA

    fake_db = MagicMock()
    fake_db.side_effect = fake_dynamodb

    with patch(PATCH_METHOD, new=fake_db):
        UserModel.create_table()


@pytest.fixture()
def project_table():
    """ Fixture that creates the project table """

    def fake_dynamodb(*args):
        return PROJECT_TABLE_DESCRIPTION

    fake_db = MagicMock()
    fake_db.side_effect = fake_dynamodb

    with patch(PATCH_METHOD, new=fake_db):
        ProjectModel.create_table(read_capacity_units=1, write_capacity_units=1)


@pytest.fixture()
def company_table():
    """ Fixture that creates the company table """

    def fake_dynamodb(*args):
        return COMPANY_TABLE_DATA

    fake_db = MagicMock()
    fake_db.side_effect = fake_dynamodb

    with patch(PATCH_METHOD, new=fake_db):
        CompanyModel.create_table()


@pytest.fixture()
def user(user_table):
    """ create user """
    with patch(PATCH_METHOD) as req:
        req.return_value = {}
        user_instance = User()
        user_instance.set_user_id("user_foo_id")
        user_instance.set_user_email("foo@gmail.com")
        user_instance.set_user_name("foo_username")
        user_instance.save()
        yield user_instance


@pytest.fixture()
def project(project_table):
    """ create project """
    with patch(PATCH_METHOD) as req:
        req.return_value = {}
        project_instance = Project()
        project_instance.set_project_id("foo_project_id")
        project_instance.set_project_external_id("foo_external_id")
        project_instance.set_project_name("foo_project_name")
        project_instance.save()
        yield project_instance


@pytest.fixture()
def company(company_table):
    """ create project """
    with patch(PATCH_METHOD) as req:
        req.return_value = {}
        company_instance = Company()
        company_instance.set_company_id("foo_company_id")
        company_instance.set_company_name("foo_company_name")
        company_instance.set_signing_entity_name("foo_comapny_entity_name")
        company_instance.save()
        yield company_instance


@pytest.fixture()
def load_project(project):
    """ Mock load project """
    with patch("cla.controllers.event.cla.utils.get_project_instance") as mock_project:
        instance = mock_project.return_value
        instance.load.return_value = project
        instance.get_project_name.return_value = project.get_project_name()
        yield instance


@pytest.fixture()
def load_company(company):
    """ Mock load company """
    with patch("cla.controllers.event.cla.utils.get_company_instance") as mock_company:
        instance = mock_company.return_value
        instance.load.return_value = company
        instance.get_company_name.return_value = company.get_company_name()
        yield instance


@pytest.fixture()
def load_user(user):
    """ Mock load user """
    with patch("cla.controllers.event.cla.utils.get_user_instance") as mock_user:
        instance = mock_user.return_value
        instance.load.return_value = user
        yield instance
