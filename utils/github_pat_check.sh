#!/bin/bash
# Quick test for GitHub PAT GraphQL permissions
# ./utils/github_pat_check.sh "$(cat ./easycla-github-oauth-token.secret)" finos fluxnova-modeler 85
# ./utils/github_pat_check.sh "$(cat /etc/github/oauth)" cncf devstats 114

if ( [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] )
then
    echo "Usage: $0 YOUR_GITHUB_TOKEN org repo pr-number"
    echo "Example: $0 ghp_xxxxxxxxxxxxxxxxxxxx cncf devstats 114"
    exit 1
fi

TOKEN="$1"
ORG="$2"
REPO="$3"
PR="$4"
echo "Testing GitHub PAT GraphQL with token: ${TOKEN:0:10} for $ORG/$REPO PR:$PR..."

echo -e "\n1. Testing Simple GraphQL Query:"
echo "--------------------------------"
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"query":"query { viewer { login } }"}' \
     https://api.github.com/graphql

echo -e "\n\n2. Checking Token Scopes:"
echo "------------------------"
curl -s -I -H "Authorization: Bearer $TOKEN" \
     https://api.github.com/user | grep -i "x-oauth-scopes" || echo "No scopes header found"

echo -e "\n\n3. Testing Repository Access:"
echo "----------------------------"
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"query":"query { repository(owner:\"'"$ORG"'\", name:\"'"$REPO"'\") { name } }"}' \
     https://api.github.com/graphql

echo -e "\n\n4. Testing PR Commits (the failing query):"
echo "------------------------------------------"
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"query":"query { repository(owner:\"'"$ORG"'\", name:\"'"$REPO"'\") { pullRequest(number:'$PR') { commits(first:1) { nodes { commit { oid } } } } } }"}' \
     https://api.github.com/graphql

echo -e "\n"
