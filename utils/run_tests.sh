#!/bin/bash
# PY=1
# GO=1
if [ ! -z "$PY" ]
then
  cd cla-backend && pytest "cla/tests" -p no:warnings
  cd ..
else
  echo "$0: skipping python backend tests, specify PY=1 to run them"
fi

if [ ! -z "$GO" ]
then
  cd cla-backend-go && make test
  cd ..
else
  echo "$0: skipping golang backend tests, specify GO=1 to run them"
fi

