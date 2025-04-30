#!/bin/bash
./utils/copy_prod_to_dev.sh users user_id 37c914df-f9de-4c38-ad6a-cbe4945b5db8
./utils/copy_prod_to_dev.sh companies company_id be081f45-237f-4c51-82e4-814d0590fe9c
./utils/copy_prod_to_dev.sh projects project_id 749a0166-38fe-4587-8b67-718d1d5d7ecf
# ECLA (this user has no ICLA)
./utils/copy_prod_to_dev.sh signatures signature_id de437689-9945-4a9a-a124-a41c4c6abb00
# CCLA
./utils/copy_prod_to_dev.sh signatures signature_id bb3e77a1-5562-4da4-9d06-2343d485b860
