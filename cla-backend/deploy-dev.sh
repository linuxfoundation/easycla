#!/usr/bin/env bash

# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# The golang lambda file list
declare -a golang_files=( "backend-aws-lambda"
   "user-subscribe-lambda"
   "metrics-aws-lambda"
   "dynamo-events-lambda"
   "zipbuilder-scheduler-lambda"
   "zipbuilder-lambda"
   "functional-tests")

echo "Installing dependencies..."
yarn install

echo "Testing if the lambdas have been copied over..."
if [[ ! -f "backend-aws-lambda" ]] || \
  [[ ! -f "user-subscribe-lambda" ]] || \
  [[ ! -f "metrics-aws-lambda" ]] || \
  [[ ! -f "dynamo-events-lambda" ]] || \
  [[ ! -f "zipbuilder-scheduler-lambda" ]] || \
  [[ ! -f "zipbuilder-lambda" ]] || \
  [[ ! -f "functional-tests" ]]; then
    echo "Missing one or more golang files - building golang binaries..."
    pushd "../cla-backend-go"
    make all-linux
    popd
    echo "Copying over files..."
    cp ${golang_files} .
fi

for i in "${golang_files[@]}"; do
  echo "Testing file: ${i}..."
  if ! diff -q "../cla-backend-go/${i}" "${i}" &>/dev/null; then
    echo "Golang file differs: ../cla-backend-go/${i} ${i}"
    exit 1
  fi
done


yarn deploy:dev
