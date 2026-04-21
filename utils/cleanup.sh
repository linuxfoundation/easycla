#!/bin/bash
rm -rf .git .gitbook/ cla-backend/auth/bin/
find . -iname "*.secret" -exec rm -f "{}" \;
