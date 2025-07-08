# Testing porting APIs from Python to golang

1) Start `python` API backend:
- `` source setenv.sh; cd cla-backend; source .venv/bin/activate; yarn serve:ext ``.

2) Start `golang` API backend:
- `` source setenv.sh; cd cla-backend-go; make swagger; make build-linux ``.
- `` PORT=5001 AUTH0_USERNAME_CLAIM_CLI='http://lfx.dev/claims/username' AUTH0_EMAIL_CLAIM_CLI='http://lfx.dev/claims/email' AUTH0_NAME_CLAIM_CLI='http://lfx.dev/claims/username' ./bin/cla ``.

3) Get `auth0` token from browser session (login using `LFID`):
- `` ./get_oauth_token.sh ``. Copy the token value.

3) Exacute API tests:
- `` export TOKEN='<value-from-3>' ``.
- `` export XACL="$(cat ../../x-acl.secret)" ``.
- `` make ``.
- `` DEBUG=1 PROJECT_UUID=88ee12de-122b-4c46-9046-19422054ed8d PY_API_URL=https://api.lfcla.dev.platform.linuxfoundation.org GO_API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service make ``.
