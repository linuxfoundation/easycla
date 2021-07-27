# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

from cla.models.dynamo_models import GitlabOrg


def test_gitlab_org_model():
    gitlab_org = GitlabOrg(organization_name="GitlabOrg1")
    assert gitlab_org.get_organization_id()
    assert gitlab_org.get_organization_name() == "GitlabOrg1"
    assert gitlab_org.get_organization_name_lower() == "gitlaborg1"
    assert not gitlab_org.get_auto_enabled()
    assert gitlab_org.get_enabled()
    assert not gitlab_org.get_branch_protection_enabled()
    assert not gitlab_org.get_project_sfid()
    assert not gitlab_org.get_organization_sfid()

    gitlab_org.set_organization_name("GitlabOrg2")
    assert gitlab_org.get_organization_name() == "GitlabOrg2"
    assert gitlab_org.get_organization_name_lower() == "gitlaborg2"

    gitlab_org.set_enabled(False)
    assert not gitlab_org.get_enabled()

    gitlab_org.set_project_sfid("project_sfid_1")
    assert gitlab_org.get_project_sfid() == "project_sfid_1"

    gitlab_org.set_organization_sfid("organization_sfid_1")
    assert gitlab_org.get_organization_sfid() == "organization_sfid_1"

    gitlab_org.set_branch_protection_enabled(True)
    assert gitlab_org.get_branch_protection_enabled()
    gitlab_org.set_auto_enabled(True)
    assert gitlab_org.get_auto_enabled()

    gitlab_org_dict = gitlab_org.to_dict()
    assert gitlab_org_dict["organization_id"] == gitlab_org.get_organization_id()
    assert gitlab_org_dict["organization_name"] == "GitlabOrg2"
    assert gitlab_org_dict["project_sfid"] == "project_sfid_1"
    assert gitlab_org_dict["organization_sfid"] == "organization_sfid_1"
