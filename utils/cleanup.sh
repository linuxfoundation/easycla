#!/bin/bash
rm -rf .git .gitbook/ cla-backend/auth/bin/ cla-backend/cryptography-layer.zip 
find . -iname "*.secret" -exec rm -f "{}" \;
