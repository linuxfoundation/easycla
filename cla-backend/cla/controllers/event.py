# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

import uuid
import json
from datetime import datetime
from functools import wraps

import hug.types
from falcon import HTTP_200, HTTP_400, HTTP_404, HTTPError

import cla
from cla.auth import AuthUser, admin_list
from cla.models import DoesNotExist
from cla.utils import get_event_instance, audit_event




def events(request, response=None):
    """
    Returns a list of events in the CLA system.
    if parameters are passed returns filtered lists

    :return: List of events in dict format
    """

    event = get_event_instance()
    events = [event.to_dict() for event in event.all()]
    if request.params:
        results = event.search_events(**request.params)
        if results:
            events = [ev.to_dict() for ev in results]
        else:
            # return empty list if search fails
            response.status = HTTP_404
            return {"events": []}

    return {"events": events}


def get_event(event_id=None, response=None):
    """
    Returns an event given an event_id

    :param event_id: The event's ID
    :type event_id: string
    :rtype: dict
    """
    if event_id is not None:
        event = get_event_instance()
        try:
            event.load(event_id)
        except DoesNotExist as err:
            response.status = HTTP_404
            return {"errors": {"event_id": str(err)}}
        return event.to_dict()
    else:
        response.status = HTTP_404
        return {"errors": "Id is not passed"}


def create_event(
    response=None,
    event_type=None,
    event_company_id=None,
    event_data=None,
    event_project_id=None,
    user_id=None,
):
    """
    Creates a new event in the CLA system.

    :param event_type: The type of the event
    :param event_company_id: The company ID associated with the event
    :param event_data: The event data
    :param event_project_id: The project ID associated with the event
    :param user_id: The user ID associated with the event
    :param response: The HTTP response object
    :return: Created event in dict format
    """
    try:
        event = get_event_instance()
        event_id = str(uuid.uuid4())
        event.set_event_id(event_id)

        if event_type:
            event.set_event_type(event_type)
        if event_data:
            event.set_event_data(event_data)
        if event_project_id:
            event.set_event_project_id(event_project_id)
        if event_company_id:
            event.set_event_company_id(event_company_id)
        if user_id:
            event.set_event_user_id(user_id)

        # Set the event date and PII flag
        event.set_event_date_and_contains_pii(contains_pii=False)
        event.save()

        return event.to_dict()
    except Exception as err:
        response.status = HTTP_400
        return {"errors": {"create_event": str(err)}}

