# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Application configuration options.

These values should be tracked in version control.

Please put custom non-tracked configuration options (debug mode, keys, database
configuration, etc) in cla_config.py somewhere in your Python path.
"""

import logging
import os
import sys
from multiprocessing.pool import ThreadPool

from boto3 import client
from botocore.exceptions import ClientError, ProfileNotFound, NoCredentialsError

region = "us-east-1"
ssm_client = client('ssm', region_name=region)


def get_ssm_key(region, key):
    """
    Fetches the specified SSM key value from the SSM key store
    """
    fn = "config.get_ssm_key"
    try:
        logging.debug(f'{fn} - Loading config with key: {key}')
        response = ssm_client.get_parameter(Name=key, WithDecryption=True)
        if response and 'Parameter' in response and 'Value' in response['Parameter']:
            logging.debug(f'{fn} - Loaded config value with key: {key}')
        return response['Parameter']['Value']
    except (ClientError, ProfileNotFound) as e:
        logging.warning(f'{fn} - Unable to load SSM config with key: {key} due to {e}')
        return None


# from utils import get_ssm_key

stage = os.environ.get('STAGE', '')
STAGE = stage

LOG_LEVEL = logging.DEBUG  #: Logging level.
#: Logging format.
LOG_FORMAT = logging.Formatter(fmt='%(asctime)s %(levelname)s %(name)s: %(message)s', datefmt='%Y-%m-%dT%H:%M:%S')

DEBUG = False  #: Debug off in production

# The linux foundation is the parent for many SF projects
THE_LINUX_FOUNDATION = 'The Linux Foundation'

# LF Projects LLC is the parent for many SF projects
LF_PROJECTS_LLC = 'LF Projects, LLC'

# Base URL used for callbacks and OAuth2 redirects.
API_BASE_URL = os.environ.get('CLA_API_BASE', '')

# Contributor Console base URL
CONTRIBUTOR_BASE_URL = os.environ.get('CLA_CONTRIBUTOR_BASE', '')
CONTRIBUTOR_V2_BASE_URL = os.environ.get('CLA_CONTRIBUTOR_V2_BASE', '')

# Corporate Console base URL
CORPORATE_BASE_URL = os.environ.get('CLA_CORPORATE_BASE', '')
CORPORATE_V2_BASE_URL = os.environ.get('CLA_CORPORATE_V2_BASE', '')

# Landing Page
CLA_LANDING_PAGE = os.environ.get('CLA_LANDING_PAGE', '')

SIGNED_CALLBACK_URL = os.path.join(API_BASE_URL, 'v2/signed')  #: Default callback once signature is completed.
ALLOW_ORIGIN = '*'  # Specify the CORS Access-Control-Allow-Origin response header value.

# Define the database we are working with.
DATABASE = 'DynamoDB'  #: Database type ('SQLite', 'DynamoDB', etc).

# Define the key-value we are working with.
KEYVALUE = 'DynamoDB'  #: Key-value store type ('Memory', 'DynamoDB', etc).

# DynamoDB-specific configurations - this is applied to each table.
DYNAMO_REGION = 'us-east-1'  #: DynamoDB AWS region.
DYNAMO_WRITE_UNITS = 1  #: DynamoDB table write units.
DYNAMO_READ_UNITS = 1  #: DynamoDB table read units.

# Define the signing service to use.
SIGNING_SERVICE = 'DocuSign'  #: The signing service to use ('DocuSign', 'HelloSign', etc)

# Repository settings.
AUTO_CREATE_REPOSITORY = True  #: Create repository in database automatically on webhook.

# GitHub Repository Service.
#: GitHub OAuth2 Authorize URL.
GITHUB_OAUTH_AUTHORIZE_URL = 'https://github.com/login/oauth/authorize'
#: GitHub OAuth2 Callback URL.
GITHUB_OAUTH_CALLBACK_URL = os.path.join(API_BASE_URL, 'v2/github/installation')
#: GitHub OAuth2 Token URL.
GITHUB_OAUTH_TOKEN_URL = 'https://github.com/login/oauth/access_token'
#: How users get notified of CLA status in GitHub ('status', 'comment', or 'status+comment').
GITHUB_PR_NOTIFICATION = 'status+comment'

# GitHub Application Service.
GITHUB_APP_WEBHOOK_SECRET = os.getenv("GITHUB_APP_WEBHOOK_SECRET", "")

# GitHub Oauth token used for authenticated GitHub API calls and testing
GITHUB_OAUTH_TOKEN = os.environ.get('GITHUB_OAUTH_TOKEN', '')

# Email Service.
EMAIL_SERVICE = 'SNS'  #: Email service to use for notification emails.
EMAIL_ON_SIGNATURE_APPROVED = True  #: Whether to email the user when signature has been approved.

# Platform Maintainers
PLATFORM_MAINTAINERS = os.environ.get('PLATFORM_MAINTAINERS', [])

# Platform Gateway URL
PLATFORM_GATEWAY_URL = os.environ.get("PLATFORM_GATEWAY_URL")

# SMTP Configuration.
#: Sender email address for SMTP service (from address).
SMTP_SENDER_EMAIL_ADDRESS = os.environ.get('SMTP_SENDER_EMAIL_ADDRESS', 'test@cla.system')
SMTP_HOST = ''  #: Host of the SMTP service.
SMTP_PORT = '0'  #: Port of the SMTP service.

# Storage Service.
STORAGE_SERVICE = 'S3Storage'  #: The storage service to use for storing CLAs.

# LocalStorage Configuration.
LOCAL_STORAGE_FOLDER = '/tmp/cla'  #: Local folder when using the LocalStorage service.

# PDF Generation.
PDF_SERVICE = 'DocRaptor'


AUTH0_PLATFORM_URL = os.getenv("AUTH0_PLATFORM_URL", "")
AUTH0_PLATFORM_CLIENT_ID = os.getenv("AUTH0_PLATFORM_CLIENT_ID", "")
AUTH0_PLATFORM_CLIENT_SECRET = os.getenv("AUTH0_PLATFORM_CLIENT_SECRET", "")
AUTH0_PLATFORM_AUDIENCE = os.getenv("AUTH0_PLATFORM_CLIENT_AUDIENCE", "")

# GH Private Key
# Moved to GitHub application class GitHubInstallation as loading this property is taking ~1 sec on startup which is
# killing our response performance - in most API calls this key/attribute is not used, so, we will lazy load this
# property on class construction
GITHUB_PRIVATE_KEY = ""

# reference to this module, cla.config
this = sys.modules[__name__]


def load_ssm_keys():
    """
    loads all the config variables that are stored is ssm
    it uses Thread Pool so can fetch things in parallel as much as Python allows
    :return:
    """

    def _load_single_key(key):
        try:
            return get_ssm_key('us-east-1', key)
        # helps with unit testing so they don't fail because of ssm creds failure
        except NoCredentialsError as ex:
            # we don't want things to fail during unit testing
            logging.warning(f"loading credentials for key : {key} failed : {str(ex)}")
            return None

    # the order is important
    keys = [
        f'cla-gh-app-private-key-{stage}',
        f'cla-auth0-platform-api-gw-{stage}',
        f'cla-auth0-platform-url-{stage}',
        f'cla-auth0-platform-client-id-{stage}',
        f'cla-auth0-platform-client-secret-{stage}',
        f'cla-auth0-platform-audience-{stage}'
    ]
    config_keys = [
        "GITHUB_PRIVATE_KEY",
        "PLATFORM_GATEWAY_URL",
        "AUTH0_PLATFORM_URL",
        "AUTH0_PLATFORM_CLIENT_ID",
        "AUTH0_PLATFORM_CLIENT_SECRET",
        "AUTH0_PLATFORM_AUDIENCE"
    ]

    # thread pool of 5 to load fetch the keys
    pool = ThreadPool(5)
    results = pool.map(_load_single_key, keys)
    pool.close()
    pool.join()

    # set the variable values at the module level so can be imported as cla.config.{VAR_NAME}
    for config_key, result in zip(config_keys, results):
        if result:
            setattr(this, config_key, result)
        else:
            logging.warning(f"skipping {config_key} setting the ssm was empty")


# when imported this will be called to load ssm keys
load_ssm_keys()
