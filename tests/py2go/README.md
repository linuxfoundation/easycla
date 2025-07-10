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
