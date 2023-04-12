# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

from cla.utils import get_co_authors_from_commit

FAKE_COMMIT = {
        "commit": {
            "author": {
                "name": "Harold Wanyama",
                "email": "hwanyama@contractor.linuxfoundation.org",
                "date": "2023-04-12T02:10:24Z",
            },
            "committer": {
                "name": "Harold Wanyama",
                "email": "hwanyama@contractor.linuxfoundation.org",
                "date": "2023-04-12T02:10:24Z",
            },
            "message": "Test 2\n\nCo-authored-by: Harold <wanyaland@gmail.com>",
            "tree": {
                "sha": "9db529464eac36c3e8825cd1c15cdaa571168d0a",
                "url": "https://api.github.com/repos/nabzo-bella/repo1/git/trees/9db529464eac36c3e8825cd1c15cdaa571168d0a",
            },
        }
    }

def test_get_commit_co_authors():
    """Test get commit co authors"""
    commit = FAKE_COMMIT
    co_authors = get_co_authors_from_commit(commit)
    assert co_authors == [('Harold', 'wanyaland@gmail.com')]
    assert isinstance(co_authors[0], tuple)
    assert co_authors[0][0] == 'Harold'
    assert co_authors[0][1] == 'wanyaland@gmail.com'

def test_get_co_author_no_name():
    """Test get commit co authors"""
    commit = FAKE_COMMIT
    commit['commit']['message'] = "Test 2\n\nCo-authored-by: <wanyaland@gmail.com"
    co_authors = get_co_authors_from_commit(commit)
    assert len(co_authors) == 0