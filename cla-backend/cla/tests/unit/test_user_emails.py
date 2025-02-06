# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
import logging
import unittest

import cla
from cla import utils
from cla.models.dynamo_models import User

def test_set_user_emails():
    usr = User()
    usr.set_user_emails('lgryglicki@cncf.io')
    assert usr.get_user_emails() == {'lgryglicki@cncf.io'}
    usr.set_user_emails(['lgryglicki@cncf.io'])
    assert usr.get_user_emails() == {'lgryglicki@cncf.io'}
    usr.set_user_emails({'lgryglicki@cncf.io'})
    assert usr.get_user_emails() == {'lgryglicki@cncf.io'}
    usr.set_user_emails([])
    assert usr.get_user_emails() == set()
    usr.set_user_emails(set())
    assert usr.get_user_emails() == set()
    usr.set_user_emails({})
    assert usr.get_user_emails() == set()
    usr.set_user_emails(None)
    assert usr.get_user_emails() == set()

