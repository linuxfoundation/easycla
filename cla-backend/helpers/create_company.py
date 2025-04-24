# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Convenience script to create companies.

Based on the database storage configuration found in config.py and config_instance.py.
"""

MANAGER_GITHUB_ID = 123

import sys
sys.path.append('../')

import cla
import uuid
from cla.utils import get_company_instance, get_user_instance

# User
manager = get_user_instance().get_user_by_github_id(MANAGER_GITHUB_ID)
cla.log.info('Creating new company with manager ID: %s', manager.get_user_id())
company = get_company_instance()
company.set_company_id(str(uuid.uuid4()))
company.set_company_external_id('company-external-id')
company.set_company_manager_id(manager.get_user_id())
company.set_company_name('Test Company')
company.set_is_sanctioned(False)
company.set_company_whitelist([])
company.set_company_whitelist_patterns(['*@listed.org'])
company.save()
