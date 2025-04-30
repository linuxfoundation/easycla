# How to check why EasyCLA is not covered

1) Open GitHub PR and check for user name: `` select * from FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_USERS where data:user_github_username = '<user-name>'; ``. Note `user_id`.
2) If user has `user_company_id` then note it and get company data: `` select * from FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_COMPANIES where company_id = '<user-company-id>'; ``.
3) First let's look for ICLAs: `` select * from FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_SIGNATURES where data:signature_reference_id = '<user-id>' and data:signature_reference_type = 'user' and data:signature_type = 'cla' and data:signature_user_ccla_company_id is null; ``.
4) If ICLAs are found, then check for which project `data:signature_project_id` field: `` select data:project_name from FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_PROJECTS where project_id = '<project-id>'; ``.
5) For ECLA (if user have `company_id` set), lookup for ECLAs: `` select * from FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_SIGNATURES where data:signature_project_id = '<project-id>' and data:signature_user_ccla_company_id = '<user-company-id>' and data:signature_reference_id = '<user-id>'; ``.
6) To find comapny's / project's CCLA: `` select * from FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_SIGNATURES where data:signature_project_id = '<project-id>' and data:signature_reference_id = '<company-id>' and data:signature_reference_type = 'company'; ``.
