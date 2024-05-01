# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

import unittest

import cla
from cla.controllers.github import (webhook_secret_failed_email_content,
                                    webhook_secret_validation)
from cla.utils import get_comment_badge

SUCCESS = ":white_check_mark:"
FAILED = ":x:"
SIGN_URL = "http://test.contributor.lfcla/sign"
SUPPORT_URL = "https://jira.linuxfoundation.org/servicedesk/customer/portal/4"
GITHUB_HELP_URL = (
    "https://help.github.com/en/github/committing-changes-to-your-project/why-are-my-commits-linked-to-the-wrong-user"
)
GITHUB_FAKE_SHA = "fake_sha"
GITHUB_FAKE_SHA_2 = "fake_sha_2"


# def test_get_comment_body_no_user_id():
#     """
#     Test CLA comment body for case CLA test failure when commit has no user ids
#     """
#     # case with missing list with no authors
#     response = get_comment_body(
#         "github",
#         SIGN_URL,
#         [],
#         [(GITHUB_FAKE_SHA, [None, "foo", "foo@bar.com"]), (GITHUB_FAKE_SHA_2, [None, "fake", "fake@gmail.com"])],
#     )
#     expected = (
#             f"<url><li> {FAILED} The commit ({' ,'.join([GITHUB_FAKE_SHA, GITHUB_FAKE_SHA_2])}) "
#             + f"is missing the User's ID, preventing the EasyCLA check. "
#             + f"<a href='{GITHUB_HELP_URL}' target='_blank'>Consult GitHub Help</a> to resolve."
#             + f"For further assistance with EasyCLA, "
#             + f"<a href='{SUPPORT_URL}' target='_blank'>please submit a support request ticket</a>."
#             + "</li></url>"
#     )
#     assert response == expected


# def test_get_comment_body_cla_fail_no_user_id_and_user_id():
#     """
#     Test CLA comment body for case CLA fail check with no user id and existing user id
#     """
#     # case with missing list with user id existing
#     author_name = "wanyaland"
#     response = get_comment_body(
#         "github",
#         SIGN_URL,
#         [],
#         [
#             (GITHUB_FAKE_SHA, ["12", author_name, "foo@gmail.com"]),
#             (GITHUB_FAKE_SHA_2, [None, author_name, " foo@gmail.com"]),
#         ],
#     )
#     expected = (
#             f"<ul><li>[{FAILED}]({SIGN_URL}) {author_name} "
#             + "The commit ("
#             + " ,".join([GITHUB_FAKE_SHA])
#             + ") is not authorized under a signed CLA. "
#             + f"[Please click here to be authorized]({SIGN_URL}). For further assistance with "
#             + f"EasyCLA, [please submit a support request ticket]({SUPPORT_URL})."
#             + "</li>"
#             + "<li> " + FAILED + " The commit ("
#             + " ,".join([GITHUB_FAKE_SHA_2])
#             + ") is missing the User's ID, preventing the EasyCLA check. [Consult GitHub Help]("
#             + GITHUB_HELP_URL
#             + ") to resolve. For further assistance with EasyCLA, "
#             + f"[please submit a support request ticket]({SUPPORT_URL})."
#             + "</li></ul>"
#     )
#
#     assert response == expected


# def test_get_comment_body_whitelisted_missing_user():
#     """
#     Test CLA comment body for case of a whitelisted user that has not confirmed affiliation
#     """
#     is_whitelisted = True
#     author = "foo"
#     signed = []
#     missing = [(GITHUB_FAKE_SHA, ["12", author, "foo@gmail.com", is_whitelisted])]
#     response = get_comment_body("github", SIGN_URL, signed, missing)
#     expected = (
#             f"<ul><li>{author} ({' ,'.join([GITHUB_FAKE_SHA])}) "
#             + "is authorized, but they must confirm "
#             + "their affiliation with their company. "
#             + f'[Start the authorization process by clicking here]({SIGN_URL}), click "Corporate",'
#             + "select the appropriate company from the list, then confirm "
#             + "your affiliation on the page that appears. For further assistance with EasyCLA, "
#             + f"[please submit a support request ticket]({SUPPORT_URL})."
#             + "</li>"
#             + "</ul>"
#     )
#     assert response == expected


def test_get_comment_badge_with_no_user_id():
    """
    Test CLA badge for CLA fail check with no user id
    """
    missing_id_badge = "cla-missing-id.svg"
    missing_user_id = True
    all_signed = False
    response = get_comment_badge("github", all_signed, SIGN_URL, "v1", missing_user_id=missing_user_id)
    assert missing_id_badge in response


def test_comment_badge_with_missing_whitelisted_user():
    """
    Test CLA badge for CLA fail check and whitelisted user
    """
    confirmation_needed_badge = "cla-confirmation-needed.svg"
    response = get_comment_badge("github", False, SIGN_URL, "v1", missing_user_id=False, is_approved_by_manager=True)
    assert confirmation_needed_badge in response


class TestWebhookSecretValidation(unittest.TestCase):
    def setUp(self) -> None:
        self.old_email = cla.config.EMAIL_SERVICE
        self.oldval = cla.config.GITHUB_APP_WEBHOOK_SECRET

    def tearDown(self) -> None:
        cla.config.GITHUB_APP_WEBHOOK_SECRET = self.oldval
        cla.config.EMAIL_SERVICE = self.oldval

    def test_webhook_secret_validation_empty(self):
        """
        Tests the webhook_secret_validation method
        """
        cla.config.GITHUB_APP_WEBHOOK_SECRET = ""
        with self.assertRaises(RuntimeError) as ex:
            _ = webhook_secret_validation("secret", b'')

    def test_webhook_secret_validation_failed(self):
        """
        Tests the webhook_secret_validation method
        """
        cla.config.GITHUB_APP_WEBHOOK_SECRET = "secret"
        assert not webhook_secret_validation("sha1=secret", ''.encode())

    def test_webhook_secret_validation_success(self):
        """
        Tests the webhook_secret_validation method
        """
        cla.config.GITHUB_APP_WEBHOOK_SECRET = "secret"
        input_data = 'data'.encode('utf-8')
        assert webhook_secret_validation("sha1=9818e3306ba5ac267b5f2679fe4abd37e6cd7b54", input_data)

    # def test_webhook_secret_failed_email(self):
    #     """
    #     Tests the email sending of the failed webhook
    #     :return:
    #     """
    #     with self.assertRaises(RuntimeError) as ex:
    #         webhook_secret_failed_email_content("repositories", {}, [])
    #
    #     s, b, m = webhook_secret_failed_email_content("repositories", {}, ["john@gmail.com"])
    #     assert s
    #     assert "Hello EasyCLA Maintainer" in b
    #     assert m == ["john@gmail.com"]
    #
    #     s, b, m = webhook_secret_failed_email_content("repositories", {
    #         "sender": {"login": "john"},
    #         "repository": {"id": "123", "full_name": "github.com/penguin/activity",
    #                        "html_url": "https://github.com/foo",
    #                        "owner": {"login": "test"},
    #                        "organization": {"login": "test"}
    #                        },
    #         "installation": {"id": 345}
    #     }, ["john@gmail.com"])
    #     assert s
    #     assert "event type: repositories" in b
    #     assert "user login: john" in b
    #     assert "repository_id: 123" in b
    #     assert "installation_id: 345" in b
    #     assert m == ["john@gmail.com"]
