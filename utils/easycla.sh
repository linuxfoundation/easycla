#!/bin/bash
# run via [NOMFA=1] . ./easycla.sh
cd ./easycla/cla-backend
source .venv/bin/activate
cd ..
if [ -z "$NOMFA" ]
then
  awsmfa.sh
fi
source setenv.sh
env | grep AWS_SESSION_TOKEN
