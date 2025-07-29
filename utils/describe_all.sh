#!/bin/bash
if [ -z "$STAGE" ]
then
  export STAGE=dev
fi
if [ -z "$REGION" ]
then
  export REGION=us-east-1
fi
> all-tables.secret
./utils/list_tables.sh | sed 's/[", ]//g' | grep -v '^$' | while read -r table; do
  tab="${table#cla-${STAGE}-}"
  echo -n "Processing table $tab ..."
  echo "Table: $tab" >> all-tables.secret
  ALL=1 ./utils/scan.sh "${tab}" >> all-tables.secret
  echo 'done'
done
