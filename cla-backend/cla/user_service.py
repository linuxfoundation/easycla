# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
import datetime
import json
import os
from typing import List
from urllib.parse import quote


import requests

import cla
from cla import log
from cla.models.dynamo_models import ProjectCLAGroup

STAGE = os.environ.get('STAGE', '')
REGION = 'us-east-1'


class UserServiceInstance:
    """
    UserService Handles external salesforce Users
    """

    access_token = None
    access_token_expires = datetime.datetime.now() + datetime.timedelta(minutes=30)

    def __init__(self):
        self.platform_gateway_url = cla.config.PLATFORM_GATEWAY_URL

    def get_user_by_sf_id(self, sf_user_id: str):
        """
        Queries the platform user service for the specified user id. The
        result will return all the details for the user as a dictionary.
        """
        fn = 'user_service.get_user_by_sf_id'

        headers = {
            'Authorization': f'bearer {self.get_access_token()}',
            'accept': 'application/json'
        }

        try:
            url = f'{self.platform_gateway_url}/user-service/v1/users/{sf_user_id}'
            log.debug(f'{fn} - sending GET request to {url}')
            r = requests.get(url, headers=headers)
            r.raise_for_status()
            response_model = json.loads(r.text)
            return response_model
        except requests.exceptions.HTTPError as err:
            msg = f'{fn} - Could not get user: {sf_user_id}, error: {err}'
            log.warning(msg)
            return None

    def _get_users_by_key_value(self, key: str, value: str) -> List[dict]:
        """
        Queries the platform user service for the specified criteria.
        The result will return summary information for the users as a
        dictionary.
        """
        fn = 'user_service._get_users_by_key_value'

        headers = {
            'Authorization': f'bearer {self.get_access_token()}',
            'accept': 'application/json'
        }

        users: List[dict] = []
        offset = 0
        pagesize = 1000

        while True:
            try:
                log.info(f'{fn} - Search User using key: {key} with value: {value}')
                url = f'{self.platform_gateway_url}/user-service/v1/users/search?' \
                      f'{key}={quote(value)}&pageSize={pagesize}&offset={offset}'
                log.debug(f'{fn} - sending GET request to {url}')
                r = requests.get(url, headers=headers)
                r.raise_for_status()
                response_model = json.loads(r.text)
                total = response_model['Metadata']['TotalSize']
                if response_model['Data']:
                    users = users + response_model['Data']
                if total < (pagesize + offset):
                    break
                offset = offset + pagesize
            except requests.exceptions.HTTPError as err:
                log.warning(f'{fn} - Could not get projects, error: {err}')
                return None

        log.debug(f'{fn} - total users : {len(users)}')
        return users

    def get_users_by_username(self, user_name: str) -> List[dict]:
        return self._get_users_by_key_value("username", user_name)

    def get_users_by_firstname(self, first_name: str) -> List[dict]:
        return self._get_users_by_key_value("firstname", first_name)

    def get_users_by_lastname(self, last_name: str) -> List[dict]:
        return self._get_users_by_key_value("lastname", last_name)

    def get_users_by_email(self, email: str) -> List[dict]:
        return self._get_users_by_key_value("email", email)
    
    def has_role(self, username: str, role: str, organization_id: str, cla_group_id: str) -> bool:
        """
        Function that checks whether lf user has a role
        :param username: The lf username
        :type username: string
        :param cla_group_id: cla_group_id associated with Project/Foundation SFIDs for role check
        :type cla_group_id: string
        :param role: given role check for user
        :type role: string
        :param organization_id: salesforce org ID
        :type organization_id: string
        :rtype: bool
        """
        scopes = {}
        function = 'has_role'
        scopes = self._list_org_user_scopes(organization_id, role)
        if scopes:
            log.info(f'{function} - Found scopes : {scopes} for organization: {organization_id}')
            log.info(f'{function} - Getting projectCLAGroups for cla_group_id: {cla_group_id}')
            pcg = ProjectCLAGroup()
            pcgs = pcg.get_by_cla_group_id(cla_group_id)
            log.info(f'{function} - Found ProjectCLAGroup Mappings: {pcgs}')
            if pcgs:
                if pcgs[0].signed_at_foundation:
                    log.info(f'{cla_group_id} signed at foundation level ')
                    log.info(f'{function} - Checking if {username} has role... ')
                    return self._has_project_org_scope(pcgs[0].get_project_sfid(), organization_id, username, scopes)
                log.info(f'{cla_group_id} signed at project level and checking user roles for user: {username}')
                has_role_project_org = {}
                for pcg in pcgs:
                    has_scope = self._has_project_org_scope(pcg.get_project_sfid(), organization_id, username, scopes)
                    has_role_project_org[username] = (pcg.get_project_sfid(), organization_id, has_scope)
                log.info(f'{function} - user_scopes_status : {has_role_project_org}')
                # check if all projects have user scope
                user_scopes = [has_scope[2] for has_scope in list(has_role_project_org.values())]
                if all(user_scopes):
                    log.info(f'{function} - {username} has role scope at project level')
                    return True

        log.info(f'{function} - {username} does not have role scope')
        return False
    
    def _has_project_org_scope(self, project_sfid: str, organization_id: str, username: str, scopes: dict) -> bool:
        """
        Helper function that checks whether there exists project_org_scope for given role
        :param project_sfid: salesforce project sfid
        :type project_sfid: string
        :param organization_id: organization ID
        :type organization_id: string
        :param username: lf username
        :type username: string
        :param scopes: service scopes for organization
        :type scopes: dict
        :rtype: bool
        """
        function = '_has_project_org_scope_role'
        try:
            user_roles = scopes['userroles']
            log.info(f'{function} - User roles: {user_roles}')
        except KeyError as err:
            log.warning(f'{function} - error: {err} ')
            return False
        for user_role in user_roles:
            if user_role['Contact']['Username'] == username:
                #Since already filtered by role ...get first item
                for scope in user_role['RoleScopes'][0]['Scopes']:
                    log.info(f'{function}- Checking objectID for scope: {project_sfid}|{organization_id}')
                    if scope['ObjectID'] == f'{project_sfid}|{organization_id}':
                        return True
        return False


    def _list_org_user_scopes(self, organization_id: str, role: str) -> dict:
        """
        Helper function that lists the org_user_scopes for a given organization related to given role
        :param organization_id : The salesforce id that is queried for user scopes
        :type organization_id: string
        :param role: role to filter the user org scopes
        :type role: string
        :param cla_group_id: cla_group_id thats mapped to salesforce projects
        :type cla_group_id: string
        :return: json dict representing org user role scopes
        :rtype: dict
        """
        function = '_list_org_user_scopes'
        headers = {
            'Authorization': f'bearer {self.get_access_token()}',
            'accept': 'application/json'
        }
        try:
            url = f'{self.platform_gateway_url}/organization-service/v1/orgs/{organization_id}/servicescopes'
            log.debug('%s - Sending GET url to %s ...', function, url)
            params = {'rolename': role}
            r = requests.get(url, headers=headers, params=params)
            return r.json()
        except requests.exceptions.HTTPError as err:
            log.warning('%s - Could not get user org scopes for organization: %s with role: %s , error: %s ', function, organization_id, role, err)
            return None

    def get_access_token(self):
        fn = 'user_service.get_access_token'
        # Use previously cached value, if not expired
        if self.access_token and datetime.datetime.now() < self.access_token_expires:
            cla.log.debug(f'{fn} - using cached access token: {self.access_token[0:10]}...')
            return self.access_token

        auth0_url = cla.config.AUTH0_PLATFORM_URL
        platform_client_id = cla.config.AUTH0_PLATFORM_CLIENT_ID
        platform_client_secret = cla.config.AUTH0_PLATFORM_CLIENT_SECRET
        platform_audience = cla.config.AUTH0_PLATFORM_AUDIENCE

        auth0_payload = {
            'grant_type': 'client_credentials',
            'client_id': platform_client_id,
            'client_secret': platform_client_secret,
            'audience': platform_audience
        }

        headers = {
            'content-type': 'application/x-www-form-urlencoded',
            'accept': 'application/json'
        }

        try:
            # logger.debug(f'Sending POST to {auth0_url} with payload: {auth0_payload}')
            log.debug(f'{fn} - sending POST to {auth0_url}')
            r = requests.post(auth0_url, data=auth0_payload, headers=headers)
            r.raise_for_status()
            json_data = json.loads(r.text)
            self.access_token = json_data["access_token"]
            self.access_token_expires = datetime.datetime.now() + datetime.timedelta(minutes=30)
            log.debug(f'{fn} - successfully obtained access_token: {self.access_token[0:10]}...')
            return self.access_token
        except requests.exceptions.HTTPError as err:
            log.warning(f'{fn} - could not get auth token, error: {err}')
            return None


UserService = UserServiceInstance()
