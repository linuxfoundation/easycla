#!/bin/bash
pytest -vvv -s cla/tests/unit/test_docusign_models.py -p no:warnings -k test_request_individual_signature
