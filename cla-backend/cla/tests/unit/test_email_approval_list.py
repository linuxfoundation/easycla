# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

from unittest.mock import MagicMock, patch

import pytest
from cla.models.dynamo_models import Signature, User, UserModel


@pytest.fixture()
def create_user():
    """ Mock user instance """
    with patch.object(User, "__init__", lambda self: None):
        user = User()
        user.model = UserModel()
        yield user


def test_email_against_pattern_with_asterix_prefix(create_user):
    """ Test given user against pattern starting with_asterix_prefix """
    emails = ["harold@bar.com"]
    patterns = ["*bar.com"]
    assert create_user.preprocess_pattern(emails, patterns) == True


def test_subdomain_against_pattern_asterix_prefix(create_user):
    """Test user email on subdomain against pattern """
    emails = ["harold@help.bar.com"]
    patterns = ["*bar.com"]
    assert create_user.preprocess_pattern(emails, patterns) == True


def test_email_multiple_domains(create_user):
    """Test emails against multiple domain lists starting with *.,* and . """
    emails = ["harold@bar.com"]
    patterns = ["*bar.com", "*.bar.com", ".bar.com"]
    assert create_user.preprocess_pattern(emails, patterns) == True
    emails = ["harold@foo.com"]
    assert create_user.preprocess_pattern(emails, patterns) == False


def test_naked_domain(create_user):
    """Test user against naked domain pattern (e.g google.com) """
    emails = ["harold@bar.com"]
    patterns = ["bar.com"]
    assert create_user.preprocess_pattern(emails, patterns) == True
    fail_emails = ["harold@help.bar.com"]
    assert create_user.preprocess_pattern(fail_emails, patterns) == False


def test_pattern_with_asterix_dot_prefix(create_user):
    """ Test given user email against pattern starting with asterix_dot_prefix """
    emails = ["harold@bar.com"]
    patterns = ["*.bar.com"]
    assert create_user.preprocess_pattern(emails, patterns) == True


def test_pattern_with_dot_prefix(create_user):
    """Test given user email against pattern starting with dot_prefix """
    emails = ["harold@bar.com"]
    patterns = [".bar.com"]
    assert create_user.preprocess_pattern(emails, patterns) == True
    domain_emails = ["harold@help.bar.com"]
    assert create_user.preprocess_pattern(domain_emails, patterns) == True


def test_email_approval_list_fail(create_user):
    """Test email that fails domain and email approval list checks """
    signature = Signature()
    signature.get_email_whitelist = MagicMock(return_value={"foo@gmail.com"})
    signature.get_domain_whitelist = MagicMock(return_value=["foo.com"])
    create_user.get_all_user_emails = MagicMock(return_value=["bar@gmail.com"])
    assert create_user.is_approved(signature) == False


def test_gerrit_project_approval_listing(create_user):
    """Test for email in signature approval list"""
    signature = Signature()
    signature.get_email_whitelist = MagicMock(return_value={"phillip.leigh@amdocs.com"})
    create_user.get_all_user_emails = MagicMock(return_value=["phillip.leigh@amdocs.com"])
    assert create_user.is_approved(signature) == True
