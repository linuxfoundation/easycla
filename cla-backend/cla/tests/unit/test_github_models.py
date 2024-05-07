# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

import unittest
from unittest import TestCase
from unittest.mock import MagicMock, Mock, patch

from cla.models.github_models import (UserCommitSummary, get_author_summary,
                                      get_co_author_commits,
                                      get_pull_request_commit_authors)


class TestGetPullRequestCommitAuthors(TestCase):
    @patch("cla.utils.get_repository_service")
    def test_get_pull_request_commit_with_co_author(self, mock_github_instance):
        # Mock data
        pull_request = MagicMock()
        pull_request.number = 123
        co_author = "co_author"
        co_author_email = "co_author_email.gmail.com"
        co_author_2 = "co_author_2"
        co_author_email_2 = "co_author_email_2.gmail.com"
        commit = MagicMock()
        commit.sha = "fake_sha"
        commit.author = MagicMock()
        commit.author.id = 1
        commit.author.login = "fake_login"
        commit.author.name = "Fake Author"
        commit.commit.message = f"fake message\n\nCo-authored-by: {co_author} <{co_author_email}>\n\nCo-authored-by: {co_author_2} <{co_author_email_2}>"

        commit.author.email = "fake_author@example.com"
        pull_request.get_commits.return_value.__iter__.return_value = [commit]

        mock_user = Mock(id=2, login="co_author_login")
        mock_user_2 = Mock(id=3, login="co_author_login_2")

        mock_github_instance.return_value.get_github_user_by_email.side_effect = (
            lambda email, _: mock_user if email == co_author_email else mock_user_2
        )

        # Call the function
        result = get_pull_request_commit_authors(pull_request, "fake_installation_id")

        # Assertions
        self.assertEqual(len(result), 3)
        self.assertIn(co_author_email, [author.author_email for author in result])
        self.assertIn(co_author_email_2, [author.author_email for author in result])
        self.assertIn("fake_login", [author.author_login for author in result])
        self.assertIn("co_author_login", [author.author_login for author in result])
    
    @patch("cla.utils.get_repository_service")
    def test_get_co_author_commits_invalid_gh_email(self, mock_github_instance):
        # Mock data
        co_author = ("co_author", "co_author_email.gmail.com")
        commit = MagicMock()
        commit.sha = "fake_sha"
        mock_github_instance.return_value.get_github_user_by_email.return_value = None
        pr = 1
        installation_id = 123

        # Call the function
        result = get_co_author_commits(co_author,commit, pr, installation_id)

        # Assertions
        self.assertEqual(result.commit_sha, "fake_sha")
        self.assertEqual(result.author_id, None)
        self.assertEqual(result.author_login, None)
        self.assertEqual(result.author_email, "co_author_email.gmail.com")
        self.assertEqual(result.author_name, "co_author")
    
    @patch("cla.utils.get_repository_service")
    def test_get_co_author_commits_valid_gh_email(self, mock_github_instance):
        # Mock data
        co_author = ("co_author", "co_author_email.gmail.com")
        commit = MagicMock()
        commit.sha = "fake_sha"
        mock_github_instance.return_value.get_github_user_by_email.return_value = Mock(
            id=123, login="co_author_login"
        )
        pr = 1
        installation_id = 123

        # Call the function
        result = get_co_author_commits(co_author,commit, pr, installation_id)

        # Assertions
        self.assertEqual(result.commit_sha, "fake_sha")
        self.assertEqual(result.author_id, 123)
        self.assertEqual(result.author_login, "co_author_login")
        self.assertEqual(result.author_email, "co_author_email.gmail.com")
        self.assertEqual(result.author_name, "co_author")


if __name__ == "__main__":
    unittest.main()
