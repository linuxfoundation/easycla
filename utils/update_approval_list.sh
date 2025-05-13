#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# TOKEN='...' - Auth0 JWT bearer token
# XACL='...' - X-ACL
# companyId=326d5582-5ce9-4ff4-8cc3-a2fcf865c39c
# projectId=a092M00001IdwOGQAZ
# claGroupId=31032d86-31c9-49dd-8478-63ffa6af47b6
# payload={
#  "AddDomainApprovalList":[],
#  "RemoveDomainApprovalList":[],
#  "AddEmailApprovalList":[],
#  "RemoveEmailApprovalList":["jarias@nec.com"],
#  "AddGithubOrgApprovalList":[],
#  "RemoveGithubOrgApprovalList":[],
#  "AddGithubUsernameApprovalList":[],
#  "RemoveGithubUsernameApprovalList":[],
#  "AddGitlabOrgApprovalList":[],
#  "RemoveGitlabOrgApprovalList":[],
#  "AddGitlabUsernameApprovalList":[],
#  "RemoveGitlabUsernameApprovalList":[]
#}
# authUser = &auth.User{
#     UserID: "uuid",
#     UserName: "lgryglicki",
#     Email: "email@domain",
#     ACL: auth.ACL{
#       Admin: true,
#       Allowed: true,
#     },
# }
# USE_FILE=1 ./utils/copy_prod_to_dev.sh companies company_id 326d5582-5ce9-4ff4-8cc3-a2fcf865c39c
# USE_FILE=1 ./utils/copy_prod_to_dev.sh projects project_id 31032d86-31c9-49dd-8478-63ffa6af47b6
# USE_FILE=1 ./utils/copy_prod_to_dev.sh projects-cla-groups project_sfid a092M00001IdwOGQAZ
# USE_FILE=1 ./utils/copy_prod_to_dev.sh signatures signature_id b7dbf3e1-267e-482c-918c-eeab006889a1
# DEBUG=1 companyId='326d5582-5ce9-4ff4-8cc3-a2fcf865c39c' projectId='a092M00001IdwOGQAZ' claGroupId='31032d86-31c9-49dd-8478-63ffa6af47b6' payload='{"AddDomainApprovalList":[],"RemoveDomainApprovalList":[],"AddEmailApprovalList":[],"RemoveEmailApprovalList":["jarias@nec.com"],"AddGithubOrgApprovalList":[],"RemoveGithubOrgApprovalList":[],"AddGithubUsernameApprovalList":[],"RemoveGithubUsernameApprovalList":[],"AddGitlabOrgApprovalList":[],"RemoveGitlabOrgApprovalList":[],"AddGitlabUsernameApprovalList":[],"RemoveGitlabUsernameApprovalList":[]}' ./utils/update_approval_list.sh

if [ -z "$TOKEN" ]
then
  # source ./auth0_token.secret
  TOKEN="$(cat ./auth0.token.secret)"
fi

if [ -z "$TOKEN" ]
then
  echo "$0: TOKEN not specified and unable to obtain one"
  exit 1
fi

if [ -z "$XACL" ]
then
  XACL="$(cat ./x-acl.secret)"
fi

if [ -z "$XACL" ]
then
  echo "$0: XACL not specified and unable to obtain one"
  exit 2
fi

if [ -z "${companyId}" ]
then
  echo "$0: you need to specify companyId='...'"
  exit 3
fi

if [ -z "${projectId}" ]
then
  echo "$0: you need to specify projectId='...'"
  exit 4
fi

if [ -z "${claGroupId}" ]
then
  echo "$0: you need to specify claGroupId='...'"
  exit 5
fi

if [ -z "${payload}" ]
then
  echo "$0: you need to specify playload='...'"
  exit 6
fi

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

export API_PATH="v4/signatures/project/${projectId}/company/${companyId}/clagroup/${claGroupId}/approval-list"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XPUT -H 'X-ACL: ${XACL}' -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' '${API_URL}/${API_PATH}' -d '${payload}' | jq -r '.'"
  echo "${payload}" | jq -r '.'
fi
curl -s -XPUT -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/${API_PATH}" -d "${payload}" | jq -r '.'
