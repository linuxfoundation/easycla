# V1/V2 python APIs still used

1. EasyCLA backend [here](https://github.com/linuxfoundation/easycla/blob/main/.github/workflows/deploy-prod.yml#L127).
- `/v2/health`. Health check while `dev`/`prod` deployment.


2. EasyCLA contributor console [here](https://github.com/communitybridge/easycla-contributor-console/blob/main/src/app/core/services/cla-contributor.service.ts), found via `` find.sh . '*' /vN ``.
- `/v1/api/health` [here](https://github.com/communitybridge/easycla-contributor-console/blob/main/test/functional/cypress/integration/api-tests/health-check.spec.ts#L4).
- `/v1/user/gerrit`.
- `/v2/user/<userId>`. Porting to `/v3/user-compat/<userId>`.
- `/v2/project/<projectId>`. Ported to `/v4/project-compat/<projectId>`.
- `/v2/user/<userId>/active-signature`. Ported to `/v4/user/<userId>/active-signature`.
- `/v2/check-prepare-employee-signature`.
- `/v2/request-employee-signature`.
- `/v2/user/<userId>/project/<projectId>/last-signature`.


3. EasyCLA corporate console [here](https://github.com/LF-Engineering/lfx-corp-cla-console/blob/main/backend/src/data/cla-api.ts):
- No V1 or V2 APIs used.


4. PCC [here](https://github.com/linuxfoundation/lfx-pcc/blob/main/apps/v1-backend/src/modules/cla-services/model/index.ts):
- No V1 or V2 APIs used.


5. GitHub/Gitlab/Gerrit ({provider}) app/bot [here]() (there is no list of which particular APIs are used by GitHub/GitLab/Gerrit):
- `/v2/github/activity`.
- `/v2/repository-provider/{provider}/sign/{installation_id}/{github_repository_id}/{change_request_id}`.
- `/v2/github/installation`.
- `/v1/user/gerrit`.
- `/v2/signed/individual/{installation_id}/{github_repository_id}/{change_request_id}` ?
- `/v2/repository-provider/{provider}/activity` ?
- `/v2/repository-provider/{provider}/oauth2_redirect` ?
- `/v2/signed/gitlab/individual/{user_id}/{organization_id}/{gitlab_repository_id}/{merge_request_id}` ?
- `/v2/signed/gerrit/individual/{user_id}` ?
- `cla-backend/cla/routes.py`: `GitHub Routes`, `Gerrit Routes`, `Gerrit instance routes`.


6. Manually check which APIs were actually called on `dev` and `prod` via:

- `prod` analysis: `` DEBUG=1 NO_ECHO=1 STAGE=prod REGION=us-east-1 DTFROM='10 days ago' DTTO='1 second ago' OUT=api-logs-prod.json ./utils/search_aws_logs.sh 'LG:api-request-path' && jq -r '.[].message' api-logs-prod.json | grep -o 'LG:api-request-path:[^[:space:]]*' | sed 's/^LG:api-request-path://' | sed -E 's/[0-9a-fA-F-]{36}/<uuid>/g' | sed -E 's/\b[0-9]{2,}\b/<id>/g' | sort | uniq -c | sort -nr ``:
```
 529509 /v2/github/activity
   2572 /v2/repository-provider/github/sign/<id>/<id>/<id>
    534 /v2/github/installation
    234 /v2/return-url/<uuid>
    228 /v2/check-prepare-employee-signature
     89 /v2/request-employee-signature
     16 /v1/file/icon/seo/<uuid>.png
     13 /v2/gerrit/<uuid>/corporate/agreementUrl.html
      9 /v1/user/gerrit
      2 /v2/health
```

- `dev` analysis (but this can contain API calls made by developer and not actually used): `` DEBUG=1 STAGE=dev REGION=us-east-1 DTFROM='10 days ago' DTTO='1 second ago' OUT=api-logs-dev.json ./utils/search_aws_logs.sh 'LG:api-request-path' && jq -r '.[].message' api-logs-dev.json | grep -o 'LG:api-request-path:[^[:space:]]*' | sed 's/^LG:api-request-path://' | sed -E 's/[0-9a-fA-F-]{36}/<uuid>/g' | sed -E ':a;s#/([0-9]{1,})(/|$)#/<id>\2#g;ta' | sort | uniq -c | sort -nr ``:
```
     39 /v2/github/activity
     13 /v2/repository-provider/github/sign/<id>/<id>/<id>
     12 /v2/user-from-token
      8 /v2/github/installation
      6 /v2/health
      1 /v2/return-url/<uuid>
      1 /v2/gerrit/<uuid>/individual/agreementUrl.html
      1 /v2/check-prepare-employee-signature
```


