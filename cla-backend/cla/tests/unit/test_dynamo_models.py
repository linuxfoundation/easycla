# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

from unittest.mock import Mock

import pytest
from cla import utils
from cla.models.dynamo_models import Company, User


@pytest.fixture
def user():
    yield  User()



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


