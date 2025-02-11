#!/bin/bash
./utils/lookup_sf.sh gerrit_instances gerrit_id project_sfid "'a09P000000DsCE5IAN'"
./utils/lookup_sf.sh projects_cla_groups project_sfid project_sfid "'a09P000000DsCE5IAN'"
OP=in ./utils/lookup_sf.sh user_permissions username projects "'a09P000000DsCE5IAN'"
./utils/lookup_sf.sh signatures signature_id signature_type "'ecla'"
./utils/lookup_sf.sh signatures signature_id project_id "'01af041c-fa69-4052-a23c-fb8c1d3bef24'"
./utils/cla_authorization.sh 01af041c-fa69-4052-a23c-fb8c1d3bef24 poojapanjwani
./utils/lookup_sf.sh projects_cla_groups project_sfid cla_group_id "'01af041c-fa69-4052-a23c-fb8c1d3bef24'"
./utils/lookup_sf.sh companies company_id company_id "'f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc'"
OUT='project_name' ./utils/lookup_sf.sh projects project_id project_id "'01af041c-fa69-4052-a23c-fb8c1d3bef24'"
OUT='project_sfid project_name foundation_sfid' ./utils/lookup_sf.sh projects_cla_groups project_sfid foundation_sfid "'a09P000000DsCE5IAN'"
# ICLA
COND="data:signature_project_id = '01af041c-fa69-4052-a23c-fb8c1d3bef24' and data:signature_reference_id = '1527f0ec-3272-11ec-a3ed-0e7521e28b4e' and data:signature_type = 'cla' and data:signature_reference_type = 'user' and data:signature_user_ccla_company_id is null and data:signature_signed and data:signature_approved" ./utils/lookup_sf.sh signatures signature_id
# ECLA
COND="data:signature_reference_id = '1527f0ec-3272-11ec-a3ed-0e7521e28b4e' and data:signature_user_ccla_company_id = 'f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc' and data:signature_project_id = '01af041c-fa69-4052-a23c-fb8c1d3bef24'" ./utils/lookup_sf.sh signatures signature_id
# CCLA
COND="data:signature_project_id = '01af041c-fa69-4052-a23c-fb8c1d3bef24' and data:signature_reference_id = 'f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc' and data:signature_type = 'ccla' and data:signature_reference_type = 'company' and data:signature_user_ccla_company_id is null and data:signature_approved and data:signature_signed" ./utils/lookup_sf.sh signatures signature_id
# M2M token generation & API call
./m2m-token-prod.secret && DEBUG=1 API_URL='https://api-gw.platform.linuxfoundation.org/cla-service' ./utils/cla_authorization.sh b71c469a-55e7-492c-9235-fd30b31da2aa andreasgeissler
