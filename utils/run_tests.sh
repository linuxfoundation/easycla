#!/bin/bash
# PY=1
# GO=1
if [ ! -z "$PY" ]
then
  cd cla-backend && pytest "cla/tests" -p no:warnings
  # pytest -vvv -s cla/tests/unit/test_docusign_models.py -p no:warnings -k test_request_individual_signature
  cd ..
else
  echo "$0: skipping python backend tests, specify PY=1 to run them"
fi

if [ ! -z "$GO" ]
then
  cd cla-backend-go && make test
  # go test github.com/linuxfoundation/easycla/cla-backend-go/signatures
  cd ..
else
  echo "$0: skipping golang backend tests, specify GO=1 to run them"
fi

