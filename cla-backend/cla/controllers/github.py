# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Controller related to the github application (CLA GitHub App).
"""
import hmac
import json
import os
import uuid
from datetime import datetime
from pprint import pprint
from typing import Optional, List

import requests

import cla
from cla.auth import AuthUser
from cla.controllers.github_application import GitHubInstallation
from cla.controllers.project import check_user_authorization
from cla.models import DoesNotExist
from cla.models.dynamo_models import Event, UserPermissions, Repository, GitHubOrg
from cla.utils import get_github_organization_instance, get_repository_service, get_oauth_client, get_email_service, \
    get_email_sign_off_content, get_project_instance, append_email_help_sign_off_content
from cla.models.event_types import EventType


def get_organizations():
    """
    Returns a list of github organizations in the CLA system.

    :return: List of github organizations in dict format.
    :rtype: [dict]
    """
    return [github_organization.to_dict() for github_organization in get_github_organization_instance().all()]


def get_organization(organization_name):
    """
    Returns the CLA github organization requested by Name.

    :param organization_name: The github organization Name.
    :type organization_name: Name
    :return: dict representation of the github organization object.
    :rtype: dict
    """
    github_organization = get_github_organization_instance()
    try:
        cla.log.debug(f'Loading GitHub by organization name: {organization_name}..')
        org = github_organization.get_organization_by_lower_name(organization_name)
        cla.log.debug(f'Loaded GitHub by organization name: {org}')
    except DoesNotExist as err:
        cla.log.warning(f'organization name {organization_name} does not exist')
        return {'errors': {'organization_name': str(err)}}
    return org.to_dict()


def get_organization_model(organization_name) -> Optional[GitHubOrg]:
    """
    Returns a GitHubOrg model based on the the CLA github organization name.

    :param organization_name: The github organization name.
    :type organization_name: str
    :return: model representation of the github organization object.
    :rtype: GitHubOrg
    """
    github_organization = get_github_organization_instance()
    try:
        cla.log.debug(f'Loading GitHub by organization name: {organization_name}..')
        org = github_organization.get_organization_by_lower_name(organization_name)
        cla.log.debug(f'Loaded GitHub by organization name: {org}')
        return org
    except DoesNotExist as err:
        cla.log.warning(f'organization name {organization_name} does not exist, error: {err}')
        return None


def create_organization(auth_user, organization_name, organization_sfid):
    """
    Creates a github organization and returns the newly created github organization in dict format.

    :param auth_user: authorization for this user.
    :type auth_user: AuthUser
    :param organization_name: The github organization name.
    :type organization_name: string
    :param organization_sfid: The SFDC ID for the github organization.
    :type organization_sfid: string/None
    :return: dict representation of the new github organization object.
    :rtype: dict
    """
    # Validate user is authorized for this SFDC ID.
    can_access = check_user_authorization(auth_user, organization_sfid)
    if not can_access['valid']:
        return can_access['errors']

    github_organization = get_github_organization_instance()
    try:
        github_organization.load(str(organization_name))
    except DoesNotExist:
        cla.log.debug('creating organization: {} with sfid: {}'.format(organization_name, organization_sfid))
        github_organization.set_organization_name(str(organization_name))
        github_organization.set_organization_sfid(str(organization_sfid))
        github_organization.set_project_sfid(str(organization_sfid))
        github_organization.save()
        return github_organization.to_dict()

    cla.log.warning('organization already exists: {} - unable to create'.format(organization_name))
    return {'errors': {'organization_name': 'This organization already exists'}}


def update_organization(organization_name,  # pylint: disable=too-many-arguments
                        organization_sfid=None,
                        organization_installation_id=None):
    """
    Updates a github organization and returns the newly updated org in dict format.
    Values of None means the field will not be updated.

    :param organization_name: The github organization name.
    :type organization_name: string
    :param organization_sfid: The SFDC identifier ID for the organization.
    :type organization_sfid: string/None
    :param organization_installation_id: The github app installation id.
    :type organization_installation_id: string/None
    :return: dict representation of the new github organization object.
    :rtype: dict
    """

    github_organization = get_github_organization_instance()
    try:
        github_organization.load(str(organization_name))
    except DoesNotExist as err:
        cla.log.warning('organization does not exist: {} - unable to update'.format(organization_name))
        return {'errors': {'repository_id': str(err)}}

    github_organization.set_organization_name(organization_name)
    if organization_installation_id:
        github_organization.set_organization_installation_id(organization_installation_id)
    if organization_sfid:
        github_organization.set_organization_sfid(organization_sfid)

    github_organization.save()
    cla.log.debug('updated organization: {}'.format(organization_name))
    return github_organization.to_dict()


def delete_organization(auth_user, organization_name):
    """
    Deletes a github organization based on Name.

    :param organization_name: The Name of the github organization.
    :type organization_name: Name
    """
    # Retrieve SFDC ID for this organization
    github_organization = get_github_organization_instance()
    try:
        github_organization.load(str(organization_name))
    except DoesNotExist as err:
        cla.log.warning('organization does not exist: {} - unable to delete'.format(organization_name))
        return {'errors': {'organization_name': str(err)}}

    organization_sfid = github_organization.get_organization_sfid()

    # Validate user is authorized for this SFDC ID.
    can_access = check_user_authorization(auth_user, organization_sfid)
    if not can_access['valid']:
        return can_access['errors']

    # Find all repositories that are under this organization
    repositories = Repository().get_repositories_by_organization(organization_name)
    for repository in repositories:
        repository.delete()
    github_organization.delete()
    return {'success': True}


def user_oauth2_callback(code, state, request):
    github = get_repository_service('github')
    return github.oauth2_redirect(state, code, request)


def user_authorization_callback(body):
    return {'status': 'nothing to do here.'}


def get_org_name_from_installation_event(body: dict) -> Optional[str]:
    """
    Attempts to extract the organization name from the GitHub installation event.

    :param body: the github installation created event body
    :return: returns either the organization name or None
    """
    try:
        # Webhook event payload
        # see: https://developer.github.com/v3/activity/events/types/#webhook-payload-example-12
        cla.log.debug('Looking for github organization name at path: installation.account.login...')
        return body['installation']['account']['login']
    except KeyError:
        cla.log.warning('Unable to grab organization name from github installation event path: '
                        'installation.account.login - looking elsewhere...')

    try:
        # some installation created events include the organization in this path
        cla.log.debug('Looking for github organization name at alternate path: organization.login...')
        return body['organization']['login']
    except KeyError:
        cla.log.warning('Unable to grab organization name from github installation event path: '
                        'organization.login - looking elsewhere...')

    try:
        # some installation created events include the organization in this path
        cla.log.debug('Looking for github organization name at alternate path: repository.owner.login...')
        return body['repository']['owner']['login']
    except KeyError:
        cla.log.warning('Unable to grab organization name from github installation event path: '
                        'repository.owner.login - giving up...')
        return None


def get_github_activity_action(body: dict) -> Optional[str]:
    """
    Returns the action value from the github activity event.

    :param body: the GitHub webhook body payload
    :type body: dict
    :return: a string representing the action, or None if it couldn't find the action value
    """
    cla.log.debug(f'locating action attribute in body: {body}')
    try:
        return body['action']
    except KeyError:
        return None


def activity(action: str, event_type: str, body: dict):
    """
    Processes the GitHub activity event.
    :param action: the action value
    :type action: str
    :param event_type: the event type string value
    :type event_type: str
    :param body: the webhook body payload
    :type body: dict
    """
    fn = 'github.activity'
    cla.log.debug(f'{fn} - received github activity event of type: {event_type}')

    if action is None:
        cla.log.warning(f'{fn} - unable to determine action type from body: {json.dumps(body)}. '
                        'Unable to process this request.')
        return

    cla.log.debug(f'{fn} - received github activity event, action: {action}...')

    # If we have the GitHub debug flag set/on...
    if bool(os.environ.get('GH_APP_DEBUG', '')):
        cla.log.debug(f'{fn} - body: {json.dumps(body)}')

    # From GitHub: Starting October 1st, 2020 - We no longer support two events which your GitHub Apps may rely on,
    #   "integration_installation" and
    #   "integration_installation_repositories".
    #
    # These events can be replaced with the:
    #   "installation" and
    #   "installation_repositories"
    #
    # events respectively.
    # see:
    # https://docs.github.com/en/developers/webhooks-and-events/webhook-events-and-payloads#installation_repositories

    # GitHub Application Installation Event
    if event_type == 'installation' or event_type == 'integration_installation':
        handle_installation_event(action, body)

    # Note: The GitHub event type: 'integration_installation_repositories' is being deprecated on October 1st, 2020
    # in favor of 'installation_repositories' - for now we will support both...payload is the same
    # Event details: https://developer.github.com/webhooks/event-payloads/#installation_repositories
    elif event_type == 'installation_repositories' or event_type == 'integration_installation_repositories':
        handle_installation_repositories_event(action, body)

    # GitHub Pull Request Event
    elif event_type == 'pull_request':
        handle_pull_request_event(action, body)

    elif event_type == "issue_comment":
        cla.log.debug(f'{fn} - received issue_comment action: {action}...')
        handle_pull_request_comment_event(action, body)

    else:
        cla.log.debug(f'{fn} - ignoring github activity event, action: {action}...')


def handle_installation_event(action: str, body: dict):
    func_name = 'github.activity.handle_installation_event'
    cla.log.debug(f'{func_name} - processing github [installation] activity callback...')

    # New Installations
    if action == 'created':
        cla.log.debug(f'{func_name} - processing github installation activity for action: {action}')

        org_name = get_org_name_from_installation_event(body)
        if org_name is None:
            cla.log.warning(f'{func_name} - Unable to determine organization name from the github installation event '
                            f'with action: {action}'
                            f'event body: {json.dumps(body)}')
            return {'status', f'GitHub installation {action} event malformed.'}

        cla.log.debug(f'Locating organization using name: {org_name}')
        existing = get_organization(org_name)
        if 'errors' in existing:
            cla.log.warning(f'{func_name} - Received github installation created event for organization: {org_name}, '
                            'but the organization is not configured in EasyCLA')
            # TODO: Need a way of keeping track of new organizations that don't have projects yet.
            return {'status': 'Github Organization must be created through the Project Management Console.'}
        elif not existing['organization_installation_id']:
            update_organization(
                existing['organization_name'],
                existing['organization_sfid'],
                body['installation']['id'],
            )
            cla.log.info(f'{func_name} - Organization enrollment completed: {existing["organization_name"]}')
            return {'status': 'Organization Enrollment Completed. CLA System is operational'}
        else:
            cla.log.info(f'{func_name} - Organization already enrolled: {existing["organization_name"]}')
            cla.log.info(f'{func_name} - Updating installation ID for '
                         f'github organization: {existing["organization_name"]}')
            update_organization(
                existing['organization_name'],
                existing['organization_sfid'],
                body['installation']['id'],
            )
            return {'status': 'Already Enrolled Organization Updated. CLA System is operational'}

    elif action == 'deleted':
        cla.log.debug(f'{func_name} - processing github installation activity for action: {action}')
        org_name = get_org_name_from_installation_event(body)
        if org_name is None:
            cla.log.warning('Unable to determine organization name from the github installation event '
                            f'with action: {action}'
                            f'event body: {json.dumps(body)}')
            return {'status', f'GitHub installation {action} event malformed.'}
        repositories = Repository().get_repositories_by_organization(org_name)
        notify_project_managers(repositories)
        return
    else:
        cla.log.debug(f'{func_name} - ignoring github installation activity for action: {action}')


def handle_pull_request_event(action: str, body: dict):
    func_name = 'github.activity.handle_pull_request_event'
    cla.log.debug(f'{func_name} - processing github pull_request activity callback...')

    # New PR opened
    if action == 'opened' or action == 'reopened' or action == 'synchronize' or action == 'enqueued':
        cla.log.debug(f'{func_name} - processing github pull_request activity for action: {action}')
        # Copied from repository_service.py
        service = cla.utils.get_repository_service('github')
        result = service.received_activity(body)
        return result
    else:
        cla.log.debug(f'{func_name} - ignoring github pull_request activity for action: {action}')


def handle_pull_request_comment_event(action: str, body: dict):
    func_name = 'github.activity.handle_pull_request_comment_event'
    cla.log.debug(f'{func_name} - processing github pull_request comment activity callback...')

    # New comment created or edited
    if action == 'created' or action == 'edited':
        cla.log.debug(f'{func_name} - processing github pull_request comment activity for action: {action}')
        service = cla.utils.get_repository_service('github')
        try:
            result = service.process_easycla_command_comment(body)
            return result
        except ValueError as ex:
            cla.log.debug(f'{func_name} - ignoring github pull_request comment: {str(ex)}')
            return None
    else:
        cla.log.debug(f'{func_name} - ignoring github pull_request comment activity for action: {action}')


def handle_installation_repositories_event(action: str, body: dict):
    func_name = 'github.activity.handle_installation_repositories_event'
    if action == 'added':
        handle_installation_repositories_added_event(action, body)
    elif action == 'removed':
        handle_installation_repositories_removed_event(action, body)
    else:
        cla.log.info(f'{func_name} - unhandled action type: {action} - ignoring')


def handle_installation_repositories_added_event(action: str, body: dict):
    func_name = 'github.activity.handle_installation_repositories_added_event'
    # Who triggered the event
    user_login = body['sender']['login']
    cla.log.debug(f'{func_name} - processing github [installation_repositories] '
                  f'activity {action} callback created by GitHub user {user_login}.')
    # Grab the list of repositories added from the event model
    repository_added = body.get('repositories_added', [])
    # Create a unique list of repositories for the email that we need to send out
    repository_list = set([repo.get('full_name', None) for repo in repository_added])
    # All the repos in the message should be under the same GitHub Organization
    organization_name = ''
    for repo in repository_added:
        # Grab the information
        repository_external_id = repo['id']  # example: 271841254
        repository_name = repo['name']  # example: PyImath
        repository_full_name = repo['full_name']  # example: AcademySoftwareFoundation/PyImath
        organization_name = repository_full_name.split('/')[0]  # example: AcademySoftwareFoundation
        # repository_private = repo['private']      # example: False

        # Lookup the GitHub Organization in our table - should be there already
        cla.log.debug(f'{func_name} - Locating organization using name: {organization_name}')
        org_model = get_organization_model(organization_name)

        if org_model is None:
            # Should we create since it's missing?
            cla.log.warning(f'Unable to locate GitHub Organization {organization_name} in our database')
            continue

        # Should we update to ensure the installation_id is set?
        if org_model.get_organization_installation_id() is None:
            # Update the installation ID
            org_model.set_organization_installation_id(body.get('installation', {}).get('id', None))
            org_model.save()

        # Check to see if the auto enabled flag is set
        if org_model.get_auto_enabled():
            # We need to check that we only have 1 CLA Group - auto-enable only works when the entire
            # Organization falls under a single CLA Group - otherwise, how would we know which CLA Group
            # to add them to? First we query all the existing repositories associated with this Github Org -
            # they should all point the the single CLA Group - let's verify this...
            existing_github_repositories = Repository().get_repositories_by_organization(organization_name)
            cla_group_ids = set(())  # hoping for only 1 unique value - set collection discards duplicates
            cla_group_repo_sfids = set(())  # keep track of the existing SFDC IDs from existing repos
            for existing_repo in existing_github_repositories:
                cla_group_ids.add(existing_repo.get_repository_project_id())
                cla_group_repo_sfids.add(existing_repo.get_repository_sfdc_id())

            # We should only have one...
            if len(cla_group_ids) != 1 or len(cla_group_repo_sfids) != 1:
                cla.log.warning(f'{func_name} - Auto Enabled set for Organization {organization_name}, '
                                f'but we found repositories or SFIDs that belong to multiple CLA Groups. '
                                'Auto Enable only works when all repositories under a given '
                                'GitHub Organization are associated with a single CLA Group. This '
                                f'organization is associated with {len(cla_group_ids)} CLA Groups and '
                                f'{len(cla_group_repo_sfids)} SFIDs.')
                return

            cla_group_id = cla_group_ids.pop()

            project_model = get_project_instance()
            try:
                project_model.load(project_id=cla_group_id)
            except DoesNotExist as err:
                cla.log.warning(f'{func_name} - unable to load project (cla_group) by '
                                f'project_id: {cla_group_id}, error: {err}')

            cla.log.debug(f'{func_name} - Organization {organization_name} has auto_enabled set - '
                          f'adding repository: {repository_name} to '
                          f'CLA Group: {project_model.get_project_name()}')
            try:
                # Create the new repository entry and associate it with the CLA Group
                new_repository = Repository(
                    repository_id=str(uuid.uuid4()),
                    repository_project_id=cla_group_id,
                    repository_name=repository_full_name,
                    repository_type='github',
                    repository_url='https://github.com/' + repository_full_name,
                    repository_organization_name=organization_name,
                    repository_external_id=repository_external_id,
                    repository_sfdc_id=cla_group_repo_sfids.pop(),
                )
                new_repository.set_enabled(True)
                new_repository.save()

                # Log the event
                msg = (f'Adding repository {repository_full_name} '
                       f'from GitHub organization : {organization_name} '
                       f'with URL: https://github.com/{repository_full_name} '
                       'to the CLA configuration. GitHub organization was set to auto-enable.')
                Event.create_event(
                    event_type=EventType.RepositoryAdded,
                    event_cla_group_id=cla_group_id,
                    event_company_id=None,
                    event_data=msg,
                    event_summary=msg,
                    event_user_id=user_login,
                    contains_pii=False,
                )
            except Exception as err:
                cla.log.warning(f'{func_name} - Could not create GitHub repository: {err}')
                return

        else:
            cla.log.debug(f'{func_name} - Auto enabled NOT set for GitHub Organization {organization_name} - '
                          f'not auto-adding repository: {repository_full_name}')
            return

    # Notify the Project Managers
    notify_project_managers_auto_enabled(organization_name, repository_list)


def handle_installation_repositories_removed_event(action: str, body: dict):
    func_name = 'github.activity.handle_installation_repositories_removed_event'
    # Who triggered the event
    user_login = body['sender']['login']
    cla.log.debug(f'{func_name} - processing github [installation_repositories] '
                  f'activity {action} callback created by GitHub user {user_login}.')
    repository_removed = body['repositories_removed']
    repositories = []
    for repo in repository_removed:
        repository_external_id = repo['id']
        ghrepo = Repository().get_repository_by_external_id(repository_external_id, 'github')
        if ghrepo is not None:
            repositories.append(ghrepo)

    # Notify the Project Managers that the following list of repositories were removed
    notify_project_managers(repositories)

    # The following list of repositories were deleted/removed from GitHub - we need to remove
    # the repo entry from our repos table
    for repo in repositories:

        project_model = get_project_instance()
        try:
            project_model.load(project_id=repo.get_repository_project_id())
        except DoesNotExist as err:
            cla.log.warning(f'{func_name} - unable to load project (cla_group) by '
                            f'project_id: {repo.get_repository_project_id()}, error: {err}')

        msg = (f'Disabling repository {repo.get_repository_name()} '
               f'from GitHub organization : {repo.get_repository_organization_name()} '
               f'with URL: {repo.get_repository_url()} '
               'from the CLA configuration.')
        cla.log.debug(msg)
        # Disable the repo and add a note
        repo.set_enabled(False)
        repo.add_note(f'{datetime.now()}  - Disabling repository due to '
                      'GitHub installation_repositories delete event '
                      f'for CLA Group {project_model.get_project_name()}')
        repo.save()

        # Log the event
        Event.create_event(
            event_type=EventType.RepositoryDisable,
            event_cla_group_id=repo.get_repository_project_id(),
            event_company_id=None,
            event_data=msg,
            event_summary=msg,
            event_user_id=user_login,
            contains_pii=False,
        )


def notify_project_managers(repositories):
    if repositories is None:
        return

    project_repos = {}
    for ghrepo in repositories:
        project_id = ghrepo.get_repository_project_id()
        if project_id in project_repos:
            project_repos[project_id].append(ghrepo.get_repository_url())
        else:
            project_repos[project_id] = [ghrepo.get_repository_url()]

    for project_id in project_repos:
        managers = cla.controllers.project.get_project_managers("", project_id, enable_auth=False)
        project = get_project_instance()
        try:
            project.load(project_id=str(project_id))
        except DoesNotExist as err:
            cla.log.warning('notify_project_managers - unable to load project (cla_group) by '
                            f'project_id: {project_id}, error: {err}')
            return {'errors': {'project_id': str(err)}}
        repositories = project_repos[project_id]
        subject, body, recipients = unable_to_do_cla_check_email_content(
            project, managers, repositories)
        get_email_service().send(subject, body, recipients)
        cla.log.debug('github.activity - sending unable to perform CLA Check email'
                      f' to managers: {recipients}'
                      f' for project {project} with '
                      f' repositories: {repositories}')


def unable_to_do_cla_check_email_content(project, managers, repositories):
    """Helper function to get unable to do cla check email subject, body, recipients"""
    cla_group_name = project.get_project_name()
    subject = f'EasyCLA: Unable to check GitHub Pull Requests for CLA Group: {cla_group_name}'
    pronoun = "this repository"
    if len(repositories) > 1:
        pronoun = "these repositories"

    repo_content = "<ul>"
    for repo in repositories:
        repo_content += "<li>" + repo + "</li>"
    repo_content += "</ul>"

    body = f"""
    <p>Hello Project Manager,</p>
    <p>This is a notification email from EasyCLA regarding the CLA Group {cla_group_name}.</p>
    <p>EasyCLA is unable to check PRs on {pronoun} due to permissions issue.</p>
    {repo_content}
    <p>Please contact the repository admin/owner to enable CLA checks.</p>
    <p>Provide the Owner/Admin the following instructions:</p>
    <ul>
    <li>Go into the "Settings" tab of the GitHub Organization</li>
    <li>Click on "installed GitHub Apps" vertical navigation</li>
    <li>Then click "Configure" associated with the EasyCLA App</li>
    <li>Finally, click the "All Repositories" radio button option</li>
    </ul>
    """
    body = append_email_help_sign_off_content(body, project.get_version())
    # body = '<p>' + body.replace('\n', '<br>') + '</p>'
    recipients = []
    for manager in managers:
        recipients.append(manager["email"])
    return subject, body, recipients


def notify_project_managers_auto_enabled(organization_name, repositories):
    if repositories is None:
        return

    project_repos = {}
    for repo in repositories:
        project_id = repo.get_repository_project_id()
        if project_id in project_repos:
            project_repos[project_id].append(repo.get_repository_url())
        else:
            project_repos[project_id] = [repo.get_repository_url()]

    for project_id in project_repos:
        managers = cla.controllers.project.get_project_managers("", project_id, enable_auth=False)
        project = get_project_instance()
        try:
            project.load(project_id=str(project_id))
        except DoesNotExist as err:
            cla.log.warning('notify_project_managers_auto_enabled - unable to load project (cla_group) by '
                            f'project_id: {project_id}, error: {err}')
            return {'errors': {'project_id': str(err)}}

        repositories = project_repos[project_id]
        subject, body, recipients = auto_enabled_repository_email_content(
            project, managers, organization_name, repositories)
        get_email_service().send(subject, body, recipients)
        cla.log.debug('notify_project_managers_auto_enabled - sending auto-enable email '
                      f' to managers: {recipients}'
                      f' for project {project} for '
                      f' GitHub Organization {organization_name} with '
                      f' repositories: {repositories}')


def auto_enabled_repository_email_content(project, managers, organization_name, repositories):
    """Helper function to update managers about auto-enabling of repositories"""
    cla_group_name = project.get_project_name()
    subject = f'EasyCLA: Auto-Enable Repository for CLA Group: {cla_group_name}'
    repo_pronoun_upper = "Repository"
    repo_pronoun = "repository"
    pronoun = "this " + repo_pronoun
    repo_was_were = repo_pronoun + " was"
    if len(repositories) > 1:
        repo_pronoun_upper = "Repositories"
        repo_pronoun = "repositories"
        pronoun = "these " + repo_pronoun
        repo_was_were = repo_pronoun + " were"

    repo_content = "<ul>"
    for repo in repositories:
        repo_content += "<li>" + repo + "<ul>"
    repo_content += "</ul>"

    body = f"""
    <p>Hello Project Manager,</p>
    <p>This is a notification email from EasyCLA regarding the CLA Group {cla_group_name}.</p>
    <p>EasyCLA was notified that the following {repo_was_were} added to the {organization_name} GitHub Organization.\
    Since auto-enable was configured within EasyCLA for GitHub Organization, the {pronoun} will now start enforcing \
    CLA checks.</p>
    <p>Please verify the repository settings to ensure EasyCLA is a required check for merging Pull Requests. \
    See: GitHub Repository -> Settings -> Branches -> Branch Protection Rules -> Add/Edit the default branch, \
    and confirm that 'Require status checks to pass before merging' is enabled and that EasyCLA is a required check.\
    Additionally, consider selecting the 'Include administrators' option to enforce all configured restrictions for \
    contributors, maintainers, and administrators.</p>
    <p>For more information on how to setup GitHub required checks, please consult the About required status checks\
    <a href="https://docs.github.com/en/github/administering-a-repository/about-required-status-checks"> \
    in the GitHub Online Help Pages</a>.</p>
    <p>{repo_pronoun_upper}:</p>
    {repo_content}
    """
    body = '<p>' + body.replace('\n', '<br>') + '</p>'
    body = append_email_help_sign_off_content(body, project.get_version())

    recipients = []
    for manager in managers:
        recipients.append(manager["email"])
    return subject, body, recipients


def get_organization_repositories(organization_name):
    github_organization = get_github_organization_instance()
    try:
        github_organization.load(str(organization_name))
        if github_organization.get_organization_installation_id() is not None:
            cla.log.debug('GitHub Organization ID: {}'.format(github_organization.get_organization_installation_id()))
            try:
                installation = GitHubInstallation(github_organization.get_organization_installation_id())
            except Exception as e:
                msg = ('Unable to load repositories from organization: {} ({}) due to GitHub '
                       'installation permission problem or other issue, error: {} - returning error response'.
                       format(organization_name, github_organization.get_organization_installation_id(), e))
                cla.log.warn(msg)
                return {'errors': {'organization_name': organization_name, 'error': msg}}

            if installation.repos:
                repos = []
                for repo in installation.repos:
                    repos.append(repo.full_name)
                return repos
            else:
                cla.log.debug('No repositories found for Github installation id: {}'.
                              format(github_organization.get_organization_installation_id()))
                return []
    except DoesNotExist as err:
        cla.log.warning('organization name {} does not exist, error: {}'.format(organization_name, err))
        return {'errors': {'organization_name': organization_name, 'error': str(err)}}


def get_organization_by_sfid(auth_user: AuthUser, sfid):
    # Check if user has permissions
    user_permissions = UserPermissions()
    try:
        user_permissions.load(auth_user.username)
    except DoesNotExist as err:
        cla.log.warning('user {} does not exist, error: {}'.format(auth_user.username, err))
        return {'errors': {'user does not exist': str(err)}}

    user_permissions_json = user_permissions.to_dict()

    authorized_projects = user_permissions_json.get('projects')
    if sfid not in authorized_projects:
        cla.log.warning('user {} is not authorized for this Salesforce ID: {}'.
                        format(auth_user.username, sfid))
        return {'errors': {'user is not authorized for this Salesforce ID.': str(sfid)}}

    # Get all organizations under an SFDC ID
    try:
        organizations = get_github_organization_instance().get_organization_by_sfid(sfid)
    except DoesNotExist as err:
        cla.log.warning('sfid {} does not exist, error: {}'.format(sfid, err))
        return {'errors': {'sfid': str(err)}}
    return [organization.to_dict() for organization in organizations]


def org_is_covered_by_cla(owner):
    orgs = get_organizations()
    for org in orgs:
        # Org urls have to match and full enrollment has to be completed.
        if org['organization_name'] == owner and \
                org['organization_project_id'] and \
                org['organization_installation_id']:
            cla.log.debug('org: {} with project id: {} is covered by cla'.
                          format(org['organization_name'], org['organization_project_id']))
            return True

    cla.log.debug('org: {} is not covered by cla'.format(owner))
    return False


def validate_organization(body):
    if 'endpoint' in body and body['endpoint']:
        endpoint = body['endpoint']
        r = requests.get(endpoint)

        if r.status_code == 200:
            if "http://schema.org/Organization" in r.content.decode('utf-8'):
                return {"status": "ok"}
            else:
                return {"status": "invalid"}
        elif r.status_code == 404:
            return {"status": "not found"}
        else:
            return {"status": "error"}


def webhook_secret_validation(webhook_signature: str, data: bytes) -> bool:
    """
    webhook_secret_validation checks if webhook_signature is same as incoming data's
    :param webhook_signature:
    :param data:
    :return:
    """
    fn = 'webhook_secret_validation'
    cla.log.debug(f'{fn} for signature {webhook_signature}')
    if cla.config.GITHUB_APP_WEBHOOK_SECRET == "":
        cla.log.warning(f'{fn} - GITHUB_APP_WEBHOOK_SECRET is empty - unable to validate webhook secret')
        raise RuntimeError("GITHUB_APP_WEBHOOK_SECRET is empty")

    if not webhook_signature:
        cla.log.warning(f'{fn} - webhook_signature not provided - unable to validate webhook callback')
        return False

    sha_name, signature = webhook_signature.split('=')
    if not sha_name == 'sha1':
        cla.log.warning(f'{fn} - unsupported sha_name: \'{sha_name}\' - unable to validate webhook callback')
        return False

    cla.log.debug(f'{fn} - calculating and comparing webhook secret...')
    mac = hmac.new(cla.config.GITHUB_APP_WEBHOOK_SECRET.encode('utf-8'), msg=data, digestmod='sha1')
    hex_digest = mac.hexdigest()
    return True if hmac.compare_digest(hex_digest, signature.strip()) else False


def webhook_secret_failed_email_content(event_type: str, req_body: dict, maintainers: List[str]):
    """Helper function to update maintainers about failed webhook secrets"""
    if not maintainers:
        cla.log.warning("webhook_secret_failed_email - maintainers list is empty can't send the email.")
        raise RuntimeError("no maintainers set")

    user_login = req_body.get('sender', {}).get('login', None)
    repository_id = req_body.get('repository', {}).get('id', None)
    repository_name = req_body.get('repository', {}).get('full_name', None)
    repository_owner = req_body.get('repository', {}).get('owner', {}).get('login', None)
    repository_url = req_body.get('repository', {}).get('html_url', None)
    parent_org = req_body.get('repository', {}).get('organization', {}).get('login', None)
    installation_id = req_body.get('installation', {}).get('id', None)
    msg = f"""<li>stage: {cla.config.stage}</li>
    <li>event type: {event_type}</li>
    <li>user login: {user_login}</li>
    <li>repository id: {repository_id}</li>
    <li>repository name: {repository_name}</li>
    <li>repository owner: {repository_owner}</li>
    <li>repository url: {repository_url}</li>
    <li>parent organization: {parent_org}</li>
    <li>installation_id: {installation_id}</li>"""

    body = f"""
    <p>Hello EasyCLA Maintainer,</p>
    <p>This is a notification email from EasyCLA regarding failure of webhook secret validation.</p>
    <p>Validation Failed:</p>
    <ul>{msg}</ul>
    <p>Please verify the EasyCLA settings to ensure EasyCLA webhook secret is set correctly. \
    See: <a href="https://github.com/organizations/LF-Engineering/settings/apps"> EasyCLA app settings</a>. \
    <p>For more information on how to setup GitHub webhook secret, please consult About Securing Your Webhooks\
    <a href="https://docs.github.com/en/free-pro-team@latest/developers/webhooks-and-events/securing-your-webhooks"> \
    in the GitHub Online Help Pages</a>.</p>
    {get_email_sign_off_content()}
    """

    subject = f'EasyCLA: Webhook Secret Failure'
    body = '<p>' + body.replace('\n', '<br>') + '</p>'
    return subject, body, maintainers


def webhook_secret_failed_email(event_type: str, req_body: dict, maintainers: List[str]):
    """
    sends the notification email for the failing webhook secret validation
    :param event_type:
    :param req_body:
    :param maintainers:
    :return:
    """
    if maintainers is None or type(maintainers) is not list:
        cla.log.warning(f'webhook_secret_failed_email - unable to emails - no maintainers defined.')
        return

    subject, body, maintainers = webhook_secret_failed_email_content(event_type, req_body, maintainers)
    get_email_service().send(subject, body, maintainers)
    cla.log.debug('webhook_secret_failed_email - sending notification email '
                  f' to maintainers: {maintainers}')


def check_namespace(namespace):
    """
    Checks if the namespace provided is a valid GitHub organization.

    :param namespace: The namespace to check.
    :type namespace: string
    :return: Whether or not the namespace is valid.
    :rtype: bool
    """
    oauth = get_oauth_client()
    response = oauth.get('https://api.github.com/users/' + namespace)
    return response.ok


def get_namespace(namespace):
    """
    Gets info on the GitHub account/organization provided.

    :param namespace: The namespace to get.
    :type namespace: string
    :return: Dict of info on the account in question.
    :rtype: dict
    """
    oauth = get_oauth_client()
    response = oauth.get('https://api.github.com/users/' + namespace)
    if response.ok:
        return response.json()
    else:
        return {'errors': {'namespace': 'Invalid GitHub account namespace'}}
