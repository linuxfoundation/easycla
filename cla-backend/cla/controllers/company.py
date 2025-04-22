# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Controller related to company operations.
"""

import uuid

import hug.types
from falcon import HTTP_409, HTTP_200, HTTPForbidden

import cla
import cla.controllers.user
from cla.auth import AuthUser, admin_list
from cla.models import DoesNotExist
from cla.models.dynamo_models import Company, Event
from cla.models.event_types import EventType


def get_companies():
    """
    Returns a list of companies in the CLA system.

    :return: List of companies in dict format.
    :rtype: [dict]
    """
    fn = 'controllers.company.get_companies'

    cla.log.debug(f'{fn} - loading all companies...')
    all_companies = [company.to_dict() for company in Company().all()]
    cla.log.debug(f'{fn} - loaded all companies')
    all_companies = sorted(all_companies, key=lambda i: i['company_name'].casefold())

    return all_companies


def get_companies_by_user(username: str):
    """
    Returns a list of companies for a user in the CLA system.

    :return: List of companies in dict format.
    :rtype: [dict]
    """
    fn = 'controllers.company.get_companies_by_user'
    cla.log.debug(f'{fn} - loading companies by user: {username}...')
    all_companies = [company.to_dict() for company in Company().all() if username in company.get_company_acl()]
    cla.log.debug(f'{fn} - load companies by user: {username}')
    all_companies = sorted(all_companies, key=lambda i: i['company_name'].casefold())

    return all_companies


def company_acl_verify(username: str, company: Company):
    if username in company.get_company_acl():
        return True

    raise HTTPForbidden('Unauthorized',
                        'Provided Token credentials does not have sufficient permissions to access resource')


def get_company(company_id: str):
    """
    Returns the CLA company requested by ID.

    :param company_id: The company's ID.
    :type company_id: ID
    :return: dict representation of the company object.
    :rtype: dict
    """
    fn = 'controllers.company.get_company'
    company = Company()
    try:
        cla.log.debug(f'{fn} - loading company by company_id: {company_id}...')
        company.load(company_id=str(company_id))
        cla.log.debug(f'{fn} - loaded company by company_id: {company_id}')
    except DoesNotExist as err:
        return {'errors': {'company_id': str(err)}}

    return company.to_dict()


def create_company(auth_user: AuthUser,
                   company_name: str = None,
                   signing_entity_name: str = None,
                   company_manager_id: str = None,
                   company_manager_user_name: str = None,
                   company_manager_user_email: str = None,
                   is_embargoed: bool = False,
                   user_id: str = None,
                   response=None):
    """
    Creates an company and returns the newly created company in dict format.

    :param auth_user: The authenticated user
    :type auth_user: object
    :param company_name: The company name.
    :type company_name: string
    :param signing_entity_name: The company's signing entity name.
    :type signing_entity_name: string
    :param company_manager_id: The ID of the company manager user.
    :type company_manager_id: string
    :param company_manager_user_name: The user name of the company manager user.
    :type company_manager_user_name: string
    :param company_manager_user_email: The user email of the company manager user.
    :type company_manager_user_email: string
    :param is_embargoed: is embargoed
    :type is_embargoed: bool
    :return: dict representation of the company object.
    :rtype: dict
    """
    fn = 'controllers.company.create_company'
    manager = cla.controllers.user.get_or_create_user(auth_user)

    for company in get_companies():
        if company.get("company_name") == company_name:
            cla.log.error({"error": "Company already exists"})
            response.status = HTTP_409
            return {"status_code": HTTP_409,
                    "data": {"error": "Company already exists.",
                             "company_id": company.get("company_id")}
                    }

    cla.log.debug(f'{fn} - creating company with name: {company_name} with signing entity name: {signing_entity_name}')
    company = Company()
    company.set_company_id(str(uuid.uuid4()))
    company.set_company_name(company_name)
    company.set_signing_entity_name(signing_entity_name)
    company.set_company_manager_id(manager.get_user_id())
    company.set_company_acl(manager.get_lf_username())
    company.set_is_embargoed(is_embargoed)
    company.save()
    cla.log.debug(f'{fn} - created company with name: {company_name} with company_id: {company.get_company_id()}')

    # Create audit trail for company
    event_data = f'User {auth_user.username} created company {company.get_company_name()} ' \
                 f'with company_id: {company.get_company_id()}.'
    event_summary = f'User {auth_user.username} created company {company.get_company_name()}.'
    Event.create_event(
        event_type=EventType.CreateCompany,
        event_company_id=company.get_company_id(),
        event_data=event_data,
        event_summary=event_summary,
        event_user_id=user_id,
        contains_pii=False,
    )

    return {"status_code": HTTP_200, "data": company.to_dict()}


def update_company(company_id: str,  # pylint: disable=too-many-arguments
                   company_name: str = None,
                   company_manager_id: str = None,
                   is_embargoed: bool = None,
                   username: str = None):
    """
    Updates an company and returns the newly updated company in dict format.
    A value of None means the field should not be updated.

    :param company_id: ID of the company to update.
    :type company_id: str
    :param company_name: New company name.
    :type company_name: string | None
    :param company_manager_id: The ID of the company manager user.
    :type company_manager_id: str
    :param username: The username of the existing company manager user who performs the company update.
    :type username: str
    :return: dict representation of the company object.
    :rtype: dict
    """
    company = Company()
    try:
        company.load(str(company_id))
    except DoesNotExist as err:
        return {'errors': {'company_id': str(err)}}

    company_acl_verify(username, company)
    update_str = ""

    if company_name is not None:
        company.set_company_name(company_name)
        update_str += f"The company name was updated to {company_name}. "
    if company_manager_id is not None:
        val = hug.types.uuid(company_manager_id)
        company.set_company_manager_id(str(val))
        update_str += f"The company company manager id was updated to {val}"
    if is_embargoed is not None:
        company.set_is_embargoed(is_embargoed)
        update_str += f"The company is_embargoed was updated to {is_embargoed}. "

    company.save()

    # Audit update event
    event_data = update_str
    Event.create_event(
        event_data=event_data,
        event_summary=event_data,
        event_type=EventType.UpdateCompany,
        event_company_id=company_id,
        contains_pii=False,
    )
    return company.to_dict()


'''
def update_company_whitelist_csv(content, company_id, username=None):
    """
    Adds the CSV of email addresses to this company's whitelist.

    :param content: The content posted to this endpoint (CSV data).
    :type content: string
    :param company_id: The ID of the company to add to the whitelist.
    :type company_id: UUID
    """
    company = Company()
    try:
        company.load(str(company_id))
    except DoesNotExist as err:
        return {'errors': {'company_id': str(err)}}

    company_acl_verify(username, company)

    # Ready email addresses.
    emails = content.split('\n')
    emails = [email for email in emails if '@' in email]
    current_whitelist = company.get_company_'whitelist'()
    new_whitelist = list(set(current_whitelist + emails))
    company.set_company_whitelist(new_whitelist)
    company.save()
    return company.to_dict()
'''


def delete_company(company_id: str, username: str = None):
    """
    Deletes an company based on ID.

    :param company_id: The ID of the company.
    :type company_id: str
    :param username: The username of the user that deleted the company
    :type username: str
    """
    company = Company()
    try:
        company.load(str(company_id))
    except DoesNotExist as err:
        return {'errors': {'company_id': str(err)}}

    company_acl_verify(username, company)
    company.delete()

    event_data = f'The company {company.get_company_name()} with company_id {company.get_company_id()} was deleted.'
    event_summary = f'The company {company.get_company_name()} was deleted.'
    Event.create_event(
        event_data=event_data,
        event_summary=event_summary,
        event_type=EventType.DeleteCompany,
        event_company_id=company_id,
        contains_pii=False,
    )
    return {'success': True}


def get_manager_companies(manager_id):
    companies = Company().get_companies_by_manager(manager_id)
    return companies


def add_permission(auth_user: AuthUser, username: str, company_id: str, ignore_auth_user=False):
    fn = 'controllers.company.add_permission'
    if not ignore_auth_user and auth_user.username not in admin_list:
        return {'error': 'unauthorized'}

    cla.log.info(f'{fn} - company ({company_id}) added for user ({username}) by {auth_user.username}')

    company = Company()
    try:
        company.load(company_id)
    except Exception as err:
        cla.log.warning(f'{fn} - unable to update company permission: {err}')
        return {'error': str(err)}

    company.add_company_acl(username)
    event_data = f'Added to user {username} to Company {company.get_company_name()} permissions list.'
    Event.create_event(
        event_data=event_data,
        event_summary=event_data,
        event_type=EventType.AddCompanyPermission,
        event_company_id=company_id,
        contains_pii=True,
    )
    company.save()


def remove_permission(auth_user: AuthUser, username: str, company_id: str):
    fn = 'controllers.company.remove_permission'
    if auth_user.username not in admin_list:
        return {'error': 'unauthorized'}

    cla.log.info(f'{fn} - company ({company_id}) removed for user ({username}) by {auth_user.username}')

    company = Company()
    try:
        company.load(company_id)
    except Exception as err:
        cla.log.warning(f'{fn} - unable to update company permission: {err}')
        return {'error': str(err)}

    company.remove_company_acl(username)
    event_data = f'Removed user {username} from Company {company.get_company_name()} permissions list.'
    Event.create_event(
        event_data=event_data,
        event_summary=event_data,
        event_company_id=company_id,
        event_type=EventType.RemoveCompanyPermission,
        contains_pii=True,
    )
    company.save()
