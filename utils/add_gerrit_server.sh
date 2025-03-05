#!/bin/bash
# STAGE=dev gerrit_name='Dev CNCF Gerrit' gerrit_url=http://147.75.85.27:8080 project_sfid=a09P000000DsCE5IAN project_id=01af041c-fa69-4052-a23c-fb8c1d3bef24 ./utils/add_gerrit_server.sh
# {
#   "version": {
#     "S": "v2"
#   },
#   "date_modified": {
#     "S": "2024-11-22T10:14:27Z"
#   },
#   "gerrit_url": {
#     "S": "https://gerrit.onap.org"
#   },
#   "gerrit_id": {
#     "S": "c2be3edf-a956-438f-ad7d-a03d7d24efce"
#   },
#   "project_sfid": {
#     "S": "a09P000000DsCE5IAN"
#   },
#   "project_id": {
#     "S": "01af041c-fa69-4052-a23c-fb8c1d3bef24"
#   },
#   "date_created": {
#     "S": "2024-11-22T10:14:27Z"
#   },
#   "gerrit_name": {
#     "S": "mock"
#   }
# }
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
dt_now=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
uuid=$(uuidgen)
if [ -z "${gerrit_url}" ]
then
  echo "$0: you need to specify gerrit_url=..."
  exit 1
fi
if [ -z "${project_sfid}" ]
then
  echo "$0: you need to specify project_sfid=..."
  exit 2
fi
if [ -z "${project_id}" ]
then
  echo "$0: you need to specify project_id=..."
  exit 3
fi
if [ -z "${gerrit_name}" ]
then
  echo "$0: you need to specify gerrit_name=..."
  exit 4
fi
if [ ! -z "$DEBUG" ]
then
  echo aws --profile "lfproduct-${STAGE}" dynamodb put-item --table-name "cla-${STAGE}-gerrit-instances" --item "{\"version\":{\"S\":\"v2\"},\"date_modified\":{\"S\":\"${dt_now}\"},\"gerrit_url\":{\"S\":\"${gerrit_url}\"},\"gerrit_id\":{\"S\":\"${uuid}\"},\"project_sfid\":{\"S\":\"${project_sfid}\"},\"project_id\":{\"S\":\"${project_id}\"},\"date_created\":{\"S\":\"${dt_now}\"},\"gerrit_name\":{\"S\":\"${gerrit_name}\"}}"
fi
aws --profile "lfproduct-${STAGE}" dynamodb put-item --table-name "cla-${STAGE}-gerrit-instances" --item "{\"version\":{\"S\":\"v2\"},\"date_modified\":{\"S\":\"${dt_now}\"},\"gerrit_url\":{\"S\":\"${gerrit_url}\"},\"gerrit_id\":{\"S\":\"${uuid}\"},\"project_sfid\":{\"S\":\"${project_sfid}\"},\"project_id\":{\"S\":\"${project_id}\"},\"date_created\":{\"S\":\"${dt_now}\"},\"gerrit_name\":{\"S\":\"${gerrit_name}\"}}"
