# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Test python API changes for Signature and User Tables
"""
from unittest.mock import patch, MagicMock

import pytest

from cla.models.dynamo_models import SignatureModel, UserModel
from cla.utils import get_user_instance, get_signature_instance, get_company_instance
from cla import utils
from cla.tests.unit.data import USER_TABLE_DATA

from pynamodb.indexes import AllProjection
import cla

PATCH_METHOD = "pynamodb.connection.Connection._make_api_call"


def test_user_external_id(user_instance):
    assert "user external id: bar" in str(user_instance)


def test_company_external_id(company_instance):
    assert "external id: external id" in str(company_instance)


def test_github_user_external_id_index():
    assert UserModel.github_user_external_id_index.query("foo")


def test_project_signature_external_id_index():
    assert SignatureModel.project_signature_external_id_index.query("foo")


def test_signature_company_signatory_index():
    assert SignatureModel.signature_company_signatory_index.query("foo")


def test_signature_company_initial_manager_index():
    assert SignatureModel.signature_company_initial_manager_index.query("foo")
