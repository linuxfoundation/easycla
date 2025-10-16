# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Holds the GitHub repository service.
"""
import concurrent.futures
import json
import os
import re
import base64
import binascii
import threading
import time
import uuid
from typing import List, Optional, Union, Tuple, Iterable

import cla
import falcon
import github
from cla.controllers.github_application import GitHubInstallation
from cla.models import DoesNotExist, repository_service_interface
from cla.models.dynamo_models import GitHubOrg, Repository, Event
from cla.models.event_types import EventType
from cla.user import UserCommitSummary
from cla.utils import (append_project_version_to_url, get_project_instance,
                       set_active_pr_metadata)
from github import PullRequest
from github.GithubException import (BadCredentialsException, GithubException,
                                    IncompletableObject,
                                    RateLimitExceededException,
                                    UnknownObjectException)
from requests_oauthlib import OAuth2Session
from dataclasses import dataclass
from itertools import islice

# some emails we want to exclude when we register the users
EXCLUDE_GITHUB_EMAILS = ["noreply.github.com"]
NOREPLY_ID_PATTERN = re.compile(r"^(\d+)\+([a-zA-Z0-9-]+)@users\.noreply\.github\.com$")
NOREPLY_USER_PATTERN = re.compile(r"^([a-zA-Z0-9-]+)@users\.noreply\.github\.com$")
# GitHub usernames must be 3-39 characters long, can only contain alphanumeric characters or hyphens,
# cannot begin or end with a hyphen, and cannot contain consecutive hyphens.
GITHUB_USERNAME_REGEX = re.compile(r'^(?!-)(?!.*--)[A-Za-z0-9-]{3,39}(?<!-)$')
NEGATIVE_CACHE_TTL = 180  # 3 minutes TTL for negative cache (all negative cases)
PROJECT_CACHE_TTL = 10800  # 3 hours TTL for project cache (positive cases)

class TTLCache:
    def __init__(self, ttl_seconds=43200):
        self.data = {}
        self.ttl = ttl_seconds
        self.lock = threading.Lock()

    def get(self, key):
        with self.lock:
            item = self.data.get(key, None)
            if item is None:
                return None, False
            value, expires_at = item
            if time.time() > expires_at:
                self.data.pop(key, None)
                return None, False
            return value, True

    def set(self, key, value):
        with self.lock:
            self.data[key] = (value, time.time() + self.ttl)

    def set_with_ttl(self, key, value, tl):
        with self.lock:
            self.data[key] = (value, time.time() + tl)

    def cleanup(self):
        with self.lock:
            now = time.time()
            keys_to_delete = [k for k, (v, expires_at) in self.data.items() if now > expires_at]
            for k in keys_to_delete:
                del self.data[k]

    def clear(self):
        with self.lock:
            self.data.clear()

github_user_cache = TTLCache(ttl_seconds=43200)
def start_cache_cleanup():
    def run():
        while True:
            time.sleep(3600)
            github_user_cache.cleanup()
    threading.Thread(target=run, daemon=True).start()

start_cache_cleanup()

def clear_caches():
    """
    Clears in-memory caches maintained by this module.
    """
    fn = "cla.models.github_models.clear_caches"
    try:
        github_user_cache.clear()
        cla.log.info(f"{fn} - cleared github_user_cache")
        return {"status": "OK"}
    except Exception as e:
        cla.log.error(f"{fn} - error clearing caches: {e}")
        return {"status": f"Error clearing caches: {e}"}

@dataclass
class CommitLite:
    sha: str
    author_id: Optional[int]
    author_login: Optional[str]
    author_name: Optional[str]
    author_email: Optional[str]
    message: Optional[str]


def str_strip_lower(s): return (s or "").strip().lower()

def dedup_and_sort(items):
    seen = set()
    uniq = []
    for s in items:
        if s is None:
            continue
        key = (
            getattr(s, "author_id", None),
            str_strip_lower(getattr(s, "author_login", None)),
            str_strip_lower(getattr(s, "author_email", None)),
            getattr(s, "commit_sha", None),
        )
        if key in seen:
            continue
        seen.add(key)
        uniq.append(s)
    uniq.sort(key=lambda s: (
        str_strip_lower(getattr(s, "author_login", None)),
        str_strip_lower(getattr(s, "author_name", None)),
        str_strip_lower(getattr(s, "author_email", None)),
        getattr(s, "commit_sha", "") or "",
    ))
    return uniq

class GitHub(repository_service_interface.RepositoryService):
    """
    The GitHub repository service.
    """

    def __init__(self):
        self.client = None

    def initialize(self, config):
        # username = config['GITHUB_USERNAME']
        # token = config['GITHUB_TOKEN']
        # self.client = self._get_github_client(username, token)
        pass

    def _get_github_client(self, username, token):  # pylint: disable=no-self-use
        return github.Github(username, token)

    def get_repository_id(self, repo_name, installation_id=None):
        """
        Helper method to get a GitHub repository ID based on repository name.

        :param repo_name: The name of the repository, example: 'linuxfoundation/cla'.
        :type repo_name: string
        :param installation_id: The github installation id
        :type installation_id: string
        :return: The repository ID.
        :rtype: integer
        """
        if installation_id is not None:
            self.client = get_github_integration_client(installation_id)
        try:
            return self.client.get_repo(repo_name).id
        except github.GithubException as err:
            cla.log.error(
                "Could not find GitHub repository (%s), ensure it exists and that "
                "your personal access token is configured with the repo scope",
                repo_name,
            )
        except Exception as err:
            cla.log.error("Unknown error while getting GitHub repository ID for repository %s: %s", repo_name, str(err))

    def received_activity(self, data):
        cla.log.debug("github_models.received_activity - Received GitHub activity: %s", data)
        if "pull_request" not in data and "merge_group" not in data:
            cla.log.debug("github_models.received_activity - Activity not related to pull request - ignoring")
            return {"message": "Not a pull request nor a merge group  - no action performed"}
        if data["action"] == "opened":
            cla.log.debug("github_models.received_activity - Handling opened pull request")
            return self.process_opened_pull_request(data)
        elif data["action"] == "reopened":
            cla.log.debug("github_models.received_activity - Handling reopened pull request")
            return self.process_reopened_pull_request(data)
        elif data["action"] == "closed":
            cla.log.debug("github_models.received_activity - Handling closed pull request")
            return self.process_closed_pull_request(data)
        elif data["action"] == "synchronize":
            cla.log.debug("github_models.received_activity - Handling synchronized pull request")
            return self.process_synchronized_pull_request(data)
        elif data["action"] == "checks_requested":
            cla.log.debug("github_models.received_activity - Handling checks requested pull request")
            return self.process_checks_requested_merge_group(data)
        else:
            cla.log.debug("github_models.received_activity - Ignoring unsupported action: {}".format(data["action"]))

    def user_from_session(self, request, get_redirect_url):
        fn = "github_models.user_from_session"
        cla.log.debug(f"{fn} - loading session from request: {request}...")
        session = self._get_request_session(request)
        cla.log.debug(f"{fn} - session: {session}")

        # We can already have token in the session
        if "github_oauth2_token" in session:
            cla.log.debug(f"{fn} - using existing session GitHub OAuth2 token")
            user = self.get_or_create_user(request)
            if user is None:
                cla.log.debug(f"{fn} - cannot find user, returning HTTP 404 status")
            else:
                cla.log.debug(f"{fn} - loaded user {user.to_dict()} returning HTTP 200 status")
            return user

        authorization_url, csrf_token = self.get_authorization_url_and_state(None, None, None, ["user:email"], state='user-from-session')
        cla.log.debug(f"{fn} - obtained GitHub OAuth2 state from authorization - storing CSRF token in the session...")
        session["github_oauth2_state"] = csrf_token
        cla.log.debug(f"{fn} - GitHub OAuth2 request with CSRF token {csrf_token} - sending user to {authorization_url}")
        # We must redirect to GitHub OAuth app for authentication, it will return you to /v2/github/installation which will handle returning user data
        if get_redirect_url:
            cla.log.debug(f"{fn} - sending redirect_url via 202 HTTP status JSON payload")
            return { "redirect_url": authorization_url }
        else:
            cla.log.debug(f"{fn} - redirecting by returning 302 and redirect URL")
            raise falcon.HTTPFound(authorization_url)

    def sign_request(self, installation_id, github_repository_id, change_request_id, request):
        """
        This method gets called when the OAuth2 app (NOT the GitHub App) needs to get info on the
        user trying to sign. In this case we begin an OAuth2 exchange with the 'user:email' scope.
        """
        fn = "github_models.sign_request"  # function name
        cla.log.debug(
            f"{fn} - Initiating GitHub sign request for installation_id: {installation_id}, "
            f"for repository {github_repository_id}, "
            f"for PR: {change_request_id}"
        )

        # Not sure if we need a different token for each installation ID...
        cla.log.debug(f"{fn} - Loading session from request: {request}...")
        session = self._get_request_session(request)
        cla.log.debug(f"{fn} - Adding github details to session: {session} which is type: {type(session)}...")
        session["github_installation_id"] = installation_id
        session["github_repository_id"] = github_repository_id
        session["github_change_request_id"] = change_request_id

        cla.log.debug(f"{fn} - Determining return URL from the inbound request...")
        origin_url = self.get_return_url(github_repository_id, change_request_id, installation_id)
        cla.log.debug(f"{fn} - Return URL from the inbound request is {origin_url}")
        session["github_origin_url"] = origin_url
        cla.log.debug(f'{fn} - Stored origin url in session as session["github_origin_url"] = {origin_url}')

        if "github_oauth2_token" in session:
            cla.log.debug(f"{fn} - Using existing session GitHub OAuth2 token")
            return self.redirect_to_console(installation_id, github_repository_id, change_request_id, origin_url, request)
        else:
            cla.log.debug(f"{fn} - No existing GitHub OAuth2 token - building authorization url and state")
            authorization_url, state = self.get_authorization_url_and_state(
                installation_id, github_repository_id, int(change_request_id), ["user:email"]
            )
            cla.log.debug(f"{fn} - Obtained GitHub OAuth2 state from authorization - storing state in the session...")
            session["github_oauth2_state"] = state
            cla.log.debug(f"{fn} - GitHub OAuth2 request with state {state} - sending user to {authorization_url}")
            raise falcon.HTTPFound(authorization_url)

    def _get_request_session(self, request) -> dict:  # pylint: disable=no-self-use
        """
        Mockable method used to get the current user session.
        """
        fn = "cla.models.github_models._get_request_session"
        session = request.context.get("session")
        if session is None:
            cla.log.warning(f"Session is empty for request: {request}")
        cla.log.debug(f"{fn} - loaded session: {session}")

        # Ensure session is a dict - getting issue where session is a string
        if isinstance(session, str):
            # convert string to a dict
            cla.log.debug(f"{fn} - session is type: {type(session)} - converting to dict...")
            session = json.loads(session)
            # Reset the session now that we have converted it to a dict
            request.context["session"] = session
            cla.log.debug(f"{fn} - session: {session} which is now type: {type(session)}...")

        return session

    def get_authorization_url_and_state(self, installation_id, github_repository_id, pull_request_number, scope, state=None):
        """
        Helper method to get the GitHub OAuth2 authorization URL and state.

        This will be used to get the user's emails from GitHub.

        :TODO: Update comments.

        :param repository_id: The ID of the repository this request was initiated in.
        :type repository_id: int
        :param pull_request_number: The PR number this request was generated in.
        :type pull_request_number: int
        :param scope: The list of OAuth2 scopes to request from GitHub.
        :type scope: [string]
        """
        # Get the PR's html_url property.
        # origin = self.get_return_url(github_repository_id, pull_request_number, installation_id)
        # Add origin to user's session here?
        fn = "github_models.get_authorization_url_and_state"
        redirect_uri = os.environ.get("CLA_API_BASE", "").strip() + "/v2/github/installation"
        github_oauth_url = cla.conf["GITHUB_OAUTH_AUTHORIZE_URL"]
        github_oauth_client_id = os.environ["GH_OAUTH_CLIENT_ID"]

        cla.log.debug(
            f"{fn} - Directing user to the github authorization url: {github_oauth_url} via "
            f"our github installation flow: {redirect_uri} "
            f"using the github oauth client id: {github_oauth_client_id[0:5]} "
            f"with scope: {scope}"
        )

        return self._get_authorization_url_and_state(
            client_id=github_oauth_client_id, redirect_uri=redirect_uri, scope=scope, authorize_url=github_oauth_url, state=state,
        )

    def _get_authorization_url_and_state(self, client_id, redirect_uri, scope, authorize_url, state=None):
        """
        Mockable helper method to do the fetching of the authorization URL and state from GitHub.
        """
        return cla.utils.get_authorization_url_and_state(client_id, redirect_uri, scope, authorize_url, state)

    def oauth2_redirect(self, state, code, request):  # pylint: disable=too-many-arguments
        """
        This is where the user will end up after having authorized the CLA system
        to get information such as email address.

        It will handle storing the OAuth2 session information for this user for
        further requests and initiate the signing workflow.
        """
        fn = "github_models.oauth2_redirect"
        cla.log.debug(f"{fn} - handling GitHub OAuth2 redirect with request: {dir(request)}")
        session = self._get_request_session(request)  # request.context['session']
        cla.log.debug(f"{fn} - state: {state}, code: {code}, session: {session}")

        if "github_oauth2_state" in session:
            session_state = session["github_oauth2_state"]
        else:
            session_state = None
            cla.log.warning(f"{fn} - github_oauth2_state not set in current session")

        if state != session_state:
            # Eventually handle user-from-session API callback
            try:
                state_data = json.loads(base64.urlsafe_b64decode(state.encode()).decode())
            except (ValueError, json.JSONDecodeError, binascii.Error):
                cla.log.warning(f"{fn} - failed to decode state: {state}, error: {err}")
                raise falcon.HTTPBadRequest("Invalid OAuth2 state", state)
            state_token = state_data["csrf"]
            value = state_data["state"]
            if value != "user-from-session":
                cla.log.warning(f"{fn} - invalid GitHub OAuth2 state {session_state} expecting {state}, value: {value}")
                raise falcon.HTTPBadRequest("Invalid OAuth2 state", state)
            if state_token != session_state:
                cla.log.warning(f"{fn} - invalid GitHub OAuth2 state {session_state} expecting {state_token} while handling user-from-session callback")
                raise falcon.HTTPBadRequest(f"Invalid OAuth2 state")
            cla.log.debug(f"handling user-from-session callback")
            token_url = cla.conf["GITHUB_OAUTH_TOKEN_URL"]
            client_id = os.environ["GH_OAUTH_CLIENT_ID"]
            cla.log.debug(f"{fn} - using client ID {client_id}")
            client_secret = os.environ["GH_OAUTH_SECRET"]
            try:
                token = self._fetch_token(client_id, state, token_url, client_secret, code)
            except Exception as err:
                cla.log.warning(f"{fn} - GitHub OAuth2 error: {err}. Likely bad or expired code, returning HTTP 404 state.")
                raise falcon.HTTPBadRequest("OAuth2 code is invalid or expired")
            cla.log.debug(f"{fn} - oauth2 token received for state {state}: {token} - storing token in session")
            session["github_oauth2_token"] = token
            user = self.get_or_create_user(request)
            if user is None:
                cla.log.debug(f"{fn} - cannot find user, returning HTTP 404 status")
            else:
                cla.log.debug(f"{fn} - loaded user {user.to_dict()} returning HTTP 200 status")
            return user.to_dict()

        # Get session information for this request.
        cla.log.debug(f"{fn} - attempting to fetch OAuth2 token for state {state}")
        installation_id = session.get("github_installation_id", None)
        github_repository_id = session.get("github_repository_id", None)
        change_request_id = session.get("github_change_request_id", None)
        origin_url = session.get("github_origin_url", None)
        state = session.get("github_oauth2_state")
        token_url = cla.conf["GITHUB_OAUTH_TOKEN_URL"]
        client_id = os.environ["GH_OAUTH_CLIENT_ID"]
        client_secret = os.environ["GH_OAUTH_SECRET"]
        cla.log.debug(
            f"{fn} - fetching token using {client_id[0:5]}... with state={state}, token_url={token_url}, "
            f"client_secret={client_secret[0:5]}, with code={code}"
        )
        token = self._fetch_token(client_id, state, token_url, client_secret, code)
        cla.log.debug(f"{fn} - oauth2 token received for state {state}: {token} - storing token in session")
        session["github_oauth2_token"] = token
        cla.log.debug(f"{fn} - redirecting the user back to the console: {origin_url}")
        return self.redirect_to_console(installation_id, github_repository_id, change_request_id, origin_url, request)

    def redirect_to_console(self, installation_id, repository_id, pull_request_id, origin_url, request):
        fn = "github_models.redirect_to_console"
        console_endpoint = cla.conf["CONTRIBUTOR_BASE_URL"]
        console_v2_endpoint = cla.conf["CONTRIBUTOR_V2_BASE_URL"]
        # Get repository using github's repository ID.
        repository = Repository().get_repository_by_external_id(repository_id, "github")
        if repository is None:
            cla.log.warning(f"{fn} - Could not find repository with the following " f"repository_id: {repository_id}")
            return None

        # Get project ID from this repository
        project_id = repository.get_repository_project_id()

        try:
            project = get_project_instance()
            project.load(str(project_id))
        except DoesNotExist as err:
            return {"errors": {"project_id": str(err)}}

        user = self.get_or_create_user(request)
        # Ensure user actually requires a signature for this project.
        # TODO: Skipping this for now - we can do this for ICLAs but there's no easy way of doing
        # the check for CCLAs as we need to know in advance what the company_id is that we're checking
        # the CCLA signature for.
        # We'll have to create a function that fetches the latest CCLA regardless of company_id.
        # icla_signature = cla.utils.get_user_signature_by_github_repository(installation_id, user)
        # ccla_signature = cla.utils.get_user_signature_by_github_repository(installation_id, user, company_id=?)
        # try:
        # document = cla.utils.get_project_latest_individual_document(project_id)
        # except DoesNotExist:
        # cla.log.debug('No ICLA for project %s' %project_idSignature)
        # if signature is not None and \
        # signature.get_signature_document_major_version() == document.get_document_major_version():
        # return cla.utils.redirect_user_by_signature(user, signature)
        # Store repository and PR info so we can redirect the user back later.
        cla.utils.set_active_signature_metadata(user.get_user_id(), project_id, repository_id, pull_request_id)

        console_url = ""

        # Temporary condition until all CLA Groups are ready for the v2 Contributor Console
        if project.get_version() == "v2":
            # Generate url for the v2 console
            console_url = (
                "https://"
                + console_v2_endpoint
                + "/#/cla/project/"
                + project_id
                + "/user/"
                + user.get_user_id()
                + "?redirect="
                + origin_url
            )
            cla.log.debug(f"{fn} - redirecting to v2 console: {console_url}...")
        else:
            # Generate url for the v1 contributor console
            console_url = (
                "https://"
                + console_endpoint
                + "/#/cla/project/"
                + project_id
                + "/user/"
                + user.get_user_id()
                + "?redirect="
                + origin_url
            )
            cla.log.debug(f"{fn} - redirecting to v1 console: {console_url}...")

        raise falcon.HTTPFound(console_url)

    def _fetch_token(
        self, client_id, state, token_url, client_secret, code
    ):  # pylint: disable=too-many-arguments,no-self-use
        """
        Mockable method to fetch a OAuth2Session token.
        """
        return cla.utils.fetch_token(client_id, state, token_url, client_secret, code)

    def sign_workflow(self, installation_id, github_repository_id, pull_request_number, request):
        """
        Once we have the 'github_oauth2_token' value in the user's session, we can initiate the
        signing workflow.
        """
        fn = "sign_workflow"
        cla.log.warning(
            f"{fn} - Initiating GitHub signing workflow for "
            f"GitHub repo {github_repository_id} "
            f"with PR: {pull_request_number}"
        )
        user = self.get_or_create_user(request)
        signature = cla.utils.get_user_signature_by_github_repository(installation_id, user)
        project_id = cla.utils.get_project_id_from_installation_id(installation_id)
        document = cla.utils.get_project_latest_individual_document(project_id)
        if (
            signature is not None
            and signature.get_signature_document_major_version() == document.get_document_major_version()
        ):
            return cla.utils.redirect_user_by_signature(user, signature)
        else:
            # Signature not found or older version, create new one and send user to sign.
            cla.utils.request_individual_signature(installation_id, github_repository_id, user, pull_request_number)

    def process_opened_pull_request(self, data):
        """
        Helper method to handle a webhook fired from GitHub for an opened PR.

        :param data: The data returned from GitHub on this webhook.
        :type data: dict
        """
        pull_request_id = data["pull_request"]["number"]
        github_repository_id = data["repository"]["id"]
        installation_id = data["installation"]["id"]
        self.update_change_request(installation_id, github_repository_id, pull_request_id)

    def process_checks_requested_merge_group(self, data):
        """
        Helper method to handle a webhook fired from GitHub for a merge group event.

        :param data: The data returned from GitHub on this webhook.
        :type data: dict
        """
        merge_group_sha = data["merge_group"]["head_sha"]
        github_repository_id = data["repository"]["id"]
        installation_id = data["installation"]["id"]
        pull_request_message = data["merge_group"]["head_commit"]["message"]

        # Extract the pull request number from the message
        pull_request_id = cla.utils.extract_pull_request_number(pull_request_message)
        self.update_merge_group(installation_id, github_repository_id, merge_group_sha, pull_request_id)

    def process_easycla_command_comment(self, data):
        """
        Processes easycla command comment if present
        :param data: github issue comment webhook event payload : https://docs.github.com/en/free-pro-team@latest/developers/webhooks-and-events/webhook-events-and-payloads#issue_comment
        :return:
        """
        comment_str = data.get("comment", {}).get("body", "")
        if not comment_str:
            raise ValueError("missing comment body, ignoring the message")

        if "/easycla" not in comment_str.split():
            raise ValueError(
                f"unsupported comment supplied: {comment_str.split()}, " "currently only the '/easycla' command is supported"
            )

        github_repository_id = data.get("repository", {}).get("id", None)
        if not github_repository_id:
            raise ValueError("missing github repository id in pull request comment")
        cla.log.debug(f"comment trigger for github_repo : {github_repository_id}")

        # turns out pull request id and issue is the same thing
        pull_request_id = data.get("issue", {}).get("number", None)
        if not pull_request_id:
            raise ValueError("missing pull request id ")
        cla.log.debug(f"comment trigger for pull_request_id : {pull_request_id}")

        cla.log.debug("installation object : ", data.get("installation", {}))
        installation_id = data.get("installation", {}).get("id", None)
        if not installation_id:
            raise ValueError("missing installation id in pull request comment")
        cla.log.debug(f"comment trigger for installation_id : {installation_id}")

        self.update_change_request(installation_id, github_repository_id, pull_request_id)

    def get_return_url(self, github_repository_id, change_request_id, installation_id):
        pull_request = self.get_pull_request(github_repository_id, change_request_id, installation_id)
        return pull_request.html_url

    def get_existing_repository(self, github_repository_id):
        fn = "get_existing_repository"
        # Queries GH for the complete repository details, see:
        # https://developer.github.com/v3/repos/#get-a-repository
        cla.log.debug(f"{fn} - fetching repository details for GH repo ID: {github_repository_id}...")
        repository = Repository().get_repository_by_external_id(str(github_repository_id), "github")
        if repository is None:
            cla.log.warning(f"{fn} - unable to locate repository by GH ID: {github_repository_id}")
            return None

        if repository.get_enabled() is False:
            cla.log.warning(f"{fn} - repository is disabled, skipping: {github_repository_id}")
            return None

        cla.log.debug(f"{fn} - found repository by GH ID: {github_repository_id}")
        return repository

    def check_org_validity(self, installation_id, repository):
        fn = "check_org_validity"
        organization_name = repository.get_organization_name()

        # Check that the Github Organization exists in our database
        cla.log.debug(f"{fn} - fetching organization details for GH org name: {organization_name}...")
        github_org = GitHubOrg()
        try:
            github_org.load(organization_name=organization_name)
        except DoesNotExist as err:
            cla.log.warning(f"{fn} - unable to locate organization by GH name: {organization_name}")
            return False

        if github_org.get_organization_installation_id() != installation_id:
            cla.log.warning(
                f"{fn} - "
                f"the installation ID: {github_org.get_organization_installation_id()} "
                f"of this organization does not match installation ID: {installation_id} "
                "given by the pull request."
            )
            return False

        cla.log.debug(f"{fn} - found organization by GH name: {organization_name}")
        return True

    def get_pull_request_retry(self, github_repository_id, change_request_id, installation_id, retries=3) -> dict:
        """
        Helper function to retry getting a pull request from GitHub.
        """
        fn = "get_pull_request_retry"
        pull_request = {}
        for i in range(retries):
            try:
                # check if change_request_id is a valid int
                _ = int(change_request_id)
                pull_request = self.get_pull_request(github_repository_id, change_request_id, installation_id)
            except ValueError as ve:
                cla.log.error(
                    f"{fn} - Invalid PR: {change_request_id} - error: {ve}. Unable to fetch "
                    f"PR {change_request_id} from GitHub repository {github_repository_id} "
                    f"using installation id {installation_id}."
                )
                if i <= retries:
                    cla.log.debug(f"{fn} - attempt {i + 1} - waiting to retry...")
                    time.sleep(2)
                    continue
                else:
                    cla.log.warning(
                        f"{fn} - attempt {i + 1} - exhausted retries - unable to load PR "
                        f"{change_request_id} from GitHub repository {github_repository_id} "
                        f"using installation id {installation_id}."
                    )
                    # TODO: DAD - possibly update the PR status?
                    return
            # Fell through - no error, exit loop and continue on
            break
        cla.log.debug(f"{fn} - retrieved pull request: {pull_request}")

        return pull_request

    def update_merge_group_status(
        self, installation_id, repository_id, pull_request, merge_commit_sha, signed, missing, any_missing, project_version
    ):
        """
        Helper function to update a merge queue entrys status based on the list of signers.
        :param installation_id: The ID of the GitHub installation
        :type installation_id: int
        :param repository_id: The ID of the GitHub repository this PR belongs to.
        :type repository_id: int
        :param pull_request: The GitHub PullRequest object for this PR.
        """
        fn = "update_merge_group_status"
        context_name = os.environ.get("GH_STATUS_CTX_NAME")
        if context_name is None:
            context_name = "communitybridge/cla"
        if missing is not None and len(missing) > 0:
            state = "failure"
            context, body = cla.utils.assemble_cla_status(context_name, signed=False)
            sign_url = cla.utils.get_full_sign_url(
                "github", str(installation_id), repository_id, pull_request.number, project_version
            )
            cla.log.debug(
                f"{fn} - Creating new CLA '{state}' status - {len(signed)} passed, {missing} failed, any_co_author_missing: {any_missing}, "
                f"signing url: {sign_url}"
            )
        elif signed is not None and len(signed) > 0:
            state = "success"
            # For status, we change the context from author_name to 'communitybridge/cla' or the
            # specified default value per issue #166
            context, body = cla.utils.assemble_cla_status(context_name, signed=True)
            sign_url = cla.conf["CLA_LANDING_PAGE"]  # Remove this once signature detail page ready.
            sign_url = os.path.join(sign_url, "#/")
            sign_url = append_project_version_to_url(address=sign_url, project_version=project_version)
            cla.log.debug(
                f"{fn} - Creating new CLA '{state}' status - {len(signed)} passed, {missing} failed, "
                f"signing url: {sign_url}"
            )
        else:
            # error condition - should have at least one committer, and they would be in one of the above
            # lists: missing or signed
            state = "failure"
            # For status, we change the context from author_name to 'communitybridge/cla' or the
            # specified default value per issue #166
            context, body = cla.utils.assemble_cla_status(context_name, signed=False)
            sign_url = cla.utils.get_full_sign_url(
                "github", str(installation_id), repository_id, pull_request.number, project_version
            )
            cla.log.debug(
                f"{fn} - Creating new CLA '{state}' status - {len(signed)} passed, {missing} failed, "
                f"signing url: {sign_url}"
            )
            cla.log.warning(
                f"{fn} - This is an error condition - "
                f"should have at least one committer in one of these lists: "
                f"{len(signed)} passed, {missing}"
            )

        # Create the commit status on the merge commit
        if self.client is None:
            self.client = get_github_integration_client(installation_id)

        # Get repository
        cla.log.debug(f"{fn} - Getting repository by ID: {repository_id}")
        repository = self.client.get_repo(int(repository_id))

        # Get the commit object
        cla.log.debug(f"{fn} - Getting commit by SHA: {merge_commit_sha}")
        commit_obj = repository.get_commit(merge_commit_sha)

        cla.log.debug(
            f"{fn} - Creating commit status for merge commit: {merge_commit_sha} "
            f"with state: {state}, context: {context}, body: {body}"
        )

        create_commit_status_for_merge_group(commit_obj, merge_commit_sha, state, sign_url, body, context)

    def is_co_authors_enabled_for_repo(self, enable_co_authors, org_repo):
        if enable_co_authors is None:
            cla.log.debug("enable_co_authors is not set on '%s', skipping co-authors", org_repo)
            return False

        repo = self.strip_org(org_repo)
        if hasattr(enable_co_authors, "as_dict"):
            enable_co_authors = enable_co_authors.as_dict()

        # 1. Exact match
        if repo in enable_co_authors:
            cla.log.debug("enable_co_authors found for repo %s: %s (exact hit)", org_repo, enable_co_authors[repo])
            return enable_co_authors[repo]

        # 2. Regex pattern (if no exact hit)
        cla.log.debug("No enable_co_authors found for repo %s, checking regex patterns", org_repo)
        for k, v in enable_co_authors.items():
            if not isinstance(k, str) or not k.startswith("re:"):
                continue
            pattern = k[3:]
            try:
                if re.search(pattern, repo):
                    cla.log.debug("Found enable_co_authors for repo %s: %s via regex pattern: %s", org_repo, v, pattern)
                    return v
            except re.error as e:
                cla.log.warning("Invalid regex in enable_co_authors: %s (%s) for repo: %s", k, e, org_repo)
                continue

        # 3. Wildcard fallback
        if '*' in enable_co_authors:
            cla.log.debug("No enable_co_authors found for repo %s, using wildcard: %s", org_repo, enable_co_authors['*'])
            return enable_co_authors['*']

        # 4. No match
        cla.log.debug("No enable_co_authors found for repo %s, skipping co-authors", org_repo)
        return False

    def update_merge_group(self, installation_id, github_repository_id, merge_group_sha, pull_request_id):
        fn = "update_queue_entry"

        # Note: late 2021/early 2022 we observed that sometimes we get the event for a PR, then go back to GitHub
        # to query for the PR details and discover the PR is 404, not available for some reason.  Added retry
        # logic to retry a couple of times to address any timing issues.

        try:
            # Get the pull request details from GitHub
            cla.log.debug(
                f"{fn} - fetching pull request details for GH repo ID: {github_repository_id} "
                f"PR ID: {pull_request_id}..."
            )
            pull_request = self.get_pull_request_retry(github_repository_id, pull_request_id, installation_id)
        except Exception as e:
            cla.log.warning(
                f"{fn} - unable to load PR {pull_request_id} from GitHub repository "
                f"{github_repository_id} using installation id {installation_id} - error: {e}"
            )
            return

        try:
            # Get existing repository info using the repository's external ID,
            # which is the repository ID assigned by github.
            cla.log.debug(f"{fn} - PR: {pull_request.number}, Loading GitHub repository by id: {github_repository_id}")
            repository = Repository().get_repository_by_external_id(github_repository_id, "github")
            if repository is None:
                cla.log.warning(
                    f"{fn} - PR: {pull_request.number}, Failed to load GitHub repository by "
                    f"id: {github_repository_id} in our DB - repository reference is None - "
                    "Is this org/repo configured in the Project Console?"
                    " Unable to update status."
                )
                # Optionally, we could add a comment or add a status to the PR informing the users that the EasyCLA
                # app/bot is enabled in GitHub (which is why we received the event in the first place), but the
                # repository is not setup/configured in EasyCLA from the administration console
                return

            # If the repository is not enabled in our database, we don't process it.
            if not repository.get_enabled():
                cla.log.warning(
                    f"{fn} - repository {repository.get_repository_url()} associated with "
                    f"PR: {pull_request.number} is NOT enabled"
                    " - ignoring PR request"
                )
                # Optionally, we could add a comment or add a status to the PR informing the users that the EasyCLA
                # app/bot is enabled in GitHub (which is why we received the event in the first place), but the
                # repository is NOT enabled in the administration console
                return

        except DoesNotExist:
            cla.log.warning(
                f"{fn} - PR: {pull_request.number}, could not find repository with the "
                f"repository ID: {github_repository_id}"
            )
            cla.log.warning(
                f"{fn} - PR: {pull_request.number}, failed to update change request of "
                f"repository {github_repository_id} - returning"
            )
            return

        # Get GitHub Organization name that the repository is configured to.
        organization_name = repository.get_repository_organization_name()
        cla.log.debug(f"{fn} - PR: {pull_request.number}, determined github organization is: {organization_name}")

        # Check that the GitHub Organization exists.
        github_org = GitHubOrg()
        try:
            github_org.load(organization_name)
        except DoesNotExist:
            cla.log.warning(
                f"{fn} - PR: {pull_request.number}, Could not find Github Organization "
                f"with the following organization name: {organization_name}"
            )
            cla.log.warning(
                f"{fn}- PR: {pull_request.number}, Failed to update change request of "
                f"repository {github_repository_id} - returning"
            )
            return

            # Ensure that installation ID for this organization matches the given installation ID
        if github_org.get_organization_installation_id() != installation_id:
            cla.log.warning(
                f"{fn} - PR: {pull_request.number}, "
                f"the installation ID: {github_org.get_organization_installation_id()} "
                f"of this organization does not match installation ID: {installation_id} "
                "given by the pull request."
            )
            cla.log.error(
                f"{fn} - PR: {pull_request.number}, Failed to update change request "
                f"of repository {github_repository_id} - returning"
            )
            return

        any_missing = False
        try:
            # Get Commit authors
            with_co_authors = self.is_co_authors_enabled_for_repo(github_org.get_enable_co_authors(), repository.get_repository_name())
            commit_authors, any_missing = get_pull_request_commit_authors(self.client, organization_name, pull_request, installation_id, with_co_authors)
            cla.log.debug(f"{fn} - commit author summaries: {commit_authors}")
        except Exception as e:
            cla.log.warning(
                f"{fn} - unable to load commit authors for PR {pull_request_id} from GitHub repository "
                f"{github_repository_id} using installation id {installation_id} - error: {e}"
            )
            return

        project_id = repository.get_repository_project_id()
        project = get_project_instance()
        project.load(project_id)

        signed = []
        missing = []

        # Check if the user has signed the CLA
        cla.log.debug(f"{fn} - checking if the user has signed the CLA...")
        for user_commit_summary in commit_authors:
            handle_commit_from_user(project, user_commit_summary, signed, missing)

        # Skip allowlisted bots per org/repo GitHub login/email regexps
        missing, allowlisted = self.skip_allowlisted_bots(github_org, repository.get_repository_name(), missing)
        if allowlisted is not None and len(allowlisted) > 0:
            cla.log.debug(f"{fn} - adding {len(allowlisted)} allowlisted actors to signed list")
            signed.extend(allowlisted)
        signed = dedup_and_sort(signed)
        missing = dedup_and_sort(missing)

        # update Merge group status
        self.update_merge_group_status(
            installation_id, github_repository_id, pull_request, merge_group_sha, signed, missing, any_missing, project.get_version()
        )

    def update_change_request(self, installation_id, github_repository_id, change_request_id):
        fn = "update_change_request"
        # Queries GH for the complete pull request details, see:
        # https://developer.github.com/v3/pulls/#response-1

        # Note: late 2021/early 2022 we observed that sometimes we get the event for a PR, then go back to GitHub
        # to query for the PR details and discover the PR is 404, not available for some reason.  Added retry
        # logic to retry a couple of times to address any timing issues.
        pull_request = {}
        tries = 3
        for i in range(tries):
            try:
                # check if change_request_id is a valid int
                _ = int(change_request_id)
                pull_request = self.get_pull_request(github_repository_id, change_request_id, installation_id)
            except ValueError as ve:
                cla.log.error(
                    f"{fn} - Invalid PR: {change_request_id} - error: {ve}. Unable to fetch "
                    f"PR {change_request_id} from GitHub repository {github_repository_id} "
                    f"using installation id {installation_id}."
                )
                if i <= tries:
                    cla.log.debug(f"{fn} - attempt {i + 1} - waiting to retry...")
                    time.sleep(2)
                    continue
                else:
                    cla.log.warning(
                        f"{fn} - attempt {i + 1} - exhausted retries - unable to load PR "
                        f"{change_request_id} from GitHub repository {github_repository_id} "
                        f"using installation id {installation_id}."
                    )
                    # TODO: DAD - possibly update the PR status?
                    return
            # Fell through - no error, exit loop and continue on
            break
        cla.log.debug(f"{fn} - retrieved pull request: {pull_request}")

        try:
            # Get existing repository info using the repository's external ID,
            # which is the repository ID assigned by github.
            cla.log.debug(f"{fn} - PR: {pull_request.number}, Loading GitHub repository by id: {github_repository_id}")
            repository = Repository().get_repository_by_external_id(github_repository_id, "github")
            if repository is None:
                cla.log.warning(
                    f"{fn} - PR: {pull_request.number}, Failed to load GitHub repository by "
                    f"id: {github_repository_id} in our DB - repository reference is None - "
                    "Is this org/repo configured in the Project Console?"
                    " Unable to update status."
                )
                # Optionally, we could add a comment or add a status to the PR informing the users that the EasyCLA
                # app/bot is enabled in GitHub (which is why we received the event in the first place), but the
                # repository is not setup/configured in EasyCLA from the administration console
                return

            # If the repository is not enabled in our database, we don't process it.
            if not repository.get_enabled():
                cla.log.warning(
                    f"{fn} - repository {repository.get_repository_url()} associated with "
                    f"PR: {pull_request.number} is NOT enabled"
                    " - ignoring PR request"
                )
                # Optionally, we could add a comment or add a status to the PR informing the users that the EasyCLA
                # app/bot is enabled in GitHub (which is why we received the event in the first place), but the
                # repository is NOT enabled in the administration console
                return

        except DoesNotExist:
            cla.log.warning(
                f"{fn} - PR: {pull_request.number}, could not find repository with the "
                f"repository ID: {github_repository_id}"
            )
            cla.log.warning(
                f"{fn} - PR: {pull_request.number}, failed to update change request of "
                f"repository {github_repository_id} - returning"
            )
            return

        # Get GitHub Organization name that the repository is configured to.
        organization_name = repository.get_repository_organization_name()
        cla.log.debug(f"{fn} - PR: {pull_request.number}, determined github organization is: {organization_name}")

        # Check that the GitHub Organization exists.
        github_org = GitHubOrg()
        try:
            github_org.load(organization_name)
        except DoesNotExist:
            cla.log.warning(
                f"{fn} - PR: {pull_request.number}, Could not find Github Organization "
                f"with the following organization name: {organization_name}"
            )
            cla.log.warning(
                f"{fn}- PR: {pull_request.number}, Failed to update change request of "
                f"repository {github_repository_id} - returning"
            )
            return

            # Ensure that installation ID for this organization matches the given installation ID
        if github_org.get_organization_installation_id() != installation_id:
            cla.log.warning(
                f"{fn} - PR: {pull_request.number}, "
                f"the installation ID: {github_org.get_organization_installation_id()} "
                f"of this organization does not match installation ID: {installation_id} "
                "given by the pull request."
            )
            cla.log.error(
                f"{fn} - PR: {pull_request.number}, Failed to update change request "
                f"of repository {github_repository_id} - returning"
            )
            return

        # Get all unique users/authors involved in this PR - returns a List[UserCommitSummary] objects
        with_co_authors = self.is_co_authors_enabled_for_repo(github_org.get_enable_co_authors(), repository.get_repository_name())
        commit_authors, any_missing = get_pull_request_commit_authors(self.client, organization_name, pull_request, installation_id, with_co_authors)

        cla.log.debug(
            f"{fn} - PR: {pull_request.number}, found {len(commit_authors)} unique commit author summaries "
            f"for pull request: {pull_request.number}"
        )

        # Retrieve project ID from the repository.
        project_id = repository.get_repository_project_id()
        project = get_project_instance()
        project.load(str(project_id))

        try:
            # Save entry into the cla-{stage}-store table for active PRs
            set_active_pr_metadata(
                github_author_username=pull_request.user.login,
                github_author_email=pull_request.user.email,
                cla_group_id=project.get_project_id(),
                repository_id=str(github_repository_id),
                pull_request_id=str(change_request_id),
            )
        except Exception as e:
            cla.log.error(f"{fn} - problem saving PR metadata for PR: {pull_request.number}, error: {e}")

        # Find users who have signed and who have not signed.
        signed = []
        missing = []
        futures = []

        cla.log.debug(
            f"{fn} - PR: {pull_request.number}, scanning users - " "determining who has signed a CLA an who has not."
        )

        with concurrent.futures.ThreadPoolExecutor(max_workers=30) as executor:
            for user_commit_summary in commit_authors:
                # cla.log.debug(f"{fn} - PR: {pull_request.number} for user: {user_commit_summary}")
                futures.append(executor.submit(handle_commit_from_user, project, user_commit_summary, signed, missing))

            # Wait for all threads to be finished before moving on
            executor.shutdown(wait=True)

        for future in concurrent.futures.as_completed(futures):
            # cla.log.debug(f"{fn} - ThreadClosed for handle_commit_from_user")
            try:
                future.result()
            except Exception as e:
                cla.log.error(f"{fn} - Exception in commit author thread for PR: {pull_request.number}, error: {e}")

        # Skip allowlisted bots per org/repo GitHub login/email regexps
        missing, allowlisted = self.skip_allowlisted_bots(github_org, repository.get_repository_name(), missing)
        if allowlisted is not None and len(allowlisted) > 0:
            cla.log.debug(f"{fn} - adding {len(allowlisted)} allowlisted actors to signed list")
            signed.extend(allowlisted)
        signed = dedup_and_sort(signed)
        missing = dedup_and_sort(missing)
        # At this point, the signed and missing lists are now filled and updated with the commit user info

        cla.log.debug(
            f"{fn} - PR: {pull_request.number}, "
            f"updating github pull request for repo: {github_repository_id}, "
            f"with signed authors: {signed} "
            f"with missing authors: {missing}"
        )
        repository_name = repository.get_repository_name()
        update_pull_request(
            installation_id=installation_id,
            github_repository_id=github_repository_id,
            pull_request=pull_request,
            repository_name=repository_name,
            signed=signed,
            missing=missing,
            any_missing=any_missing,
            project_version=project.get_version(),
        )

    def property_matches(self, pattern, value):
        """
        Returns True if value matches the pattern.
        - '*' matches anything
        - '' matches None or empty string
        - 're:...' matches regex - value must be set
        - otherwise, exact match
        """
        try:
            if pattern == '*':
                return True
            if pattern == '' and (value is None or value == ''):
                return True
            if value is None or value == '':
                return False
            if pattern.startswith('re:'):
                regex = pattern[3:]
                return re.search(regex, value) is not None
            return value == pattern
        except Exception as exc:
            cla.log.warning("Error in property_matches: pattern=%s, value=%s, exc=%s", pattern, value, exc)
            return False

    def is_actor_skipped(self, actor, config):
        """
        Returns True if the actor should be skipped (allowlisted) based on config pattern.
        config: '<login_pattern>;<email_pattern>;<name_pattern>'
        If any pattern is missing, it defaults to '' which is special and matches None or empty string.
        It returns true if ANY config entry matches or false if there is no match in any config entry.
        """
        try:
            # If config is a list/array, check all
            if isinstance(config, (list, tuple)):
                for entry in config:
                    if self.is_actor_skipped(actor, entry):
                        return True
                return False
            # Otherwise, treat as string pattern
            parts = config.split(';')
            while len(parts) < 3:
                parts.append('')
            login_pattern, email_pattern, name_pattern = parts[:3]
            login = getattr(actor, "author_login", None)
            email = getattr(actor, "author_email", None)
            name = getattr(actor, "author_name", None)
            return (
                self.property_matches(login_pattern, login) and
                self.property_matches(email_pattern, email) and
                self.property_matches(name_pattern, name)
            )
        except Exception as exc:
            cla.log.warning("Error in is_actor_skipped: config=%s, actor=%s, exc=%s", config, actor, exc)
            return False


    def strip_org(self, repo_full):
        """
        Removes the organization part from the repository name.
        """
        if '/' in repo_full:
            return repo_full.split('/', 1)[1]
        return repo_full

    def parse_config_patterns(self, config):
        """
        Returns a list of pattern strings.
        - If config starts with '[' and ends with ']', splits by '||'.
        - Otherwise, returns [config].
        """
        config = config.strip()
        if config.startswith('[') and config.endswith(']'):
            inner = config[1:-1]
            return [p.strip() for p in inner.split('||')]
        else:
            return [config]

    def safe_getattr(self, obj, attr, default='(null)'):
        """Returns obj.attr or default if attr is missing or None."""
        val = getattr(obj, attr, default)
        if val is None:
            return default
        return val

    def skip_allowlisted_bots(self, org_model, org_repo, actors_missing_cla) -> Tuple[List[UserCommitSummary], List[UserCommitSummary]]:
        """
        Check if the actors are allowlisted based on the skip_cla configuration.
        Returns a tuple of two lists:
        - actors_missing_cla: actors who still need to sign the CLA after checking skip_cla
        - allowlisted_actors: actors who are skipped due to skip_cla configuration
        :param org_model: The GitHub organization model instance.
        :param org_repo: The repository name in the format 'org/repo'.
        :param actors_missing_cla: List of UserCommitSummary objects representing actors who are missing CLA.
        :return: Tuple of (actors_missing_cla, allowlisted_actors)
        : in cla-{stage}-github-orgs table there can be a skip_cla field which is a dict with the following structure:
        {
            "repo-name": "<login_pattern>;<email_pattern>;<name_pattern>",
            "re:repo-regexp": "[<login_pattern>;<email_pattern>;<name_pattern>||...]",
            "*": "<login_pattern>"
        }
        where:
        - repo-name is the exact repository name under given org (e.g., "my-repo" not "my-org/my-repo")
        - re:repo-regexp is a regex pattern to match repository names
        - * is a wildcard that applies to all repositories
        - <login_pattern> is a GitHub login pattern (exact match or regex prefixed by re: or match all '*') - defaults to '' if not set
        - <email_pattern> is a GitHub email pattern (exact match or regex prefixed by re: or match all '*') - defaults to '' if not set
        - <name_pattern> is a GitHub name pattern (exact match or regex prefixed by re: or match all '*') - defaults to '' if not set
        :note: '' is a special pattern that matches None or empty string.
        :note: The login (sometimes called username it's the same), email and name patterns are separated by a semicolon (;).
        :note: There can be an array of patterns - it must start with [ and with ] and be || separated.
        :note: If the skip_cla is not set, it will skip the allowlisted bots check.
        """
        try:
            repo = self.strip_org(org_repo)
            skip_cla = org_model.get_skip_cla()
            if skip_cla is None:
                cla.log.debug("skip_cla is not set on '%s', skipping allowlisted bots check", org_repo)
                return actors_missing_cla, []

            if hasattr(skip_cla, "as_dict"):
                skip_cla = skip_cla.as_dict()
            config = ''
            # 1. Exact match
            if repo in skip_cla:
                cla.log.debug("skip_cla config found for repo %s: %s (exact hit)", org_repo, skip_cla[repo])
                config = skip_cla[repo]

            # 2. Regex pattern (if no exact hit)
            if config == '':
                cla.log.debug("No skip_cla config found for repo %s, checking regex patterns", org_repo)
                for k, v in skip_cla.items():
                    if not isinstance(k, str) or not k.startswith("re:"):
                        continue
                    pattern = k[3:]
                    try:
                        if re.search(pattern, repo):
                            config = v
                            cla.log.debug("Found skip_cla config for repo %s: %s via regex pattern: %s", org_repo, config, pattern)
                            break
                    except re.error as e:
                        cla.log.warning("Invalid regex in skip_cla: %s (%s) for repo: %s", k, e, org_repo)
                        continue

            # 3. Wildcard fallback
            if config == '' and '*' in skip_cla:
                cla.log.debug("No skip_cla config found for repo %s, using wildcard config", org_repo)
                config = skip_cla['*']

            # 4. No match
            if config == '':
                cla.log.debug("No skip_cla config found for repo %s, skipping allowlisted bots check", org_repo)
                return actors_missing_cla, []

            actor_debug_data = [
                f"id='{self.safe_getattr(a, 'author_id')}',"
                f"login='{self.safe_getattr(a, 'author_login')}',"
                f"name='{self.safe_getattr(a, 'author_name')}',"
                f"email='{self.safe_getattr(a, 'author_email')}'"
                for a in actors_missing_cla
            ]
            config = self.parse_config_patterns(config)
            cla.log.debug("final skip_cla config for repo %s is %s; actors_missing_cla: [%s]", org_repo, config, ", ".join(actor_debug_data))
            out_actors_missing_cla = []
            allowlisted_actors = []
            seen_actors = set()
            for actor in actors_missing_cla:
                if actor is None:
                    continue
                try:
                    actor_data = "id='{}',login='{}',name='{}',email='{}'".format(
                        self.safe_getattr(actor, "author_id"),
                        self.safe_getattr(actor, "author_login"),
                        self.safe_getattr(actor, "author_name"),
                        self.safe_getattr(actor, "author_email"),
                    )
                    cla.log.debug("Checking actor: %s for skip_cla config: %s", actor_data, config)
                    if self.is_actor_skipped(actor, config):
                        if not actor_data in seen_actors:
                            seen_actors.add(actor_data)
                            msg = "Skipping CLA check for repo='{}', actor: {} due to skip_cla config: '{}'".format(
                                org_repo,
                                actor_data,
                                config,
                            )
                            cla.log.info(msg)
                            Event.create_event(
                                event_type=EventType.BypassCLA,
                                event_data=msg,
                                event_summary=msg,
                                event_user_name=actor_data,
                                contains_pii=True,
                            )
                        actor.authorized = True
                        allowlisted_actors.append(actor)
                        continue
                except Exception as e:
                    cla.log.warning(
                        "Error checking skip_cla for actor '%s' (id='%s', login='%s', name='%s', email='%s'): %s",
                        actor,
                        self.safe_getattr(actor, "author_id"),
                        self.safe_getattr(actor, "author_login"),
                        self.safe_getattr(actor, "author_name"),
                        self.safe_getattr(actor, "author_email"),
                        e,
                    )
                out_actors_missing_cla.append(actor)

            return out_actors_missing_cla, allowlisted_actors
        except Exception as exc:
            cla.log.error(
                "Exception in skip_allowlisted_bots: %s (repo=%s, actors=%s). Disabling skip_cla logic for this run.",
                exc, org_repo, actors_missing_cla
            )
            # Always return all actors if something breaks
            return actors_missing_cla, []


    def get_pull_request(self, github_repository_id, pull_request_number, installation_id):
        """
        Helper method to get the pull request object from GitHub.

        :param github_repository_id: The ID of the GitHub repository.
        :type github_repository_id: int
        :param pull_request_number: The number (not ID) of the GitHub PR.
        :type pull_request_number: int
        :param installation_id: The ID of the GitHub application installed on this repository.
        :type installation_id: int | None
        """
        cla.log.debug("Getting PR %s from GitHub repository %s", pull_request_number, github_repository_id)
        if self.client is None:
            self.client = get_github_integration_client(installation_id)
        repo = self.client.get_repo(int(github_repository_id))
        try:
            return repo.get_pull(int(pull_request_number))
        except UnknownObjectException:
            cla.log.error(
                "Could not find pull request %s for repository %s - ensure it "
                'exists and that your personal access token has the "repo" scope enabled',
                pull_request_number,
                github_repository_id,
            )
        except BadCredentialsException as err:
            cla.log.error("Invalid GitHub credentials provided: %s", str(err))

    def get_github_user_by_email(self, email, installation_id):
        """
        Helper method to get the GitHub user object from GitHub.

        :param email: The email of the GitHub user.
        :type email: string
        :param name: The name of the GitHub user.
        :type name: string
        :param installation_id: The ID of the GitHub application installed on this repository.
        :type installation_id: int | None
        """
        cla.log.debug("Getting GitHub user %s", email)
        if self.client is None:
            self.client = get_github_integration_client(installation_id)
        try:
            cla.log.debug("Searching for GitHub user by email handle: %s", email)
            users_by_email = self.client.search_users(f"{email} in:email")
            if len(list(users_by_email)) == 0:
                cla.log.debug("No GitHub user found with email handle: %s", email)
                return None
            return list(users_by_email)[0]
        except UnknownObjectException:
            cla.log.error("Could not find GitHub user %s", email)
        except BadCredentialsException as err:
            cla.log.error("Invalid GitHub credentials provided: %s", str(err))


    def get_github_user_by_login(self, login, installation_id):
        """
        Helper method to get the GitHub user object from GitHub by their login (username).

        :param login: The login (username) of the GitHub user.
        :type login: string
        :param installation_id: The ID of the GitHub application installed on this repository.
        :type installation_id: int | None
        """
        cla.log.debug("Getting GitHub user by login: %s", login)
        if self.client is None:
            self.client = get_github_integration_client(installation_id)
        try:
            user = self.client.get_user(login)
            return user
        except UnknownObjectException:
            cla.log.error("Could not find GitHub user with login: %s", login)
            return None
        except BadCredentialsException as err:
            cla.log.error("Invalid GitHub credentials provided: %s", str(err))
            return None

    def get_github_user_by_id(self, github_id, installation_id):
        """
        Helper method to get the GitHub user object from GitHub by their numeric ID.

        :param github_id: The numeric GitHub user ID.
        :type github_id: int
        :param installation_id: The ID of the GitHub app installation for this repo.
        :type installation_id: int | None
        """
        cla.log.debug("Getting GitHub user by ID: %s", github_id)
        if self.client is None:
            self.client = get_github_integration_client(installation_id)
        try:
            user = self.client.get_user_by_id(github_id)
            return user
        except UnknownObjectException:
            cla.log.error("Could not find GitHub user with ID: %s", github_id)
            return None
        except BadCredentialsException as err:
            cla.log.error("Invalid GitHub credentials provided: %s", str(err))
            return None


    def get_or_create_user(self, request):
        """
        Helper method to either get or create a user based on the GitHub request made.

        :param request: The hug request object for this API call.
        :type request: Request
        """
        fn = "github_models.get_or_create_user"
        session = self._get_request_session(request)
        github_user = self.get_user_data(session, os.environ["GH_OAUTH_CLIENT_ID"])
        if "error" in github_user:
            # Could not get GitHub user data - maybe user revoked CLA app permissions?
            session = self._get_request_session(request)

            del session["github_oauth2_state"]
            del session["github_oauth2_token"]
            cla.log.warning(f"{fn} - Deleted OAuth2 session data - retrying token exchange next time")
            raise falcon.HTTPError(
                "400 Bad Request", "github_oauth2_token", "Token permissions have been rejected, please try again"
            )

        emails = self.get_user_emails(session, os.environ["GH_OAUTH_CLIENT_ID"])
        if len(emails) < 1:
            cla.log.warning(
                f"{fn} - GitHub user has no verified email address: %s (%s)", github_user["name"], github_user["login"]
            )
            raise falcon.HTTPError(
                "412 Precondition Failed", "email", "Please verify at least one email address with GitHub"
            )

        cla.log.debug(f"{fn} - Trying to load GitHub user by GitHub ID: %s", github_user["id"])
        users = cla.utils.get_user_instance().get_user_by_github_id(github_user["id"])
        if users is not None:
            # Users search can return more than one match - so it's an array - we set the first record value for now??
            user = users[0]
            cla.log.debug(
                f"{fn} - Loaded GitHub user by GitHub ID: %s - %s (%s)",
                user.get_user_name(),
                user.get_user_emails(),
                user.get_user_github_id(),
            )

            # update/set the github username if available
            cla.utils.update_github_username(github_user, user)

            user.set_user_emails(emails)
            user.save()
            return user

        # User not found by GitHub ID, trying by email.
        cla.log.debug(f"{fn} - Could not find GitHub user by GitHub ID: %s", github_user["id"])
        # TODO: This is very slow and needs to be improved - may need a DB schema change.
        # LG: at least it now tries search by index lf_email first and only falls back to slow scan if nothing is found
        users = None
        user = cla.utils.get_user_instance()
        for email in emails:
            users = user.get_user_by_email_fast(email)
            if users is not None:
                break

        if users is not None:
            # Users search can return more than one match - so it's an array - we set the first record value for now??
            user = users[0]
            # Found user by email, setting the GitHub ID
            user.set_user_github_id(github_user["id"])

            # update/set the github username if available
            cla.utils.update_github_username(github_user, user)

            user.set_user_emails(emails)
            user.save()
            cla.log.debug(f"{fn} - Loaded GitHub user by email: {user}")
            return user

        # User not found, create.
        cla.log.debug(f"{fn} - Could not find GitHub user by email: {emails}")
        cla.log.debug(
            f'{fn} - Creating new GitHub user {github_user["name"]} - '
            f'({github_user["id"]}/{github_user["login"]}), '
            f"emails: {emails}"
        )
        user = cla.utils.get_user_instance()
        user.set_user_id(str(uuid.uuid4()))
        user.set_user_emails(emails)
        user.set_user_name(github_user["name"])
        user.set_user_github_id(github_user["id"])
        user.set_user_github_username(github_user["login"])
        user.save()
        return user

    def get_user_data(self, session, client_id):  # pylint: disable=no-self-use
        """
        Mockable method to get user data. Returns all GitHub user data we have
        on the user based on the current OAuth2 session.

        :param session: The current user session.
        :type session: dict
        :param client_id: The GitHub OAuth2 client ID.
        :type client_id: string
        """
        fn = "cla.models.github_models.get_user_data"
        token = session.get("github_oauth2_token")
        if token is None:
            cla.log.error(f"{fn} - unable to load github_oauth2_token from session, session is: {session}")
            return {"error": "could not get user data from session"}

        oauth2 = OAuth2Session(client_id, token=token)
        request = oauth2.get("https://api.github.com/user")
        github_user = request.json()
        cla.log.debug(f"{fn} - GitHub user data: %s", github_user)
        if "message" in github_user:
            cla.log.error(f'{fn} - Could not get user data with OAuth2 token: {github_user["message"]}')
            return {"error": "Could not get user data: %s" % github_user["message"]}
        return github_user

    def get_user_emails(self, session: dict, client_id: str) -> Union[List[str], dict]:  # pylint: disable=no-self-use
        """
        Mockable method to get all user emails based on OAuth2 session.

        :param session: The current user session.
        :type session: dict
        :param client_id: The GitHub OAuth2 client ID.
        :type client_id: string
        """
        emails = self._fetch_github_emails(session=session, client_id=client_id)
        cla.log.debug("GitHub user emails: %s", emails)
        if "error" in emails:
            return emails

        verified_emails = [item["email"] for item in emails if item["verified"]]
        excluded_emails = [email for email in verified_emails if any([email.endswith(e) for e in EXCLUDE_GITHUB_EMAILS])]
        included_emails = [email for email in verified_emails if not any([email.endswith(e) for e in EXCLUDE_GITHUB_EMAILS])]

        if len(included_emails) > 0:
            return included_emails

        # something we're not very happy about but probably it can happen
        return excluded_emails

    def get_primary_user_email(self, request) -> Union[Optional[str], dict]:
        """
        gets the user primary email from the registered emails from the github api
        """
        fn = "github_models.get_primary_user_email"
        try:
            cla.log.debug(f"{fn} - fetching Github primary email")
            session = self._get_request_session(request)
            client_id = os.environ["GH_OAUTH_CLIENT_ID"]
            emails = self._fetch_github_emails(session=session, client_id=client_id)
            if "error" in emails:
                return None

            for email in emails:
                if email.get("verified", False) and email.get("primary", False):
                    return email["email"]
        except Exception as e:
            cla.log.warning(f"{fn} - lookup failed - {e} - returning None")
            return None
        return None

    def _fetch_github_emails(self, session: dict, client_id: str) -> Union[List[dict], dict]:
        """
        Method is responsible for fetching the user emails from /user/emails endpoint
        :param session:
        :param client_id:
        :return:
        """
        fn = "github_models._fetch_github_emails"  # function name
        # Use the user's token to fetch their public email(s) - don't use the system token as this endpoint won't work
        # as expected
        token = session.get("github_oauth2_token")
        if token is None:
            cla.log.warning(f"{fn} - unable to load github_oauth2_token from the session - session is empty")
        oauth2 = OAuth2Session(client_id, token=token)
        request = oauth2.get("https://api.github.com/user/emails")
        resp = request.json()
        if "message" in resp:
            cla.log.warning(f'{fn} - could not get user emails with OAuth2 token: {resp["message"]}')
            return {"error": "Could not get user emails: %s" % resp["message"]}
        return resp

    def process_reopened_pull_request(self, data):
        """
        Helper method to process a re-opened GitHub PR.

        Simply calls the self.process_opened_pull_request() method with the data provided.

        :param data: The data provided by the GitHub webhook.
        :type data: dict
        """
        return self.process_opened_pull_request(data)

    def process_closed_pull_request(self, data):
        """
        Helper method to process a closed GitHub PR.

        :param data: The data provided by the GitHub webhook.
        :type data: dict
        """
        pass

    def process_synchronized_pull_request(self, data):
        """
        Helper method to process a synchronized GitHub PR.

        Should be called when a new commit comes through on the PR.
        Simply calls the self.process_opened_pull_request() method with the data provided.
        This should re-check all commits for author information.

        :param data: The data provided by the GitHub webhook.
        :type data: dict
        """
        return self.process_opened_pull_request(data)


def create_repository(data):
    """
    Helper method to create a repository object in the CLA database given PR data.

    :param data: The data provided by the GitHub webhook.
    :type data: dict
    :return: The newly created repository object - already in the DB.
    :rtype: cla.models.model_interfaces.Repository
    """
    try:
        repository = cla.utils.get_repository_instance()
        repository.set_repository_id(str(uuid.uuid4()))
        # TODO: Need to use an ID unique across all repository providers instead of namespace.
        full_name = data["repository"]["full_name"]
        namespace = full_name.split("/")[0]
        repository.set_repository_project_id(namespace)
        repository.set_repository_external_id(data["repository"]["id"])
        repository.set_repository_name(full_name)
        repository.set_repository_type("github")
        repository.set_repository_url(data["repository"]["html_url"])
        repository.save()
        return repository
    except Exception as err:
        cla.log.warning("Could not create GitHub repository automatically: %s", str(err))
        return None

def update_cache_after_signature(user, project):
    """
    Helper method to update cache for a user after signature completion.
    This ensures the user is marked as authorized for the project in the cache.

    :param user: The user who completed the signature
    :type user: User
    :param project: The project for which the signature was completed
    :type project: Project
    """
    fn = "update_cache_after_signature"

    project_id = project.get_project_id()
    github_id = user.get_user_github_id()
    github_username = user.get_user_github_username()

    if not github_id or not github_username or github_username == '':
        cla.log.debug(f"{fn} - user {user.get_user_id()} lacks GitHub ID or username - skipping cache update")
        return

    if not cla.utils.user_signed_project_signature(user, project):
        cla.log.debug(f"{fn} - user {user.get_user_id()} is not yet authorized for project {project_id} - skipping cache update")
        return

    affiliated = user.get_user_company_id() is not None
    github_username = github_username.lower()

    user_emails = user.get_all_user_emails() or []
    emails = list(dict.fromkeys(email.strip().lower() for email in user_emails if email))
    if not emails:
        cla.log.debug(f"{fn} - user {user.get_user_id()} has no emails - skipping cache update")
        return
    all_emails = ",".join(emails)

    for email in emails:
        project_cache_key = (
            project_id,
            github_id,
            github_username,
            email,
        )
        cache_key = (
            github_id,
            github_username,
            email,
        )

        # Update project-specific cache with authorized=True
        # Format: (user, check_aff: True/False, authorized, affiliated)
        # LG: to write with non-aff mode
        # github_user_cache.set_with_ttl(project_cache_key, (user, False, True, affiliated), PROJECT_CACHE_TTL)
        github_user_cache.set_with_ttl(project_cache_key, (user, True, True, affiliated), PROJECT_CACHE_TTL)

        # Update general cache
        # Format: (user, check_aff: True/False)
        # LG: to write with non-aff mode
        # github_user_cache.set(cache_key, (user, False))
        github_user_cache.set(cache_key, (user, True))

    cla.log.info(f"{fn} - updated github_user_cache for user {github_username} (ID: {github_id}, emails: {all_emails}) "
                 f"on project {project_id} - marked as authorized")

def handle_commit_from_user(
    project, user_commit_summary: UserCommitSummary, signed: List[UserCommitSummary], missing: List[UserCommitSummary]
):  # pylint: disable=too-many-arguments
    """
    Helper method to triage commits between signed and not-signed user signatures.

    :param: project: The project model for this GitHub PR organization.
    :type: project: Project
    :param: user_commit_summary: a user commit summary object
    :type: UserCommitSummary
    :param signed: A list of authors who have signed.
        Should be modified in-place to add the signer information.
    :type: List[UserCommitSummary]
    :param missing: A list of authors who have not signed yet.
        Should be modified in-place to add the missing signer information.
    :type: List[UserCommitSummary]
    """

    fn = "cla.models.github_models.handle_commit_from_user"
    # handle edge case of non existant users
    if not user_commit_summary.is_valid_user():
        cla.log.debug(f"{fn} - summary for an unknown user, adding to missing: {user_commit_summary}")
        missing.append(user_commit_summary)
        return

    # LG: cache_authors - start
    project_cache_key = (
        project.get_project_id(),
        user_commit_summary.author_id,
        (user_commit_summary.author_login or '').lower(),
        (user_commit_summary.author_email or '').strip().lower(),
    )
    # Per-project cache - also caches per-project signatures status and affiliation
    # (project_id, id, login, email) -> (user || None, check_aff, authorized, affiliated)
    # check_aff flag is needed because below code only checked for affiliation in else branch (when user was found by author_id)
    value, hit = github_user_cache.get(project_cache_key)
    cla.log.debug(f"{fn} - per-project cache: {project_cache_key} -> ({value}, {hit})")
    if hit:
        user, check_aff, authorized, affiliated = value
        if user is None:
            missing.append(user_commit_summary)
            cla.log.debug(f"{fn} - per-project cache: negative case: aff mode: {check_aff}")
            return
        if check_aff:
            cla.log.debug(f"{fn} - per-project cache: aff mode, user: {user}")
            if authorized:
                user_commit_summary.authorized = True
                signed.append(user_commit_summary)
                cla.log.debug(f"{fn} - per-project cache: aff mode: authorized & signed")
                return
            if not affiliated:
                missing.append(user_commit_summary)
                cla.log.debug(f"{fn} - per-project cache: aff mode: no company_id, missing")
                return
            user_commit_summary.affiliated = True
            # LG: this should return user_commit_summary as signed IMHO (see flow for general cache, it also adds to missing as the original code does the same)
            # General caching checks for project signature but also adds to missing no matter if signature is found or not, same with "cold" code path (no cache hit)
            cla.log.debug(f"{fn} - per-project cache: aff mode: affiliated, but adding to missing")
            missing.append(user_commit_summary)
        else:
            cla.log.debug(f"{fn} - per-project cache: non-aff mode, user: {user}")
            if authorized:
                user_commit_summary.authorized = True
                signed.append(user_commit_summary)
                cla.log.debug(f"{fn} - per-project cache: non-aff mode: authorized & signed")
                return
            cla.log.debug(f"{fn} - per-project cache: non-aff mode: no authorized, missing")
            missing.append(user_commit_summary)
        cla.log.debug(f"{fn} - per-project cache: done, returning")
        return
    # General cache (without project) - can only cache author details, but not per-project signature details
    # (id, login, email) -> (user || None, check_aff)
    cache_key = (
        user_commit_summary.author_id,
        (user_commit_summary.author_login or '').lower(),
        (user_commit_summary.author_email or '').strip().lower(),
    )
    value, hit = github_user_cache.get(cache_key)
    cla.log.debug(f"{fn} - cache: {cache_key} -> ({value}, {hit})")
    if hit:
        user, check_aff = value
        if user is None:
            missing.append(user_commit_summary)
            cla.log.debug(f"{fn} - cache: negative case: aff mode: {check_aff}")
            github_user_cache.set_with_ttl(project_cache_key, (None, False, False, False), NEGATIVE_CACHE_TTL)
            return
        if check_aff:
            cla.log.debug(f"{fn} - cache: aff mode, user: {user}")
            if cla.utils.user_signed_project_signature(user, project):
                user_commit_summary.authorized = True
                signed.append(user_commit_summary)
                cla.log.debug(f"{fn} - cache: aff mode: authorized & signed")
                github_user_cache.set_with_ttl(project_cache_key, (user, True, True, False), PROJECT_CACHE_TTL)
                return
            if user.get_user_company_id() is None:
                missing.append(user_commit_summary)
                cla.log.debug(f"{fn} - cache: aff mode: no company_id, missing")
                github_user_cache.set_with_ttl(project_cache_key, (user, True, False, False), NEGATIVE_CACHE_TTL)
                return
            user_commit_summary.affiliated = True
            cla.log.debug(f"{fn} - cache: aff mode: affiliated")
            signatures = cla.utils.get_signature_instance().get_signatures_by_project(
                project_id=project.get_project_id(),
                signature_signed=True,
                signature_approved=True,
                signature_type="ccla",
                signature_reference_type="company",
                signature_reference_id=user.get_user_company_id(),
                signature_user_ccla_company_id=None,
            )
            cla.log.debug(f"{fn} - cache: aff mode: #signatures: {len(signatures)}")
            approved = False
            for signature in signatures:
                if cla.utils.is_approved(
                    signature,
                    email=user_commit_summary.author_email,
                    github_id=user_commit_summary.author_id,
                    github_username=user_commit_summary.author_login,
                ):
                    user_commit_summary.authorized = True
                    approved = True
                    cla.log.debug(f"{fn} - cache: aff mode: authorized signature")
                    break
            if approved:
                # LG: this should return user_commit_summary as signed IMHO, but I'm keeping this logic for compatibility
                cla.log.debug(f"{fn} - cache: aff mode: authorized found, but adding to missing")
            else:
                cla.log.debug(f"{fn} - cache: aff mode: no authorized found, adding to missing")
            missing.append(user_commit_summary)
            github_user_cache.set_with_ttl(project_cache_key, (user, True, False, True), NEGATIVE_CACHE_TTL)
        else:
            cla.log.debug(f"{fn} - cache: non-aff mode, user: {user}")
            if cla.utils.user_signed_project_signature(user, project):
                user_commit_summary.authorized = True
                signed.append(user_commit_summary)
                cla.log.debug(f"{fn} - cache: non-aff mode: authorized & signed")
                github_user_cache.set_with_ttl(project_cache_key, (user, False, True, False), PROJECT_CACHE_TTL)
                return
            cla.log.debug(f"{fn} - cache: non-aff mode: no authorized, missing")
            missing.append(user_commit_summary)
            github_user_cache.set_with_ttl(project_cache_key, (user, False, False, False), NEGATIVE_CACHE_TTL)
        cla.log.debug(f"{fn} - cache: done, returning")
        return
    # LG: cache_authors - end

    # attempt to lookup the user in our database by GH id -
    # may return multiple users that match this author_id
    users = cla.utils.get_user_instance().get_user_by_github_id(user_commit_summary.author_id)
    if users is None:
        # GitHub user not in system yet, signature does not exist for this user.
        cla.log.debug(
            f"{fn} - User commit summary: {user_commit_summary} "
            f"lookup by github numeric id not found in our database, "
            "attempting to looking up user by email..."
        )

        # Try looking up user by email as a fallback
        users = cla.utils.get_user_instance().get_user_by_email(user_commit_summary.author_email)
        if users is None:
            # Try looking up user by github username
            cla.log.debug(
                f"{fn} - User commit summary: {user_commit_summary} "
                f"lookup by github email not found in our database, "
                "attempting to looking up user by github username..."
            )
            users = cla.utils.get_user_instance().get_user_by_github_username(user_commit_summary.author_login)

        # Got one or more records by searching the email or username
        if users is not None:
            cla.log.debug(
                f"{fn} - Found {len(users)} GitHub user(s) matching " f"github email: {user_commit_summary.author_email}"
            )

            for user in users:
                cla.log.debug(f"{fn} - GitHub user found in our database: {user}")

                # For now, accept non-github users as legitimate users.
                # Does this user have a signed signature for this project? If so, add to the signed list and return,
                # no reason to continue looking
                if cla.utils.user_signed_project_signature(user, project):
                    user_commit_summary.authorized = True
                    signed.append(user_commit_summary)
                    # set check_aff flag to false as in this case we didn't check affiliated flag
                    cla.log.debug(f"{fn} - store cache non-aff mode: authorized: {project_cache_key}: {users}")
                    github_user_cache.set_with_ttl(project_cache_key, (user, False, True, False), PROJECT_CACHE_TTL)
                    github_user_cache.set(cache_key, (user, False))
                    return

            # Didn't find a signed signature for this project - add to our missing bucket list
            # author_info consists of: [author_id, author_login, author_username, author_email]
            missing.append(user_commit_summary)
        else:
            # Not seen this user before - no record on file in our user's database
            cla.log.debug(
                f"{fn} - User commit summary: {user_commit_summary} " f"lookup by email in our database failed - not found"
            )

            # This bit of logic below needs to be reconsidered - query logic takes a very long time for large
            # projects like CNCF which significantly delays updating the GH PR status.
            # Revisit once we add more indexes to the table

            # # Check to see if not found user is allowlisted to assist in triaging github comment
            # # Search for the CCLA signatures for this project - wish we had a company ID to restrict the query...
            # signatures = cla.utils.get_signature_instance().get_signatures_by_project(
            #     project.get_project_id(),
            #     signature_signed=True,
            #     signature_approved=True,
            #     signature_reference_type='company')
            #
            # list_author_info = list(author_info)
            # for signature in signatures:
            #     if cla.utils.is_allowlisted(
            #             signature,
            #             email=author_email,
            #             github_id=author_id,
            #             github_username=author_username
            #     ):
            #         # Append allowlisted flag to the author info list
            #         cla.log.debug(f'Github user(id:{author_id}, '
            #                       f'user: {author_username}, '
            #                       f'email {author_email}) is allowlisted but not a CLA user')
            #         list_author_info.append(True)
            #         break
            # missing.append((commit_sha, list_author_info))

            # For now - we'll just return the author info as a list without the flag to indicate that they have been on
            # the approved list for any company/signature
            # author_info consists of: [author_id, author_login, author_username, author_email]
            missing.append(user_commit_summary)
        # set check_aff flag to false as in this case we didn't check affiliated flag, this can also store None (negative cache)
        cla.log.debug(f"{fn} - store cache non-aff mode: missing: {project_cache_key}: {users}")
        github_user_cache.set_with_ttl(project_cache_key, (None, False, False, False), NEGATIVE_CACHE_TTL)
        github_user_cache.set_with_ttl(cache_key, (None, False), NEGATIVE_CACHE_TTL)
    else:
        cla.log.debug(
            f"{fn} - Found {len(users)} GitHub user(s) matching "
            f"github id: {user_commit_summary.author_id} in our database"
        )
        if len(users) > 1:
            cla.log.warning(
                f"{fn} - more than 1 user found in our user database - user: {users} - " f"will ONLY evaluate the first one"
            )

        # Just review the first user that we were able to fetch from our DB
        user = users[0]
        cla.log.debug(f"{fn} - GitHub user found in our database: {user}")

        # Does this user have a signed signature for this project? If so, add to the signed list and return,
        # no reason to continue looking
        if cla.utils.user_signed_project_signature(user, project):
            user_commit_summary.authorized = True
            signed.append(user_commit_summary)
            # set check_aff flag to true in this case, as this code branch checks for affiliation, also store only 1st user as this branches considers only 1st user
            cla.log.debug(f"{fn} - store cache aff mode: authorized: {project_cache_key}: {user}")
            github_user_cache.set_with_ttl(project_cache_key, (user, True, True, False), PROJECT_CACHE_TTL)
            github_user_cache.set(cache_key, (user, True))
            return

        # If the user does not have a company ID assigned, then they have not been associated with a company as
        # part of the Contributor console workflow
        if user.get_user_company_id() is None:
            # User is not affiliated with a company
            missing.append(user_commit_summary)
            # set check_aff flag to true in this case, as this code branch checks for affiliation, also store only 1st user as this branches considers only 1st user
            cla.log.debug(f"{fn} - store cache aff mode: no company_id: {project_cache_key}: {user}")
            github_user_cache.set_with_ttl(project_cache_key, (user, True, False, False), NEGATIVE_CACHE_TTL)
            github_user_cache.set_with_ttl(cache_key, (user, True), NEGATIVE_CACHE_TTL)
            return

        # Mark the user as having a company affiliation
        user_commit_summary.affiliated = True

        # Perform a specific search for the user's project + company + CCLA
        signatures = cla.utils.get_signature_instance().get_signatures_by_project(
            project_id=project.get_project_id(),
            signature_signed=True,
            signature_approved=True,
            signature_type="ccla",
            signature_reference_type="company",
            signature_reference_id=user.get_user_company_id(),
            signature_user_ccla_company_id=None,
        )

        # Should only return one signature record
        cla.log.debug(
            f"{fn} - Found {len(signatures)} CCLA signatures for company: {user.get_user_company_id()}, "
            f"project: {project.get_project_id()} in our database."
        )

        # Should never happen - warn if we see this
        if len(signatures) > 1:
            cla.log.warning(f"{fn} - more than 1 CCLA signature record found in our database - signatures: {signatures}")

        for signature in signatures:
            if cla.utils.is_approved(
                signature,
                email=user_commit_summary.author_email,
                github_id=user_commit_summary.author_id,
                github_username=user_commit_summary.author_login,  # double check this...
            ):
                cla.log.debug(
                    f"{fn} - User Commit Summary: {user_commit_summary}, "
                    "is on one of the approval lists, but not affiliated with a company"
                )
                user_commit_summary.authorized = True
                # LG: user_commit_summary should be added to signed in this case IMHO, but not changing it now, if changed then caching must be updated as well (it currently keeps the same logic for compatibility)
                break

        missing.append(user_commit_summary)
        # set check_aff flag to true in this case, as this code branch checks for affiliation, also store only 1st user as this branches considers only 1st user
        cla.log.debug(f"{fn} - store cache aff mode: missing: {project_cache_key}: {user}")
        github_user_cache.set_with_ttl(project_cache_key, (user, True, False, True), NEGATIVE_CACHE_TTL)
        github_user_cache.set_with_ttl(cache_key, (user, True), NEGATIVE_CACHE_TTL)


def get_merge_group_commit_authors(merge_group_sha, installation_id=None) -> List[UserCommitSummary]:
    """
    Helper function to extract all committer information for a GitHub merge group.

    :param: merge_group_sha: A GitHub merge group sha to examine.
    :type: merge_group_sha: string
    :return: A list of User Commit Summary objects containing the commit sha and available user information
    """

    fn = "cla.models.github_models.get_merge_group_commit_authors"
    cla.log.debug(f"Querying merge group {merge_group_sha} for commit authors...")
    commit_authors = []
    try:
        g = cla.utils.get_github_integration_instance(installation_id=installation_id)
        commit = g.get_commit(merge_group_sha)
        for parent in commit.parents:
            try:
                cla.log.debug(f"{fn} - Querying parent commit {parent.sha} for commit authors...")
                commit = g.get_commit(parent.sha)
                cla.log.debug(f"{fn} - Found {commit.commit.author.name} as the author of parent commit {parent.sha}")
                commit_authors.append(
                    UserCommitSummary(
                        parent.sha,
                        commit.author.id,
                        commit.author.login,
                        commit.author.name,
                        commit.author.email,
                        False,
                        False,
                    )
                )
            except (GithubException, IncompletableObject) as e:
                cla.log.warning(f"{fn} - Unable to query parent commit {parent.sha} for commit authors: {e}")
                commit_authors.append(UserCommitSummary(parent.sha, None, None, None, None, False, False))

    except Exception as e:
        cla.log.warning(f"{fn} - Unable to query merge group {merge_group_sha} for commit authors: {e}")

    return commit_authors

def expand_with_co_authors(commit, pr, installation_id, commit_authors) -> bool:
    """
    Helper to append UserCommitSummary objects for all co-authors to commit_authors list.
    """
    co_authors = cla.utils.get_co_authors_from_commit(commit)
    missing = False
    for co_author in co_authors:
        summary, found = get_co_author_commits(co_author, commit.sha, pr, installation_id)
        commit_authors.append(summary)
        if not missing and not found:
            missing = True
    return missing

def expand_with_co_authors_from_message(commit_sha: str, message: Optional[str], pr: int, installation_id, commit_authors) -> bool:
    """
    Append UserCommitSummary objects for co-authors parsed from message.
    """
    co_authors = cla.utils.get_co_authors_from_message(message)
    missing = False
    for co_author in co_authors:
        summary, found = get_co_author_commits(co_author, commit_sha, pr, installation_id)
        commit_authors.append(summary)
        if not missing and not found:
            missing = True
    return missing


def get_author_summary(commit, pr, installation_id, with_co_authors) -> Tuple[List[UserCommitSummary], bool]:
    """
    Helper function to extract author information from a GitHub commit.
    :param commit: A GitHub commit object.
    :type commit: github.Commit.Commit
    :param pr: PR number
    :type pr: int
    """
    fn = "cla.models.github_models.get_author_summary"
    commit_authors = []

    # get id, login, name, email from commit.author and commit.commit.author
    id, login, name, email = None, None, None, None
    try:
        id = commit.author.id
    except (AttributeError, GithubException, IncompletableObject):
        pass

    try:
        login = commit.author.login
    except (AttributeError, GithubException, IncompletableObject):
        pass

    try:
        name = commit.author.name
        if name is None or name.strip() == "":
            name = commit.commit.author.name
    except (AttributeError, GithubException, IncompletableObject):
        try:
            name = commit.commit.author.name
        except (AttributeError, GithubException, IncompletableObject):
            pass

    try:
        email = commit.author.email
        if email is None or email.strip() == "":
            email = commit.commit.author.email
    except (AttributeError, GithubException, IncompletableObject):
        try:
            email = commit.commit.author.email
        except (AttributeError, GithubException, IncompletableObject):
            pass

    cla.log.debug(f"{fn}: (id: {id}, login: {login}, name: {name}, email: {email})")
    commit_author_summary = UserCommitSummary(
        commit.sha,
        id,
        login,
        name,
        email,
        False,
        False,  # default not authorized - will be evaluated and updated later
    )
    cla.log.debug(f"{fn} - PR: {pr}, {commit_author_summary}")
    commit_authors.append(commit_author_summary)
    missing = False
    if with_co_authors is True:
        missing = expand_with_co_authors(commit, pr, installation_id, commit_authors)
    return (commit_authors, missing)

def pygithub_graphql(g, query: str, variables: dict | None = None):
    """
    Minimal GraphQL client using PyGithub's internal requester.
    Works on older PyGithub versions lacking Github.graphql().
    """
    try:
        # LG: note that this uses internal PyGithub API - may break in future versions:
        # g._Github__requester.requestJsonAndCheck
        if hasattr(g, "graphql"):
            return g.graphql(query, variables or {})

        headers = {
            "Accept": "application/vnd.github+json",
            "Content-Type": "application/json",
        }
        _, data = g._Github__requester.requestJsonAndCheck(
            "POST",
            "/graphql",
            input={"query": query, "variables": variables or {}},
            headers=headers,
        )
        if isinstance(data, dict) and data.get("errors"):
            errs = data["errors"]
            paths = [e.get("path") for e in errs]
            msgs = [e.get("message") for e in errs]
            cla.log.error(f"GraphQL errors: {msgs} (paths={paths}, all={errs!r})")
            return None
        return data.get("data")
    except Exception as exc:
        cla.log.error(f"GraphQL query: {query} failed: {exc}")
        return None

def iter_pr_commits_full(g, owner: str, repo_name: str, number: int, page_size: int = 100):
    page_size = max(1, min(100, page_size))
    query = """
    query($owner:String!, $name:String!, $number:Int!, $pageSize:Int!, $cursor:String) {
      repository(owner:$owner, name:$name) {
        pullRequest(number:$number) {
          commits(first:$pageSize, after:$cursor) {
            pageInfo { hasNextPage endCursor }
            nodes {
              commit {
                oid
                message
                author {
                  name       # commit metadata author name
                  email      # commit metadata author email
                  user {
                    databaseId   # numeric id (REST-compatible)
                    login
                    name         # profile name (preferred)
                    email        # profile email (often null without extra scopes)
                  }
                }
              }
            }
          }
        }
      }
    }"""
    cursor = None
    while True:
        res = pygithub_graphql(
            g,
            query,
            {"owner": owner, "name": repo_name, "number": number,
             "pageSize": page_size, "cursor": cursor},
        )
        if res is None:
            cla.log.error(f"Failed to fetch commits for {owner}/{repo_name} PR #{number}")
            raise ValueError("failed to fetch commits using GraphQL")
        commits = res["repository"]["pullRequest"]["commits"]
        for n in commits["nodes"]:
            c = n["commit"]
            a = c.get("author") or {}
            u = a.get("user") or {}

            # id     := commit.author.id
            # login  := commit.author.login
            # name   := commit.author.name or commit.commit.author.name
            # email  := commit.author.email or commit.commit.author.email
            author_id    = u.get("databaseId")
            author_login = u.get("login")

            user_name    = (u.get("name") or "").strip() if isinstance(u.get("name"), str) else None
            commit_name  = (a.get("name") or "").strip() if isinstance(a.get("name"), str) else None
            author_name  = user_name or commit_name or None

            user_email   = (u.get("email") or "").strip() if isinstance(u.get("email"), str) else None
            commit_email = (a.get("email") or "").strip() if isinstance(a.get("email"), str) else None
            author_email = user_email or commit_email or None

            yield CommitLite(
                sha=c["oid"],
                author_id=author_id,
                author_login=author_login,
                author_name=author_name,
                author_email=author_email,
                message=c.get("message"),
            )

        if not commits["pageInfo"]["hasNextPage"]:
            break
        cursor = commits["pageInfo"]["endCursor"]

def get_author_summary_gql(commit: CommitLite, pr: int, installation_id, with_co_authors) -> Tuple[List[UserCommitSummary], bool]:
    fn = "cla.models.github_models.get_author_summary_gql"
    commit_authors: List[UserCommitSummary] = []

    # Prefer linked user fields when present; fallback to raw author fields.
    id_val    = commit.author_id
    login_val = commit.author_login
    name_val  = commit.author_name
    email_val = commit.author_email

    # Nothing to "try/except": GraphQL gives plain values; just normalize empties.
    def norm(s):
        return s if isinstance(s, str) and s.strip() else None

    name_val  = norm(name_val)
    email_val = norm(email_val)

    cla.log.debug(f"{fn}: (id: {id_val}, login: {login_val}, name: {name_val}, email: {email_val})")

    commit_author_summary = UserCommitSummary(
        commit.sha,
        id_val,
        login_val,
        name_val,
        email_val,
        False,
        False,  # default not authorized - will be evaluated and updated later
    )
    cla.log.debug(f"{fn} - PR: {pr}, {commit_author_summary}")
    commit_authors.append(commit_author_summary)

    missing = False
    if with_co_authors and commit.message:
        # Use the message string instead of the PyGithub commit object
        missing = expand_with_co_authors_from_message(commit.sha, commit.message, pr, installation_id, commit_authors)

    return (commit_authors, missing)

def get_pr_commit_count_gql(g, owner: str, repo: str, number: int) -> int | None:
    # Single, cheap GraphQL count for logging (no pagination)
    query = """
    query($owner:String!, $name:String!, $number:Int!) {
      repository(owner:$owner, name:$name) {
        pullRequest(number:$number) {
          commits { totalCount }
        }
      }
    }"""
    try:
        data = pygithub_graphql(g, query, {"owner": owner, "name": repo, "number": number})
        if data is None:
            cla.log.debug(f"get_pr_commit_count_gql: no data returned")
            return None
        repo_obj = data.get("repository")
        if not repo_obj:
            cla.log.debug("get_pr_commit_count_gql: repository null (no access?)")
            return None
        pr = repo_obj.get("pullRequest")
        if not pr:
            cla.log.debug("get_pr_commit_count_gql: pullRequest null (bad number or no access?)")
            return None
        commits = pr.get("commits") or {}
        return commits.get("totalCount")
    except Exception as e:
        cla.log.debug(f"get_pr_commit_count_gql: failed to fetch count: {e}")
        return None

def iter_chunks(it: Iterable, n: int):
    it = iter(it)
    while True:
        chunk = list(islice(it, n))
        if not chunk:
            return
        yield chunk

def get_pull_request_commit_authors(client, org, pull_request, installation_id, with_co_authors) -> Tuple[List[UserCommitSummary], bool]:
    """
    Helper function to extract all committer information for a GitHub PR.

    For pull_request data model, see:
    https://developer.github.com/v3/pulls/
    For commits on a pull request, see:
    https://developer.github.com/v3/pulls/#list-commits-on-a-pull-request
    For activity callback, see:
    https://developer.github.com/v3/activity/events/types/#pullrequestevent

    :param: pull_request: A GitHub pull request to examine.
    :type: pull_request: GitHub.PullRequest
    :return: A list of User Commit Summary objects containing the commit sha and available user information
    :rtype: List[UserCommitSummary]
    """
    fn = "cla.models.github_models.get_pull_request_commit_authors"
    cla.log.debug(f"{fn} - Querying pull request commits for author information...")

    pr_number = pull_request.number
    repo_name = pull_request.base.repo.name
    gql_ok = True
    pr_commits = None

    count = get_pr_commit_count_gql(client, org, repo_name, pr_number)
    if count is not None:
        cla.log.debug(f"{fn} - PR: {pr_number}, number of commits (GraphQL): {count}")
    else:
        cla.log.debug(f"{fn} - PR: {pr_number}, failed to get commits count using GraphQL, fallback to REST")
        gql_ok = False
        try:
            pr_commits = pull_request.get_commits()
            count = pr_commits.totalCount
        except Exception as exc:
            cla.log.warning(f"{fn} - PR: {pr_number}, get PR commits raised: {exc}")
            raise
        cla.log.debug(f"{fn} - PR: {pr_number}, number of commits (REST API): {count}")
        if count == 250:
            cla.log.warning(f"{fn} - PR: {pr_number}, commit count is 250, which is the max for REST API, there can be more commits")

    commit_authors = []
    any_missing = False
    max_workers = 16
    submit_chunk = 256

    if gql_ok:
        try:
            with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as executor:
                for chunk in iter_chunks(iter_pr_commits_full(client, org, repo_name, pr_number), submit_chunk):
                    futures = [executor.submit(get_author_summary_gql, c, pr_number, installation_id, with_co_authors) for c in chunk]
                    for fut in concurrent.futures.as_completed(futures):
                        authors, missing = fut.result()
                        any_missing = any_missing or missing
                        commit_authors.extend(authors)
        except Exception as exc:
            cla.log.warning(f"{fn} - PR: {pr_number}, GraphQL processing failed: {exc}, falling back to REST")
            gql_ok = False
            commit_authors = []
            any_missing = False
    if not gql_ok:
        if pr_commits is None:
            try:
                pr_commits = pull_request.get_commits()
            except Exception as exc:
                cla.log.warning(f"{fn} - PR: {pr_number}, fallback get PR commits raised: {exc}")
                raise
        try:
            with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as executor:
                futures = [executor.submit(get_author_summary, c, pr_number, installation_id, with_co_authors) for c in pr_commits]
                for fut in concurrent.futures.as_completed(futures):
                    authors, missing = fut.result()
                    any_missing = any_missing or missing
                    commit_authors.extend(authors)
        except Exception as exc:
            cla.log.warning(f"{fn} - PR: {pr_number}, REST processing failed: {exc}")
            raise

    return (commit_authors, any_missing)

def is_valid_github_username(username: str) -> bool:
    return bool(GITHUB_USERNAME_REGEX.match(username))

def get_co_author_commits(co_author, commit_sha, pr, installation_id) -> Tuple[UserCommitSummary, bool]:
    fn = "cla.models.github_models.get_co_author_commits"
    # check if co-author is a github user
    co_author_summary = None
    login, github_id = None, None
    # We don't need to strip() or lower() here, as get_co_authors_from_commit already does that
    email = co_author[1]
    name = co_author[0]
    lname = name.lower()

    # caching starts
    cache_key = (lname, email)
    cached_user, hit = github_user_cache.get(cache_key)

    if hit:
        found = False
        if cached_user is not None:
            cla.log.debug(f"{fn} - GitHub user found in cache for name/email: {name}/{email}: {cached_user}")
            # Build UserCommitSummary using cached_user
            summary = UserCommitSummary(
                commit_sha,
                getattr(cached_user, 'id', None),
                getattr(cached_user, 'login', None),
                name,
                email,
                False,
                False,
            )
            found = getattr(cached_user, 'id', None) is not None
        else:
            cla.log.debug(f"{fn} - GitHub user found in cache for name/email: {name}/{email}: (information that this user is missing)")
            summary = UserCommitSummary(
                commit_sha,
                None,
                None,
                name,
                email,
                False,
                False,
            )

        cla.log.debug(f"{fn} - PR: {pr}, {summary} (from cache)")
        return (summary, found)
    # caching ends

    # get repository service
    github = cla.utils.get_repository_service("github")
    user = None

    cla.log.debug(f"{fn} - getting co-author details: {co_author}, email: {email}, name: {name}")

    # 1. Check for "id+username@users.noreply.github.com"
    m = NOREPLY_ID_PATTERN.match(email)
    if m:
        id_str, login_str = m.groups()
        try:
            github_id = int(id_str)
            cla.log.debug(f"{fn} - Detected noreply GitHub email with ID: {id_str}, login: {login_str}")
            user = github.get_github_user_by_id(github_id, installation_id)
        except Exception as ex:
            cla.log.warning(f"{fn} - Error fetching user by ID {id_str}: {ex}")
            user = None

    # 2. Check for "username@users.noreply.github.com"
    if user is None:
        m = NOREPLY_USER_PATTERN.match(email)
        if m:
            login_str = m.group(1)
            try:
                cla.log.debug(f"{fn} - Detected noreply GitHub email with login: {login_str}")
                user = github.get_github_user_by_login(login_str, installation_id)
            except Exception as ex:
                cla.log.warning(f"{fn} - Error fetching user by login {login_str}: {ex}")
                user = None

    # 3. Try to find user by email via GitHub APIs
    if user is None:
        try:
            cla.log.debug(f"{fn} - Lookup via GitHub email: {email}")
            user = github.get_github_user_by_email(email, installation_id)
        except (GithubException, IncompletableObject, RateLimitExceededException) as ex:
            # user not found
            cla.log.debug(f"{fn} - co-author github user not found via github email {email}: {co_author} with exception: {ex}")
            user = None

    # 3b. Try to find user by email in our database
    if user is None:
        try:
            cla.log.debug(f"{fn} - Lookup via lf email: {email}")
            user_model = cla.utils.get_user_instance()
            db_users = user_model.get_user_by_lf_email(email)
            github_id = None
            if db_users is not None:
                for db_user in db_users:
                    github_id = db_user.get_user_github_id()
                    if github_id is not None:
                        break
            if not github_id:
                db_users = user_model.get_user_by_email(email)
                if db_users is not None:
                    for db_user in db_users:
                        github_id = db_user.get_user_github_id()
                        if github_id is not None:
                            break
            if github_id:
                cla.log.debug(f"{fn} - Found GitHub ID {github_id} for lf email: {email} in EasyCLA DB")
                try:
                    user = github.get_github_user_by_id(github_id, installation_id)
                except Exception as ex:
                    cla.log.warning(f"{fn} - Error fetching user by ID {github_id}: {ex}")
                    user = None
        except Exception as ex:
            # user not found
            cla.log.debug(f"{fn} - co-author github user not found via lf email {email}: {co_author} with exception: {ex}")
            user = None

    # 4. Last resort: try to find by name (login)
    if user is None and is_valid_github_username(name):
        try:
            # Note that Co-authored-by: name <email> is not actually a GitHub login but rather a name - but we are trying hard to find a GitHub profile
            cla.log.debug(f"{fn} - Lookup via login=name: {name}")
            user = github.get_github_user_by_login(lname, installation_id)
        except (GithubException, IncompletableObject, RateLimitExceededException) as ex:
            # user not found
            cla.log.debug(f"{fn} - co-author github user not found via login=name: {name}: {co_author} with exception: {ex}")
            user = None

    cla.log.debug(f"{fn} - co-author: {co_author}, user: {user}")

    found = False
    if user:
        login = user.login
        github_id = user.id
        final_name = name
        final_email = email
        try:
            n = user.name
            if isinstance(n, str) and n.strip():
                final_name = n
        except (AttributeError, GithubException, IncompletableObject):
            pass
        try:
            e = user.email
            if isinstance(e, str) and e.strip():
                final_email = e
        except (AttributeError, GithubException, IncompletableObject):
            pass
        cla.log.debug(f"{fn} - co-author github user details found: {co_author}, user: {user}, login: {login}, id: {github_id}, name: {final_name}, email: {final_email}")
        co_author_summary = UserCommitSummary(
            commit_sha,
            github_id,
            login,
            final_name,
            final_email,
            False,
            False,  # default not authorized - will be evaluated and updated later
        )
        cla.log.debug(f"{fn} - PR: {pr}, {co_author_summary}")
        found = github_id is not None
    else:
        co_author_summary = UserCommitSummary(
            commit_sha, None, None, name, email, False, False  # default not authorized - will be evaluated and updated later
        )
        cla.log.debug(f"{fn} - co-author github user details not found: {co_author}")

    if found:
        github_user_cache.set(cache_key, user)
    else:
        # negative cache for 30 minutes (this is for GitHub user not found)
        github_user_cache.set_with_ttl(cache_key, user, 1800)
    return (co_author_summary, found)


def has_check_previously_passed_or_failed(pull_request: PullRequest):
    """
    Review the status updates in the PR. Identify 1 or more previous failed|passed
    updates from the EasyCLA bot. If we fine one, return True with the comment, otherwise
    return False, None

    :param pull_request: the GitHub pull request object
    :return: True with the comment if the EasyCLA bot check previously failed, otherwise return False, None
    """
    comments = pull_request.get_issue_comments()
    # Look through all the comments
    for comment in comments:
        # Our bot comments include the following text
        # A previously failed check has 'not authorized' somewhere in the body
        if "is not authorized under a signed CLA" in comment.body:
            return True, comment
        if "they must confirm their affiliation" in comment.body:
            return True, comment
        if "is missing the User" in comment.body:
            return True, comment
        if "are authorized under a signed CLA" in comment.body:
            return True, comment
        if "is not linked to the GitHub account" in comment.body:
            return True, comment
    return False, None

def normalize_comment(s: str) -> str:
    s = (s or "").replace("\r\n", "\n").replace("\r", "\n")
    lines = [ln.rstrip(" \t") for ln in s.split("\n")]
    while lines and lines[-1] == "":
        lines.pop()
    return "\n".join(lines)

def update_pull_request(
    installation_id,
    github_repository_id,
    pull_request,
    repository_name,
    signed: List[UserCommitSummary],
    missing: List[UserCommitSummary],
    any_missing: bool,
    project_version,
):  # pylint: disable=too-many-locals
    """
    Helper function to update a PR's comment and status based on the list of signers.

    :param: installation_id: The ID of the GitHub installation
    :type: installation_id: int
    :param: github_repository_id: The ID of the GitHub repository this PR belongs to.
    :type: github_repository_id: int
    :param: pull_request: The GitHub PullRequest object for this PR.
    :type: pull_request: GitHub.PullRequest
    :param: repository_name: The GitHub repository name for this PR.
    :type: repository_name: string
    :param: signed: The list of User Commit Summary objects for this PR.
    :type: signed: List[UserCommitSummary]
    :param: missing: The list of User Commit Summary objects for this PR.
    :type: missing: List[UserCommitSummary]
    :param: any_missing: Boolean flag indicating if any co-authors are missing.
    :type: any_missing: bool
    :param: project_version: Project version associated with PR
    :type: missing: string
    """
    fn = "cla.models.github_models.update_pull_request"
    notification = cla.conf["GITHUB_PR_NOTIFICATION"]
    both = notification == "status+comment" or notification == "comment+status"
    last_commit_sha = getattr(getattr(pull_request, "head", None), "sha", None)
    if not last_commit_sha:
        cla.log.error(f"{fn} - PR {pull_request.number}: missing head.sha; cannot create statuses")
        return
    commit_obj = pull_request.base.repo.get_commit(last_commit_sha)

    # Here we update the PR status by adding/updating the PR body - this is the way the EasyCLA app
    # knows if it is pass/fail.
    # Create check run for users that haven't yet signed and/or affiliated
    if missing:
        text = ""
        help_url = ""

        for user_commit_summary in missing:
            # Check for valid GitHub id
            # old tuple: (sha, (author_id, author_login_or_name, author_email, optionalTrue))
            if not user_commit_summary.is_valid_user():
                help_url = "https://help.github.com/en/github/committing-changes-to-your-project/why-are-my-commits-linked-to-the-wrong-user"
            else:
                help_url = cla.utils.get_full_sign_url(
                    "github", str(installation_id), github_repository_id, pull_request.number, project_version
                )

            # check if unsigned user is allowlisted
            if user_commit_summary.commit_sha != last_commit_sha:
                continue

            text += user_commit_summary.get_display_text(tag_user=True)

        payload = {
            "name": "CLA check",
            "head_sha": last_commit_sha,
            "status": "completed",
            "conclusion": "action_required",
            "details_url": help_url,
            "output": {
                "title": "EasyCLA: Signed CLA not found",
                "summary": "One or more committers are authorized under a signed CLA.",
                "text": text,
            },
        }
        client = GitHubInstallation(installation_id)
        client.create_check_run(repository_name, json.dumps(payload))

    # Update the comment
    if both or notification == "comment":
        body = cla.utils.assemble_cla_comment(
            "github", str(installation_id), github_repository_id, pull_request.number, signed, missing, any_missing, project_version
        )
        previously_pass_or_failed, comment = has_check_previously_passed_or_failed(pull_request)
        if not missing:
            # After Issue #167 was in place, they decided via Issue #289 that we
            # DO want to update the comment, but only after we've previously failed
            if previously_pass_or_failed and normalize_comment(comment.body) != normalize_comment(body):
                cla.log.debug(f"{fn} - Found previously passed or failed checks and comment body changed - updating CLA comment in PR.")
                cla.log.debug(f"{fn} - Old comment: {comment.body}")
                cla.log.debug(f"{fn} - New comment: {body}")
                comment.edit(body)
            cla.log.debug(f"{fn} - EasyCLA App checks pass for PR: {pull_request.number} with authors: {signed}")
        else:
            # Per Issue #167, only add a comment if check fails
            # update_cla_comment(pull_request, body)
            if previously_pass_or_failed:
                if normalize_comment(comment.body) != normalize_comment(body):
                    cla.log.debug(f"{fn} - Found previously failed checks and comment body changed - updating CLA comment in PR.")
                    cla.log.debug(f"{fn} - Old comment: {comment.body}")
                    cla.log.debug(f"{fn} - New comment: {body}")
                    comment.edit(body)
            else:
                pull_request.create_issue_comment(body)

            cla.log.debug(
                f"{fn} - EasyCLA App checks fail for PR: {pull_request.number}. "
                f"CLA signatures with signed authors: {signed} and "
                f"with missing authors: {missing}"
            )

    if both or notification == "status":
        context_name = os.environ.get("GH_STATUS_CTX_NAME")
        if context_name is None:
            context_name = "communitybridge/cla"

        # if we have ANY committers who have failed the check - update the status with overall failure
        if missing is not None and len(missing) > 0:
            state = "failure"
            # For status, we change the context from author_name to 'communitybridge/cla' or the
            # specified default value per issue #166
            context, body = cla.utils.assemble_cla_status(context_name, signed=False)
            sign_url = cla.utils.get_full_sign_url(
                "github", str(installation_id), github_repository_id, pull_request.number, project_version
            )
            cla.log.debug(
                f"{fn} - Creating new CLA '{state}' status - {len(signed)} passed, {missing} failed, "
                f"signing url: {sign_url}"
            )
            create_commit_status(commit_obj, state, sign_url, body, context)
        elif signed is not None and len(signed) > 0:
            state = "success"
            # For status, we change the context from author_name to 'communitybridge/cla' or the
            # specified default value per issue #166
            context, body = cla.utils.assemble_cla_status(context_name, signed=True)
            sign_url = cla.conf["CLA_LANDING_PAGE"]  # Remove this once signature detail page ready.
            sign_url = os.path.join(sign_url, "#/")
            sign_url = append_project_version_to_url(address=sign_url, project_version=project_version)
            cla.log.debug(
                f"{fn} - Creating new CLA '{state}' status - {len(signed)} passed, {missing} failed, "
                f"signing url: {sign_url}"
            )
            create_commit_status(commit_obj, state, sign_url, body, context)
        else:
            # error condition - should have at least one committer, and they would be in one of the above
            # lists: missing or signed
            state = "failure"
            # For status, we change the context from author_name to 'communitybridge/cla' or the
            # specified default value per issue #166
            context, body = cla.utils.assemble_cla_status(context_name, signed=False)
            sign_url = cla.utils.get_full_sign_url(
                "github", str(installation_id), github_repository_id, pull_request.number, project_version
            )
            cla.log.debug(
                f"{fn} - Creating new CLA '{state}' status - {len(signed)} passed, {missing} failed, "
                f"signing url: {sign_url}"
            )
            cla.log.warning(
                f"{fn} - This is an error condition - "
                f"should have at least one committer in one of these lists: "
                f"{len(signed)} passed, {missing}"
            )
            create_commit_status(commit_obj, state, sign_url, body, context)


def create_commit_status_for_merge_group(commit_obj, merge_commit_sha, state, sign_url, body, context):
    """
    Helper function to create a pull request commit status message.

    :param commit_obj: The commit object to post a status on.
    :type commit_obj: Commit
    :param merge_commit_sha: The commit hash to post a status on.
    :type merge_commit_sha: string
    :param state: The state of the status.
    :type state: string
    :param sign_url: The link the user will be taken to when clicking on the status message.
    :type sign_url: string
    :param body: The contents of the status message.
    :type body: string
    """
    try:
        # Create status
        cla.log.debug(f"Creating commit status for merge commit {merge_commit_sha}")
        commit_obj.create_status(state=state, target_url=sign_url, description=body, context=context)

    except Exception as e:
        cla.log.warning(f"Unable to create commit status for  " f"and merge commit {merge_commit_sha}: {e}")


def create_commit_status(commit_obj, state, sign_url, body, context):
    """
    Helper function to create a commit status message given the commit object.

    :param commit_obj: The commit to post a status on.
    :type commit_obj: Commit
    :param state: The state of the status.
    :type state: string
    :param sign_url: The link the user will be taken to when clicking on the status message.
    :type sign_url: string
    :param body: The contents of the status message.
    :type body: string
    """
    try:
        sha = getattr(commit_obj, "sha", "(unknown)")
        resp = commit_obj.create_status(state, sign_url, body, context)
        cla.log.info(
            f"Successfully posted status '{state}': Commit {sha} "
            f"with SignUrl : {sign_url} with response: {resp}"
        )
    except GithubException as exc:
        sha = getattr(commit_obj, "sha", "(unknown)")
        cla.log.error(
            f"Could not post status '{state}' on "
            f"Commit: {sha}, "
            f"Response Code: {exc.status}, "
            f"Message: {exc.data}"
        )

# def update_cla_comment(pull_request, body):
#     """
#     Helper function to create/edit a comment on the GitHub PR.
#
#     :param pull_request: The PR object in question.
#     :type pull_request: GitHub.PullRequest
#     :param body: The contents of the comment.
#     :type body: string
#     """
#     comment = get_existing_cla_comment(pull_request)
#     if comment is not None:
#         cla.log.debug(f'Updating existing CLA comment for PR: {pull_request.number} with body: {body}')
#         comment.edit(body)
#     else:
#         cla.log.debug(f'Creating a new CLA comment for PR: {pull_request.number} with body: {body}')
#         pull_request.create_issue_comment(body)


# def get_existing_cla_comment(pull_request):
#     """
#     Helper function to get an existing comment from the CLA system in a GitHub PR.
#
#     :param pull_request: The PR object in question.
#     :type pull_request: GitHub.PullRequest
#     """
#     comments = pull_request.get_issue_comments()
#     for comment in comments:
#         if '[![CLA Check](' in comment.body:
#             cla.log.debug('Found matching CLA comment for PR: %s', pull_request.number)
#             return comment


def get_github_integration_client(installation_id):
    """
    GitHub App integration client used for authenticated client actions through an installed app.
    """
    return GitHubInstallation(installation_id).api_object


def get_github_client(organization_id):
    github_org = cla.utils.get_github_organization_instance()
    github_org.load(organization_id)
    installation_id = github_org.get_organization_installation_id()
    return get_github_integration_client(installation_id)


class MockGitHub(GitHub):
    """
    The GitHub repository service mock class for testing.
    """

    def __init__(self, oauth2_token=False):
        super().__init__()
        self.oauth2_token = oauth2_token

    def _get_github_client(self, username, token):
        return MockGitHubClient(username, token)

    def _get_authorization_url_and_state(self, client_id, redirect_uri, scope, authorize_url, state=None):
        authorization_url = "http://authorization.url"
        state = "random-state-here"
        return authorization_url, state

    def _fetch_token(self, client_id, state, token_url, client_secret, code):  # pylint: disable=too-many-arguments
        return "random-token"

    def _get_request_session(self, request) -> dict:
        if self.oauth2_token:
            return {
                "github_oauth2_token": "random-token", # LG: comment this out to see how Mock class would attempt to fetch GitHub token using state & code
                "github_oauth2_state": "random-state",
                "github_origin_url": "http://github/origin/url",
                "github_installation_id": 1,
            }
        return {}

    def get_user_data(self, session, client_id) -> dict:
        # LG:
        return { "id": 20250522666, "login": "mock-user-py-20250522", "name": "Mock User Py 2025-05-22", "email": "u20250522@mock.user.py.pl" }
        # return {"email": "test@user.com", "name": "Test User", "login": "testuser", "id": 123}

    def get_user_emails(self, session, client_id):
        # LG: updated MockGitHub to return emails in the same way as GitHub class
        return ["u20250522@mock.user.py.pl"]
        # return [{"email": "test@user.com", "verified": True, "primary": True, "visibility": "public"}]

    def get_pull_request(self, github_repository_id, pull_request_number, installation_id):
        return MockGitHubPullRequest(pull_request_number)


class MockGitHubClient(object):  # pylint: disable=too-few-public-methods
    """
    The GitHub Client object mock class for testing.
    """

    def __init__(self, username, token):
        self.username = username
        self.token = token

    def get_repo(self, repository_id):  # pylint: disable=no-self-use
        """
        Mock version of the GitHub Client object's get_repo method.
        """
        return MockGitHubRepository(repository_id)


class MockGitHubRepository(object):  # pylint: disable=too-few-public-methods
    """
    The GitHub Repository object mock class for testing.
    """

    def __init__(self, repository_id):
        self.id = repository_id

    def get_pull(self, pull_request_id):  # pylint: disable=no-self-use
        """
        Mock version of the GitHub Repository object's get_pull method.
        """
        return MockGitHubPullRequest(pull_request_id)


class MockGitHubPullRequest(object):  # pylint: disable=too-few-public-methods
    """
    The GitHub PullRequest object mock class for testing.
    """

    def __init__(self, pull_request_id):
        self.number = pull_request_id
        self.html_url = "http://test-github.com/user/repo/" + str(self.number)

    def get_commits(self):  # pylint: disable=no-self-use
        """
        Mock version of the GitHub PullRequest object's get_commits method.
        """
        lst = MockPaginatedList()
        lst._elements = [MockGitHubCommit()]  # pylint: disable=protected-access
        return lst

    def get_issue_comments(self):  # pylint: disable=no-self-use
        """
        Mock version of the GitHub PullRequest object's get_issue_comments method.
        """
        return [MockGitHubComment()]

    def create_issue_comment(self, body):  # pylint: disable=no-self-use
        """
        Mock version of the GitHub PullRequest object's create_issue_comment method.
        """
        pass


class MockGitHubComment(object):  # pylint: disable=too-few-public-methods
    """
    A GitHub mock issue comment object for testing.
    """

    body = "Test"


class MockPaginatedList(github.PaginatedList.PaginatedListBase):  # pylint: disable=too-few-public-methods
    """Mock GitHub paginated list for testing purposes."""

    def __init__(self):
        super().__init__()
        # Need to use our own elements list (self.__elements from PaginatedListBase does not
        # work as expected).
        self._elements = []

    @property
    def reversed(self):
        """Fake reversed property."""
        return [MockGitHubCommit()]

    def __iter__(self):
        for element in self._elements:
            yield element


class MockGitHubCommit(object):  # pylint: disable=too-few-public-methods
    """
    The GitHub Commit object mock class for testing.
    """

    def __init__(self):
        self.author = MockGitHubAuthor()
        self.sha = "sha-test-commit"

    def create_status(self, state, sign_url, body):
        """
        Mock version of the GitHub Commit object's create_status method.
        """
        pass


class MockGitHubAuthor(object):  # pylint: disable=too-few-public-methods
    """
    The GitHub Author object mock class for testing.
    """

    def __init__(self, author_id=1):
        self.id = author_id
        self.login = "user"
        self.email = "user@github.com"
