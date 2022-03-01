# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

import unittest

from cla.user import UserCommitSummary
from cla.utils import get_comment_body


class TestUserCommitSummary(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        pass

    @classmethod
    def tearDownClass(cls) -> None:
        pass

    def setUp(self) -> None:
        pass

    def tearDown(self) -> None:
        pass

    def test_user_commit_summary_is_valid(self) -> None:
        t = UserCommitSummary("some_sha", 1234, 'login_value', 'author name', 'foo@bar.com', False, False)
        self.assertTrue(t.is_valid_user())
        t = UserCommitSummary("some_sha", 1234, None, None, 'foo@bar.com', False, False)
        self.assertFalse(t.is_valid_user())

    def test_user_commit_summary_get_comment_body(self) -> None:
        s = UserCommitSummary("some_sha", 1234, 'login_value', 'author name', 'foo@bar.com', True, True)
        signed = [s]
        m = UserCommitSummary("some_other_sha", 123456, 'login_value2', 'author name2', 'foo2@bar.com', False, False)
        missing = [m]
        body = get_comment_body('github', 'https://foo.com', signed, missing)
        self.assertTrue(':white_check_mark:' in body)
        self.assertTrue(':x:' in body)
