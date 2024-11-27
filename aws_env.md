# Setting up AWS environment

You need to have MFA enabled for your AWS user, your `~/.aws/config` shoudl look like this:
```
[profile lfproduct-dev]
role_arn = arn:aws:iam::395594542180:role/product-contractors-role
source_profile = lfproduct
region = us-east-1
output = json

[profile lfproduct-test]
role_arn = arn:aws:iam::726224182707:role/product-contractors-role
source_profile = lfproduct
region = us-east-1
output = json

[profile lfproduct-staging]
role_arn = arn:aws:iam::844390194980:role/product-contractors-role
source_profile = lfproduct
region = us-east-1
output = json

[profile lfproduct-prod]
role_arn = arn:aws:iam::716487311010:role/product-contractors-role
source_profile = lfproduct
region = us-east-1
output = json

[default]
region = us-east-1
output = json
```

It defines 4 profiles to use: `dev`, `staging`, `test` and `prod`.

You will be using one of them.


Your `~/.aws/credentials` file shoudl initially look like this (replace `redacted`):
```
[lfproduct-long-term]
aws_secret_access_key = [access_key_redacted]
aws_access_key_id = [key_id_redacted]
aws_mfa_device = arn:aws:iam::[arn_number_redacted]:mfa/[your_aws_user_redacted]

[default]
aws_access_key_id = [key_id_redacted]
aws_secret_access_key = [access_key_redacted]
```

Now every 36 hours or less you need to refresh your MFA key by calling: `aws-mfa --force --duration 129600 --profile lfproduct`.

When called it adds or replaces the following section (`[lfproduct]` which is used as a source profile for `dev`, `test`, `staging` or `prod` in aws config) in `~/.aws/credentials`:
```
[lfproduct]
assumed_role = False
aws_access_key_id = [key_id_redacted]
aws_secret_access_key = [secret_access_key_redacted]
aws_session_token = [session_token_redacted]
aws_security_token = [session_token_redacted]
expiration = 2024-11-28 16:54:59 [now + 36 hours]

```


Once you have all of this, you must set a correct set of environment variables to run either `python` or `golang` backends.

To do so you need to get credentials for a specific profile `lfproduct-`: `dev`, `test`, `staging`, `prod`. To see full one-time set of credentials you can call:
- for `dev`:  `` aws sts assume-role --role-arn arn:aws:iam::395594542180:role/product-contractors-role --profile lfproduct --role-session-name lfproduct-dev-session ``.
- for `prod`: `` aws sts assume-role --role-arn arn:aws:iam::716487311010:role/product-contractors-role --profile lfproduct --role-session-name lfproduct-prod-session ``.

Note - just replace the iam::[number] depending on environment type (`[stage]`) and update `lfproduct-[stage]-name`.

You can set up a script like `setenv.sh` which will set all required variables, example for `dev`:
```
#!/bin/bash

rm -rf /tmp/aws
cp -R /root/.aws /tmp/.aws

data="$(aws sts assume-role --role-arn arn:aws:iam::395594542180:role/product-contractors-role --profile lfproduct --role-session-name lfproduct-dev-session)"
export AWS_ACCESS_KEY_ID="$(echo "${data}" | jq -r '.Credentials.AccessKeyId')"
export AWS_SECRET_ACCESS_KEY="$(echo "${data}" | jq -r '.Credentials.SecretAccessKey')"
export AWS_SESSION_TOKEN="$(echo "${data}" | jq -r '.Credentials.SessionToken')"
export AWS_SECURITY_TOKEN="$(echo "${data}" | jq -r '.Credentials.SessionToken')"

export AWS_SDK_LOAD_CONFIG=true
export AWS_PROFILE='lfproduct-dev'
export AWS_REGION='us-east-1'
export AWS_DEFAULT_REGION='us-east-1'
export DYNAMODB_AWS_REGION='us-east-1'
export REGION='us-east-1'

export PRODUCT_DOMAIN='dev.lfcla.com'
export ROOT_DOMAIN='lfcla.dev.platform.linuxfoundation.org'
export PORT='5000'
export STAGE='dev'
# export STAGE='local'
export GH_ORG_VALIDATION=false
export DISABLE_LOCAL_PERMISSION_CHECKS=true
export COMPANY_USER_VALIDATION=false
export CLA_SIGNATURE_FILES_BUCKET=cla-signature-files-dev
```

Call it via `` . ./setenv.sh ``  or `` source setenv.sh `` to execute in the current shell.

You can reset environment variables by exiting the shell session or calling the following `unsetenv.sh` in the current shell via: `` . ./unsetenv.sh `` or `` source unsetenv.sh ``:
```
#!/bin/bash
rm -rf /tmp/.aws
unset AWS_PROFILE
unset AWS_REGION
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY
unset PRODUCT_DOMAIN
unset ROOT_DOMAIN
unset PORT
unset STAGE
unset AWS_SESSION_TOKEN
unset AWS_SECURITY_TOKEN
unset GH_ORG_VALIDATION
unset DISABLE_LOCAL_PERMISSION_CHECKS
unset COMPANY_USER_VALIDATION
unset CLA_SIGNATURE_FILES_BUCKET
unset DYNAMODB_AWS_REGION
unset REGION
unset AWS_ROLE_ARN
unset AWS_TOKEN_SERIAL
unset AWS_SDK_LOAD_CONFIG
```
