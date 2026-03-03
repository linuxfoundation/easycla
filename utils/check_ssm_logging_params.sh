#!/bin/bash
echo "dev us-east-1:"
echo -n 'cla-dd-site-dev: '; aws --profile lfproduct-dev --region us-east-1 ssm get-parameters --names "cla-dd-site-dev" --with-decryption | jq -r '.Parameters[0].Value'
echo -n 'cla-dd-api-key-secret-arn-dev: '; aws --profile lfproduct-dev --region us-east-1 ssm get-parameters --names "cla-dd-api-key-secret-arn-dev" --with-decryption | jq -r '.Parameters[0].Value'
echo 'cla-dd-api-key-dev:'; aws --profile lfproduct-dev --region us-east-1 secretsmanager get-secret-value --secret-id "$(aws --profile lfproduct-dev --region us-east-1 ssm get-parameters --names "cla-dd-api-key-secret-arn-dev" --with-decryption | jq -r '.Parameters[0].Value')" --query SecretString --output text
echo -n 'cla-dd-extension-layer-arn-dev: '; aws --profile lfproduct-dev --region us-east-1 ssm get-parameters --names "cla-dd-extension-layer-arn-dev" --with-decryption | jq -r '.Parameters[0].Value'
echo -n 'cla-ddb-api-logging-dev: '; aws --profile lfproduct-dev --region us-east-1 ssm get-parameters --names "cla-ddb-api-logging-dev" --with-decryption | jq -r '.Parameters[0].Value'
echo -n 'cla-otel-datadog-api-logging-dev: '; aws --profile lfproduct-dev --region us-east-1 ssm get-parameters --names "cla-otel-datadog-api-logging-dev" --with-decryption | jq -r '.Parameters[0].Value'

echo "dev us-east-2"
echo -n 'cla-dd-site-dev: '; aws --profile lfproduct-dev --region us-east-2 ssm get-parameters --names "cla-dd-site-dev" --with-decryption | jq -r '.Parameters[0].Value'
echo -n 'cla-dd-api-key-secret-arn-dev: '; aws --profile lfproduct-dev --region us-east-2 ssm get-parameters --names "cla-dd-api-key-secret-arn-dev" --with-decryption | jq -r '.Parameters[0].Value'
echo 'cla-dd-api-key-dev:'; aws --profile lfproduct-dev --region us-east-2 secretsmanager get-secret-value --secret-id "$(aws --profile lfproduct-dev --region us-east-2 ssm get-parameters --names "cla-dd-api-key-secret-arn-dev" --with-decryption | jq -r '.Parameters[0].Value')" --query SecretString --output text
echo -n 'cla-dd-extension-layer-arn-dev: '; aws --profile lfproduct-dev --region us-east-2 ssm get-parameters --names "cla-dd-extension-layer-arn-dev" --with-decryption | jq -r '.Parameters[0].Value'
echo -n 'cla-ddb-api-logging-dev: '; aws --profile lfproduct-dev --region us-east-2 ssm get-parameters --names "cla-ddb-api-logging-dev" --with-decryption | jq -r '.Parameters[0].Value'
echo -n 'cla-otel-datadog-api-logging-dev: '; aws --profile lfproduct-dev --region us-east-2 ssm get-parameters --names "cla-otel-datadog-api-logging-dev" --with-decryption | jq -r '.Parameters[0].Value'

echo "prod us-east-1"
echo -n 'cla-dd-site-prod: '; aws --profile lfproduct-prod --region us-east-1 ssm get-parameters --names "cla-dd-site-prod" --with-decryption | jq -r '.Parameters[0].Value'
echo -n 'cla-dd-api-key-secret-arn-prod: '; aws --profile lfproduct-prod --region us-east-1 ssm get-parameters --names "cla-dd-api-key-secret-arn-prod" --with-decryption | jq -r '.Parameters[0].Value'
echo 'cla-dd-api-key-prod:'; aws --profile lfproduct-prod --region us-east-1 secretsmanager get-secret-value --secret-id "$(aws --profile lfproduct-prod --region us-east-1 ssm get-parameters --names "cla-dd-api-key-secret-arn-prod" --with-decryption | jq -r '.Parameters[0].Value')" --query SecretString --output text
echo -n 'cla-dd-extension-layer-arn-prod: '; aws --profile lfproduct-prod --region us-east-1 ssm get-parameters --names "cla-dd-extension-layer-arn-prod" --with-decryption | jq -r '.Parameters[0].Value'
echo -n 'cla-ddb-api-logging-prod: '; aws --profile lfproduct-prod --region us-east-1 ssm get-parameters --names "cla-ddb-api-logging-prod" --with-decryption | jq -r '.Parameters[0].Value'
echo -n 'cla-otel-datadog-api-logging-prod: '; aws --profile lfproduct-prod --region us-east-1 ssm get-parameters --names "cla-otel-datadog-api-logging-prod" --with-decryption | jq -r '.Parameters[0].Value'

echo "prod us-east-2"
echo -n 'cla-dd-site-prod: '; aws --profile lfproduct-prod --region us-east-2 ssm get-parameters --names "cla-dd-site-prod" --with-decryption | jq -r '.Parameters[0].Value'
echo -n 'cla-dd-api-key-secret-arn-prod: '; aws --profile lfproduct-prod --region us-east-2 ssm get-parameters --names "cla-dd-api-key-secret-arn-prod" --with-decryption | jq -r '.Parameters[0].Value'
echo 'cla-dd-api-key-prod:'; aws --profile lfproduct-prod --region us-east-2 secretsmanager get-secret-value --secret-id "$(aws --profile lfproduct-prod --region us-east-2 ssm get-parameters --names "cla-dd-api-key-secret-arn-prod" --with-decryption | jq -r '.Parameters[0].Value')" --query SecretString --output text
echo -n 'cla-dd-extension-layer-arn-prod: '; aws --profile lfproduct-prod --region us-east-2 ssm get-parameters --names "cla-dd-extension-layer-arn-prod" --with-decryption | jq -r '.Parameters[0].Value'
echo -n 'cla-ddb-api-logging-prod: '; aws --profile lfproduct-prod --region us-east-2 ssm get-parameters --names "cla-ddb-api-logging-prod" --with-decryption | jq -r '.Parameters[0].Value'
echo -n 'cla-otel-datadog-api-logging-prod: '; aws --profile lfproduct-prod --region us-east-2 ssm get-parameters --names "cla-otel-datadog-api-logging-prod" --with-decryption | jq -r '.Parameters[0].Value'
