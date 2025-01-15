# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Convenience script to create a ICLA and CCLA signature in the CLA system.
"""
import uuid
import sys
sys.path.append('../')

import cla
from cla.utils import get_signature_instance, get_user_instance, get_project_instance, \
                      get_company_instance

PROJECT_EXTERNAL_ID1 = 'a090t0000008DEiAAM'
PROJECT_EXTERNAL_ID2 = 'a090t0000008E7iAAE'
COMPANY_EXTERNAL_ID = 'company-external-id'
USER_GITHUB_ID = 123

user = get_user_instance().get_user_by_github_id(USER_GITHUB_ID)
project1 = get_project_instance().get_projects_by_external_id(PROJECT_EXTERNAL_ID1)[0]
project2 = get_project_instance().get_projects_by_external_id(PROJECT_EXTERNAL_ID2)[0]
company_list = get_company_instance().get_company_by_external_id(COMPANY_EXTERNAL_ID)
company = None
if company_list:
    company = company_list[0]

# Test ICLA Agreement.
sig_id = str(uuid.uuid4())
cla.log.info('Creating ICLA signature for user %s and project %s: %s' \
             %(user.get_user_name(), project1.get_project_external_id(), sig_id))
signature = get_signature_instance()
signature.set_signature_id(sig_id)
signature.set_signature_project_id(project1.get_project_id())
signature.set_signature_signed(True)
signature.set_signature_approved(True)
signature.set_signature_embargo_acked(True)
signature.set_signature_type('cla')
signature.set_signature_reference_id(user.get_user_id())
signature.set_signature_reference_type('user')
signature.set_signature_document_major_version(1)
signature.set_signature_document_minor_version(0)
signature.save()

# Test CCLA Agreement with project one.
sig_id = str(uuid.uuid4())
cla.log.info('Creating CCLA signature for company %s and project %s: %s' \
             %(company.get_company_external_id(), project1.get_project_external_id(), sig_id))
signature = get_signature_instance()
signature.set_signature_id(sig_id)
signature.set_signature_project_id(project1.get_project_id())
signature.set_signature_signed(True)
signature.set_signature_approved(True)
signature.set_signature_embargo_acked(True)
signature.set_signature_type('cla')
signature.set_signature_reference_id(company.get_company_id())
signature.set_signature_reference_type('company')
signature.set_signature_document_major_version(1)
signature.set_signature_document_minor_version(0)
signature.save()

# Test CCLA Agreement with project two.
sig_id = str(uuid.uuid4())
cla.log.info('Creating CCLA signature for company %s and project %s: %s' \
             %(company.get_company_external_id(), project2.get_project_external_id(), sig_id))
signature = get_signature_instance()
signature.set_signature_id(sig_id)
signature.set_signature_project_id(project2.get_project_id())
signature.set_signature_signed(True)
signature.set_signature_approved(True)
signature.set_signature_embargo_acked(True)
signature.set_signature_type('cla')
signature.set_signature_reference_id(company.get_company_id())
signature.set_signature_reference_type('company')
signature.set_signature_document_major_version(1)
signature.set_signature_document_minor_version(0)
signature.save()
