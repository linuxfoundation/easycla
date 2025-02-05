# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
import logging
import unittest

import cla
from cla import utils
from cla.models.dynamo_models import User


class TestUserModels(unittest.TestCase):
    tests_enabled = False

    @classmethod
    def setUpClass(cls) -> None:
        pass

    @classmethod
    def tearDownClass(cls) -> None:
        pass

    def setUp(self) -> None:
        # Only show critical logging stuff
        cla.log.level = logging.CRITICAL

    def tearDown(self) -> None:
        pass

    def test_user_get_user_by_username(self) -> None:
        """
        Test that we can get a user by username
        """
        # TODO - should use mock data - disable tests for now :-(
        if self.tests_enabled:
            user_instance = utils.get_user_instance()
            users = user_instance.get_user_by_username('ddeal')
            self.assertIsNotNone(users, 'User lookup by username is not None')
            self.assertEqual(len(users), 1, 'User lookup by username found 1')

            # some invalid username
            users = user_instance.get_user_by_username('foo')
            self.assertIsNone(users, 'User lookup by username is None')

    def test_user_get_user_by_email(self) -> None:
        """
        Test that we can get a user by email
        """
        # TODO - should use mock data - disable tests for now :-(
        if self.tests_enabled:
            users = User().get_user_by_email('ddeal@linuxfoundation.org')
            self.assertIsNotNone(users, 'User lookup by email is not None')
            self.assertEqual(len(users), 1, 'User lookup by email found 1')

            # some invalid email
            users = User().get_user_by_email('foo@bar.org')
            self.assertIsNone(users, 'User lookup by email is None')

    def test_user_get_user_by_github_id(self) -> None:
        """
        Test that we can get a user by github id
        """
        # TODO - should use mock data - disable tests for now :-(
        if self.tests_enabled:
            users = User().get_user_by_github_id(519609)
            self.assertIsNotNone(users, 'User lookup by github id is not None')
            self.assertEqual(len(users), 2, 'User lookup by github id found 2')

            # some invalid number
            users = User().get_user_by_github_id(9999999)
            self.assertIsNone(users, 'User lookup by github id is None')

    def test_user_get_user_by_github_username(self) -> None:
        """
        Test that we can get a user by github username
        """
        # TODO - should use mock data - disable tests for now :-(
        if self.tests_enabled:
            users = User().get_user_by_github_username('dealako')
            self.assertIsNotNone(users, 'User lookup by github username is not None')
            self.assertEqual(len(users), 1, 'User lookup by github username found 1')

            # some invalid username
            users = User().get_user_by_github_username('foooooo')
            self.assertIsNone(users, 'User lookup by github username is None')

    def test_get_user_email(self):
        # TODO - should use mock data - disable tests for now :-(
        if self.tests_enabled:
            user = User()
            user.set_lf_email(None)
            user.set_user_emails([])
            assert user.get_user_email() is None
    
            user.set_lf_email("test1@test.com")
            assert user.get_user_email() == "test1@test.com"
    
            user = User(user_email="test2@test.com")
            assert user.get_user_email() == "test2@test.com"
    
            user = User(user_email="test3@test.com", preferred_email="test3@test.com")
            assert user.get_user_email() == "test3@test.com"
            user.set_user_emails(["test4@test.com", "test5@test.com"])
            user.set_lf_email("test3@test.com")
            assert user.get_user_email() == "test3@test.com"
    
            # the scenario where have multiple emails
            user = User(preferred_email="test5@test.com")
            user.set_user_emails(["test1@test.com", "test2@test.com", "test5@test.com"])
            assert user.get_user_email() == "test5@test.com"
            assert user.get_user_email(preferred_email="test2@test.com") == "test2@test.com"
            assert user.get_user_email(preferred_email="test10@test.com") != "test10@test.com"
            user.set_lf_email("test4@test.com")
            assert user.get_user_email() == "test5@test.com"
            assert user.get_user_email(preferred_email="test4@test.com") == "test4@test.com"
            assert user.get_user_email(preferred_email="test2@test.com") == "test2@test.com"
            assert user.get_user_email(preferred_email="test10@test.com") == "test4@test.com"


if __name__ == '__main__':
    unittest.main()
