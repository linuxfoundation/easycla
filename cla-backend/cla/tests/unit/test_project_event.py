# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

from unittest.mock import Mock, patch

import cla
from cla.auth import AuthUser
from cla.controllers import project as project_controller
from cla.models.dynamo_models import Document, Project, User, UserPermissions
from cla.models.event_types import EventType

PATCH_METHOD = "pynamodb.connection.Connection._make_api_call"


@patch('cla.controllers.project.Event.create_event')
def test_event_delete_project(mock_event, project):
    """ Test Delete Project event """
    project_controller.create_event = Mock()
    Project.load = Mock()
    Project.get_project_name = Mock(
        return_value=project.get_project_name()
    )
    Project.get_project_acl = Mock(return_value="test_user")
    project_id = project.get_project_id()
    # invoke delete function
    project_controller.delete_project(project_id, "test_user")
    expected_event_type = EventType.DeleteProject
    expected_event_data = "Project-{} deleted".format(project.get_project_name())
    # Check whether audit event service is invoked
    mock_event.assert_called_with(
        event_type=expected_event_type,
        event_cla_group_id=project_id,
        event_data=expected_event_data,
        event_summary=expected_event_data,
        contains_pii=False,
    )


@patch('cla.controllers.project.Event.create_event')
def test_event_create_project(mock_event):
    """ Test Create Project event """

    event_type = EventType.CreateProject
    project_id = "foo_project_id"
    expected_event_data = "Project-{} created".format("foo_project_name")
    project_external_id = "foo_external_id"
    project_name = "foo_project_name"
    project_icla_enabled = True
    project_ccla_enabled = True
    project_ccla_requires_icla_signature = True
    project_acl_username = "foo_acl_username"
    project_controller.create_event = Mock()
    Project.load = Mock()
    Project.save = Mock()
    Project.get_project_id = Mock(return_value=project_id)
    Project.get_project_name = Mock(return_value=project_name)

    # invoke project create
    project_controller.create_project(
        project_external_id,
        project_name,
        project_icla_enabled,
        project_ccla_enabled,
        project_ccla_requires_icla_signature,
        project_acl_username,
    )
    # Test for audit event
    mock_event.assert_called_with(
        event_type=event_type,
        event_cla_group_id=project_id,
        event_project_id=project_external_id,
        event_data=expected_event_data,
        event_summary=expected_event_data,
        contains_pii=False,
    )


@patch('cla.controllers.project.Event.create_event')
def test_event_update_project(mock_event, project):
    """ Test Update Project event """

    event_type = EventType.UpdateProject
    project_id = project.get_project_id()
    new_project_name = "new_test_name"
    Project.load = Mock()
    Project.save = Mock()
    Project.get_project_id = Mock(
        return_value=project.get_project_id()
    )
    project_controller.project_acl_verify = Mock(return_value=True)

    # Update project name
    project_controller.update_project(
        project_id, project_name=new_project_name, username="foo_user"
    )
    updated_string = f" project_name changed to {new_project_name} \n"
    expected_event_data = f"Project- {project_id} Updates: {updated_string}"

    mock_event.assert_called_with(
        event_type=event_type,
        event_data=expected_event_data,
        event_summary=expected_event_data,
        event_cla_group_id=project_id,
        contains_pii=False,
    )


@patch('cla.controllers.project.Event.create_event')
def test_create_project_document(mock_event, project):
    """ Test create project Document event """
    event_type = EventType.CreateProjectDocument
    project_id = project.get_project_id()
    document_type = "individual"
    document_content_type = "pdf"
    document_content = "content"
    document_preamble = "preamble"
    document_legal_entity_name = "legal"
    document_name = "foo_document"
    Project.load = Mock()
    Project.save = Mock()
    Project.get_project_name = Mock(
        return_value=project.get_project_name()
    )
    Project.add_project_individual_document = Mock()
    project_controller.project_acl_verify = Mock(return_value=True)
    cla.utils.get_last_version = Mock(return_value=(1, 1))
    event_data = "Created new document for Project-{} ".format(
        project.get_project_name()
    )

    project_controller.post_project_document(
        project_id,
        document_type,
        document_name,
        document_content_type,
        document_content,
        document_preamble,
        document_legal_entity_name,
        new_major_version=False,
    )
    mock_event.assert_called_with(
        event_type=event_type,
        event_cla_group_id=project_id,
        event_data=event_data,
        event_summary=event_data,
        contains_pii=False,
    )


@patch('cla.controllers.project.Event.create_event')
def test_create_project_document_template(mock_event, project):
    """ Test creating project document with existing template event """
    event_type = EventType.CreateProjectDocumentTemplate
    project_id = project.get_project_id()
    document_type = "individual"
    document_preamble = "preamble"
    document_legal_entity_name = "legal"
    document_name = "foo_document"
    template_name = "template"
    event_data = "Project Document created for project {} created with template {}".format(
        project.get_project_name(), template_name
    )

    # Mock document template process
    Project.load = Mock()
    Project.save = Mock()
    cla.resources.contract_templates = Mock()
    project_controller.get_pdf_service = Mock()
    Document.set_document_content = Mock()
    Document.set_raw_document_tabs = Mock()
    Project.get_project_name = Mock(
        return_value=project.get_project_name()
    )
    Project.add_project_individual_document = Mock()

    project_controller.post_project_document_template(
        project_id,
        document_type,
        document_name,
        document_preamble,
        document_legal_entity_name,
        template_name,
    )

    mock_event.assert_called_with(
        event_type=event_type,
        event_cla_group_id=project_id,
        event_data=event_data,
        event_summary=event_data,
        contains_pii=False,
    )


@patch('cla.controllers.project.Event.create_event')
def test_delete_project_document(mock_event):
    """ Test event for deleting document from the specified project """
    project = Project()
    project.set_project_id("foo_project_id")
    project.set_project_name("foo_project_name")
    event_type = EventType.DeleteProjectDocument
    project_id = project.get_project_id()
    document_type = "individual"
    major_version = "v1"
    minor_version = "v1"
    event_data = (
            f'Project {project.get_project_name()} with {document_type} :'
            + f'document type , minor version : {minor_version}, major version : {major_version}  deleted'
    )

    Project.load = Mock()
    Project.save = Mock()
    Project.remove_project_individual_document = Mock()
    project_controller.project_acl_verify = Mock()
    cla.utils.get_project_document = Mock()

    project_controller.delete_project_document(
        project_id, document_type, major_version, minor_version
    )

    mock_event.assert_called_with(
        event_type=event_type,
        event_cla_group_id=project_id,
        event_data=event_data,
        event_summary=event_data,
        contains_pii=False,
    )


@patch('cla.controllers.project.Event.create_event')
def test_project_add_permission_existing_user(mock_event, project):
    """ Test adding permissions to project event """
    auth_claims = {
        'auth0_username_claim': 'http:/localhost/foo',
        'email': 'foo@gmail.com',
        'sub': 'bar',
        'name': 'name'
    }
    username = 'harry'
    auth_user = AuthUser(auth_claims)
    auth_user.username = 'ddeal'
    event_type = EventType.AddPermission
    project_sfdc_id = 'project_sfdc_id'

    UserPermissions.load = Mock()
    UserPermissions.add_project = Mock()
    UserPermissions.save = Mock()

    project_controller.add_permission(
        auth_user,
        username,
        project_sfdc_id
    )

    event_data = 'User {} given permissions to project {}'.format(username, project_sfdc_id)

    mock_event.assert_called_with(
        event_type=event_type,
        event_data=event_data,
        event_summary=event_data,
        event_project_id=project_sfdc_id,
        contains_pii=True,
    )


@patch('cla.controllers.project.Event.create_event')
def test_project_remove_permission(mock_event):
    """ Test removing permissions to project event """
    auth_claims = {
        'auth0_username_claim': 'http:/localhost/foo',
        'email': 'foo@gmail.com',
        'sub': 'bar',
        'name': 'name'
    }
    username = 'harry'
    auth_user = AuthUser(auth_claims)
    auth_user.username = 'ddeal'
    event_type = EventType.RemovePermission
    project_sfdc_id = 'project_sfdc_id'

    UserPermissions.load = Mock()
    UserPermissions.remove_project = Mock()
    UserPermissions.save = Mock()

    project_controller.remove_permission(
        auth_user,
        username,
        project_sfdc_id
    )

    event_data = 'User {} permission removed to project {}'.format(username, project_sfdc_id)

    mock_event.assert_called_with(
        event_type=event_type,
        event_data=event_data,
        event_summary=event_data,
        event_project_id=project_sfdc_id,
        contains_pii=True,
    )


@patch('cla.controllers.project.Event.create_event')
def test_add_project_manager(mock_event, project):
    """ Tests event logging where LFID is added to the project ACL """
    event_type = EventType.AddProjectManager
    username = 'foo'
    lfid = 'manager'
    Project.load = Mock()
    Project.get_project_name = Mock(return_value=project.get_project_name())
    Project.save = Mock()
    user = User()
    user.set_user_name('foo')
    Project.get_managers_by_project_acl = Mock(return_value=[user])
    Project.add_project_acl = Mock()
    Project.get_project_acl = Mock(return_value=('foo'))

    project_controller.add_project_manager(
        username,
        project.get_project_id(),
        lfid
    )
    event_data = '{} added {} to project {}'.format(username, lfid, project.get_project_name())

    mock_event.assert_called_with(
        event_type=event_type,
        event_data=event_data,
        event_summary=event_data,
        event_cla_group_id=project.get_project_id(),
        contains_pii=True,
    )


@patch('cla.controllers.project.Event.create_event')
def test_remove_project_manager(mock_event, project):
    """ Test event logging where lfid is removed from the project acl """
    event_type = EventType.RemoveProjectManager
    Project.load = Mock()
    Project.get_project_acl = Mock(return_value=('foo', 'bar'))
    Project.remove_project_acl = Mock()
    Project.save = Mock()

    project_controller.remove_project_manager(
        'foo',
        project.get_project_id(),
        'foo_lfid'
    )
    event_data = f'foo_lfid removed from project {project.get_project_id()}'
    mock_event.assert_called_once_with(
        event_type=event_type,
        event_data=event_data,
        event_summary=event_data,
        event_cla_group_id=project.get_project_id(),
        contains_pii=True,
    )
