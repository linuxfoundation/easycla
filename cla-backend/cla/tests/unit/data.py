# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

import os

stage = os.environ.get("STAGE", "")

USER_TABLE_DATA = {
    "Table": {
        "AttributeDefinitions": [
            {"AttributeName": "user_id", "AttributeType": "S"},
            {"AttributeName": "user_external_id", "AttributeType": "S"},
        ],
        "ItemCount": 0,
        "KeySchema": [{"AttributeName": "user_id", "KeyType": "HASH"}],
        "GlobalSecondaryIndexes": [
            {
                "IndexName": "github-user-external-id-index",
                "KeySchema": [{"AttributeName": "user_external_id", "KeyType": "HASH"}],
                "Projection": {"ProjectionType": "ALL"},
                "ProvisionedThroughput": {"ReadCapacityUnits": 1, "WriteCapacityUnits": 1},
            }
        ],
        "ProvisionedThroughput": {"ReadCapacityUnits": 1, "WriteCapacityUnits": 1},
    },
    "TableName" : "cla-{}-users".format(stage)
}

COMPANY_TABLE_DATA = {
    "Table": {
        "AttributeDefinitions": [{"AttributeName": "company_id", "AttributeType": "S"}],
        "KeySchema": [{"AttributeName": "company_id", "KeyType": "HASH"}],
        "GlobalSecondaryIndexes": [
            {
                "IndexName": "company-name-index",
                "KeySchema": [{"AttributeName": "company_name", "KeyType": "HASH"}],
                "Projection": {"ProjectionType": "ALL"},
                "ProvisionedThroughput": {"ReadCapacityUnits": 1, "WriteCapacityUnits": 1},
            }
        ],
    },
    "TableName" : "cla-{}-companies".format(stage)
}

SIGNATURE_TABLE_DATA = {
    "Table": {
        "AttributeDefinitions": [
            {"AttributeName": "signature_id", "AttributeType": "S",},
            {"AttributeName": "signature_project_external_id", "AttrubuteType": "S"},
            {"AttributeName": "signature_company_project_external_id", "AttributeType": "S"},
            {"AttributeName": "signature_company_initial_manager_id", "AttributeType": "S"},
            {"AttributeName": "signature_company_secondary_manager_list", "AttributeType": "M"},
            {"AttributeName": "signature_reference_id", "AttributeType": "S"},
            {"AttributeName": "signature_project_id", "AttributeType": "S"},
        ],
        "KeySchema": [{"AttributeName": "signature_id", "KeyType": "HASH"}],
        "GlobalSecondaryIndexes": [
            {
                "IndexName": "project-signature-external-id-index",
                "KeySchema": [{"AttributeName": "signature_project_external_id", "KeyType": "HASH"}],
                "Projection": {"ProjectionType": "ALL"},
                "ProvisionedThroughput": {"ReadCapacityUnits": 1, "WriteCapacityUnits": 1},
            },
            {
                "IndexName": "signature-company-signatory-index",
                "KeySchema": [{"AttributeName": "signature_company_project_external_id", "KeyType": "HASH"}],
                "Projection": {"ProjectionType": "ALL"},
                "ProvisionedThroughput": {"ReadCapacityUnits": 1, "WriteCapcityUnits": 1},
            },
            {
                "IndexName": "signature-company-initial-manager-index",
                "KeySchema": [{"AttributeName": "signature_company_initial_manager_id", "KeyType": "HASH"}],
                "Projection": {"ProjectionType": "ALL"},
                "ProvisionedThroughput": {"ReadCapacityUnits": 1, "WriteCapacityUnits": 1},
            },
            {
                "IndexName": "signature-project-reference-index",
                "KeySchema": [{"AttributeName": "signature_project_id", "KeyType": "HASH"},
                              {"AttributeName": "signature_reference_id", "KeyType": "RANGE"}],
                "Projection": {"ProjectionType": "ALL"},
                "ProvisionedThroughput": {"ReadCapacityUnits": 1, "WriteCapacityUnits": 1},
            }
        ],
    },
    "TableName" : "cla-{}-signatures".format(stage)
}

EVENT_TABLE_DESCRIPTION = {
    "Table": {
        "AttributeDefinitions": [{"AttributeName": "event_id", "AttributeType": "S"}],
        "KeySchema": [{"AttributeName": "event_id", "KeyType": "HASH"}],
    },
    "TableName" : "cla-{}-events".format(stage)
}

PROJECT_TABLE_DESCRIPTION = {
    "Table": {
        "AttributeDefinitions": [{"AttributeName": "project_id", "AttributeType": "S"}],
        "KeySchema": [{"AttributeName": "project_id", "KeyType": "HASH"}],
    },
    "TableName" : "cla-{}-projects".format(stage)
}

GH_TABLE_DESCRIPTION = {
    "Table": {
        "AttributeDefinitions": [{"AttributeName": "organization_name", "AttributeType": "S"}],
        "KeySchema": [{"AttributeName": "organization_name", "KeyType": "HASH"}],
    }, 
    "TableName" : "cla-{}-github-orgs".format(stage)
}
