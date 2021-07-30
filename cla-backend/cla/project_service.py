# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
import datetime
import json
import os
from typing import Optional

import requests

import cla
from cla import log
from cla.config import THE_LINUX_FOUNDATION, LF_PROJECTS_LLC

STAGE = os.environ.get('STAGE', '')
REGION = 'us-east-1'


class ProjectServiceInstance:
    """
    ProjectService Handles external salesforce Project
    """

    access_token = None
    access_token_expires = datetime.datetime.now() + datetime.timedelta(minutes=30)

    def __init__(self):
        self.platform_gateway_url = cla.config.PLATFORM_GATEWAY_URL

    def is_standalone(self, project_sfid) -> bool:
        """
        Checks if salesforce project is a stand alone project (No subprojects and parent)
        :param project_sfid: salesforce project_id
        :type project_sfid: string
        :return: Check whether sf project is a stand alone
        :rtype: Boolean
        """
        project = self.get_project_by_id(project_sfid)
        if project:
            parent_name = self.get_parent_name(project)
            if parent_name is None or (parent_name == THE_LINUX_FOUNDATION or parent_name == LF_PROJECTS_LLC) \
                    and not project.get('Projects'):
                return True
            else:
                return False
        return False

    def is_lf_supported(self, project_sfid) -> bool:
        """
        Checks if salesforce project is a LF Supported project
        :param project_sfid: salesforce project_id
        :type project_sfid: string
        :return: Check whether sf project is a stand alone
        :rtype: Boolean
        """
        project = self.get_project_by_id(project_sfid)
        if project:
            parent_name = self.get_parent_name(project)
            return (project.get('Funding', None) == 'Unfunded' or
                    project.get('Funding', None) == 'Supported By Parent') and \
                   (parent_name == THE_LINUX_FOUNDATION or parent_name == LF_PROJECTS_LLC)
        return False

    def has_parent(self, project) -> bool:
        """ checks if project has parent """
        fn = 'project_service.has_parent'
        try:
            log.info(f"{fn} - Checking if {project['Name']} has parent project")
            if project and project['Foundation']['ID'] != '' and project['Foundation']['Name'] != '':
                return True
        except KeyError as err:
            log.debug(f"{fn} - Failed to find parent for {project['Name']}, error: {err}")
            return False
        return False

    def get_parent_name(self, project) -> Optional[str]:
        """ returns the project parent name if exists, otherwise returns None """
        fn = 'project_service.get_parent_name'
        try:
            log.info(f"{fn} - Checking if {project['Name']} has parent project")
            if project and project['Foundation']['ID'] != '' and project['Foundation']['Name'] != '':
                return project['Foundation']['Name']
        except KeyError as err:
            log.debug(f"{fn} - Failed to find parent for {project['Name']}, error: {err}")
            return None
        return None

    def is_parent(self, project) -> bool:
        """ 
        checks whether salesforce project is a parent
        :param project: salesforce project
        :type project: dict
        :return: Whether salesforce project is a parent
        :rtype: Boolean
        """
        fn = 'project_service.is_parent'
        try:
            log.info(f"{fn} - Checking if {project['Name']} is a parent")
            project_type = project['ProjectType']
            if project_type == 'Project Group':
                return True
        except KeyError as err:
            log.debug(f"{fn} - Failed to get ProjectType for project: {project['Name']}  error: {err}")
            return False
        return False

    def get_project_by_id(self, project_id):
        """
        Gets Salesforce project by ID
        """
        fn = 'project_service.get_project_by_id'
        headers = {
            'Authorization': f'bearer {self.get_access_token()}',
            'accept': 'application/json'
        }
        try:
            url = f'{self.platform_gateway_url}/project-service/v1/projects/{project_id}'
            cla.log.debug(f'{fn} - sending GET request to {url}')
            r = requests.get(url, headers=headers)
            r.raise_for_status()
            response_model = json.loads(r.text)
            return response_model
        except requests.exceptions.HTTPError as err:
            msg = f'{fn} - Could not get project: {project_id}, error: {err}'
            cla.log.warning(msg)
            return None

    def get_access_token(self):
        fn = 'project_service.get_access_token'
        # Use previously cached value, if not expired
        if self.access_token and datetime.datetime.now() < self.access_token_expires:
            cla.log.debug(f'{fn} - using cached access token')
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


ProjectService = ProjectServiceInstance()
