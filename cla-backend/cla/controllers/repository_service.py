# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Controller related to repository service provider activity.
"""

import cla
from falcon import HTTP_202, HTTP_404

def received_activity(provider, data):
    """
    Handles receiving webhook activity from the repository provider.

    Forwards the data to the appropriate provider.
    """
    service = cla.utils.get_repository_service(provider)
    result = service.received_activity(data)
    return result


def oauth2_redirect(provider, state, code, installation_id, github_repository_id, change_request_id, request): # pylint: disable=too-many-arguments
    """
    Properly triages the OAuth2 redirect to the appropriate provider.
    """
    service = cla.utils.get_repository_service(provider)
    return service.oauth2_redirect(state, code, installation_id, github_repository_id, change_request_id, request)


def sign_request(provider, installation_id, github_repository_id, change_request_id, request):
    """
    Properly triage the sign request to the appropriate provider.
    """
    service = cla.utils.get_repository_service(provider)
    return service.sign_request(installation_id, github_repository_id, change_request_id, request)

def user_from_session(get_redirect_url, request, response=None):
    """
    Return user from OAuth2 session
    """
    # LG: to test with other GitHub APP and BASE API URL (for OAuth redirects)
    # import os
    # os.environ["GH_OAUTH_CLIENT_ID"] = os.getenv("GH_OAUTH_CLIENT_ID_CLI", os.environ["GH_OAUTH_CLIENT_ID"])
    # os.environ["GH_OAUTH_SECRET"] = os.getenv("GH_OAUTH_SECRET_CLI", os.environ["GH_OAUTH_SECRET"])
    # os.environ["CLA_API_BASE"] = os.getenv("CLA_API_BASE_CLI", os.environ["CLA_API_BASE"])
    # LG: to test using MockGitHub class
    # from cla.models.github_models import MockGitHub
    # user = MockGitHub(os.environ["GITHUB_OAUTH_TOKEN"]).user_from_session(request, get_redirect_url)
    user = cla.utils.get_repository_service('github').user_from_session(request, get_redirect_url)
    if user is None:
        response.status = HTTP_404
        return {"errors": "Cannot find user from session"}
    if isinstance(user, dict):
        response.status = HTTP_202
        return user
    return user.to_dict()
