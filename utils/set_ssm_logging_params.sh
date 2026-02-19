#!/bin/bash
# REGION=us-east-1
# STAGE=dev
# SET=1 
# STAGE=dev REGION=us-east-1 SET=1 ./set_ssm_logging_params.sh
if [ -z "$STAGE" ]
then
  export STAGE=dev
fi
if [ -z "$REGION" ]
then
  export REGION=us-east-1
fi
if [ ! -z "$SET" ]
then
  ./utils/set_ssm_value.sh cla-dd-site-dev 'datadoghq.com' String
  # ./utils/set_ssm_value.sh cla-dd-site-dev 'app.datadoghq.com' String
  ./utils/set_ssm_value.sh cla-dd-api-key-secret-arn-dev "$(cat ./DD_API_KEY_SECRET_ARN.secret)" String
  ./utils/set_ssm_value.sh cla-dd-extension-layer-arn-dev "$(cat ./DD_EXTENSION_LAYER_ARN_DEV.secret)" String
  ./utils/set_ssm_value.sh cla-ddb-api-logging-dev true String
  ./utils/set_ssm_value.sh cla-otel-datadog-api-logging-dev true String
fi

./utils/get_ssm_value.sh cla-dd-site-dev
./utils/get_ssm_value.sh cla-dd-api-key-secret-arn-dev
./utils/get_ssm_value.sh cla-dd-extension-layer-arn-dev
./utils/get_ssm_value.sh cla-ddb-api-logging-dev
./utils/get_ssm_value.sh cla-otel-datadog-api-logging-dev
