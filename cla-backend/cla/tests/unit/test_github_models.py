# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
import logging
import unittest
from unittest.mock import patch, MagicMock

from github import Github

import cla
from cla.models.dynamo_models import Signature, Project
from cla.models.github_models import GitHub as GithubModel
from cla.models.github_models import get_pull_request_commit_authors, handle_commit_from_user, MockGitHub
from cla.user import UserCommitSummary


class TestGitHubModels(unittest.TestCase):

    @classmethod
    def setUpClass(cls) -> None:
        cls.mock_user_patcher = patch('cla.models.github_models.cla.utils.get_user_instance')
        cls.mock_signature_patcher = patch('cla.models.github_models.cla.utils.get_signature_instance')
        cls.mock_utils_patcher = patch('cla.models.github_models.cla.utils')
        cls.mock_utils_get = cls.mock_utils_patcher.start()
        cls.mock_user_get = cls.mock_user_patcher.start()
        cls.mock_signature_get = cls.mock_signature_patcher.start()

    @classmethod
    def tearDownClass(cls) -> None:
        cls.mock_user_patcher.stop()
        cls.mock_signature_patcher.stop()
        cls.mock_utils_patcher.stop()

    def setUp(self) -> None:
        # Only show critical logging stuff
        cla.log.level = logging.CRITICAL
        #self.assertTrue(cla.conf['GITHUB_OAUTH_TOKEN'] != '',
        #                'Missing GITHUB_OAUTH_TOKEN environment variable - required to run unit tests')
        # cla.log.debug('Using GITHUB_OAUTH_TOKEN: {}...'.format(cla.conf['GITHUB_OAUTH_TOKEN'][:5]))

    def tearDown(self) -> None:
        pass

    @unittest.skip("todo - need to mock GitHub service")
    def test_commit_authors_with_named_user(self) -> None:
        """
        Test that we can load commit authors from a pull request that does have the traditional
        github.NamedUser.NamedUser object filled out
        """
        g = Github(cla.conf['GITHUB_OAUTH_TOKEN'])
        repo = g.get_repo(27729926)  # grpc/grpc-java
        pr = repo.get_pull(6142)  # example: https://github.com/grpc/grpc-java/pull/6142
        cla.log.info("Retrieved GitHub PR: {}".format(pr))
        commits = pr.get_comments()
        cla.log.info("Retrieved GitHub PR: {}, commits: {}".format(pr, commits))

        # Returns a list tuples, which look like (commit_sha_string, (author_id, author_username, author_email),
        # which, as you can see, the second element of the tuple is another tuple containing the author information
        commit_authors = get_pull_request_commit_authors(pr)
        # cla.log.info("Result: {}".format(commit_authors))
        # cla.log.info([author_info[1] for commit, author_info in commit_authors])
        self.assertTrue(4779759 in [user_commit_summary.author_id for user_commit_summary in commit_authors])

    @unittest.skip("todo - need to mock GitHub service")
    def test_commit_authors_no_named_user(self) -> None:
        """
        Test that we can load commit authors from a pull request that does NOT have the traditional
        github.NamedUser.NamedUser object filled out
        """
        # We need to mock this service so that we can test our business logic - disabling this test for now
        # as they closed the PR
        g = Github(cla.conf['GITHUB_OAUTH_TOKEN'])
        repo = g.get_repo(27729926)  # grpc/grpc-java
        pr = repo.get_pull(6152)  # example: https://github.com/grpc/grpc-java/pull/6152
        cla.log.info("Retrieved GitHub PR: {}".format(pr))
        commits = pr.get_comments()
        cla.log.info("Retrieved GitHub PR: {}, commits: {}".format(pr, commits))

        # Returns a list tuples, which look like (commit_sha_string, (author_id, author_username, author_email),
        # which, as you can see, the second element of the tuple is another tuple containing the author information
        # commit_authors = get_pull_request_commit_authors(pr)
        # cla.log.info("Result: {}".format(commit_authors))
        # cla.log.info([author_info[1] for commit, author_info in commit_authors])
        # self.assertTrue('snalkar' in [author_info[1] for commit, author_info in commit_authors])

    def test_handle_commit_author_whitelisted(self) -> None:
        """
        Test case where commit authors have no signatures but have been whitelisted and should
        return missing list containing a whitelisted flag
        """
        # Mock user not existing and happens to be whitelisted
        self.mock_user_get.return_value.get_user_by_github_id.return_value = None
        self.mock_user_get.return_value.get_user_by_email.return_value = None
        self.mock_signature_get.return_value.get_signatures_by_project.return_value = [Signature()]
        self.mock_utils_get.return_value.is_approved.return_value = True
        user_commit_summary = UserCommitSummary('fake_sha', 123, 'foo', None, 'foo@gmail.com', True, True)
        missing = []
        signed = []
        project = Project()
        project.set_project_id('fake_project_id')
        handle_commit_from_user(project, user_commit_summary, signed, missing)
        # We commented out this functionality for now - re-enable if we add it back
        self.assertEqual(missing, [user_commit_summary])
        self.assertEqual(signed, [])

    def test_handle_invalid_author(self) -> None:
        """
        Test case handling non-existent author tagged to a given commit
        """
        project = Project()
        author_info = UserCommitSummary('fake_sha', None, None, None, None, False, False)
        signed = []
        missing = []
        handle_commit_from_user(project, author_info, signed, missing)
        self.assertEqual(signed, [])
        self.assertEqual(missing, [author_info])


class TestGithubModelsPrComment(unittest.TestCase):

    def setUp(self) -> None:
        self.github = MockGitHub()
        self.github.update_change_request = MagicMock()

    def tearDown(self) -> None:
        pass

    def test_process_easycla_command_comment(self):
        with self.assertRaisesRegex(ValueError, "missing comment body"):
            self.github.process_easycla_command_comment({})

        with self.assertRaisesRegex(ValueError, "unsupported comment supplied"):
            self.github.process_easycla_command_comment({
                "comment": {"body": "/otherbot"}
            })

        with self.assertRaisesRegex(ValueError, "missing github repository id"):
            self.github.process_easycla_command_comment({
                "comment": {"body": "/easycla"},
            })

        with self.assertRaisesRegex(ValueError, "missing pull request id"):
            self.github.process_easycla_command_comment({
                "comment": {"body": "/easycla"},
                "repository": {"id": 123},
            })

        with self.assertRaisesRegex(ValueError, "missing installation id"):
            self.github.process_easycla_command_comment({
                "comment": {"body": "/easycla"},
                "repository": {"id": 123},
                "issue": {"number": 1},
            })

        self.github.process_easycla_command_comment({
            "comment": {"body": "/easycla"},
            "repository": {"id": 123},
            "issue": {"number": 1},
            "installation": {"id": 1},
        })


class TestGithubUserEmails(unittest.TestCase):

    def test_empty_emails(self):
        with patch.object(GithubModel, "_fetch_github_emails") as _fetch_github_emails:
            _fetch_github_emails.return_value = []
            github = GithubModel()
            emails = github.get_user_emails(None, "fake_client_id")
            assert not emails

    def test_emails_with_noreply(self):
        with patch.object(GithubModel, "_fetch_github_emails") as _fetch_github_emails:
            _fetch_github_emails.return_value = [
                {
                    "email": "octocat@users.noreply.github.com",
                    "verified": True,
                    "primary": True,
                    "visibility": "public"
                },
                {
                    "email": "pumacat@gmail.com",
                    "verified": True,
                    "primary": True,
                    "visibility": "public"
                },
                {
                    "email": "pumacat+notveried@gmail.com",
                    "verified": False,
                    "primary": True,
                    "visibility": "public"
                }
            ]
            github = GithubModel()
            emails = github.get_user_emails(None, "fake_client_id")
            assert emails
            assert len(emails) == 1
            assert emails == ["pumacat@gmail.com"]

    def test_emails_with_noreply_single(self):
        with patch.object(GithubModel, "_fetch_github_emails") as _fetch_github_emails:
            _fetch_github_emails.return_value = [
                {
                    "email": "octocat@users.noreply.github.com",
                    "verified": True,
                    "primary": True,
                    "visibility": "public"
                },
            ]
            github = GithubModel()
            emails = github.get_user_emails(None, "fake_client_id")
            assert emails
            assert len(emails) == 1
            assert emails == ["octocat@users.noreply.github.com"]

    def test_emails_without_noreply(self):
        with patch.object(GithubModel, "_fetch_github_emails") as _fetch_github_emails:
            _fetch_github_emails.return_value = [
                {
                    "email": "pumacat@gmail.com",
                    "verified": True,
                    "primary": True,
                    "visibility": "public"
                },
                {
                    "email": "pumacat2@gmail.com",
                    "verified": True,
                    "primary": True,
                    "visibility": "public"
                },
                {
                    "email": "pumacat+notveried@gmail.com",
                    "verified": False,
                    "primary": True,
                    "visibility": "public"
                }
            ]
            github = GithubModel()
            emails = github.get_user_emails(None, "fake_client_id")
            assert emails
            assert len(emails) == 2
            assert "pumacat@gmail.com" in emails
            assert "pumacat2@gmail.com" in emails


if __name__ == '__main__':
    unittest.main()
