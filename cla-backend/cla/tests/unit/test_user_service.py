# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# TODO - Need to mock this set of tests so that it doesn't require the real service
# from unittest.mock import patch
#
# import pytest
#
# from cla.user_service import UserService
# from cla.models.dynamo_models import ProjectCLAGroup
#
#
# @pytest.fixture
# def mock_pcg():
#     pcg = ProjectCLAGroup()
#     pcg.set_project_sfid('foo_project_sfid')
#     pcg.set_foundation_sfid('foo_foundation_sfid')
#     pcg.set_cla_group_id('foo_cla_group_id')
#     yield pcg
#
#
# @patch('cla.user_service.ProjectCLAGroup.get_by_cla_group_id')
# @patch('cla.user_service.UserService._list_org_user_scopes')
# def test_user_has_role_scope(mock_user_scopes, mock_pcgs, mock_pcg):
#     """ Check if given user has role scope """
#     mock_user_scopes.return_value = {
#         'userroles': [
#             {
#                 'RoleScopes' : [
#                     {
#                         'RoleID': 'foo_role_id',
#                         'RoleName': 'cla-maanger',
#                         'Scopes' : [
#                             {
#                                 'ObjectID' : 'foo_project_sfid|foo_company_sfid',
#                                 'ObjectName' : 'foo_project_name|foo_company_name',
#                                 'ObjectTypeID': 11,
#                                 'ObjectTypeName': 'project|organization',
#                                 'ScopeID': 'foo_scope_id'
#                             }
#                         ]
#                     }
#                 ],
#                 'Contact' : {
#                     'ID': 'foo_id',
#                     'Username': 'foo_username',
#                     'EmailAddress': 'foo@gmail.com',
#                     'Name': 'foo',
#                     'LogoURL': 'http://logo.com',
#                 }
#             },
#         ]
#     }
#     mock_pcgs.return_value = [mock_pcg]
#     user_service = UserService
#     assert user_service.has_role('foo_username', 'cla-manager', 'foo_company_sfid', 'foo_cla_group_id')
#     assert user_service.has_role('foo_no_role','cla-manager', 'foo_company_sfid', 'foo_cla_group_id') == False
#
