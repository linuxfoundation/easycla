cd cla-backend-go
go build -o bin/cla-fbsd main.go
cd ..
source setenv.sh
# source setenv-prod.sh.secret
GH_ORG_VALIDATION=false PORT=8080 ./cla-backend-go/bin/cla-fbsd
# ~/get_oauth_token.sh && vim auth0.token.secret
# curl -s localhost:8080/v4/ops/health
# remote: GITHUB_ID=2469783 GITHUB_USERNAME=lukaszgryglicki ./utils/my_clas.sh
# local non-admin: PRINCIPAL=lgryglicki ADMIN=false GITHUB_ID=2469783 GITHUB_USERNAME=lukaszgryglicki ./utils/my_clas.sh
# local admin: PRINCIPAL=lgryglicki ADMIN=true GITHUB_ID=2469783 GITHUB_USERNAME=lukaszgryglicki ./utils/my_clas.sh
# prod:
# ~/get_oauth_token_prod.sh && vim easycla-github-oauth-token.secret
# STAGE=prod TOKEN="$(cat ./easycla-github-oauth-token.secret)" ./utils/get_user_svc.sh lgryglicki
# STAGE=prod TOKEN="$(cat ./easycla-github-oauth-token.secret)" ./utils/get_ddb_user_identities.sh lukaszgryglicki
# PRINCIPAL=lgryglicki ADMIN=false GITHUB_ID=2469783 GITHUB_USERNAME=lukaszgryglicki ./utils/my_clas.sh
# PRINCIPAL=lgryglicki ADMIN=true GITHUB_ID=2469783 GITHUB_USERNAME=lukaszgryglicki ./utils/my_clas.sh
# PRINCIPAL=lgryglicki ADMIN=true GITHUB_ID=2469783 GITHUB_USERNAME=lukaszgryglicki ./utils/my_clas.sh daa3aa69-d1d9-46d4-9aed-8e44365171a0
# PRINCIPAL=lukaszgryglicki ADMIN=false SECONDARY_EMAIL="lgryglicki@cncf.io,lgryglicki@contractor.linuxfoundation.org,lukaszgryglicki1982@gmail.com,lukaszgryglicki@o2.pl,justacakala@o2.pl" ./utils/my_clas.sh
# PRINCIPAL=lgryglicki ADMIN=false EMAIL="lgryglicki@cncf.io,lgryglicki@contractor.linuxfoundation.org,lukaszgryglicki1982@gmail.com,lukaszgryglicki@o2.pl,justacakala@o2.pl" ./utils/my_clas.sh
# wget -O icla.pdf 'https://cla-signature-files-prod.s3.amazonaws.com/contract-group/d8cead54-92b7-48c5-a2c8-b1e295e8f7f1/icla/29df5aa6-a396-4543-968c-f6e15f011d43/daa3aa69-d1d9-46d4-9aed-8e44365171a0.pdf?X-Amz-Algorithm[...]'
# xpdf icla.pdf
# rm icla.pdf
