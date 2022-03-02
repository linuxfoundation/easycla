# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
user.py contains the user class and hug directive.
"""

import re
from dataclasses import dataclass
from typing import Optional

from hug.directives import _built_in_directive
from jose import jwt

import cla


@_built_in_directive
def cla_user(default=None, request=None, **kwargs):
    """Returns the current logged in CLA user"""

    headers = request.headers
    if headers is None:
        cla.log.error('Error reading headers')
        return default

    bearer_token = headers.get('Authorization') or headers.get('AUTHORIZATION')

    if bearer_token is None:
        cla.log.error('Error reading authorization header')
        return default

    bearer_token = bearer_token.replace('Bearer ', '')
    try:
        token_params = jwt.get_unverified_claims(bearer_token)
    except Exception as e:
        cla.log.error('Error parsing Bearer token: {}'.format(e))
        return default

    if token_params is not None:
        return CLAUser(token_params)
    cla.log.error('Failed to get user information from request')
    return default


class CLAUser(object):
    def __init__(self, data):
        self.data = data
        self.user_id = data.get('sub', None)
        self.name = data.get('name', None)
        self.session_state = data.get('session_state', None)
        self.resource_access = data.get('resource_access', None)
        self.preferred_username = data.get('preferred_username', None)
        self.given_name = data.get('given_name', None)
        self.family_name = data.get('family_name', None)
        self.email = data.get('email', None)
        self.roles = data.get('realm_access', {}).get('roles', [])


@dataclass
class UserCommitSummary:
    commit_sha: str
    author_id: Optional[int]  # numeric ID of the user
    author_login: Optional[str]  # login identifier of the user
    author_name: Optional[str]  # english name of the user, typically First name Last name format.
    author_email: Optional[str]  # public email address of the user
    authorized: bool
    affiliated: bool

    def __str__(self) -> str:
        return (f'User Commit Summary, '
                f'commit SHA: {self.commit_sha}, '
                f'author id: {self.author_id}, '
                f'login: {self.author_login}, '
                f'name: {self.author_name}, '
                f'email: {self.author_email}.')

    def is_valid_user(self) -> bool:
        return self.author_id is not None and (self.author_login is not None or self.author_name is not None)

    def get_user_info(self) -> str:
        user_info = ''
        if self.author_login:
            user_info += f'login: {self.author_login} / '
        if self.author_name:
            user_info += f'name: {self.author_name} / '
        if self.author_email:
            user_info += f'email: {self.author_email}'

        pattern = r'/ $'
        return re.sub(pattern, '', user_info)

    def get_display_text(self) -> str:

        if not self.author_id:
            return f'{self.author_email} is not linked to this commit.\n'

        text = self.get_user_info()

        if not self.is_valid_user():
            return 'Invalid author details.\n'

        if self.authorized and self.affiliated:
            text += ' is authorized.\n'
            return text

        if self.affiliated:
            text += ' is associated with a company, but not on an approval list.\n'
        else:
            text += ' is not associated with a company.\n'

        return text
