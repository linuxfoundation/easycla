#!/bin/bash
qid=$(aws --profile lfproduct-dev --region us-east-1 logs start-query   --log-group-name /aws/lambda/cla-backend-dev-githubactivity   --start-time "$(date -u -d '2026-04-21 13:05:00' +%s)"   --end-time "$(date -u -d '2026-04-21 13:20:00' +%s)"   --query-string '                                                                                                        
fields @timestamp, @logStream, @message
| filter @message like /\/v2\/github\/activity/
   or @message like /active_pr:u:lukaszgryglicki/
   or @message like /792406957/                      
   or @message like /pull_request/
| sort @timestamp asc
| limit 100                                 
' | jq -r ".queryId")
aws --profile lfproduct-dev --region us-east-1 logs get-query-results --query-id "$qid"

qid=$(aws --profile lfproduct-dev --region us-east-2 logs start-query   --log-group-name /aws/lambda/cla-backend-go-api-v4-lambda   --start-time "$(date -u -d '2026-04-21 13:05:00' +%s)"   --end-time "$(date -u -d '2026-04-21 13:20:00' +%s)"   --query-string '
fields @timestamp, @logStream, functionName, level, msg, repositoryID, pullRequestID, projectID, @message
| filter @message like /pullRequestID.:56/
   or @message like /PR: 56/
   or @message like /792406957/
   or @message like /updateChangeRequestLegacyCompat/
   or @message like /GetPullRequestCommitAuthors/
   or @message like /GetCommitAuthorsSignedStatuses/
   or @message like /Created success status/
| sort @timestamp asc
| limit 200
' | jq -r ".queryId")
aws --profile lfproduct-dev --region us-east-2 logs get-query-results --query-id "$qid"
