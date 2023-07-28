# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

.PHONY: setup
setup:
	npm install;
	pip3 install virtualenv;
	virtualenv -p python3 ~/.env/lf-cla;
	source ~/.env/lf-cla/bin/activate; \
	pip3 install -r requirements.txt; \

.PHONY: setup_circle
setup_circle:
	npm install; \
	sudo pip install -r requirements.txt; \

# username: LFID username
# projects: quoted list of SFDC project IDs. e.g. '\"abcd\",\"1234\"' 
.PHONY: add_project_manager
add_project_manager:
	aws dynamodb put-item \
		--table-name "cla-$(STAGE)-user-permissions" \
		--item "{ \"username\": { \"S\": \"$(username)\" }, \"projects\": { \"SS\": [$(projects)] } }" \
		--region "us-east-1";

# company_id: company ID (UUID)
# username: LFID username
.PHONY: add_cla_manager
add_cla_manager:
	aws dynamodb update-item \
		--table-name "cla-$(STAGE)-companies" \
		--key "{\"company_id\": {\"S\": \"$(company_id)\"}}" \
		--update-expression "ADD company_acl :value" \
		--expression-attribute-values "{\":value\":{\"SS\": [\"$(username)\"]}}" \
		--region "us-east-1" ;

# username: LFID username
# project_sfdc_id: project SFDC ID
# bearer_token: Auth0 JWT
# base_url: the base API url
.PHONY: add_project_manager_permission
add_project_manager_permission:
	curl -XPOST \
		-H "Content-type: application/json" \
		-H "Authorization: Bearer $(bearer_token)" \
		-d '{"username": "$(username)", "project_sfdc_id": "$(project_sfdc_id)"}' \
		"$(base_url)/v1/project/permission"

# username: LFID username
# project_sfdc_id: project SFDC ID
# bearer_token: Auth0 JWT
# base_url: the base API url
.PHONY: remove_project_manager_permission
remove_project_manager_permission:
	curl -XDELETE \
		-H "Content-type: application/json" \
		-H "Authorization: Bearer $(bearer_token)" \
		-d '{"username": "$(username)", "project_sfdc_id": "$(project_sfdc_id)"}' \
		"$(base_url)/v1/project/permission"

# username: LFID username
# company_id: company ID
# bearer_token: Auth0 JWT
# base_url: the base API url
.PHONY: add_cla_manager_permission
add_cla_manager_permission:
	curl -XPOST \
		-H "Content-type: application/json" \
		-H "Authorization: Bearer $(bearer_token)" \
		-d '{"username": "$(username)", "company_id": "$(company_id)"}' \
		"$(base_url)/v1/company/permission"

# file_path: path to local logo png
# project_sfdc_id: project SFDC ID
# bearer_token: Auth0 JWT
# base_url: the base API url
.PHONY: upload_logo
upload_logo:
	file_path=$(file_path) \
	project_sfdc_id=$(project_sfdc_id) \
	bearer_token=$(bearer_token) \
	base_url=$(base_url) \
	../scripts/credential-updates/upload_logo.sh \

.PHONY: run_dynamo
run_dynamo:
	source ~/.env/lf-cla/bin/activate; \
	node_modules/.bin/serverless dynamodb install -s 'local'; \
	node_modules/.bin/serverless dynamodb start -s 'local'; \

.PHONY: run_s3
run_s3:
	source ~/.env/lf-cla/bin/activate; \
	node_modules/.bin/serverless s3 start -s 'local'; \

.PHONY: run_lambda
run_lambda:
	source ~/.env/lf-cla/bin/activate; \
	node_modules/.bin/serverless wsgi serve -s 'local'; \

.PHONY: deploy
deploy:
	# Stop deployment if env.json exists
	test ! -s ./env.json || exit 1

	source ~/.env/lf-cla/bin/activate; \
	node_modules/.bin/serverless deploy --force --stage ${STAGE} --region us-east-1; \

.PHONY: deploy-dynamodb-backups
deploy-dynamodb-backups:
	source ~/.env/lf-cla/bin/activate; \
	cd serverless-dynamodb-backups; \
	../node_modules/.bin/serverless deploy --region us-east-1; \
