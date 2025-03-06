#!/bin/bash
echo "copy project"
./utils/copy_prod_to_dev.sh projects project_id 43c546ff-bc79-4a32-9454-77dabd6afaee
echo "copy user"
./utils/copy_prod_to_dev.sh users user_id 65d22813-1ac0-4292-bb68-fdcb278473a5
echo "copy company"
./utils/copy_prod_to_dev.sh companies company_id 4930fe6e-e023-4f56-9767-6f1996a7b730
echo "copy signature (CCLA)"
./utils/copy_prod_to_dev.sh signatures signature_id b90452c9-97b3-411e-9004-b58260297fcb
echo "copy signature (ECLA)"
./utils/copy_prod_to_dev.sh signatures signature_id 167e04e4-650a-40ef-a138-c1087201231e

echo "project:"
./utils/scan.sh projects project_id 43c546ff-bc79-4a32-9454-77dabd6afaee
echo "user:"
./utils/scan.sh users user_id 65d22813-1ac0-4292-bb68-fdcb278473a5
echo "company:"
./utils/scan.sh companies company_id 4930fe6e-e023-4f56-9767-6f1996a7b730
echo "CCLA signature:"
./utils/scan.sh signatures signature_id b90452c9-97b3-411e-9004-b58260297fcb
echo "ECLA signature:"
./utils/scan.sh signatures signature_id 167e04e4-650a-40ef-a138-c1087201231e
