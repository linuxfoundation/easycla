# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Controller related to signature operations.
"""
import copy
import uuid
from datetime import datetime
from typing import List, Optional

import hug.types
import requests

import cla.hug_types
from cla.auth import AuthUser
from cla.controllers import company
from cla.models import DoesNotExist
from cla.models.dynamo_models import User, Project, Signature, Company, Event
from cla.models.event_types import EventType
from cla.utils import get_email_service, append_email_help_sign_off_content


def get_signatures():
    """
    Returns a list of signatures in the CLA system.

    :return: List of signatures in dict format.
    :rtype: [dict]
    """
    signatures = [signature.to_dict() for signature in Signature().all()]
    return signatures


def get_signature(signature_id):
    """
    Returns the CLA signature requested by UUID.

    :param signature_id: The signature UUID.
    :type signature_id: UUID
    :return: dict representation of the signature object.
    :rtype: dict
    """
    signature = Signature()
    try:
        signature.load(signature_id=str(signature_id))
    except DoesNotExist as err:
        return {'errors': {'signature_id': str(err)}}
    return signature.to_dict()


def create_signature(signature_project_id,  # pylint: disable=too-many-arguments
                     signature_reference_id,
                     signature_reference_type,
                     signature_type='cla',
                     signature_approved=False,
                     signature_signed=False,
                     signature_embargo_acked=False,
                     signature_return_url=None,
                     signature_sign_url=None,
                     signature_user_ccla_company_id=None,
                     signature_acl=None):
    """
    Creates an signature and returns the newly created signature in dict format.

    :param signature_project_id: The project ID for this new signature.
    :type signature_project_id: string
    :param signature_reference_id: The user or company ID for this signature.
    :type signature_reference_id: string
    :param signature_reference_type: The type of reference ('user' or 'company')
    :type signature_reference_type: string
    :param signature_type: The signature type ('cla' or 'dco')
    :type signature_type: string
    :param signature_signed: Whether or not the signature has been signed.
    :type signature_signed: boolean
    :param signature_approved: Whether or not the signature has been approved.
    :type signature_approved: boolean
    :param signature_embargo_acked: Whether or not the embargo was acknowledged
    :type signature_embargo_acked: boolean
    :param signature_return_url: The URL the user will be redirected to after signing.
    :type signature_return_url: string
    :param signature_sign_url: The URL the user must visit to sign the signature.
    :type signature_sign_url: string
    :param signature_user_ccla_company_id: The company ID if creating an employee signature.
    :type signature_user_ccla_company_id: string
    :return: A dict of a newly created signature.
    :param signature_acl: a list with the signature access control list values
    :type signature_acl: list of strings
    :rtype: dict
    """
    signature: Signature = cla.utils.get_signature_instance()
    signature.set_signature_id(str(uuid.uuid4()))
    project: Project = cla.utils.get_project_instance()
    try:
        project.load(project_id=str(signature_project_id))
    except DoesNotExist as err:
        return {'errors': {'signature_project_id': str(err)}}
    signature.set_signature_project_id(str(signature_project_id))
    if signature_reference_type == 'user':
        user: User = cla.utils.get_user_instance()
        try:
            user.load(signature_reference_id)
        except DoesNotExist as err:
            return {'errors': {'signature_reference_id': str(err)}}
        try:
            document = project.get_project_individual_document()
        except DoesNotExist as err:
            return {'errors': {'signature_project_id': str(err)}}
    else:
        company: Company = cla.utils.get_company_instance()
        try:
            company.load(signature_reference_id)
        except DoesNotExist as err:
            return {'errors': {'signature_reference_id': str(err)}}
        try:
            document = project.get_project_corporate_document()
        except DoesNotExist as err:
            return {'errors': {'signature_project_id': str(err)}}

    # Set username to this signature ACL
    if signature_acl is not None:
        signature.set_signature_acl(signature_acl)

    signature.set_signature_document_minor_version(document.get_document_minor_version())
    signature.set_signature_document_major_version(document.get_document_major_version())
    signature.set_signature_reference_id(str(signature_reference_id))
    signature.set_signature_reference_type(signature_reference_type)
    signature.set_signature_type(signature_type)
    signature.set_signature_signed(signature_signed)
    signature.set_signature_approved(signature_approved)
    signature.set_signature_embargo_acked(signature_embargo_acked)
    signature.set_signature_return_url(signature_return_url)
    signature.set_signature_sign_url(signature_sign_url)
    if signature_user_ccla_company_id is not None:
        signature.set_signature_user_ccla_company_id(str(signature_user_ccla_company_id))
    signature.save()

    event_data = f'Signature added. Signature_id - {signature.get_signature_id()} for Project - {project.get_project_name()}'
    Event.create_event(
        event_data=event_data,
        event_summary=event_data,
        event_type=EventType.CreateSignature,
        event_cla_group_id=str(signature_project_id),
        contains_pii=False,
    )

    return signature.to_dict()


def update_signature(signature_id,  # pylint: disable=too-many-arguments,too-many-return-statements,too-many-branches
                     auth_user,
                     signature_project_id=None,
                     signature_reference_id=None,
                     signature_reference_type=None,
                     signature_type=None,
                     signature_approved=None,
                     signature_signed=None,
                     signature_embargo_acked=None,
                     signature_return_url=None,
                     signature_sign_url=None,
                     domain_whitelist=None,
                     email_whitelist=None,
                     github_whitelist=None,
                     github_org_whitelist=None):
    """
    Updates an signature and returns the newly updated signature in dict format.
    A value of None means the field should not be updated.

    :param signature_id: ID of the signature.
    :type signature_id: ID | None
    :param auth_user: the authenticated user
    :type auth_user: string
    :param signature_project_id: Project ID for this signature.
    :type signature_project_id: string | None
    :param signature_reference_id: Reference ID for this signature.
    :type signature_reference_id: string | None
    :param signature_reference_type: Reference type for this signature.
    :type signature_reference_type: ['user' | 'company'] | None
    :param signature_type: New signature type ('cla' or 'dco').
    :type signature_type: string | None
    :param signature_signed: Whether this signature is signed or not.
    :type signature_signed: boolean | None
    :param signature_approved: Whether this signature is approved or not.
    :type signature_approved: boolean | None
    :param signature_embargo_acked: Whether this signature's embargo is acknowledged
    :type signature_embargo_acked: boolean | None
    :param signature_return_url: The URL the user will be sent to after signing.
    :type signature_return_url: string | None
    :param signature_sign_url: The URL the user must visit to sign the signature.
    :type signature_sign_url: string | None
    :param domain_whitelist:  the domain whitelist
    :param email_whitelist:  the email whitelist
    :param github_whitelist:  the github username whitelist
    :param github_org_whitelist:  the github org whitelist
    :return: dict representation of the signature object.
    :rtype: dict
    """
    fn = 'controllers.signature.update_signature'
    cla.log.debug(f'{fn} - loading signature by id: {str(signature_id)}')
    signature = Signature()
    try:  # Try to load the signature to update.
        signature.load(str(signature_id))
        old_signature = copy.deepcopy(signature)
    except DoesNotExist as err:
        return {'errors': {'signature_id': str(err)}}
    update_str = f'signature {signature_id} updates: \n '
    if signature_project_id is not None:
        # make a note if the project id is set and doesn't match
        if signature.get_signature_project_id() != str(signature_project_id):
            cla.log.warning(f'{fn} - project IDs do not match => '
                            f'record project id: {signature.get_signature_project_id()} != '
                            f'parameter project id: {str(signature_project_id)}')
        try:
            signature.set_signature_project_id(str(signature_project_id))
            update_str += f'signature_project_id updated to {signature_project_id} \n'
        except DoesNotExist as err:
            return {'errors': {'signature_project_id': str(err)}}
    # TODO: Ensure signature_reference_id exists.
    if signature_reference_id is not None:
        if signature.get_signature_reference_id() != str(signature_reference_id):
            cla.log.warning(f'{fn} - signature reference IDs do not match => '
                            f'record signature ref id: {signature.get_signature_reference_id()} != '
                            f'parameter signature ref id: {str(signature_reference_id)}')
        signature.set_signature_reference_id(signature_reference_id)
    if signature_reference_type is not None:
        signature.set_signature_reference_type(signature_reference_type)
        update_str += f'signature_reference_type updated to {signature_reference_type} \n'
    if signature_type is not None:
        if signature_type in ['cla', 'dco']:
            signature.set_signature_type(signature_type)
            update_str += f'signature_type updated to {signature_type} \n'
        else:
            return {'errors': {'signature_type': 'Invalid value passed. The accepted values are: (cla|dco)'}}
    if signature_signed is not None:
        try:
            val = hug.types.smart_boolean(signature_signed)
            signature.set_signature_signed(val)
            update_str += f'signature_signed updated to {signature_signed} \n'
        except KeyError:
            return {'errors': {'signature_signed': 'Invalid value passed in for true/false field'}}
    if signature_approved is not None:
        try:
            val = hug.types.smart_boolean(signature_approved)
            update_signature_approved(signature, val)
            update_str += f'signature_approved updated to {val} \n'
        except KeyError:
            return {'errors': {'signature_approved': 'Invalid value passed in for true/false field'}}
    if signature_embargo_acked is not None:
        try:
            val = hug.types.smart_boolean(signature_embargo_acked)
            signature.set_signature_embargo_acked(val)
            update_str += f'signature_embargo_acked updated to {val} \n'
        except KeyError:
            return {'errors': {'signature_embargo_acked': 'Invalid value passed in for true/false field'}}
    if signature_return_url is not None:
        try:
            val = cla.hug_types.url(signature_return_url)
            signature.set_signature_return_url(val)
            update_str += f'signature_return_url updated to {val} \n'
        except KeyError:
            return {'errors': {'signature_return_url': 'Invalid value passed in for URL field'}}
    if signature_sign_url is not None:
        try:
            val = cla.hug_types.url(signature_sign_url)
            signature.set_signature_sign_url(val)
            update_str += f'signature_sign_url updated to {val} \n'
        except KeyError:
            return {'errors': {'signature_sign_url': 'Invalid value passed in for URL field'}}

    if domain_whitelist is not None:
        try:
            domain_whitelist = hug.types.multiple(domain_whitelist)
            signature.set_domain_whitelist(domain_whitelist)
            update_str += f'domain_whitelist updated to {domain_whitelist} \n'
        except KeyError:
            return {'errors': {
                'domain_whitelist': 'Invalid value passed in for the domain whitelist'
            }}

    if email_whitelist is not None:
        try:
            email_whitelist = hug.types.multiple(email_whitelist)
            signature.set_email_whitelist(email_whitelist)
            update_str += f'email_whitelist updated to {email_whitelist} \n'
        except KeyError:
            return {'errors': {
                'email_whitelist': 'Invalid value passed in for the email whitelist'
            }}

    if github_whitelist is not None:
        try:
            github_whitelist = hug.types.multiple(github_whitelist)
            signature.set_github_whitelist(github_whitelist)

            # A little bit of special logic to for GitHub whitelists that have bots
            bot_list = [github_user for github_user in github_whitelist if is_github_bot(github_user)]
            if bot_list is not None:
                handle_bots(bot_list, signature)
            update_str += f'github_whitelist updated to {github_whitelist} \n'
        except KeyError:
            return {'errors': {
                'github_whitelist': 'Invalid value passed in for the github whitelist'
            }}

    if github_org_whitelist is not None:
        try:
            github_org_whitelist = hug.types.multiple(github_org_whitelist)
            signature.set_github_org_whitelist(github_org_whitelist)
            update_str += f'github_org_whitelist updated to {github_org_whitelist} \n'
        except KeyError:
            return {'errors': {
                'github_org_whitelist': 'Invalid value passed in for the github org whitelist'
            }}

    event_data = update_str
    Event.create_event(
        event_data=event_data,
        event_summary=event_data,
        event_type=EventType.UpdateSignature,
        event_cla_group_id=signature.get_signature_project_id(),
        contains_pii=True,
    )

    signature.save()
    notify_whitelist_change(auth_user=auth_user, old_signature=old_signature, new_signature=signature)
    return signature.to_dict()


def change_in_list(old_list, new_list, msg_added, msg_deleted):
    if old_list is None:
        old_list = []
    if new_list is None:
        new_list = []
    added = list(set(new_list) - set(old_list))
    deleted = list(set(old_list) - set(new_list))
    change = []
    if len(added) > 0:
        change.append(msg_added.format('\n'.join(added)))
    if len(deleted) > 0:
        change.append(msg_deleted.format('\n'.join(deleted)))
    return change, added, deleted


def notify_whitelist_change(auth_user, old_signature: Signature, new_signature: Signature):
    company_name = new_signature.get_signature_reference_name()
    project = cla.utils.get_project_instance()
    project.load(new_signature.get_signature_project_id())
    project_name = project.get_project_name()

    changes = []
    domain_msg_added = 'The domain {} was added to the domain approval list.'
    domain_msg_deleted = 'The domain {} was removed from the domain approval list.'
    domain_changes, _, _ = change_in_list(old_list=old_signature.get_domain_whitelist(),
                                          new_list=new_signature.get_domain_whitelist(),
                                          msg_added=domain_msg_added,
                                          msg_deleted=domain_msg_deleted)
    changes = changes + domain_changes

    email_msg_added = 'The email address {} was added to the email approval list.'
    email_msg_deleted = 'The email address {} was removed from the email approval list.'
    email_changes, email_added, email_deleted = change_in_list(old_list=old_signature.get_email_whitelist(),
                                                               new_list=new_signature.get_email_whitelist(),
                                                               msg_added=email_msg_added,
                                                               msg_deleted=email_msg_deleted)
    changes = changes + email_changes

    github_msg_added = 'The GitHub user {} was added to the GitHub approval list.'
    github_msg_deleted = 'The GitHub user {} was removed from the github approval list.'
    github_changes, github_added, github_deleted = change_in_list(old_list=old_signature.get_github_whitelist(),
                                                                  new_list=new_signature.get_github_whitelist(),
                                                                  msg_added=github_msg_added,
                                                                  msg_deleted=github_msg_deleted)
    changes = changes + github_changes

    github_org_msg_added = 'The GitHub organization {} was added to the GitHub organization approval list.'
    github_org_msg_deleted = 'The GitHub organization {} was removed from the GitHub organization approval list.'
    github_org_changes, _, _ = change_in_list(old_list=old_signature.get_github_org_whitelist(),
                                              new_list=new_signature.get_github_org_whitelist(),
                                              msg_added=github_org_msg_added,
                                              msg_deleted=github_org_msg_deleted)
    changes = changes + github_org_changes

    if len(changes) > 0:
        # send email to cla managers about change
        cla_managers = new_signature.get_managers()
        subject, body, recipients = approval_list_change_email_content(
            project, company_name, project_name, cla_managers, changes)
        if len(recipients) > 0:
            get_email_service().send(subject, body, recipients)

    cla_manager_name = auth_user.name
    # send email to contributors
    notify_whitelist_change_to_contributors(project=project,
                                            email_added=email_added,
                                            email_removed=email_deleted,
                                            github_users_added=github_added,
                                            github_users_removed=github_deleted,
                                            company_name=company_name,
                                            project_name=project_name,
                                            cla_manager_name=cla_manager_name)
    event_data = " ,".join(changes)
    Event.create_event(
        event_data=event_data,
        event_summary=event_data,
        event_type=EventType.NotifyWLChange,
        event_company_name=company_name,
        event_project_name=project_name,
        event_cla_group_id=new_signature.get_signature_project_id(),
        contains_pii=True,
    )


def notify_whitelist_change_to_contributors(project, email_added, email_removed,
                                            github_users_added, github_users_removed,
                                            company_name, project_name, cla_manager_name):
    for email in email_added:
        subject, body, recipients = get_contributor_whitelist_update_email_content(
            project, 'added', company_name, project_name, cla_manager_name, email)
        get_email_service().send(subject, body, recipients)

    for email in email_removed:
        subject, body, recipients = get_contributor_whitelist_update_email_content(
            project, 'deleted', company_name, project_name, cla_manager_name, email)
        get_email_service().send(subject, body, recipients)

    for github_username in github_users_added:
        user = cla.utils.get_user_instance()
        users = user.get_user_by_github_username(github_username)
        if users is not None:
            user = users[0]
            email = user.get_user_email()
            subject, body, recipients = get_contributor_whitelist_update_email_content(
                project, 'added', company_name, project_name, cla_manager_name, email)
            get_email_service().send(subject, body, recipients)

    for github_username in github_users_removed:
        user = cla.utils.get_user_instance()
        users = user.get_user_by_github_username(github_username)
        if users is not None:
            user = users[0]
            email = user.get_user_email()
            subject, body, recipients = get_contributor_whitelist_update_email_content(
                project, 'deleted', company_name, project_name, cla_manager_name, email)
            get_email_service().send(subject, body, recipients)


def get_contributor_whitelist_update_email_content(project, action, company_name, project_name, cla_manager, email):
    subject = f'EasyCLA: Approval List Update for {project_name}'
    preposition = 'to'
    if action == 'deleted':
        preposition = 'from'
    body = f"""
<p>Hello,</p> \
<p>This is a notification email from EasyCLA regarding the project {project_name}.</p> \
<p>You have been {action} {preposition} the Approval List of {company_name} for {project_name} by \
CLA Manager {cla_manager}. This means that you are now authorized to contribute to {project_name} \
on behalf of {company_name}.</p> \
<p>If you had previously submitted one or more pull requests to {project_name} that had failed, you should \
close and re-open the pull request to force a recheck by the EasyCLA system.</p> 
"""
    body = '<p>' + body.replace('\n', '<br>') + '</p>'
    body = append_email_help_sign_off_content(body, project.get_version())
    recipients = [email]
    return subject, body, recipients


def approval_list_change_email_content(project, company_name, project_name, cla_managers, changes):
    """Helper function to get whitelist change email subject, body, recipients"""
    subject = f'EasyCLA: Approval List Update for {project_name}'
    # Append suffix / prefix to strings in list
    changes = ["<li>" + txt + "</li>" for txt in changes]
    change_string = "<ul>\n" + "\n".join(changes) + "\n</ul>\n"
    body = f"""
<p>Hello,</p> \
<p>This is a notification email from EasyCLA regarding the project {project_name}.</p> \
<p>The EasyCLA approval list for {company_name} for project {project_name} was modified.</p> \
<p>The modification was as follows:</p> \
{change_string} \
<p>Contributors with previously failed pull requests to {project_name} can close \
and re-open the pull request to force a recheck by the EasyCLA system.</p> 
"""
    body = append_email_help_sign_off_content(body, project.get_version())
    recipients = []
    for manager in cla_managers:
        email = manager.get_user_email()
        if email is not None:
            recipients.append(email)
    return subject, body, recipients


def handle_bots(bot_list: List[str], signature: Signature) -> None:
    fn = 'controllers.signature.handle_bots'
    cla.log.debug(f'{fn} - Bots: {bot_list}')
    for bot_name in bot_list:
        try:
            user = cla.utils.get_user_instance()
            users = user.get_user_by_github_username(bot_name)
            if users is None:
                cla.log.debug(f'{fn} - Bot: {bot_name} does not have a user record (None)')
                bot_user: User = create_bot(bot_name, signature)
                if bot_user is not None:
                    create_bot_signature(bot_user, signature)
            else:
                # Bot does have a user account in the EasyCLA system
                found = False
                # Search the list of user records to see if we have a matching company
                for u in users:
                    if u.get_user_company_id() == signature.get_signature_reference_id():
                        found = True
                        cla.log.debug('{fn} - found bot user account - ensuring the signature exists...')
                        create_bot_signature(u, signature)
                        break

                # We found matching users in our system, but didn't find one with a matching company
                if not found:
                    cla.log.debug(f'{fn} - unable to find user {bot_name} '
                                  f'for company: {signature.get_signature_reference_id()} - '
                                  'creating user record that matches this company...')
                    bot_user: User = create_bot(bot_name, signature)
                    if bot_user is not None:
                        create_bot_signature(bot_user, signature)
                    else:
                        cla.log.warning(f'{fn} - failed to create user record for: {bot_name}')
        except DoesNotExist as err:
            cla.log.debug(f'{fn} - bot: {bot_name} does not have a user record (DoesNotExist)')


def create_bot_signature(bot_user: User, signature: Signature) -> Optional[Signature]:
    fn = 'controllers.signature.create_bot_signature'
    cla.log.debug(f'{fn} - locating Bot Signature for: {bot_user.get_user_name()}...')
    project: Project = cla.utils.get_project_instance()
    try:
        project.load(signature.get_signature_project_id())
    except DoesNotExist as err:
        cla.log.warning(f'{fn} - unable to load project by id: {signature.get_signature_project_id()}'
                        f' Unable to create bot: {bot_user}')
        return None

    the_company: Company = cla.utils.get_company_instance()
    try:
        the_company.load(signature.get_signature_reference_id())
    except DoesNotExist as err:
        cla.log.warning(f'{fn} - unable to load company by id: {signature.get_signature_reference_id()}'
                        f' Unable to create bot: {bot_user}')
        return None

    bot_sig: Signature = cla.utils.get_signature_instance()

    # First, before we create a new one, grab a list of employee signatures for this company/project
    existing_sigs: List[Signature] = bot_sig.get_employee_signatures_by_company_project_model(
        company_id=bot_user.get_user_company_id(), project_id=signature.get_signature_project_id())

    # Check to see if we have an existing signature for this user/company/project combo
    for sig in existing_sigs:
        if sig.get_signature_reference_id() == bot_user.get_user_id():
            cla.log.debug('{fn} - found existing bot signature '
                          f'for user: {bot_user} '
                          f'with company: {the_company} '
                          f'for project: {project}')
            return sig

    # Didn't find an existing signature, let's create a new one
    cla.log.debug(f'{fn} - creating Bot Signature: {bot_user.get_user_name()}...')
    bot_sig.set_signature_id(str(uuid.uuid4()))
    bot_sig.set_signature_project_id(signature.get_signature_project_id())
    bot_sig.set_signature_reference_id(bot_user.get_user_id())
    bot_sig.set_signature_document_major_version(signature.get_signature_document_major_version())
    bot_sig.set_signature_document_minor_version(signature.get_signature_document_minor_version())
    bot_sig.set_signature_approved(True)
    bot_sig.set_signature_signed(True)
    # should bot signature by automaticaly set to "embargo acknowledged"?
    bot_sig.set_signature_embargo_acked(True)
    bot_sig.set_signature_type('cla')
    bot_sig.set_signature_reference_type('user')
    bot_sig.set_signature_user_ccla_company_id(bot_user.get_user_company_id())
    bot_sig.set_note(f'{datetime.utcnow().strftime("%Y%m%dT%H%M%SZ")} Added as part of '
                     f'{project.get_project_name()}, approval list by '
                     f'{the_company.get_company_name()}')
    bot_sig.save()
    cla.log.debug(f'{fn} - created Bot Signature: {bot_sig}')
    return bot_sig


def create_bot(bot_name: str, signature: Signature) -> Optional[User]:
    fn = 'controllers.signature.create_bot'
    cla.log.debug(f'{fn} - creating Bot: {bot_name}...')
    user_github_id = lookup_github_user(bot_name)
    if user_github_id != 0:
        project: Project = cla.utils.get_project_instance()
        try:
            project.load(signature.get_signature_project_id())
        except DoesNotExist as err:
            cla.log.warning(f'{fn} - Unable to load project by id: {signature.get_signature_project_id()}'
                            f' Unable to create bot: {bot_name}')
            return None

        the_company: Company = cla.utils.get_company_instance()
        try:
            the_company.load(signature.get_signature_reference_id())
        except DoesNotExist as err:
            cla.log.warning(f'{fn} - Unable to load company by id: {signature.get_signature_reference_id()}'
                            f' Unable to create bot: {bot_name}')
            return None

        user: User = cla.utils.get_user_instance()
        user.set_user_id(str(uuid.uuid4()))
        user.set_user_name(bot_name)
        user.set_user_github_username(bot_name)
        user.set_user_github_id(user_github_id)
        user.set_user_company_id(signature.get_signature_reference_id())
        user.set_note(f'{datetime.utcnow().strftime("%Y%m%dT%H%M%SZ")} Added as part of '
                      f'{project.get_project_name()}, approval list by '
                      f'{the_company.get_company_name()}')
        user.save()
        cla.log.debug(f'{fn} - created bot user: {user}')
        return user

    cla.log.warning(f'{fn} - unable to create bot user: {bot_name} - unable to lookup name in GitHub.')
    return None


def is_github_bot(username: str) -> bool:
    """
    Queries the GitHub public user endpoint for the specified username. Returns true if the user is a GitHub bot.

    :param username: the user's github name
    :return: True if the user is a GitHub bot, False otherwise
    """
    fn = 'controllers.signature.is_github_bot'
    cla.log.debug(f'{fn} - looking up GH user: {username}')
    r = requests.get('https://api.github.com/users/' + username)
    if r.status_code == requests.codes.ok:
        # cla.log.info(f'Response content type: {r.headers["Content-Type"]}')
        # cla.log.info(f'Response body: {r.json()}')
        response = r.json()
        cla.log.debug(f'{fn} - Lookup succeeded for GH user: {username} with id: {response["id"]}')
        if 'type' in response:
            return response['type'].lower() == 'bot'
        else:
            return False
    elif r.status_code == requests.codes.not_found:
        cla.log.debug(f'{fn} - Lookup failed for GH user: {username} - not found')
        return False
    else:
        cla.log.warning(f'{fn} - Error looking up GitHub user by username: {username}. '
                        f'Error: {r.status_code} - {r.text}')
    return False


def lookup_github_user(username: str) -> int:
    """
    Queries the GitHub public user endpoint for the specified username. Returns the user's GitHub ID.

    :param username: the user's github name
    :return: the user's GitHub ID
    """
    fn = 'controllers.signature.lookup_github_user'
    cla.log.debug(f'{fn} - uooking up GH user: {username}')
    r = requests.get('https://api.github.com/users/' + username)
    if r.status_code == requests.codes.ok:
        # cla.log.info(f'Response content type: {r.headers["Content-Type"]}')
        # cla.log.info(f'Response body: {r.json()}')
        response = r.json()
        cla.log.debug(f'{fn} - Lookup succeeded for GH user: {username} with id: {response["id"]}')
        return response['id']
    elif r.status_code == requests.codes.not_found:
        cla.log.debug(f'{fn} - Lookup failed for GH user: {username} - not found')
        return 0
    else:
        cla.log.warning(f'{fn} - Error looking up GitHub user by username: {username}. '
                        f'Error: {r.status_code} - {r.text}')
    return 0


def update_signature_approved(signature, value):
    """Helper function to update the signature approval status and send emails if necessary."""
    previous = signature.get_signature_approved()
    signature.set_signature_approved(value)
    email_approval = cla.conf['EMAIL_ON_SIGNATURE_APPROVED']
    if email_approval and not previous and value:  # Just got approved.
        subject, body, recipients = get_signature_approved_email_content(signature)
        get_email_service().send(subject, body, recipients)


def get_signature_approved_email_content(signature):  # pylint: disable=invalid-name
    """Helper function to get signature approval email subject, body, and recipients."""
    if signature.get_signature_reference_type() != 'user':
        cla.log.info('Not sending signature approved emails for CCLAs')
        return
    subject = 'CLA Signature Approved'
    user: User = cla.utils.get_user_instance()
    user.load(signature.get_signature_reference_id())
    project: Project = cla.utils.get_project_instance()
    project.load(signature.get_signature_project_id())
    recipients = [user.get_user_id()]
    body = 'Hello %s. Your Contributor License Agreement for %s has been approved!' \
           % (user.get_user_name(), project.get_project_name())
    return subject, body, recipients


def delete_signature(signature_id):
    """
    Deletes an signature based on UUID.

    :param signature_id: The UUID of the signature.
    :type signature_id: UUID
    """
    signature = Signature()
    cla_group_id = ''
    try:  # Try to load the signature to delete.
        signature.load(str(signature_id))
        cla_group_id = signature.get_signature_project_id()
    except DoesNotExist as err:
        # Should we bother sending back an error?
        return {'errors': {'signature_id': str(err)}}
    signature.delete()
    event_data = f'Deleted signature {signature_id}'
    Event.create_event(
        event_data=event_data,
        event_summary=event_data,
        event_cla_group_id=cla_group_id,
        event_type=EventType.DeleteSignature,
        contains_pii=False,
    )

    return {'success': True}


def get_user_signatures(user_id):
    """
    Get all signatures for user.

    :param user_id: The ID of the user in question.
    :type user_id: string
    """
    signatures = Signature().get_signatures_by_reference(str(user_id), 'user')
    return [signature.to_dict() for signature in signatures]


def get_user_project_signatures(user_id, project_id, signature_type=None):
    """
    Get all signatures for user filtered by a project.

    :param user_id: The ID of the user in question.
    :type user_id: string
    :param project_id: The ID of the project to filter by.
    :type project_id: string
    :param signature_type: The signature type to filter by.
    :type signature_type: string (one of 'individual', 'employee')
    :return: The list of signatures requested.
    :rtype: [cla.models.model_interfaces.Signature]
    """
    sig = Signature()
    signatures = sig.get_signatures_by_project(str(project_id),
                                               signature_reference_type='user',
                                               signature_reference_id=str(user_id))
    ret = []
    for signature in signatures:
        if signature_type is not None:
            if signature_type == 'individual' and \
                    signature.get_signature_user_ccla_employee_id() is not None:
                continue
            elif signature_type == 'employee' and \
                    signature.get_signature_user_ccla_employee_id() is None:
                continue
        ret.append(signature.to_dict())
    return ret


def get_company_signatures(company_id):
    """
    Get all signatures for company.

    :param company_id: The ID of the company in question.
    :type company_id: string
    """
    signatures = Signature().get_signatures_by_reference(company_id,
                                                         'company')

    return [signature.to_dict() for signature in signatures]


def get_company_signatures_by_acl(username, company_id):
    """
    Get all signatures for company filtered by it's ACL.
    A company's signature will be returned only if the provided
    username appears in the signature's ACL.

    :param username: The username of the authenticated user
    :type username: string
    :param company_id: The ID of the company in question.
    :type company_id: string
    """
    # Get signatures by company reference
    all_signatures = Signature().get_signatures_by_reference(company_id, 'company')

    # Filter signatures this manager is authorized to see
    signatures = []
    for signature in all_signatures:
        if username in signature.get_signature_acl():
            signatures.append(signature)

    return [signature.to_dict() for signature in signatures]


def get_project_signatures(project_id):
    """
    Get all signatures for project.

    :param project_id: The ID of the project in question.
    :type project_id: string
    """
    signatures = Signature().get_signatures_by_project(str(project_id), signature_signed=True)
    return [signature.to_dict() for signature in signatures]


def get_project_company_signatures(company_id, project_id):
    """
    Get all company signatures for project specified and a company specified

    :param company_id: The ID of the company in question
    :param project_id: The ID of the project in question
    :type company_id: string
    :type project_id: string
    """
    signatures = Signature().get_signatures_by_company_project(str(company_id),
                                                               str(project_id))
    return signatures


def get_project_employee_signatures(company_id, project_id):
    """
    Get all employee signatures for project specified and a company specified

    :param company_id: The ID of the company in question 
    :param project_id: The ID of the project in question
    :type company_id: string
    :type project_id: string
    """
    signatures = Signature().get_employee_signatures_by_company_project(str(company_id),
                                                                        str(project_id))
    return signatures


def get_cla_managers(username, signature_id):
    """
    Returns CLA managers from the CCLA signature ID.

    :param username: The LF username
    :type username: string
    :param signature_id: The Signature ID of the CCLA signed.
    :type signature_id: string
    :return: dict representation of the project managers.
    :rtype: dict
    """
    signature = Signature()
    try:
        signature.load(str(signature_id))
    except DoesNotExist as err:
        return {'errors': {'signature_id': str(err)}}

    # Get Signature ACL
    signature_acl = signature.get_signature_acl()

    if username not in signature_acl:
        return {'errors': {'user_id': 'You are not authorized to see the managers.'}}

    return get_managers_dict(signature_acl)


def get_project(project_id):
    try:
        project = Project()
        project.load(project_id)
    except DoesNotExist as err:
        raise DoesNotExist('errors: {project_id: %s}' % str(err))
    return project


def get_company(company_id):
    try:
        company = Company()
        company.load(company_id)
    except DoesNotExist as err:
        raise DoesNotExist('errors: {company_id: %s}' % str(err))
    return company


def add_cla_manager_email_content(lfid, project, company, managers):
    """ Helper function to send email to newly added CLA Manager """

    # Get emails of newly added Manager
    recipients = get_user_emails(lfid)

    if not recipients:
        raise Exception('Issue getting emails for lfid : %s', lfid)

    subject = f'CLA: Access to Corporate CLA for Project {project.get_project_name()}'

    manager_list = ['%s <%s>' % (mgr.get('name', ' '), mgr.get('email', ' ')) for mgr in managers]
    manager_list_str = '-'.join(manager_list) + '\n'
    body = f"""
    <p>Hello {lfid}, </p> \
    <p>This is a notification email from EasyCLA regarding the project {project.get_project_name()}.</p> \
    <p>You have been granted access to the project {project.get_project_name()} for the organization \
       {company.get_company_name()}.</p> \
    <p> If you have further questions, please contact one of the existing CLA Managers: </p> \
    {manager_list_str}
    """
    body = '<p>' + body.replace('\n', '<br>') + '</p>'
    body = append_email_help_sign_off_content(body, project.get_version())
    return subject, body, recipients


def remove_cla_manager_email_content(lfid, project, company, managers):
    """ Helper function to send email to newly added CLA Manager """
    # Get emails of newly added Manager
    recipients = get_user_emails(lfid)

    if not recipients:
        raise Exception('Issue getting emails for lfid : %s', lfid)

    subject = f'CLA: Access to Corporate CLA for Project {project.get_project_name()}'

    manager_list = ['%s <%s>' % (mgr.get('name', ' '), mgr.get('email', ' ')) for mgr in managers]
    manager_list_str = '-'.join(manager_list) + '\n'
    body = f"""
    <p> Hello {lfid}, </p> \
    <p>This is a notification email from EasyCLA regarding the project {project.get_project_name()}.</p> \
    <p>You have been removed as a CLA Manager from the project: {project.get_project_name()} for the organization \
       {company.get_company_name()} </p> \
    <p> If you have further questions, please contact one of the existing CLA Managers: </p> \
    {manager_list_str}
    """
    body = '<p>' + body.replace('\n', '<br>') + '</p>'
    body = append_email_help_sign_off_content(body, project.get_version())
    return subject, body, recipients


def get_user_emails(lfid):
    """ Helper function that gets user emails of given lf_username """
    user = User()
    users = user.get_user_by_username(lfid)
    return [user.get_user_email() for user in users]


def add_cla_manager(auth_user: AuthUser, signature_id: str, lfid: str):
    """
    Adds the LFID to the signature ACL and returns a new list of CLA Managers.

    :param auth_user: username of the user
    :type auth_user: string
    :param signature_id: The ID of the project
    :type signature_id: UUID
    :param lfid: the lfid (manager username) to be added to the project acl
    :type lfid: string
    """

    # Find project
    signature = Signature()
    try:
        signature.load(signature_id)
    except DoesNotExist as err:
        return {'errors': {'project_id': str(err)}}

    # Get Signature ACL
    signature_acl = signature.get_signature_acl()

    if auth_user.username not in signature_acl:
        return {'errors': {'user_id': 'You are not authorized to see the managers.'}}

    company.add_permission(auth_user, lfid, signature.get_signature_reference_id(), ignore_auth_user=True)
    # Get Company and Project instances
    try:
        project = get_project(signature.get_signature_project_id())
    except DoesNotExist as err:
        return err
    try:
        company_instance = get_company(signature.get_signature_reference_id())
    except DoesNotExist as err:
        return err

    # get cla managers for email content
    managers = get_cla_managers(auth_user.username, signature_id)

    # Add lfid to acl
    signature.add_signature_acl(lfid)
    signature.save()

    # send email to newly added CLA manager
    try:
        subject, body, recipients = add_cla_manager_email_content(lfid, project, company_instance, managers)
        get_email_service().send(subject, body, recipients)
    except Exception as err:
        return {'errors': {'Failed to send email for lfid: %s , %s ' % (lfid, err)}}

    event_data = f'{lfid} added as cla manager to Signature ACL for {signature.get_signature_id()}'
    Event.create_event(
        event_data=event_data,
        event_cla_group_id=signature.get_signature_project_id(),
        event_summary=event_data,
        event_type=EventType.AddCLAManager,
        contains_pii=True,
    )

    return get_managers_dict(signature_acl)


def remove_cla_manager(username, signature_id, lfid):
    """
    Removes the LFID from the project ACL

    :param username: username of the user
    :type username: string
    :param project_id: The ID of the project
    :type project_id: UUID
    :param lfid: the lfid (manager username) to be removed to the project acl
    :type lfid: string
    """
    # Find project
    signature = Signature()
    try:
        signature.load(str(signature_id))
    except DoesNotExist as err:
        return {'errors': {'signature_id': str(err)}}

    # Validate user is the manager of the project
    signature_acl = signature.get_signature_acl()
    if username not in signature_acl:
        return {'errors': {'user': "You are not authorized to manage this CCLA."}}

    # Avoid to have an empty acl
    if len(signature_acl) == 1 and username == lfid:
        return {'errors': {'user': "You cannot remove this manager because a CCLA must have at least one CLA manager."}}
    # Remove LFID from the acl
    signature.remove_signature_acl(lfid)
    signature.save()

    # get cla managers for email content
    managers = get_cla_managers(username, signature_id)

    # Get Company and Project instances
    try:
        project = get_project(signature.get_signature_project_id())
    except DoesNotExist as err:
        return err
    try:
        company_instance = get_company(signature.get_signature_reference_id())
    except DoesNotExist as err:
        return err

    # Send email to removed CLA manager
    # send email to newly added CLA manager
    try:
        subject, body, recipients = remove_cla_manager_email_content(lfid, project, company_instance, managers)
        get_email_service().send(subject, body, recipients)
    except Exception as err:
        return {'errors': {'Failed to send email for lfid: %s , %s ' % (lfid, err)}}

    event_data = f'User with lfid {lfid} removed from project ACL with signature {signature.get_signature_id()}'

    Event.create_event(
        event_data=event_data,
        event_summary=event_data,
        event_type=EventType.RemoveCLAManager,
        event_cla_group_id=project.get_project_id(),
        contains_pii=True,
    )

    # Return modified managers
    return get_managers_dict(signature_acl)


def get_managers_dict(signature_acl):
    # Helper function to get a list of all cla managers from a CCLA Signature ACL
    # Generate managers dict
    managers_dict = []
    for lfid in signature_acl:
        user = cla.utils.get_user_instance()
        users = user.get_user_by_username(str(lfid))
        if users is not None:
            if len(users) > 1:
                cla.log.warning(f'More than one user record was returned ({len(users)}) from user '
                                f'username: {lfid} query')
            user = users[0]
            # Manager found, fill with it's information
            managers_dict.append({
                'name': user.get_user_name(),
                'email': user.get_user_email(),
                'alt_emails': user.get_user_emails(),
                'github_user_id': user.get_user_github_id(),
                'github_username': user.get_user_github_username(),
                'lfid': user.get_lf_username()
            })
        else:
            # Manager not in database yet, only set the lfid
            managers_dict.append({
                'lfid': str(lfid)
            })

    return managers_dict
