#!/bin/bash
find cypress/ -type f -iname "*.ts" -exec  npx prettier --write "{}" \;
find cypress/ -type f -iname "*.js" -exec  npx prettier --write "{}" \;
