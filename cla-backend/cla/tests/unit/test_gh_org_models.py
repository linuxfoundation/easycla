# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

from unittest.mock import MagicMock, Mock, patch

import cla
import pynamodb
import pytest
from cla.models.dynamo_models import GitHubOrg, GitHubOrgModel
from cla.tests.unit.data import GH_TABLE_DESCRIPTION
from cla.utils import get_github_organization_instance

PATCH_METHOD = "pynamodb.connection.Connection._make_api_call"


@pytest.fixture()
def gh_instance():
    """ GitHubOrg instance """
    with patch(PATCH_METHOD) as req:
        req.return_value = GH_TABLE_DESCRIPTION
        gh_org = cla.utils.get_github_organization_instance()
        gh_name = "FOO"
        gh_org.set_organization_name(gh_name)
        gh_org.set_organization_sfid("foo_sf_id")
        gh_org.set_project_sfid("foo_sf_id")
        gh_org.save()
        yield gh_org


def test_set_organization_name(gh_instance):
    """ Test setting GitHub org name #1126 """
    assert gh_instance.get_organization_name_lower() == "foo"


def test_get_org_by_name_lower(gh_instance):
    """ Test getting GitHub org with case insensitive search """
    gh_org = cla.utils.get_github_organization_instance()
    gh_org.model.scan = Mock(return_value=[gh_instance.model])
    found_gh_org = gh_org.get_organization_by_lower_name(gh_instance.get_organization_name())
    assert found_gh_org.get_organization_name_lower() == gh_instance.get_organization_name_lower()
