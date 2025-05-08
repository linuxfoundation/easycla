# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

from unittest.mock import Mock

import pytest
from cla import utils
from cla.models.dynamo_models import Company, User, Project, Document


@pytest.fixture
def user():
    yield User()


@pytest.fixture
def project():
    yield Project()

@pytest.fixture
def document_factory():
    def create_document(major, minor, date):
        mock_document = Mock(spec=Document)
        mock_document.get_document_major_version.return_value = major
        mock_document.get_document_minor_version.return_value = minor
        mock_document.get_document_creation_date.return_value = date
        return mock_document
    return create_document

@pytest.fixture
def document_15(document_factory):
    return document_factory(1, 5, "2025-02-17T15:00:13Z")

@pytest.fixture
def document_29(document_factory):
    return document_factory(2, 9, "2024-02-17T15:00:13Z")

@pytest.fixture
def document_29_newer(document_factory):
    return document_factory(2, 9, "2024-02-18T15:00:13Z")

@pytest.fixture
def document_210(document_factory):
    return document_factory(2, 10, "2023-02-17T15:00:13Z")

@pytest.fixture
def document_210_newer(document_factory):
    return document_factory(2, 10, "2023-02-18T15:00:13Z")

@pytest.fixture
def document_30(document_factory):
    return document_factory(3, 0, "2022-02-18T15:00:13Z")

@pytest.fixture
def document_310(document_factory):
    return document_factory(3, 10, "2022-02-18T15:00:13Z")

@pytest.fixture
def document_31(document_factory):
    return document_factory(3, 1, "2022-02-18T15:00:13Z")

def test_get_latest_version(project, document_15, document_29, document_29_newer, document_210, document_210_newer, document_30, document_31, document_310):
    assert project._get_latest_version([]) == (0, -1, None)
    assert project._get_latest_version([document_29]) == (2, 9, document_29)
    assert project._get_latest_version([document_29_newer, document_29]) == (2, 9, document_29_newer)
    assert project._get_latest_version([document_29, document_29_newer]) == (2, 9, document_29_newer)
    assert project._get_latest_version([document_29, document_210]) == (2, 10, document_210)
    assert project._get_latest_version([document_29_newer, document_210]) == (2, 10, document_210)
    assert project._get_latest_version([document_29, document_210_newer]) == (2, 10, document_210_newer)
    assert project._get_latest_version([document_29_newer, document_210_newer]) == (2, 10, document_210_newer)
    assert project._get_latest_version([document_29, document_29_newer, document_210]) == (2, 10, document_210)
    assert project._get_latest_version([document_29, document_210, document_210_newer]) == (2, 10, document_210_newer)
    assert project._get_latest_version([document_29, document_29_newer, document_210, document_210_newer]) == (2, 10, document_210_newer)
    assert project._get_latest_version([document_210, document_210_newer, document_29_newer, document_29]) == (2, 10, document_210_newer)
    assert project._get_latest_version([document_210, document_15, document_210_newer, document_29_newer, document_29]) == (2, 10, document_210_newer)
    assert project._get_latest_version([document_210, document_210_newer, document_29_newer, document_30, document_29]) == (3, 0, document_30)
    assert project._get_latest_version([document_210, document_15, document_210_newer, document_29_newer, document_30, document_29]) == (3, 0, document_30)
    assert project._get_latest_version([document_15, document_30, document_29]) == (3, 0, document_30)
    assert project._get_latest_version([document_31, document_310, document_30]) == (3, 10, document_310)

def test_get_user_email_with_private_email(user):
    """ Test user with a single private email instance """
    user.model.user_emails = set(["harold@noreply.github.com"])
    assert utils.get_public_email(user) is None

def test_get_user_email_mix(user):
    """ Test user with both private and normal email """
    user.model.user_emails = set(["harold@noreply.github.com", "wanyaland@gmail.com"])
    assert utils.get_public_email(user) == "wanyaland@gmail.com"

def test_get_user_email(user):
    """ Test getting user email with valid email """
    user.model.user_emails = set(["wanyaland@gmail.com"])
    assert utils.get_public_email(user) == "wanyaland@gmail.com"
