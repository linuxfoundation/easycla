# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

from unittest.mock import Mock, patch

import pytest
from cla.controllers import user as user_controller
from cla.models.dynamo_models import (CCLAWhitelistRequest, Company,
                                      CompanyInvite, Project, User)
from cla.models.event_types import EventType


@pytest.fixture
def create_event_user():
    user_controller.create_event = Mock()


class TestRequestCompanyApprovalList:

    def setup(self) -> None:
        self.old_load = User.load
        self.old_get_user_name = User.get_user_name
        self.get_user_emails = User.get_user_emails
        self.get_user_email = User.get_user_email

        self.company_load = Company.load
        self.get_company_name = Company.get_company_name

        self.project_load = Project.load
        self.get_project_name = Project.get_project_name

    def teardown(self) -> None:
        User.load = self.old_load
        User.get_user_name = self.old_get_user_name
        User.get_user_emails = self.get_user_emails
        User.get_user_email = self.get_user_email

        Company.load = self.company_load
        Company.get_company_name = self.get_company_name

        Project.load = self.project_load
        Project.get_project_name = self.get_project_name

    def test_request_company_approval_list(self, create_event_user, project, company, user):
        """ Test user requesting to be added to the Approved List event """
        with patch('cla.controllers.user.Event.create_event') as mock_event:
            event_type = EventType.RequestCompanyWL
            User.load = Mock()
            User.get_user_name = Mock(return_value=user.get_user_name())
            User.get_user_emails = Mock(return_value=[user.get_user_email()])
            User.get_user_email = Mock(return_value=user.get_user_email())
            Company.load = Mock()
            Company.get_company_name = Mock(return_value=company.get_company_name())
            Project.load = Mock()
            Project.get_project_name = Mock(return_value=project.get_project_name())
            Project.get_project_id = Mock(return_value=project.get_project_id())
            user_controller.get_email_service = Mock()
            user_controller.send = Mock()
            user_controller.request_company_whitelist(
                user.get_user_id(),
                company.get_company_id(),
                user.get_user_name(),
                user.get_user_email(),
                project.get_project_id(),
                message="Please add",
                recipient_name="Recipient Name",
                recipient_email="Recipient Email",
            )

            event_data = (f'CLA: contributor {user.get_user_name()} requests to be Approved for the '
                          f'project: {project.get_project_name()} '
                          f'organization: {company.get_company_name()} '
                          f'as {user.get_user_name()} <{user.get_user_email()}>')

            mock_event.assert_called_once_with(
                event_user_id=user.get_user_id(),
                event_cla_group_id=project.get_project_id(),
                event_company_id=company.get_company_id(),
                event_type=event_type,
                event_data=event_data,
                event_summary=event_data,
                contains_pii=True,
            )


class TestInviteClaManager:

    def setup(self):
        self.user_load = User.load
        self.load_project_by_name = Project.load_project_by_name
        self.save = CCLAWhitelistRequest.save

    def teardown(self):
        User.load = self.user_load
        Project.load_project_by_name = self.load_project_by_name
        CCLAWhitelistRequest.save = self.save

    @patch('cla.controllers.user.Event.create_event')
    def test_invite_cla_manager(self, mock_event, create_event_user, user):
        """ Test send email to CLA manager event """
        User.load = Mock()
        Project.load_project_by_name = Mock()
        Company.load_company_by_name = Mock()
        Company.get_company_id = Mock(return_value='foo_id')
        User.get_user_id = Mock(return_value='foo_id')
        CompanyInvite.save = Mock()
        CCLAWhitelistRequest.save = Mock()
        user_controller.send_email_to_cla_manager = Mock()
        contributor_id = user.get_user_id()
        contributor_name = user.get_user_name()
        contributor_email = user.get_user_email()
        cla_manager_name = "admin"
        cla_manager_email = "foo@admin.com"
        project_name = "foo_project"
        project_id = "foo_project_id"
        company_name = "Test Company"
        event_data = (f'sent email to CLA Manager: {cla_manager_name} with email {cla_manager_email} '
                      f'for project {project_name} and company {company_name} '
                      f'to user {contributor_name} with email {contributor_email}')
        # TODO FIX Unit test - need to mock Project load_project_by_name() function
        user_controller.invite_cla_manager(contributor_id, contributor_name, contributor_email,
                                           cla_manager_name, cla_manager_email,
                                           project_name, company_name)
        mock_event.assert_called_once_with(
            event_user_id=contributor_id,
            event_project_name=project_name,
            event_data=event_data,
            event_summary=event_data,
            event_type=EventType.InviteAdmin,
            event_cla_group_id=project_id,
            contains_pii=True,
        )


class TestRequestCompanyCCLA:

    def setup(self):
        self.user_load = User.load
        self.get_user_name = User.get_user_name
        self.company_load = Company.load
        self.project_load = Project.load
        self.get_project_name = Project.get_project_name
        self.get_managers = Company.get_managers

    def teardown(self):
        User.load = self.user_load
        User.get_user_name = self.get_user_name
        Company.load = self.company_load
        Project.load = self.project_load
        Project.get_project_name = self.get_project_name
        Company.get_managers = self.get_managers

    @patch('cla.controllers.user.Event.create_event')
    def test_request_company_ccla(self, mock_event, create_event_user, user, project, company):
        """ Test request company ccla event """
        User.load = Mock()
        User.get_user_name = Mock(return_value=user.get_user_name())
        email = user.get_user_email()
        Company.load = Mock()
        Project.load = Mock()
        Project.get_project_name = Mock(return_value=project.get_project_name())
        Project.get_project_id = Mock(return_value=project.get_project_id())
        manager = User(lf_username="harold", user_email="foo@gmail.com")
        Company.get_managers = Mock(return_value=[manager, ])
        event_data = f"Sent email to sign ccla for {project.get_project_name()}"
        CCLAWhitelistRequest.save = Mock(return_value=None)
        user_controller.request_company_ccla(
            user.get_user_id(), email, company.get_company_id(), project.get_project_id()
        )
        mock_event.assert_called_once_with(
            event_data=event_data,
            event_summary=event_data,
            event_type=EventType.RequestCCLA,
            event_user_id=user.get_user_id(),
            event_company_id=company.get_company_id(),
            event_cla_group_id=project.get_project_id(),
            contains_pii=False,
        )
