# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
docusign_auth.py contains all necessary objects and functions to perform authentication and authorization.
"""


import requests
import os
import jwt
from time import time
import cla
import math


INTEGRATION_KEY = cla.config.DOCUSIGN_INTEGRATOR_KEY
INTEGRATION_SECRET = cla.config.DOCUSIGN_PRIVATE_KEY
USER_ID = cla.config.DOCUSIGN_USER_ID
OAUTH_BASE_URL = os.environ.get('DOCUSIGN_AUTH_SERVER')


def request_access_token() -> str:
    """
    Requests an access token from the DocuSign OAuth2 service.
    """
    try:
        cla.log.debug('Requesting access token from DocuSign OAuth2 service...')
        url = f'https://{OAUTH_BASE_URL}/oauth/token'
        headers = {
            'Content-Type': 'application/x-www-form-urlencoded',
        }
        claims = {
            "iss": INTEGRATION_KEY,
            "sub": USER_ID,
            "aud": OAUTH_BASE_URL,
            "iat": time(),
            "exp": time() + 3600,
            "scope": "signature impersonation"
        }
        cla.log.debug(f'Claims: {claims}')
        # Note from the docs: If you are planning on encoding or decoding tokens using certain digital signature
        # algorithms # (like RSA or ECDSA), you will need to install the cryptography library. This can be installed
        # explicitly, or as a required extra in the pyjwt requirement: $ pip install pyjwt[crypto]
        encoded_jwt = jwt.encode(claims, INTEGRATION_SECRET.encode(), algorithm='RS256')

        payload = {
            'grant_type': 'urn:ietf:params:oauth:grant-type:jwt-bearer',
            'assertion': encoded_jwt
        }

        response = requests.post(url, headers=headers, data=payload)
        data = response.json()
        if 'token_type' in data and 'access_token' in data:
            cla.log.debug('Successfully requested access token from DocuSign OAuth2 service.')
            return data['access_token']
        else:
            cla.log.error('Unable to request access token from DocuSign OAuth2 service: ' + str(data))
            raise Exception('Unable to request access token from DocuSign OAuth2 service: ' + str(data))

    except Exception as err:
        cla.log.error('Unable to request access token from DocuSign OAuth2 service: ' + str(err))
        raise err



