#!/bin/bash
# DRY=1 - dry run
# DRY=1 ./utils/delete_user_repository_project_signature.sh mlehotskylf-org2/easycla-dev lukaszgryglicki
if [ -z "$STAGE" ]
then
  export STAGE=dev
fi
if [ -z "$1" ]
then
  echo "$0: you need to provide the repository name, for example: 'mlehotskylf-org2/easycla-dev'"
  exit 1
fi
REPO_NAME=$1
if [ -z "$2" ]
then
  echo "$0: you need to provide the GitHub user name, for example: 'lukaszgryglicki'"
  exit 2
fi
GITHUB_USER=$2
PROJECT_IDS=$(./utils/scan.sh repositories repository_name "${REPO_NAME}" | jq -r '.[].repository_project_id.S')
if [ -z "$PROJECT_IDS" ] || [ "$PROJECT_IDS" == "null" ]
then
  echo "$0: cannot find project IDs for repository ${REPO_NAME}"
  exit 3
fi
echo "Project IDs: ${PROJECT_IDS}"
USER_IDS=$(./utils/scan.sh users user_github_username "${GITHUB_USER}" | jq -r '.[].user_id.S')
if [ -z "$USER_IDS" ] || [ "$USER_IDS" == "null" ]
then
  echo "$0: cannot find user ID for GitHub user ${GITHUB_USER}"
  exit 4
fi
echo "User IDs: ${USER_IDS}"
for PROJECT_ID in $PROJECT_IDS
do
  for USER_ID in $USER_IDS
  do
    echo "Deleting user repository project signature for user ID ${USER_ID} and project ID ${PROJECT_ID}"
    SIGS=$(aws dynamodb query \
      --profile "lfproduct-${STAGE}" \
      --table-name "cla-${STAGE}-signatures" \
      --index-name "signature-project-reference-index" \
      --key-condition-expression "signature_project_id = :pid AND signature_reference_id = :uid" \
      --filter-expression "signature_reference_type = :rtype AND signature_signed = :true AND signature_approved = :true" \
      --expression-attribute-values '{
        ":pid":  {"S":"'"${PROJECT_ID}"'"},
        ":uid":  {"S":"'"${USER_ID}"'"},
        ":rtype":{"S":"user"},
        ":true":{"BOOL": true}
      }' --output json | jq -r '.Items[].signature_id.S')
    for SIG in $SIGS
    do
      if [ ! -z "$DRY" ]
      then
          echo "DRY RUN: would delete this signature:"
          ./utils/scan.sh signatures signature_id "${SIG}"
          echo "DRY RUN: would delete this ^ signature."
          continue
      fi
      echo "Deleting signature ID ${SIG}"
      aws dynamodb delete-item --profile "lfproduct-${STAGE}" --table-name "cla-${STAGE}-signatures" --key '{"signature_id": {"S":"'"${SIG}"'"}}'
    done
  done
done
