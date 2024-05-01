# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
import logging
import unittest
from unittest.mock import Mock, patch

import cla
from cla import utils
from cla.models.dynamo_models import Project, Signature, User
from cla.utils import (append_email_help_sign_off_content,
                       append_project_version_to_url, get_email_help_content,
                       get_email_sign_off_content, get_full_sign_url)


class TestUtils(unittest.TestCase):
    tests_enabled = False

    @classmethod
    def setUpClass(cls) -> None:
        cls.mock_get_patcher = patch('cla.utils.requests.get')
        cls.mock_get = cls.mock_get_patcher.start()

    @classmethod
    def tearDownClass(cls) -> None:
        cls.mock_get_patcher.stop()

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

    def test_lookup_user_github_username(self) -> None:
        """
        Test that we can lookup a given gihub user by id
        """
        # TODO - should use mock data - disable tests for now :-(
        if self.tests_enabled:
            self.assertEqual('dealako', cla.utils.lookup_user_github_username(519609), 'Found github username')
            # some invalid username
            self.assertIsNone(cla.utils.lookup_user_github_username(5196090000), 'None response from invalid github id')

    def test_lookup_user_github_id(self) -> None:
        """
        Test that we can lookup a given gihub id by the username
        """
        # TODO - should use mock data - disable tests for now :-(
        if self.tests_enabled:
            self.assertEqual(519609, cla.utils.lookup_user_github_id('dealako'), 'Found github id')
            # some invalid username
            self.assertIsNone(cla.utils.lookup_user_github_id('dealakooooooooo'),
                              'None response from invalid github username')

    def test_lookup_github_organizations(self) -> None:
        """
        Test that we can lookup a user's github organizations
        """
        # TODO - should use mock data - disable tests for now :-(
        if self.tests_enabled:
            organizations = cla.utils.lookup_github_organizations('dealako')
            self.assertEqual(3, len(organizations), 'Find github organizations')
            # some invalid username
            organizations = cla.utils.lookup_github_organizations('dealakooooooooo')
            self.assertTrue('error' in organizations, 'Find 0 github organizations')

    def test_is_whitelisted_for_email(self) -> None:
        """
        Test a given email to check if whitelisted against ccla_signature
        """
        signature = Signature()
        signature.get_email_whitelist = Mock(return_value={"foo@gmail.com"})
        self.assertTrue(utils.is_approved(signature, email="foo@gmail.com"))
        self.assertFalse(utils.is_approved(signature, email="bar@gmail.com"))

    def test_is_whitelisted_for_domain(self) -> None:
        """
        Test a given email passes domain whitelist check against ccla_signature
        """
        signature = Signature()
        signature.get_domain_whitelist = Mock(return_value=[".gmail.com"])
        self.assertTrue(utils.is_approved(signature, email="random@gmail.com"))
        self.assertFalse(utils.is_approved(signature, email="foo@invalid.com"))

    def test_is_whitelisted_for_github(self) -> None:
        """
        Test given github user passes github whitelist check against ccla_signature
        """
        signature = Signature()
        signature.get_github_whitelist = Mock(return_value=['foo'])
        self.assertTrue(utils.is_approved(signature, github_username='foo'))
        self.assertFalse(utils.is_approved(signature, github_username='bar'))

    def test_is_whitelisted_for_github_org(self) -> None:
        """
        Test given github user passes github org check against ccla_signature
        """
        self.mock_get.return_value.ok = True
        github_orgs = [{
            'login': 'foo-org',
        }]
        self.mock_get.return_value = Mock()
        self.mock_get.return_value.json.return_value = github_orgs
        signature = Signature()
        signature.get_github_org_whitelist = Mock(return_value=['foo-org'])
        self.assertTrue(utils.is_approved(signature, github_username='foo'))


def test_append_email_help_sign_off_content():
    body = "hello John,"
    new_bod = append_email_help_sign_off_content(body, "v2")
    assert body in new_bod
    assert get_email_help_content(True) in new_bod
    assert get_email_sign_off_content() in new_bod

    new_body_v1 = append_email_help_sign_off_content(body, "v1")
    assert body in new_body_v1
    assert get_email_help_content(False) in new_body_v1
    assert get_email_sign_off_content() in new_body_v1


def test_get_full_sign_url():
    p = Project()
    p.set_version("v1")
    url = get_full_sign_url("github", "1234", 456, 1, p.get_version())
    assert "?version=1" in url

    p = Project()
    p.set_version("v2")
    url = get_full_sign_url("github", "1234", 456, 1, p.get_version())
    assert "?version=2" in url

    p = Project()
    url = get_full_sign_url("github", "1234", 456, 1, p.get_version())
    assert "?version=1" in url


def test_append_project_version_to_url():
    original_url = "http://localhost:5000/v1/sign"
    url = append_project_version_to_url(address=original_url, project_version="v1")
    print(url)
    assert "?version=1" in url
    assert original_url in url

    original_url = "http://localhost:5000/v1/sign"
    url = append_project_version_to_url(address=original_url, project_version="v2")
    print(url)
    assert "?version=2" in url
    assert "http://localhost:5000/v1/sign?version=2" == url
    assert original_url in url

    original_url = "http://localhost:5000/v1/sign"
    url = append_project_version_to_url(address=original_url, project_version=None)
    print(url)
    assert "?version=1" in url
    assert original_url in url

    original_url = "http://localhost:5000/v1/sign"
    url = append_project_version_to_url(address=original_url, project_version="invalid")
    print(url)
    assert "?version=1" in url
    assert original_url in url

    original_url = "http://localhost:5000/v1/sign?something=else"
    url = append_project_version_to_url(address=original_url, project_version="v2")
    print(url)
    assert "version=2" in url
    assert "something=else" in url
    assert original_url in url

    original_url = "http://localhost:5000/v1/sign?version=1"
    url = append_project_version_to_url(address=original_url, project_version="v2")
    print(url)
    assert "version=2" not in url
    assert "version=1" in url
    assert original_url in url

    original_url = "http://localhost:5000/v1/sign?something=else&version=1"
    url = append_project_version_to_url(address=original_url, project_version="v2")
    print(url)
    assert "version=2" not in url
    assert "version=1" in url
    assert "something=else" in url
    assert original_url in url

    # try the weird case with # in url
    original_url = "https://dev.lfcla.com/#/"
    url = append_project_version_to_url(address=original_url, project_version="v2")
    print(url)
    assert "version=2" in url
    assert "version=1" not in url
    assert original_url in url

    original_url = "https://dev.lfcla.com/#/"
    url = append_project_version_to_url(address=original_url, project_version="")
    print(url)
    assert "version=1" in url
    assert "version=2" not in url
    assert original_url in url

    original_url = "https://dev.lfcla.com/#/"
    url = append_project_version_to_url(address=original_url, project_version=None)
    print(url)
    assert "version=1" in url
    assert "version=2" not in url
    assert original_url in url

    original_url= "https://dev.lfcla.com/#/#/?something=else"
    url = append_project_version_to_url(address=original_url, project_version="")
    print(url)
    assert "version=1" in url
    assert "something=else" in url
    assert "version=2" not in url
    assert original_url in url

    # check for crazier example ...
    original_url = "https://dev.lfcla.com/1/#/2/#/3/#/?something=else&this=that"
    url = append_project_version_to_url(address=original_url, project_version="")
    print(url)
    assert "version=1" in url
    assert "something=else" in url
    assert "this=that" in url
    assert "version=2" not in url
    assert original_url in url


if __name__ == '__main__':
    unittest.main()
