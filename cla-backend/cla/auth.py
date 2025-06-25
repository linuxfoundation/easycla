# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
auth.py contains all necessary objects and functions to perform authentication and authorization.
"""
import os

import requests
from jose import jwt

import cla

# LG: for local environment override
# os.environ["AUTH0_USERNAME_CLAIM"] = os.getenv("AUTH0_USERNAME_CLAIM_CLI", os.environ["AUTH0_USERNAME_CLAIM"])

auth0_base_url = os.environ.get('AUTH0_DOMAIN', '')
auth0_username_claim = os.environ.get('AUTH0_USERNAME_CLAIM', '')
algorithms = [os.environ.get('AUTH0_ALGORITHM', '')]

# This list represents admin users who can perform logo
# uploads and project and cla manager permission updates
admin_list = ['vnaidu', 'ddeal', 'bryan.stone']


class AuthError(Exception):
    """
    Authentication error class
    """
    def __init__(self, response):
        super().__init__()
        self.response = response


class AuthUser:
    """
    This user object is built from Auth0 JWT claims.
    """

    def __init__(self, auth_claims):
        super().__init__()
        self.name = auth_claims.get('name')
        self.email = auth_claims.get('email')
        self.username = auth_claims.get(auth0_username_claim)
        self.sub = auth_claims.get('sub')


def get_auth_token(headers):
    """
    Obtains the Access Token from the Authorization Header
    """
    auth = headers.get('Authorization')
    if not auth:
        auth = headers.get('AUTHORIZATION')
    if not auth:
        raise AuthError('missing authorization header')

    parts = auth.split()

    if parts[0].lower() != 'bearer':
        raise AuthError({'authorization header must begin with \"Bearer\"'})
    elif len(parts) == 1:
        raise AuthError('token not found')
    elif len(parts) > 2:
        raise AuthError('authorization header must be of the form \"Bearer token\"')

    return parts[1]

# LG: for local testing
def fake_authenticate_user(headers):
    return AuthUser({'name': 'Lukasz Gryglicki', 'email': 'lukaszgryglicki@o2.pl', 'username': 'lukaszgryglicki', 'sub': ''})

def authenticate_user(headers):
    """
    Determines if the Access Token is valid
    """
    token = get_auth_token(headers)
    try:
        jwks_url = os.path.join('https://', auth0_base_url, '.well-known/jwks.json')
        jwks = requests.get(jwks_url).json()
    except Exception as e:
        cla.log.error(e)
        raise AuthError('unable to fetch well known jwks')

    try:
        unverified_header = jwt.get_unverified_header(token)
    except jwt.JWTError as e:
        cla.log.error(e)
        raise AuthError('unable to decode claims')

    rsa_key = {}
    for key in jwks["keys"]:
        if key["kid"] == unverified_header["kid"]:
            rsa_key = {
                "kty": key["kty"],
                "kid": key["kid"],
                "use": key["use"],
                "n": key["n"],
                "e": key["e"]
            }
    # print("Token kid:", unverified_header["kid"])
    # print("JWKS kids:", [key["kid"] for key in jwks["keys"]])
    if rsa_key:
        try:
            payload = jwt.decode(
                token,
                rsa_key,
                algorithms=algorithms,
                options={
                    'verify_at_hash': False,
                    'verify_aud': False
                }
            )
        except jwt.ExpiredSignatureError as e:
            cla.log.error(e)
            raise AuthError('token is expired')
        except jwt.JWTClaimsError as e:
            cla.log.error(e)
            raise AuthError('incorrect claims')
        except Exception as e:
            cla.log.error(e)
            raise AuthError('unable to parse authentication')

        username = payload.get(auth0_username_claim)
        if username is None:
            # LG: to have more info
            # raise AuthError(f"username not found in {auth0_username_claim}")
            raise AuthError('username claim not found')

        auth_user = AuthUser(payload)

        return auth_user

    raise AuthError({"code": "invalid_header", "description": "Unable to find appropriate key"})
