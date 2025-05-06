#!/bin/bash
# 10bde6b1-3061-4972-9c6a-17dd9a175a5c - dev LF
# 0ca30016-6457-466c-bc41-a09560c1f9bf - dev CNCF
# 0016s000006UKKqAAO - dev LF SFID
# 0014100000Te0yqAAB - dev CNCF SFID
./utils/update_company_is_sanctioned.sh 0ca30016-6457-466c-bc41-a09560c1f9bf true
# Python APIs
./utils/get_company_py.sh 0ca30016-6457-466c-bc41-a09560c1f9bf
./utils/get_user_py.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5
./utils/request_employee_signature_py_post.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5 0ca30016-6457-466c-bc41-a09560c1f9bf 88ee12de-122b-4c46-9046-19422054ed8d github 'http://localhost'
./utils/request_corporate_signature_py_post.sh 0ca30016-6457-466c-bc41-a09560c1f9bf 88ee12de-122b-4c46-9046-19422054ed8d github 'http://localhost'
# Golang APIs
V=v3 ./utils/get_company_go.sh '0ca30016-6457-466c-bc41-a09560c1f9bf'
V=v4 ./utils/get_company_go.sh '0ca30016-6457-466c-bc41-a09560c1f9bf'
./utils/get_company_by_name_go.sh 'Cloud Native Computing Foundation'
./utils/request_corporate_signature_go_post.sh 0014100000Te0yqAAB lfbrdgbVFK7QngqnzD github 'http://localhost'
