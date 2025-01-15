# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

import datetime
import time
from unittest.mock import Mock

import pytest
from cla.models import event_types
from cla.models.dynamo_models import Company, Event, Project, User


@pytest.fixture()
def mock_event():
    event = Event()
    event.model.save = Mock()
    yield event


def test_event_user_id(user_instance):
    """ Test event_user_id """
    Event.save = Mock()
    User.load = Mock()
    event_data = "test user id"
    response = Event.create_event(
        event_type=event_types.EventType.CreateProject,
        event_data=event_data,
        event_summary=event_data,
        event_user_id=user_instance.get_user_id()
    )
    assert 'data' in response


def test_event_company_id(company):
    """ Test creation of event instance """
    # Case for creating Company
    Event.save = Mock()
    Company.load = Mock()
    event_data = 'test company created'
    response = Event.create_event(
        event_type=event_types.EventType.DeleteCompany,
        event_data=event_data,
        event_summary=event_data,
        event_company_id=company.get_company_id()
    )
    assert 'data' in response


def test_event_project_id(project):
    """ Test event with event_project_id """
    Event.save = Mock()
    Project.load = Mock()
    event_data = 'project id loaded'
    response = Event.create_event(
        event_data=event_data,
        event_summary=event_data,
        event_type=event_types.EventType.DeleteProject,
        event_cla_group_id=project.get_project_id()
    )
    assert project.get_project_id() == response['data']['event_cla_group_id']


def test_event_user_id_attribute(user_instance, mock_event):
    """ Test event_user_id attribute """
    mock_event.set_event_user_id(user_instance.get_user_id())
    mock_event.save()
    assert mock_event.get_event_user_id() == user_instance.get_user_id()


def test_event_company_name_lower_attribute(mock_event):
    """ Test company_name_lower attribute """
    mock_event.set_event_company_name("Company_lower")
    mock_event.save()
    assert mock_event.get_event_company_name_lower() == "company_lower"


def test_event_username_attribute(mock_event):
    """ Test event_username attribute """
    mock_event.set_event_user_name("foo_username")
    mock_event.save()
    assert mock_event.get_event_user_name() == "foo_username"


def test_event_user_name_lower_attribute(mock_event):
    """ Test event_user_name_lower attribute """
    mock_event.set_event_user_name("Username")
    mock_event.save()
    assert mock_event.get_event_user_name_lower() == "username"


def test_event_project_name_lower_attribute(mock_event):
    """ Test getting project """
    mock_event.set_event_project_name("Project")
    mock_event.save()
    assert mock_event.get_event_project_name_lower() == "project"


def test_event_time(mock_event):
    """ Test event time  """
    mock_event.save()
    assert mock_event.get_event_time() <= datetime.datetime.utcnow()




def test_company_id_external_project_id(mock_event):
    mock_event.set_event_project_sfid("external_id")
    mock_event.set_event_company_id("company_id")
    mock_event.set_company_id_external_project_id()
    assert mock_event.get_company_id_external_project_id() == "company_id#external_id"


def test_company_id_external_project_id_empty_test1(mock_event):
    mock_event.set_event_project_sfid("external_id")
    mock_event.set_company_id_external_project_id()
    assert mock_event.get_company_id_external_project_id() == None


def test_company_id_external_project_id_empty_test2(mock_event):
    mock_event.set_event_company_id("company_id")
    mock_event.set_company_id_external_project_id()
    assert mock_event.get_company_id_external_project_id() == None


def test_company_id_external_project_id_empty_test3(mock_event):
    mock_event.set_company_id_external_project_id()
    assert mock_event.get_company_id_external_project_id() == None
