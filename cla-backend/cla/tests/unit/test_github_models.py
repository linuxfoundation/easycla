# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

import unittest
from unittest import TestCase
from unittest.mock import MagicMock, Mock, patch

from cla.models.github_models import (UserCommitSummary, get_author_summary,
                                      get_co_author_commits, github_user_cache,
                                      get_pull_request_commit_authors,
                                      iter_pr_commits_compare)

class TestGetPullRequestCommitAuthors(TestCase):
    def setUp(self):
        # Clear the GitHub user cache before each test to avoid cross-test pollution
        with github_user_cache.lock:
            github_user_cache.data.clear()
    # @patch("cla.utils.get_repository_service")
    # def test_get_pull_request_commit_with_co_author(self, mock_github_instance):
    #     # Mock data
    #     pull_request = MagicMock()
    #     pull_request.number = 123
    #     co_author = "co_author"
    #     co_author_email = "co_author_email.gmail.com"
    #     co_author_2 = "co_author_2"
    #     co_author_email_2 = "co_author_email_2.gmail.com"
    #     commit = MagicMock()
    #     commit.sha = "fake_sha"
    #     commit.author = MagicMock()
    #     commit.author.id = 1
    #     commit.author.login = "fake_login"
    #     commit.author.name = "Fake Author"
    #     commit.commit.message = f"fake message\n\nCo-authored-by: {co_author} <{co_author_email}>\n\nCo-authored-by: {co_author_2} <{co_author_email_2}>"

    #     commit.author.email = "fake_author@example.com"
    #     pull_request.get_commits.return_value.__iter__.return_value = [commit]

    #     mock_user = Mock(id=2, login="co_author_login")
    #     mock_user_2 = Mock(id=3, login="co_author_login_2")

    #     mock_github_instance.return_value.get_github_user_by_email.side_effect = (
    #         lambda email, _: mock_user if email == co_author_email else mock_user_2
    #     )

    #     # Call the function
    #     result = get_pull_request_commit_authors(pull_request, "fake_installation_id")

    #     # Assertions
    #     self.assertEqual(len(result), 3)
    #     self.assertIn(co_author_email, [author.author_email for author in result])
    #     self.assertIn(co_author_email_2, [author.author_email for author in result])
    #     self.assertIn("fake_login", [author.author_login for author in result])
    #     self.assertIn("co_author_login", [author.author_login for author in result])
    
    @patch("cla.utils.get_repository_service")
    def test_get_co_author_commits_invalid_gh_email(self, mock_github_instance):
        # Mock data
        co_author = ("co_author", "co_author_email.gmail.com")
        commit = MagicMock()
        commit.sha = "fake_sha"
        mock_github_instance.return_value.get_github_user_by_email.return_value = None
        mock_github_instance.return_value.get_github_user_by_login.return_value = None
        pr = 1
        installation_id = 123

        # Call the function
        result, _ = get_co_author_commits(co_author, commit.sha, pr, installation_id)

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
        mock_github_instance.return_value.get_github_user_by_login.return_value = None
        mock_github_instance.return_value.get_github_user_by_email.return_value = Mock(
            id=123, login="co_author_login"
        )
        pr = 1
        installation_id = 123

        # Call the function
        result, _ = get_co_author_commits(co_author, commit.sha, pr, installation_id)

        # Assertions
        self.assertEqual(result.commit_sha, "fake_sha")
        self.assertEqual(result.author_id, 123)
        self.assertEqual(result.author_login, "co_author_login")
        self.assertEqual(result.author_email, "co_author_email.gmail.com")
        self.assertEqual(result.author_name, "co_author")

    @patch("cla.models.github_models.time.sleep")
    def test_iter_pr_commits_compare_retries_then_succeeds(self, mock_sleep):
        g = MagicMock()
        repo = MagicMock()
        pr = MagicMock()
        pr.base.sha = "base_sha"
        pr.head.sha = "head_sha"
        repo.get_pull.return_value = pr
        g.get_repo.return_value = repo

        requester = MagicMock()
        g._Github__requester = requester
        requester.requestJsonAndCheck.side_effect = [
            Exception("boom-1"),
            Exception("boom-2"),
            (
                {},
                {
                    "commits": [
                        {
                            "sha": "abc123",
                            "author": {
                                "id": 1,
                                "login": "login1",
                                "name": "Profile Name",
                                "email": "profile@example.com",
                            },
                            "commit": {
                                "message": "hello",
                                "author": {
                                    "name": "Commit Name",
                                    "email": "commit@example.com",
                                },
                            },
                        }
                    ]
                },
            ),
            ({}, {"commits": []}),
        ]

        commits = list(iter_pr_commits_compare(g, "o", "r", 7))

        self.assertEqual(len(commits), 1)
        self.assertEqual(commits[0].sha, "abc123")
        self.assertEqual(commits[0].author_id, 1)
        self.assertEqual(commits[0].author_login, "login1")
        self.assertEqual(commits[0].author_name, "Commit Name")
        self.assertEqual(commits[0].author_email, "profile@example.com")
        mock_sleep.assert_called_once_with(1)

    @patch("cla.models.github_models.iter_pr_commits_compare", side_effect=Exception("compare failed"))
    @patch("cla.models.github_models.get_pr_commit_count_gql", return_value=None)
    def test_get_pull_request_commit_authors_blocks_unsafe_rest_fallback_for_truncated_pr(
        self,
        _mock_count,
        _mock_iter_compare,
    ):
        client = MagicMock()
        pull_request = MagicMock()
        pull_request.number = 123
        pull_request.base.repo.name = "repo"

        pr_commits = MagicMock()
        pr_commits.totalCount = 250
        pull_request.get_commits.return_value = pr_commits

        with self.assertRaises(ValueError):
            get_pull_request_commit_authors(client, "org", pull_request, 1, False)


if __name__ == "__main__":
    unittest.main()
