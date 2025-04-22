# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

import sys
sys.path.append('../')

import uuid
import cla
from cla.utils import get_project_instance, get_user_instance, get_document_instance, get_github_organization_instance, get_project_instance, get_pdf_service, get_company_instance, get_signature_instance
from cla.resources.contract_templates import CNCFTemplate


# Project external ID.
PROJECT_EXTERNAL_ID = 'a0941000002wByZAAU'

## Project
cla.log.info('Creating first project for %s', PROJECT_EXTERNAL_ID)
project = get_project_instance()
project.set_project_id(str(uuid.uuid4()))
project.set_project_external_id(PROJECT_EXTERNAL_ID)
project.set_project_name('Test Project One')
project.set_project_ccla_requires_icla_signature(False)
project.save()

## Create CCLA Document
corporate_template = CNCFTemplate(document_type='Corporate',
                 major_version=1,
                 minor_version=0)
content = corporate_template.get_html_contract("", "")
pdf_generator = get_pdf_service()
pdf_content = pdf_generator.generate(content)


## CCLA
corporate_document = get_document_instance()
corporate_document.set_document_name('Test CCLA Document')
corporate_document.set_document_file_id(str(uuid.uuid4()))
corporate_document.set_document_content_type('storage+pdf')
corporate_document.set_document_content(pdf_content, b64_encoded=False)
corporate_document.set_document_major_version(1)
corporate_document.set_document_minor_version(0)
corporate_document.set_raw_document_tabs(corporate_template.get_tabs())
project.add_project_corporate_document(corporate_document)
project.save()

## Create Github Org
GITHUB_ORGANIZATION_NAME = 'linuxfoundation'
GITHUB_INSTALLATION_ID = 72228 # NOT THE APP ID - find it in the webhook request JSON or URL when viewing installed apps.
cla.log.info('Creating GitHub Organization: %s' %GITHUB_ORGANIZATION_NAME)
github_org = get_github_organization_instance()
github_org.set_organization_name(GITHUB_ORGANIZATION_NAME)
github_org.set_organization_project_id(project.get_project_id())
# This will be different everytime the CLA app is installed.
github_org.set_organization_installation_id(GITHUB_INSTALLATION_ID)
github_org.save()

## User (For Company Management)
cla.log.info('Creating company manager user')
manager = get_user_instance()
manager.set_user_id(str(uuid.uuid4()))
manager.set_user_name('First User')
manager.set_user_email('firstuser@domain.org')
manager.set_user_email('foobarski@linuxfoundation.org')
manager.set_user_github_id(123)
manager.save()

## Company
cla.log.info('Creating new company with manager ID: %s', manager.get_user_id())
company = get_company_instance()
company.set_company_id(str(uuid.uuid4()))
company.set_company_external_id('company-external-id')
company.set_company_manager_id(manager.get_user_id())
company.set_company_name('Test Company')
company.set_eligible_to_contribute(True)
company.set_company_whitelist([])
company.set_company_whitelist_patterns(['*@listed.org'])
company.save()

## Add another company with same manager ID
cla.log.info('Creating new company with manager ID: %s', manager.get_user_id())
company = get_company_instance()
company.set_company_id(str(uuid.uuid4()))
company.set_company_external_id('company-external-id')
company.set_company_manager_id(manager.get_user_id())
company.set_company_name('Test Company 2')
company.set_eligible_to_contribute(True)
company.set_company_whitelist([])
company.set_company_whitelist_patterns(['*@listed.org'])
company.save()

## Signature: Corporate
corporate_signature_id = str(uuid.uuid4())
cla.log.info('Creating CCLA signature for company %s and project %s: %s' \
             %(company.get_company_external_id(), project.get_project_external_id(), corporate_signature_id))
corporate_signature = get_signature_instance()
corporate_signature.set_signature_id(corporate_signature_id)
corporate_signature.set_signature_project_id(project.get_project_id())
corporate_signature.set_signature_signed(True)
corporate_signature.set_signature_approved(True)
corporate_signature.set_signature_embargo_acked(True)
corporate_signature.set_signature_type('cla')
corporate_signature.set_signature_reference_id(company.get_company_id())
corporate_signature.set_signature_reference_type('company')
corporate_signature.set_signature_document_major_version(1)
corporate_signature.set_signature_document_minor_version(0)
corporate_signature.save()

## Create CCLA Document
individual_template = CNCFTemplate(document_type='Individual',
                 major_version=1,
                 minor_version=0)
individual_content = individual_template.get_html_contract("", "")
pdf_generator = get_pdf_service()
pdf_content = pdf_generator.generate(individual_content)

## ICLA
individual_document = get_document_instance()
individual_document.set_document_name('Test CCLA Document')
individual_document.set_document_file_id(str(uuid.uuid4()))
individual_document.set_document_content_type('storage+pdf')
individual_document.set_document_content(pdf_content, b64_encoded=False)
individual_document.set_document_major_version(1)
individual_document.set_document_minor_version(0)
individual_document.set_raw_document_tabs(individual_template.get_tabs())
project.add_project_individual_document(individual_document)
project.save()

## User (For Individual Contributor)
cla.log.info('Creating individual signer user')
individual = get_user_instance()
individual.set_user_id(str(uuid.uuid4()))
individual.set_user_name('A Tester')
individual.set_user_email('icla@domain.org')
individual.set_user_email('user@intel.com')
individual.set_user_github_id(234)
individual.save()

## Signature: Individual
individual_signature_id = str(uuid.uuid4())
cla.log.info('Creating ICLA signature')
individual_signature = get_signature_instance()
individual_signature.set_signature_id(individual_signature_id)
individual_signature.set_signature_project_id(project.get_project_id())
individual_signature.set_signature_signed(True)
individual_signature.set_signature_approved(True)
individual_signature.set_signature_embargo_acked(True)
individual_signature.set_signature_type('cla')
individual_signature.set_signature_reference_id(individual.get_user_id())
individual_signature.set_signature_reference_type('user')
individual_signature.set_signature_document_major_version(1)
individual_signature.set_signature_document_minor_version(0)
individual_signature.save()

## Signature: Individual
individual_signature_id = str(uuid.uuid4())
cla.log.info('Creating ICLA signature')
individual_signature = get_signature_instance()
individual_signature.set_signature_id(individual_signature_id)
individual_signature.set_signature_project_id(project.get_project_id())
individual_signature.set_signature_signed(True)
individual_signature.set_signature_approved(True)
individual_signature.set_signature_embargo_acked(True)
individual_signature.set_signature_type('cla')
individual_signature.set_signature_reference_id(individual.get_user_id())
individual_signature.set_signature_reference_type('user')
individual_signature.set_signature_document_major_version(1)
individual_signature.set_signature_document_minor_version(1)
individual_signature.save()

## Signature: Individual
individual_signature_id = str(uuid.uuid4())
cla.log.info('Creating ICLA signature')
individual_signature = get_signature_instance()
individual_signature.set_signature_id(individual_signature_id)
individual_signature.set_signature_project_id(project.get_project_id())
individual_signature.set_signature_signed(True)
individual_signature.set_signature_approved(True)
individual_signature.set_signature_embargo_acked(True)
individual_signature.set_signature_type('cla')
individual_signature.set_signature_reference_id(individual.get_user_id())
individual_signature.set_signature_reference_type('user')
individual_signature.set_signature_document_major_version(2)
individual_signature.set_signature_document_minor_version(0)
individual_signature.save()

## User: B Tester
cla.log.info('Creating individual signer user')
individual_b = get_user_instance()
individual_b.set_user_id(str(uuid.uuid4()))
individual_b.set_user_name('B Tester')
individual_b.set_user_email('icla@example.org')
individual_b.set_user_github_id(567)
individual_b.save()

## Signature: Individual
individual_b_signature_id = str(uuid.uuid4())
cla.log.info('Creating ICLA signature')
individual_b_signature = get_signature_instance()
individual_b_signature.set_signature_id(individual_b_signature_id)
individual_b_signature.set_signature_project_id(project.get_project_id())
individual_b_signature.set_signature_signed(True)
individual_b_signature.set_signature_approved(True)
individual_b_signature.set_signature_embargo_acked(True)
individual_b_signature.set_signature_type('cla')
individual_b_signature.set_signature_reference_id(individual_b.get_user_id())
individual_b_signature.set_signature_reference_type('user')
individual_b_signature.set_signature_document_major_version(2)
individual_b_signature.set_signature_document_minor_version(0)
individual_b_signature.save()

## Signature: A Tester Employee
employee_signature_id = str(uuid.uuid4())
cla.log.info('Creating Employee CLA signature for company %s and project %s: %s' \
             %(company.get_company_external_id(), project.get_project_external_id(), employee_signature_id))
employee_signature = get_signature_instance()
employee_signature.set_signature_id(employee_signature_id)
employee_signature.set_signature_project_id(project.get_project_id())
employee_signature.set_signature_signed(True)
employee_signature.set_signature_approved(True)
employee_signature.set_signature_embargo_acked(True)
employee_signature.set_signature_type('cla')
employee_signature.set_signature_reference_id(individual.get_user_id())
employee_signature.set_signature_reference_type('user')
employee_signature.set_signature_document_major_version(1)
employee_signature.set_signature_document_minor_version(0)
employee_signature.set_signature_user_ccla_company_id(company.get_company_id())
employee_signature.save()
