#!/bin/bash
if [ -z "$DD_API_KEY_SECRET_ARN" ]
then
  source setenv.sh
fi
if [ -z "$DD_API_KEY_SECRET_ARN" ]
then
  echo "DD_API_KEY_SECRET_ARN is not set. Please set it in setenv.sh and try again."
  exit 1
fi

docker run --rm -it \
  -p 4318:4318 \
  -p 8888:8888 \
  -e DD_API_KEY -e DD_SITE \
  -v "$PWD/utils/otel_dd/otelcol-dd.yaml:/etc/otelcol/config.yaml:ro" \
  -v /etc/passwd:/etc/passwd:ro \
  otel/opentelemetry-collector-contrib:latest \
  --config /etc/otelcol/config.yaml
