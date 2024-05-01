# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

import json
import os
from http import HTTPStatus
from unittest.mock import MagicMock, Mock, patch

import cla
import pytest
from cla.models.dynamo_models import UserPermissions
from cla.salesforce import get_project, get_projects


@pytest.fixture()
def user():
    """ Patch authenticated user """
    with patch("cla.auth.authenticate_user") as mock_user:
        mock_user.username.return_value = "test_user"
        yield mock_user


@pytest.fixture()
def user_permissions():
    """ Patch permissions """
    with patch("cla.salesforce.UserPermissions") as mock_permissions:
        yield mock_permissions


@patch.dict(cla.salesforce.os.environ,{'CLA_BUCKET_LOGO_URL':'https://s3.amazonaws.com/cla-project-logo-dev'})
@patch("cla.salesforce.requests.get")
def test_get_salesforce_projects(mock_get, user, user_permissions):
    """ Test getting salesforce projects via project service """

    #breakpoint()
    cla.salesforce.get_access_token = Mock(return_value=("token", HTTPStatus.OK))
    sf_projects = [
        {
            "Description": "Test Project 1",
            "ID": "foo_id_1",
            "ProjectLogo": "https://s3/logo_1",
            "Name": "project_1",
        },
        {
            "Description": "Test Project 2",
            "ID": "foo_id_2",
            "ProjectLogo": "https://s3/logo_2",
            "Name": "project_2",
        },
    ]

    user_permissions.projects.return_value = set({"foo_id_1", "foo_id_2"})

    # Fake event
    event = {"httpMethod": "GET", "path": "/v1/salesforce/projects"}

    # Mock project service response
    response = json.dumps({"Data": sf_projects})
    mock_get.return_value.text = response
    mock_get.return_value.status_code = HTTPStatus.OK

    expected_response = [
        {
            "name": "project_1",
            "id": "foo_id_1",
            "description": "Test Project 1",
            "logoUrl": "https://s3.amazonaws.com/cla-project-logo-dev/foo_id_1.png"
        },
        {
            "name": "project_2",
            "id": "foo_id_2",
            "description": "Test Project 2",
            "logoUrl": "https://s3.amazonaws.com/cla-project-logo-dev/foo_id_2.png"
        },
    ]
    assert get_projects(event, None)["body"] == json.dumps(expected_response)


@patch.dict(cla.salesforce.os.environ,{'CLA_BUCKET_LOGO_URL':'https://s3.amazonaws.com/cla-project-logo-dev'})
@patch("cla.salesforce.requests.get")
def test_get_salesforce_project_by_id(mock_get, user, user_permissions):
    """ Test getting salesforce project given id """

    # Fake event
    event = {
        "httpMethod": "GET",
        "path": "/v1/salesforce/project/",
        "queryStringParameters": {"id": "foo_id"},
    }

    sf_projects = [
        {
            "Description": "Test Project",
            "ID": "foo_id",
            "ProjectLogo": "https://s3/logo_1",
            "Name": "project_1",
        },
    ]

    user_permissions.return_value.to_dict.return_value = {"projects": set(["foo_id"])}
    mock_get.return_value.json.return_value = {"Data": sf_projects}
    mock_get.return_value.status_code = HTTPStatus.OK

    expected_response = {
        "name": "project_1",
        "id": "foo_id",
        "description": "Test Project",
        "logoUrl": "https://s3.amazonaws.com/cla-project-logo-dev/foo_id.png"
    }
    assert get_project(event, None)["body"] == json.dumps(expected_response)
