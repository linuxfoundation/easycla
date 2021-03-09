# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Utility functions for the models
"""
from uuid import UUID


def is_uuidv4(uuid_string: str) -> bool:
    """
    Helper function for determining if the specified string is a UUID v4 value.
    :param uuid_string: the string representing a UUID
    :return: True if the specified string is a UUID v4 value, False otherwise
    """
    try:
        UUID(uuid_string, version=4)
        return True
    except TypeError:
        # If it's a value error, then the string is not a valid UUID.
        return False
    except ValueError:
        # If it's a value error, then the string is not a valid UUID.
        return False
