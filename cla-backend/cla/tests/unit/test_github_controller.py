# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
import logging
import unittest
from unittest.mock import Mock

import cla
from cla.controllers.github import (get_github_activity_action,
                                    get_org_name_from_installation_event,
                                    notify_project_managers)
from cla.controllers.repository import Repository
from cla.models.ses_models import MockSES


class TestGitHubController(unittest.TestCase):
    example_1 = {
        'action': 'created',
        'installation': {
            'id': 2,
            'account': {
                'login': 'Linux Foundation',
                'id': 1,
                'node_id': 'MDQ6VXNlcjE=',
                'avatar_url': 'https://github.com/images/error/octocat_happy.gif',
                'gravatar_id': '',
                'url': 'https://api.github.com/users/octocat',
                'html_url': 'https://github.com/octocat',
                'followers_url': 'https://api.github.com/users/octocat/followers',
                'following_url': 'https://api.github.com/users/octocat/following{/other_user}',
                'gists_url': 'https://api.github.com/users/octocat/gists{/gist_id}',
                'starred_url': 'https://api.github.com/users/octocat/starred{/owner}{/repo}',
                'subscriptions_url': 'https://api.github.com/users/octocat/subscriptions',
                'organizations_url': 'https://api.github.com/users/octocat/orgs',
                'repos_url': 'https://api.github.com/users/octocat/repos',
                'events_url': 'https://api.github.com/users/octocat/events{/privacy}',
                'received_events_url': 'https://api.github.com/users/octocat/received_events',
                'type': 'User',
                'site_admin': False
            },
            'repository_selection': 'selected',
            'access_tokens_url': 'https://api.github.com/installations/2/access_tokens',
            'repositories_url': 'https://api.github.com/installation/repositories',
            'html_url': 'https://github.com/settings/installations/2',
            'app_id': 5725,
            'target_id': 3880403,
            'target_type': 'User',
            'permissions': {
                'metadata': 'read',
                'contents': 'read',
                'issues': 'write'
            },
            'events': [
                'push',
                'pull_request'
            ],
            'created_at': 1525109898,
            'updated_at': 1525109899,
            'single_file_name': 'config.yml'
        }
    }

    example_2 = {
        "action": "created",
        "comment": {
            "url": "https://api.github.com/repos/grpc/grpc/pulls/comments/134346",
            "pull_request_review_id": 134346,
            "id": 134346,
            "node_id": "MDI0OlB1bGxSZXF1ZXN0UmVredacted==",
            "path": "setup.py",
            "position": 16,
            "original_position": 17,
            "commit_id": "4bc9820redacted",
            "original_commit_id": "d5515redacted",
            "user": {
                "login": "redacted",
                "id": 134566,
                "node_id": "MDQ6VXNlcjI3OTMyODI=",
                "avatar_url": "https://avatars3.githubusercontent.com/u/2793282?v=4",
                "gravatar_id": "",
                "url": "https://api.github.com/users/veblush",
                "html_url": "https://github.com/veblush",
                "followers_url": "https://api.github.com/users/veblush/followers",
                "following_url": "https://api.github.com/users/veblush/following{/other_user}",
                "gists_url": "https://api.github.com/users/veblush/gists{/gist_id}",
                "starred_url": "https://api.github.com/users/veblush/starred{/owner}{/repo}",
                "subscriptions_url": "https://api.github.com/users/veblush/subscriptions",
                "organizations_url": "https://api.github.com/users/veblush/orgs",
                "repos_url": "https://api.github.com/users/veblush/repos",
                "events_url": "https://api.github.com/users/veblush/events{/privacy}",
                "received_events_url": "https://api.github.com/users/veblush/received_events",
                "type": "User",
                "site_admin": False
            },
            "pull_request_url": "https://api.github.com/repos/grpc/grpc/pulls/134566",
            "author_association": "CONTRIBUTOR",
            "_links": {
                "self": {
                    "href": "https://api.github.com/repos/grpc/grpc/pulls/comments/134566"
                },
                "html": {
                    "href": "https://github.com/grpc/grpc/pull/20414#discussion_r134566"
                },
                "pull_request": {
                    "href": "https://api.github.com/repos/grpc/grpc/pulls/134566"
                }
            },
            "in_reply_to_id": 1345667
        },
        "pull_request": {
            "url": "https://api.github.com/repos/grpc/grpc/pulls/20414",
            "id": 134566,
            "node_id": "MDExxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
            "html_url": "https://github.com/grpc/grpc/pull/20414",
            "diff_url": "https://github.com/grpc/grpc/pull/20414.diff",
            "patch_url": "https://github.com/grpc/grpc/pull/20414.patch",
            "issue_url": "https://api.github.com/repos/grpc/grpc/issues/20414",
            "number": 134566,
            "state": "open",
            "locked": False,
            "title": "Added lib to gRPC python",
            "user": {
                "login": "redacted",
                "id": 12345677,
                "node_id": "MDQ6666666666666666=",
                "avatar_url": "https://avatars3.githubusercontent.com/u/2793282?v=4",
                "gravatar_id": "",
                "url": "https://api.github.com/users/veblush",
                "html_url": "https://github.com/veblush",
                "followers_url": "https://api.github.com/users/veblush/followers",
                "following_url": "https://api.github.com/users/veblush/following{/other_user}",
                "gists_url": "https://api.github.com/users/veblush/gists{/gist_id}",
                "starred_url": "https://api.github.com/users/veblush/starred{/owner}{/repo}",
                "subscriptions_url": "https://api.github.com/users/veblush/subscriptions",
                "organizations_url": "https://api.github.com/users/veblush/orgs",
                "repos_url": "https://api.github.com/users/veblush/repos",
                "events_url": "https://api.github.com/users/veblush/events{/privacy}",
                "received_events_url": "https://api.github.com/users/veblush/received_events",
                "type": "User",
                "site_admin": False
            },
            "body": "Try to fix #20400 and #20174",
            "created_at": "2019-10-01T06:08:53Z",
            "updated_at": "2019-10-07T18:19:12Z",
            "closed_at": None,
            "merged_at": None,
            "merge_commit_sha": "5bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
            "assignee": None,
            "assignees": [],
            "requested_reviewers": [],
            "requested_teams": [],
            "labels": [
                {
                    "id": 12345,
                    "node_id": "MDU66llllllllllllllllll=",
                    "url": "https://api.github.com/repos/grpc/grpc/labels/area/build",
                    "name": "area/build",
                    "color": "efdb40",
                    "default": False
                },
                {
                    "id": 12345,
                    "node_id": "MDU66666666666666666666=",
                    "url": "https://api.github.com/repos/grpc/grpc/labels/lang/Python",
                    "name": "lang/Python",
                    "color": "fad8c7",
                    "default": False
                },
                {
                    "id": 12345677,
                    "node_id": "MDUuuuuuuuuuuuuuuuuuuuu=",
                    "url": "https://api.github.com/repos/grpc/grpc/labels/release%20notes:%20no",
                    "name": "release notes: no",
                    "color": "0f5f75",
                    "default": False
                }
            ],
            "milestone": None,
            "commits_url": "https://api.github.com/repos/grpc/grpc/pulls/1234/commits",
            "review_comments_url": "https://api.github.com/repos/grpc/grpc/pulls/12345/comments",
            "review_comment_url": "https://api.github.com/repos/grpc/grpc/pulls/comments{/number}",
            "comments_url": "https://api.github.com/repos/grpc/grpc/issues/12345/comments",
            "statuses_url": "https://api.github.com/repos/grpc/grpc/statuses/4444444444444444444444444444444444444444",
            "head": {
                "label": "redacted:fix-xyz",
                "ref": "fix-xyz",
                "sha": "4444444444444444444444444444444444444444",
                "user": {
                    "login": "redacted",
                    "id": 1234556,
                    "node_id": "MDQ66llllllllllllll=",
                    "avatar_url": "https://avatars3.githubusercontent.com/u/2793282?v=4",
                    "gravatar_id": "",
                    "url": "https://api.github.com/users/veblush",
                    "html_url": "https://github.com/veblush",
                    "followers_url": "https://api.github.com/users/veblush/followers",
                    "following_url": "https://api.github.com/users/veblush/following{/other_user}",
                    "gists_url": "https://api.github.com/users/veblush/gists{/gist_id}",
                    "starred_url": "https://api.github.com/users/veblush/starred{/owner}{/repo}",
                    "subscriptions_url": "https://api.github.com/users/veblush/subscriptions",
                    "organizations_url": "https://api.github.com/users/veblush/orgs",
                    "repos_url": "https://api.github.com/users/veblush/repos",
                    "events_url": "https://api.github.com/users/veblush/events{/privacy}",
                    "received_events_url": "https://api.github.com/users/veblush/received_events",
                    "type": "User",
                    "site_admin": False
                },
                "repo": {
                    "id": 123456789,
                    "node_id": "MDEwwwwwwwwwwwwwwwwwwwwwwwwwwww=",
                    "name": "grpc",
                    "full_name": "redacted/grpc",
                    "private": False,
                    "owner": {
                        "login": "redacted",
                        "id": 1234567,
                        "node_id": "MDQ6666666666666666=",
                        "avatar_url": "https://avatars3.githubusercontent.com/u/2793282?v=4",
                        "gravatar_id": "",
                        "url": "https://api.github.com/users/veblush",
                        "html_url": "https://github.com/veblush",
                        "followers_url": "https://api.github.com/users/veblush/followers",
                        "following_url": "https://api.github.com/users/veblush/following{/other_user}",
                        "gists_url": "https://api.github.com/users/veblush/gists{/gist_id}",
                        "starred_url": "https://api.github.com/users/veblush/starred{/owner}{/repo}",
                        "subscriptions_url": "https://api.github.com/users/veblush/subscriptions",
                        "organizations_url": "https://api.github.com/users/veblush/orgs",
                        "repos_url": "https://api.github.com/users/veblush/repos",
                        "events_url": "https://api.github.com/users/veblush/events{/privacy}",
                        "received_events_url": "https://api.github.com/users/veblush/received_events",
                        "type": "User",
                        "site_admin": False
                    },
                    "html_url": "https://github.com/veblush/grpc",
                    "description": "The C based gRPC (C++, Python, Ruby, Objective-C, PHP, C#)",
                    "fork": True,
                    "url": "https://api.github.com/repos/veblush/grpc",
                    "forks_url": "https://api.github.com/repos/veblush/grpc/forks",
                    "keys_url": "https://api.github.com/repos/veblush/grpc/keys{/key_id}",
                    "collaborators_url": "https://api.github.com/repos/veblush/grpc/collaborators{/collaborator}",
                    "teams_url": "https://api.github.com/repos/veblush/grpc/teams",
                    "hooks_url": "https://api.github.com/repos/veblush/grpc/hooks",
                    "issue_events_url": "https://api.github.com/repos/veblush/grpc/issues/events{/number}",
                    "events_url": "https://api.github.com/repos/veblush/grpc/events",
                    "assignees_url": "https://api.github.com/repos/veblush/grpc/assignees{/user}",
                    "branches_url": "https://api.github.com/repos/veblush/grpc/branches{/branch}",
                    "tags_url": "https://api.github.com/repos/veblush/grpc/tags",
                    "blobs_url": "https://api.github.com/repos/veblush/grpc/git/blobs{/sha}",
                    "git_tags_url": "https://api.github.com/repos/veblush/grpc/git/tags{/sha}",
                    "git_refs_url": "https://api.github.com/repos/veblush/grpc/git/refs{/sha}",
                    "trees_url": "https://api.github.com/repos/veblush/grpc/git/trees{/sha}",
                    "statuses_url": "https://api.github.com/repos/veblush/grpc/statuses/{sha}",
                    "languages_url": "https://api.github.com/repos/veblush/grpc/languages",
                    "stargazers_url": "https://api.github.com/repos/veblush/grpc/stargazers",
                    "contributors_url": "https://api.github.com/repos/veblush/grpc/contributors",
                    "subscribers_url": "https://api.github.com/repos/veblush/grpc/subscribers",
                    "subscription_url": "https://api.github.com/repos/veblush/grpc/subscription",
                    "commits_url": "https://api.github.com/repos/veblush/grpc/commits{/sha}",
                    "git_commits_url": "https://api.github.com/repos/veblush/grpc/git/commits{/sha}",
                    "comments_url": "https://api.github.com/repos/veblush/grpc/comments{/number}",
                    "issue_comment_url": "https://api.github.com/repos/veblush/grpc/issues/comments{/number}",
                    "contents_url": "https://api.github.com/repos/veblush/grpc/contents/{+path}",
                    "compare_url": "https://api.github.com/repos/veblush/grpc/compare/{base}...{head}",
                    "merges_url": "https://api.github.com/repos/veblush/grpc/merges",
                    "archive_url": "https://api.github.com/repos/veblush/grpc/{archive_format}{/ref}",
                    "downloads_url": "https://api.github.com/repos/veblush/grpc/downloads",
                    "issues_url": "https://api.github.com/repos/veblush/grpc/issues{/number}",
                    "pulls_url": "https://api.github.com/repos/veblush/grpc/pulls{/number}",
                    "milestones_url": "https://api.github.com/repos/veblush/grpc/milestones{/number}",
                    "notifications_url": "https://api.github.com/repos/veblush/grpc/notifications{?since,all,participating}",
                    "labels_url": "https://api.github.com/repos/veblush/grpc/labels{/name}",
                    "releases_url": "https://api.github.com/repos/veblush/grpc/releases{/id}",
                    "deployments_url": "https://api.github.com/repos/veblush/grpc/deployments",
                    "created_at": "2019-04-12T16:55:24Z",
                    "updated_at": "2019-10-03T17:32:29Z",
                    "pushed_at": "2019-10-05T03:41:45Z",
                    "git_url": "git://github.com/veblush/grpc.git",
                    "ssh_url": "git@github.com:veblush/grpc.git",
                    "clone_url": "https://github.com/veblush/grpc.git",
                    "svn_url": "https://github.com/veblush/grpc",
                    "homepage": "https://grpc.io",
                    "size": 218962,
                    "stargazers_count": 0,
                    "watchers_count": 0,
                    "language": "C++",
                    "has_issues": False,
                    "has_projects": True,
                    "has_downloads": False,
                    "has_wiki": True,
                    "has_pages": False,
                    "forks_count": 0,
                    "mirror_url": None,
                    "archived": False,
                    "disabled": False,
                    "open_issues_count": 0,
                    "license": {
                        "key": "apache-2.0",
                        "name": "Apache License 2.0",
                        "spdx_id": "Apache-2.0",
                        "url": "https://api.github.com/licenses/apache-2.0",
                        "node_id": "MDccccccccccccc="
                    },
                    "forks": 0,
                    "open_issues": 0,
                    "watchers": 0,
                    "default_branch": "master"
                }
            },
            "base": {
                "label": "grpc:master",
                "ref": "master",
                "sha": "9999999999999999999999999999999999999999",
                "user": {
                    "login": "grpc",
                    "id": 7802525,
                    "node_id": "MDEyyyyyyyyyyyyyyyyyyyyyyyyyyyy=",
                    "avatar_url": "https://avatars1.githubusercontent.com/u/7802525?v=4",
                    "gravatar_id": "",
                    "url": "https://api.github.com/users/grpc",
                    "html_url": "https://github.com/grpc",
                    "followers_url": "https://api.github.com/users/grpc/followers",
                    "following_url": "https://api.github.com/users/grpc/following{/other_user}",
                    "gists_url": "https://api.github.com/users/grpc/gists{/gist_id}",
                    "starred_url": "https://api.github.com/users/grpc/starred{/owner}{/repo}",
                    "subscriptions_url": "https://api.github.com/users/grpc/subscriptions",
                    "organizations_url": "https://api.github.com/users/grpc/orgs",
                    "repos_url": "https://api.github.com/users/grpc/repos",
                    "events_url": "https://api.github.com/users/grpc/events{/privacy}",
                    "received_events_url": "https://api.github.com/users/grpc/received_events",
                    "type": "Organization",
                    "site_admin": False
                },
                "repo": {
                    "id": 27729880,
                    "node_id": "MDEwwwwwwwwwwwwwwwwwwwwwwwwwww==",
                    "name": "grpc",
                    "full_name": "grpc/grpc",
                    "private": False,
                    "owner": {
                        "login": "grpc",
                        "id": 7802525,
                        "node_id": "MDEyyyyyyyyyyyyyyyyyyyyyyyyyyyy=",
                        "avatar_url": "https://avatars1.githubusercontent.com/u/7802525?v=4",
                        "gravatar_id": "",
                        "url": "https://api.github.com/users/grpc",
                        "html_url": "https://github.com/grpc",
                        "followers_url": "https://api.github.com/users/grpc/followers",
                        "following_url": "https://api.github.com/users/grpc/following{/other_user}",
                        "gists_url": "https://api.github.com/users/grpc/gists{/gist_id}",
                        "starred_url": "https://api.github.com/users/grpc/starred{/owner}{/repo}",
                        "subscriptions_url": "https://api.github.com/users/grpc/subscriptions",
                        "organizations_url": "https://api.github.com/users/grpc/orgs",
                        "repos_url": "https://api.github.com/users/grpc/repos",
                        "events_url": "https://api.github.com/users/grpc/events{/privacy}",
                        "received_events_url": "https://api.github.com/users/grpc/received_events",
                        "type": "Organization",
                        "site_admin": False
                    },
                    "html_url": "https://github.com/grpc/grpc",
                    "description": "The C based gRPC (C++, Python, Ruby, Objective-C, PHP, C#)",
                    "fork": False,
                    "url": "https://api.github.com/repos/grpc/grpc",
                    "forks_url": "https://api.github.com/repos/grpc/grpc/forks",
                    "keys_url": "https://api.github.com/repos/grpc/grpc/keys{/key_id}",
                    "collaborators_url": "https://api.github.com/repos/grpc/grpc/collaborators{/collaborator}",
                    "teams_url": "https://api.github.com/repos/grpc/grpc/teams",
                    "hooks_url": "https://api.github.com/repos/grpc/grpc/hooks",
                    "issue_events_url": "https://api.github.com/repos/grpc/grpc/issues/events{/number}",
                    "events_url": "https://api.github.com/repos/grpc/grpc/events",
                    "assignees_url": "https://api.github.com/repos/grpc/grpc/assignees{/user}",
                    "branches_url": "https://api.github.com/repos/grpc/grpc/branches{/branch}",
                    "tags_url": "https://api.github.com/repos/grpc/grpc/tags",
                    "blobs_url": "https://api.github.com/repos/grpc/grpc/git/blobs{/sha}",
                    "git_tags_url": "https://api.github.com/repos/grpc/grpc/git/tags{/sha}",
                    "git_refs_url": "https://api.github.com/repos/grpc/grpc/git/refs{/sha}",
                    "trees_url": "https://api.github.com/repos/grpc/grpc/git/trees{/sha}",
                    "statuses_url": "https://api.github.com/repos/grpc/grpc/statuses/{sha}",
                    "languages_url": "https://api.github.com/repos/grpc/grpc/languages",
                    "stargazers_url": "https://api.github.com/repos/grpc/grpc/stargazers",
                    "contributors_url": "https://api.github.com/repos/grpc/grpc/contributors",
                    "subscribers_url": "https://api.github.com/repos/grpc/grpc/subscribers",
                    "subscription_url": "https://api.github.com/repos/grpc/grpc/subscription",
                    "commits_url": "https://api.github.com/repos/grpc/grpc/commits{/sha}",
                    "git_commits_url": "https://api.github.com/repos/grpc/grpc/git/commits{/sha}",
                    "comments_url": "https://api.github.com/repos/grpc/grpc/comments{/number}",
                    "issue_comment_url": "https://api.github.com/repos/grpc/grpc/issues/comments{/number}",
                    "contents_url": "https://api.github.com/repos/grpc/grpc/contents/{+path}",
                    "compare_url": "https://api.github.com/repos/grpc/grpc/compare/{base}...{head}",
                    "merges_url": "https://api.github.com/repos/grpc/grpc/merges",
                    "archive_url": "https://api.github.com/repos/grpc/grpc/{archive_format}{/ref}",
                    "downloads_url": "https://api.github.com/repos/grpc/grpc/downloads",
                    "issues_url": "https://api.github.com/repos/grpc/grpc/issues{/number}",
                    "pulls_url": "https://api.github.com/repos/grpc/grpc/pulls{/number}",
                    "milestones_url": "https://api.github.com/repos/grpc/grpc/milestones{/number}",
                    "notifications_url": "https://api.github.com/repos/grpc/grpc/notifications{?since,all,participating}",
                    "labels_url": "https://api.github.com/repos/grpc/grpc/labels{/name}",
                    "releases_url": "https://api.github.com/repos/grpc/grpc/releases{/id}",
                    "deployments_url": "https://api.github.com/repos/grpc/grpc/deployments",
                    "created_at": "2014-12-08T18:58:53Z",
                    "updated_at": "2019-10-07T16:10:54Z",
                    "pushed_at": "2019-10-07T17:24:21Z",
                    "git_url": "git://github.com/grpc/grpc.git",
                    "ssh_url": "git@github.com:grpc/grpc.git",
                    "clone_url": "https://github.com/grpc/grpc.git",
                    "svn_url": "https://github.com/grpc/grpc",
                    "homepage": "https://grpc.io",
                    "size": 240231,
                    "stargazers_count": 23364,
                    "watchers_count": 23364,
                    "language": "C++",
                    "has_issues": True,
                    "has_projects": True,
                    "has_downloads": False,
                    "has_wiki": True,
                    "has_pages": True,
                    "forks_count": 5530,
                    "mirror_url": None,
                    "archived": False,
                    "disabled": False,
                    "open_issues_count": 886,
                    "license": {
                        "key": "apache-2.0",
                        "name": "Apache License 2.0",
                        "spdx_id": "Apache-2.0",
                        "url": "https://api.github.com/licenses/apache-2.0",
                        "node_id": "MDccccccccccccc="
                    },
                    "forks": 5530,
                    "open_issues": 886,
                    "watchers": 23364,
                    "default_branch": "master"
                }
            },
            "_links": {
                "self": {
                    "href": "https://api.github.com/repos/grpc/grpc/pulls/20414"
                },
                "html": {
                    "href": "https://github.com/grpc/grpc/pull/20414"
                },
                "issue": {
                    "href": "https://api.github.com/repos/grpc/grpc/issues/20414"
                },
                "comments": {
                    "href": "https://api.github.com/repos/grpc/grpc/issues/20414/comments"
                },
                "review_comments": {
                    "href": "https://api.github.com/repos/grpc/grpc/pulls/20414/comments"
                },
                "review_comment": {
                    "href": "https://api.github.com/repos/grpc/grpc/pulls/comments{/number}"
                },
                "commits": {
                    "href": "https://api.github.com/repos/grpc/grpc/pulls/20414/commits"
                },
                "statuses": {
                    "href": "https://api.github.com/repos/grpc/grpc/statuses/4bc982024113a7c2ced7d19af23c913adcb6bf08"
                }
            },
            "author_association": "CONTRIBUTOR"
        },
        "repository": {
            "id": 27729880,
            "node_id": "MDEwwwwwwwwwwwwwwwwwwwwwwwwwww==",
            "name": "grpc",
            "full_name": "grpc/grpc",
            "private": False,
            "owner": {
                "login": "grpc",
                "id": 7802525,
                "node_id": "MDEyyyyyyyyyyyyyyyyyyyyyyyyyyyy=",
                "avatar_url": "https://avatars1.githubusercontent.com/u/7802525?v=4",
                "gravatar_id": "",
                "url": "https://api.github.com/users/grpc",
                "html_url": "https://github.com/grpc",
                "followers_url": "https://api.github.com/users/grpc/followers",
                "following_url": "https://api.github.com/users/grpc/following{/other_user}",
                "gists_url": "https://api.github.com/users/grpc/gists{/gist_id}",
                "starred_url": "https://api.github.com/users/grpc/starred{/owner}{/repo}",
                "subscriptions_url": "https://api.github.com/users/grpc/subscriptions",
                "organizations_url": "https://api.github.com/users/grpc/orgs",
                "repos_url": "https://api.github.com/users/grpc/repos",
                "events_url": "https://api.github.com/users/grpc/events{/privacy}",
                "received_events_url": "https://api.github.com/users/grpc/received_events",
                "type": "Organization",
                "site_admin": False
            },
            "html_url": "https://github.com/grpc/grpc",
            "description": "The C based gRPC (C++, Python, Ruby, Objective-C, PHP, C#)",
            "fork": False,
            "url": "https://api.github.com/repos/grpc/grpc",
            "forks_url": "https://api.github.com/repos/grpc/grpc/forks",
            "keys_url": "https://api.github.com/repos/grpc/grpc/keys{/key_id}",
            "collaborators_url": "https://api.github.com/repos/grpc/grpc/collaborators{/collaborator}",
            "teams_url": "https://api.github.com/repos/grpc/grpc/teams",
            "hooks_url": "https://api.github.com/repos/grpc/grpc/hooks",
            "issue_events_url": "https://api.github.com/repos/grpc/grpc/issues/events{/number}",
            "events_url": "https://api.github.com/repos/grpc/grpc/events",
            "assignees_url": "https://api.github.com/repos/grpc/grpc/assignees{/user}",
            "branches_url": "https://api.github.com/repos/grpc/grpc/branches{/branch}",
            "tags_url": "https://api.github.com/repos/grpc/grpc/tags",
            "blobs_url": "https://api.github.com/repos/grpc/grpc/git/blobs{/sha}",
            "git_tags_url": "https://api.github.com/repos/grpc/grpc/git/tags{/sha}",
            "git_refs_url": "https://api.github.com/repos/grpc/grpc/git/refs{/sha}",
            "trees_url": "https://api.github.com/repos/grpc/grpc/git/trees{/sha}",
            "statuses_url": "https://api.github.com/repos/grpc/grpc/statuses/{sha}",
            "languages_url": "https://api.github.com/repos/grpc/grpc/languages",
            "stargazers_url": "https://api.github.com/repos/grpc/grpc/stargazers",
            "contributors_url": "https://api.github.com/repos/grpc/grpc/contributors",
            "subscribers_url": "https://api.github.com/repos/grpc/grpc/subscribers",
            "subscription_url": "https://api.github.com/repos/grpc/grpc/subscription",
            "commits_url": "https://api.github.com/repos/grpc/grpc/commits{/sha}",
            "git_commits_url": "https://api.github.com/repos/grpc/grpc/git/commits{/sha}",
            "comments_url": "https://api.github.com/repos/grpc/grpc/comments{/number}",
            "issue_comment_url": "https://api.github.com/repos/grpc/grpc/issues/comments{/number}",
            "contents_url": "https://api.github.com/repos/grpc/grpc/contents/{+path}",
            "compare_url": "https://api.github.com/repos/grpc/grpc/compare/{base}...{head}",
            "merges_url": "https://api.github.com/repos/grpc/grpc/merges",
            "archive_url": "https://api.github.com/repos/grpc/grpc/{archive_format}{/ref}",
            "downloads_url": "https://api.github.com/repos/grpc/grpc/downloads",
            "issues_url": "https://api.github.com/repos/grpc/grpc/issues{/number}",
            "pulls_url": "https://api.github.com/repos/grpc/grpc/pulls{/number}",
            "milestones_url": "https://api.github.com/repos/grpc/grpc/milestones{/number}",
            "notifications_url": "https://api.github.com/repos/grpc/grpc/notifications{?since,all,participating}",
            "labels_url": "https://api.github.com/repos/grpc/grpc/labels{/name}",
            "releases_url": "https://api.github.com/repos/grpc/grpc/releases{/id}",
            "deployments_url": "https://api.github.com/repos/grpc/grpc/deployments",
            "created_at": "2014-12-08T18:58:53Z",
            "updated_at": "2019-10-07T16:10:54Z",
            "pushed_at": "2019-10-07T17:24:21Z",
            "git_url": "git://github.com/grpc/grpc.git",
            "ssh_url": "git@github.com:grpc/grpc.git",
            "clone_url": "https://github.com/grpc/grpc.git",
            "svn_url": "https://github.com/grpc/grpc",
            "homepage": "https://grpc.io",
            "size": 240231,
            "stargazers_count": 23364,
            "watchers_count": 23364,
            "language": "C++",
            "has_issues": True,
            "has_projects": True,
            "has_downloads": False,
            "has_wiki": True,
            "has_pages": True,
            "forks_count": 5530,
            "mirror_url": None,
            "archived": False,
            "disabled": False,
            "open_issues_count": 886,
            "license": {
                "key": "apache-2.0",
                "name": "Apache License 2.0",
                "spdx_id": "Apache-2.0",
                "url": "https://api.github.com/licenses/apache-2.0",
                "node_id": "MDccccccccccccc="
            },
            "forks": 5530,
            "open_issues": 886,
            "watchers": 23364,
            "default_branch": "master"
        },
        "organization": {
            "login": "grpc",
            "id": 7802525,
            "node_id": "MDEEEEEEEEEEEEEEEEEEEEEEEEEEEEE=",
            "url": "https://api.github.com/orgs/grpc",
            "repos_url": "https://api.github.com/orgs/grpc/repos",
            "events_url": "https://api.github.com/orgs/grpc/events",
            "hooks_url": "https://api.github.com/orgs/grpc/hooks",
            "issues_url": "https://api.github.com/orgs/grpc/issues",
            "members_url": "https://api.github.com/orgs/grpc/members{/member}",
            "public_members_url": "https://api.github.com/orgs/grpc/public_members{/member}",
            "avatar_url": "https://avatars1.githubusercontent.com/u/7802525?v=4",
            "description": "A high performance, open source, general-purpose RPC framework"
        },
        "sender": {
            "login": "redacted",
            "id": 12345692,
            "node_id": "MDQVVVVVVVVVVVVVVVV=",
            "avatar_url": "https://avatars3.githubusercontent.com/u/2793282?v=4",
            "gravatar_id": "",
            "url": "https://api.github.com/users/veblush",
            "html_url": "https://github.com/veblush",
            "followers_url": "https://api.github.com/users/veblush/followers",
            "following_url": "https://api.github.com/users/veblush/following{/other_user}",
            "gists_url": "https://api.github.com/users/veblush/gists{/gist_id}",
            "starred_url": "https://api.github.com/users/veblush/starred{/owner}{/repo}",
            "subscriptions_url": "https://api.github.com/users/veblush/subscriptions",
            "organizations_url": "https://api.github.com/users/veblush/orgs",
            "repos_url": "https://api.github.com/users/veblush/repos",
            "events_url": "https://api.github.com/users/veblush/events{/privacy}",
            "received_events_url": "https://api.github.com/users/veblush/received_events",
            "type": "User",
            "site_admin": False
        },
        "installation": {
            "id": 1656153,
            "node_id": "zzzzzzzzzzzzzz=="
        }
    }

    example_3 = {
        "action": "deleted",
        "comment": {
            "url": "https://api.github.com/repos/grpc/grpc/pulls/comments/134346",
            "pull_request_review_id": 134346,
            "id": 134346,
            "node_id": "MDI0OlB1bGxSZXF1ZXN0UmVredacted==",
            "path": "setup.py",
            "position": 16,
            "original_position": 17,
            "commit_id": "4bc9820redacted",
            "original_commit_id": "d5515redacted",
            "user": {
                "login": "redacted",
                "id": 134566,
                "node_id": "MDQ6VXNlcjI3OTMyODI=",
                "avatar_url": "https://avatars3.githubusercontent.com/u/2793282?v=4",
                "gravatar_id": "",
                "url": "https://api.github.com/users/veblush",
                "html_url": "https://github.com/veblush",
                "followers_url": "https://api.github.com/users/veblush/followers",
                "following_url": "https://api.github.com/users/veblush/following{/other_user}",
                "gists_url": "https://api.github.com/users/veblush/gists{/gist_id}",
                "starred_url": "https://api.github.com/users/veblush/starred{/owner}{/repo}",
                "subscriptions_url": "https://api.github.com/users/veblush/subscriptions",
                "organizations_url": "https://api.github.com/users/veblush/orgs",
                "repos_url": "https://api.github.com/users/veblush/repos",
                "events_url": "https://api.github.com/users/veblush/events{/privacy}",
                "received_events_url": "https://api.github.com/users/veblush/received_events",
                "type": "User",
                "site_admin": False
            }
        }
    }

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

    def test_get_org_name_from_event(self) -> None:
        # Webhook event payload
        # see: https://developer.github.com/v3/activity/events/types/#webhook-payload-example-12
        # body['installation']['account']['login']
        self.assertEqual('Linux Foundation', get_org_name_from_installation_event(self.example_1), 'GitHub Org Matches')

    def test_get_org_name_from_event_2(self) -> None:
        # Webhook event payload
        # see: https://developer.github.com/v3/activity/events/types/#webhook-payload-example-12
        # body['installation']['account']['login']
        self.assertEqual('grpc', get_org_name_from_installation_event(self.example_2),
                         'GitHub Org Matches grpc example')

    def test_get_org_name_from_event_empty(self) -> None:
        self.assertIsNone(get_org_name_from_installation_event({}), 'GitHub Org Does Not Match')

    def test_get_github_activity_action_1(self) -> None:
        self.assertEqual('created', get_github_activity_action(self.example_1), 'GitHub Event Created Action')

    def test_get_github_activity_action_2(self) -> None:
        self.assertEqual('deleted', get_github_activity_action(self.example_3), 'GitHub Event Deleted Action')

    def test_notify_cla_managers(self):
        r1 = Repository()
        r1.set_repository_project_id('project_1')
        r1.set_repository_url('github.com/repo1')
        r2 = Repository()
        r2.set_repository_project_id('project_1')
        r2.set_repository_url('github.com/repo2')
        r3 = Repository()
        r3.set_repository_project_id('project_2')
        r3.set_repository_url('github.com/repo3')
        repositories = [r1, r2, r3]

        cla.controllers.project.get_project_managers = Mock(side_effect=mock_get_project_managers)
        cla.controllers.project.get_project = Mock(side_effect=mock_get_project)
        cla.controllers.project.load_project_by_name = Mock(side_effect=mock_load_project_by_name)
        sesClient = MockSES()
        cla.controllers.github.get_email_service = Mock()
        cla.controllers.github.get_email_service.return_value = sesClient

        notify_project_managers(repositories)

        # cla.controllers.project.get_project.assert_any_call('project_1')
        # cla.controllers.project.get_project.assert_called_with('project_2')

        cla.controllers.project.get_project_managers.assert_any_call('', 'project_1', enable_auth=False)
        cla.controllers.project.get_project_managers.assert_called_with('', 'project_2', enable_auth=False)

        self.assertEqual(len(sesClient.emails_sent), 2)
        msg1 = sesClient.emails_sent[0]
        self.assertEqual(msg1['Subject'], 'EasyCLA: Unable to check GitHub Pull Requests for CLA Group: None')
        self.assertEqual(msg1['To'], ['pm1@linuxfoundation.org', 'pm2@linuxfoundation.org'])
        msg2 = sesClient.emails_sent[1]
        self.assertEqual(msg2['Subject'], 'EasyCLA: Unable to check GitHub Pull Requests for CLA Group: None')
        self.assertEqual(msg2['To'], ['pm3@linuxfoundation.org'])


def mock_get_project_managers(username, project_id, enable_auth):
    if project_id == 'project_1':
        return [
            {
                'name': 'project manager1',
                'email': 'pm1@linuxfoundation.org',
                'lfid': 'pm1'
            },
            {
                'name': 'project manager2',
                'email': 'pm2@linuxfoundation.org',
                'lfid': 'pm2'
            }
        ]
    if project_id == 'project_2':
        return [{
            'name': 'project manager3',
            'email': 'pm3@linuxfoundation.org',
            'lfid': 'pm3'
        }]


def mock_get_project(project_id, user_id=None):
    if project_id == 'project_1':
        return {
            "project_id": "project_1",
            "project_name": 'Kubernetes'
        }
    if project_id == 'project_2':
        return {
            "project_id": "project_2",
            "project_name": 'Prometheus'
        }


def mock_load_project_by_name(project_name):
    if project_name == "Kubernetes":
        return {
            "project_id": "project_1",
            "project_name": 'Kubernetes'
        }
    if project_name == "Prometheus":
        return {
            "project_id": "project_2",
            "project_name": 'Prometheus'
        }


if __name__ == '__main__':
    unittest.main()
