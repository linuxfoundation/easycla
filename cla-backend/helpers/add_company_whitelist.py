# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Convenience script to create companies.

Based on the database storage configuration found in config.py and config_instance.py.
"""

import sys
sys.path.append('../')

if len(sys.argv) != 2:
    print('Usage: python3 add_company_whitelist.py <email@address.com>')
    exit()
whitelist = sys.argv[1]

import cla
from cla.utils import get_company_instance

cla.log.info('Adding whitelist item to all companies: %s', whitelist)
# User
companies = get_company_instance().all()
for company in companies:
    company.add_company_whitelist(whitelist)
    company.save()
