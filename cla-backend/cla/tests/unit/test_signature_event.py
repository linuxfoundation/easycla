# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

from unittest.mock import patch, Mock

import pytest

from cla.models.dynamo_models import Signature, Project, Company
from cla.controllers import signature as signature_controller
from cla.controllers import company
from cla.models.event_types import EventType
from cla.auth import AuthUser
from unittest.mock import patch, Mock

import pytest

from cla.auth import AuthUser
from cla.controllers import company
from cla.controllers import signature as signature_controller
from cla.models.dynamo_models import Signature, Project, Company
from cla.models.event_types import EventType


@pytest.fixture()
def create_event_signature():
    signature_controller.create_event = Mock()


@pytest.fixture()
def auth_user():
    with patch.object(AuthUser, "__init__", lambda self: None):
        user = AuthUser()
        yield user


@patch('cla.controllers.signature.Event.create_event')
def test_create_signature(mock_event, create_event_signature, project):
    """ Test create signature event """
    Project.load = Mock()
    Company.load = Mock()
    Signature.set_signature_document_major_version = Mock()
    Signature.set_signature_document_minor_version = Mock()
    Project.get_project_corporate_document = Mock()
    Project.get_project_name = Mock(return_value=project.get_project_name())
    Signature.save = Mock()
    event_type = EventType.CreateSignature
    signature_id = 'new_signature_id'
    Signature.get_signature_id = Mock(return_value=signature_id)
    Signature.get_signature_project_id = Mock(return_value=project.get_project_id())
    project_id = project.get_project_id()
    project = project.get_project_name()
    event_data = f'Signature added. Signature_id - {signature_id} for Project - {project}'
    signature_controller.create_signature(
        project_id, 'signature_reference_id', 'signature_reference_type'
    )
    mock_event.assert_called_once_with(
        event_data=event_data,
        event_summary=event_data,
        event_type=event_type,
        event_cla_group_id=project_id,
        contains_pii=False,
    )


@patch('cla.controllers.signature.Event.create_event')
def test_update_signature(mock_event, auth_user, create_event_signature, signature_instance):
    """ Test update signature """
    Signature.load = Mock()
    auth_user.name = 'ddeal'
    event_type = EventType.UpdateSignature
    signature_controller.notify_whitelist_change = Mock()
    # test signature_reference_type_check
    signature_controller.update_signature(
        signature_instance.get_signature_id(),
        auth_user,
        signature_reference_type='type'
    )

    event_data = f'signature {signature_instance.get_signature_id()} updates: \n signature_reference_type updated to type \n'
    mock_event.assert_called_once_with(
        event_data=event_data,
        event_summary=event_data,
        event_cla_group_id=signature_instance.get_signature_project_id(),
        event_type=event_type,
        contains_pii=True,
    )


@patch('cla.controllers.signature.Event.create_event')
def test_delete_signature(mock_event, create_event_signature, signature_instance):
    """ Test delete signature """
    event_type = EventType.DeleteSignature
    event_data = f'Deleted signature {signature_instance.get_signature_id()}'
    signature_controller.delete_signature(
        signature_instance.get_signature_id()
    )
    mock_event.assert_called_once_with(
        event_data=event_data,
        event_summary=event_data,
        event_cla_group_id=signature_instance.get_signature_project_id(),
        event_type=event_type,
        contains_pii=False,
    )


@patch('cla.controllers.signature.Event.create_event')
def test_add_cla_manager(mock_event, auth_user, signature_instance, create_event_signature):
    """ Test add cla manager event """
    Signature.load = Mock()
    auth_user.username = 'harold'
    Signature.get_signature_acl = Mock(return_value=('harold'))
    company.add_permission = Mock()
    Signature.add_signature_acl = Mock()
    Signature.save = Mock()
    signature_controller.get_managers_dict = Mock()
    lfid = 'foo_lfid'
    subject = 'Add CLA Manager'
    body = 'Added %s' % lfid
    recipients = ['foo@gmail.com']
    signature_controller.add_cla_manager_email_content = Mock(return_value=(subject, body, recipients))
    signature_controller.get_email_service = Mock()
    signature_controller.add_cla_manager(
        auth_user,
        signature_instance.get_signature_id(),
        lfid
    )
    event_data = f'{lfid} added as cla manager to Signature ACL for {signature_instance.get_signature_id()}'

    mock_event.assert_called_once_with(
        event_data=event_data,
        event_summary=event_data,
        event_cla_group_id=signature_instance.get_signature_project_id(),
        event_type=EventType.AddCLAManager,
        contains_pii=True,
    )


@patch('cla.controllers.signature.Event.create_event')
def test_remove_cla_manager(mock_event, signature_instance, create_event_signature):
    """ Test remove cla_manager """
    Signature.get_signature_acl = Mock(return_value=('harold'))
    Signature.load = Mock()
    Signature.remove_signature_acl = Mock()
    Signature.save = Mock()
    signature_controller.get_managers_dict = Mock()
    event_type = EventType.RemoveCLAManager
    lfid = 'nachwera'
    subject = 'Removed CLA Manager'
    body = 'Removed %s' % lfid
    recipients = ['foo@gmail.com']
    signature_controller.remove_cla_manager_email_content = Mock(return_value=(subject, body, recipients))
    signature_controller.get_email_service = Mock()
    signature_controller.remove_cla_manager(
        'harold', signature_instance.get_signature_id(), lfid
    )
    event_data = f'User with lfid {lfid} removed from project ACL with signature {signature_instance.get_signature_id()}'
    mock_event.assert_called_once_with(
        event_data=event_data,
        event_summary=event_data,
        event_type=event_type,
        event_cla_group_id=signature_instance.get_signature_project_id(),
        contains_pii=True,
    )
