# Testing porting APIs from Python to golang

1) Start `python` API backend:
- `` source setenv.sh; cd cla-backend; source .venv/bin/activate; yarn serve:ext ``.

2) Start `golang` API backend:
- `` source setenv.sh; cd cla-backend-go; make swagger; make build-linux ``.
- `` PORT=5001 AUTH0_USERNAME_CLAIM_CLI='http://lfx.dev/claims/username' AUTH0_EMAIL_CLAIM_CLI='http://lfx.dev/claims/email' AUTH0_NAME_CLAIM_CLI='http://lfx.dev/claims/username' ./bin/cla ``.
- Or: `` ../utils/run_go_api_server.sh ``.

3) Get `auth0` token from browser session (login using `LFID`):
- `` ./get_oauth_token.sh ``. Copy the token value.

3) Exacute API tests:
- `` export TOKEN='<value-from-3>' ``.
- `` export XACL="$(cat ../../x-acl.secret)" ``.
- `` make ``.
- `` DEBUG=1 PROJECT_UUID=88ee12de-122b-4c46-9046-19422054ed8d PY_API_URL=https://api.lfcla.dev.platform.linuxfoundation.org GO_API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service make ``.
- `` MAX_PARALLEL=8 PY_API_URL=https://api.lfcla.dev.platform.linuxfoundation.org go test -v -run '^TestAllProjectsCompatAPI$' ``.
- To run a specific test case(s): `` DEBUG=1 PROJECT_UUID=88ee12de-122b-4c46-9046-19422054ed8d PY_API_URL=https://api.lfcla.dev.platform.linuxfoundation.org go test -v -run '^TestProjectCompatAPI$' ``.
- Manually via `cURL`: `` curl -s -XGET http://127.0.0.1:5001/v4/project-compat/01af041c-fa69-4052-a23c-fb8c1d3bef24 | jq . ``.
- To manually see given project values if APIs differ (to dewbug): `` aws --region us-east-1 --profile lfproduct-dev dynamodb get-item --table-name cla-dev-projects --key '{"project_id": {"S": "4a855799-0aea-4e01-98b7-ef3da09df478"}}' | jq '.Item' ``.
- And `` aws --region us-east-1 --profile lfproduct-dev dynamodb query --table-name cla-dev-projects-cla-groups --index-name cla-group-id-index --key-condition-expression "cla_group_id = :project_id" --expression-attribute-values '{":project_id":{"S":"4a855799-0aea-4e01-98b7-ef3da09df478"}}' | jq '.Items' ``.
- `` DEBUG='' USER_UUID=b817eb57-045a-4fe0-8473-fbb416a01d70 PY_API_URL=https://api.lfcla.dev.platform.linuxfoundation.org go test -v -run '^TestUserActiveSignatureAPI$' ``.
- `` REPO_ID=466156917 PR_ID=3 DEBUG=1 go test -v -run '^TestUserActiveSignatureAPI$' ``.
- `` MAX_PARALLEL=2 DEBUG='' go test -v -run '^TestAllUserActiveSignatureAPI$' ``.
- `` [STAGE=prod] [PY_API_URL=local|dev|prod] [GO_API_URL=local|dev|prod] make ``.
- `` GO_API_URL=dev PY_API_URL=dev go test -v -run '^TestUserActiveSignatureAPIWithNonV4UUID$' ``.
- `` GO_API_URL=dev PY_API_URL=dev go test -v -run '^TestUserActiveSignatureAPIWithInvalidUUID$' ``.
- `` GO_API_URL=dev PY_API_URL=dev go test -v -run '^TestProjectCompatAPIWithNonV4UUID$' ``.
- `` GO_API_URL=dev PY_API_URL=dev go test -v -run '^TestProjectCompatAPIWithInvalidUUID$' ``.
- `` GO_API_URL=local PY_API_URL=dev go test -v -run '^TestAllUsersCompatAPI$' ``.
- `` USER_UUID=7b596ccc-b087-11ef-b61d-6a99096124c1 GO_API_URL=local PY_API_URL=dev go test -v -run '^TestUserCompatAPI$' ``.
