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
  export REGION='us-east-1'
fi
if [ "$REGIN" = "us-east-1" ]
then
  export REG_NUM=1
fi
if [ "$REGIN" = "us-east-2" ]
then
  export REG_NUM=2
fi
if [ ! -z "$SET" ]
then
  # ./utils/set_ssm_value.sh "cla-dd-site-${STAGE}" "app.datadoghq.com" String
  ./utils/set_ssm_value.sh "cla-dd-site-${STAGE}" "datadoghq.com" String
  ./utils/set_ssm_value.sh "cla-dd-version-${STAGE}" "$(git rev-parse --short=9 HEAD 2>/dev/null || echo '1.0')" String
  ./utils/set_ssm_value.sh "cla-dd-api-key-secret-arn-${STAGE}" "$(cat ./DD_API_KEY_SECRET_ARN-${REG_NUM}.secret)" String
  ./utils/set_ssm_value.sh "cla-dd-extension-layer-arn-${STAGE}" "$(cat ./DD_EXTENSION_LAYER_ARN_DEV.secret)" String
  ./utils/set_ssm_value.sh "cla-ddb-api-logging-${STAGE}" true String
  ./utils/set_ssm_value.sh "cla-otel-datadog-api-logging-${STAGE}" true String
fi

./utils/get_ssm_value.sh "cla-dd-site-${STAGE}"
./utils/get_ssm_value.sh "cla-dd-version-${STAGE}"
./utils/get_ssm_value.sh "cla-dd-api-key-secret-arn-${STAGE}"
./utils/get_ssm_value.sh "cla-dd-extension-layer-arn-${STAGE}"
./utils/get_ssm_value.sh "cla-ddb-api-logging-${STAGE}"
./utils/get_ssm_value.sh "cla-otel-datadog-api-logging-${STAGE}"
