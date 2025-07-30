## Whitelisting Bots

You can allow specific bot users to automatically pass the CLA check. 

This can be done on the GitHub organization level by setting the `skip_cla` property on `cla-{stage}-github-orgs` DynamoDB table.

This property is a Map attribute that contains mapping from repository pattern to bot username and email pattern.

Each pattern is a string and can be one of three possible types:
- `"name"` - exact match for repository name, GitHub username, or email address.
- `"re:regexp"` - regular expression match for repository name, GitHub username, or email address.
- `"*"` - matches all.

So the format is like `"repository_pattern": "github_username_pattern;email_pattern"`.

There can be multiple entries under one Github Organization DynamoDB entry.

Example:
```
{
(...)
    "organization_name": {
      "S": "linuxfoundation"
    },
    "skip_cla": {
      "M": {
        "*": {
          "S": "copilot-swe-agent[bot];*"
        },
        "repo1": {
          "S": "re:vee?rendra;*"
        }
      }
    },
(...)
}
```

Algorithm to match pattern is as follows:
- First we check repository name for exact match. Repository name is without the organization name, so for `https://github.com/linuxfoundation/easycla` it is just `easycla`. If we find an entry in `skip_cla` for `easycla` that entry is used and we stop searching.
- If no exact match is found, we check for regular expression match. Only keys starting with `re:` are considered. If we find a match, we use that entry and stop searching.
- If no match is found, we check for `*` entry. If it exists, we use that entry and stop searching.
- If no match is found, we don't skip CLA check.
- Now when we have the entry, it is in the following format: `github_username_pattern;email_pattern`.
- We check both GitHub username and email address against the patterns. Algorith is the same - username and email patterns can be either direct match or `re:regexp` or `*`.
- If both username and email match the patterns, we skip CLA check. If username or email is not set but the pattern is `*` it means hit.
- So setting pattern to `username_pattern;*` means that we only check for username match and assume all emails are valid.
- If we set `repo_pattern` to `*` it means that this configuration applies to all repositories in the organization. If there are also specific repository patterns, they will be checked first.


There is a script that allows you to update the `skip_cla` property in the DynamoDB table. It is located in `utils/skip_cla_entry.sh`. You can run it like this:
- `` MODE=mode ./utils/skip_cla_entry.sh 'org-name' 'repo-pattern' 'github-username-pattern' 'email-pattern' ``.
- `` MODE=add-key ./utils/skip_cla_entry.sh 'sun-test-org' '*' 'copilot-swe-agent[bot]' '*' ``.

`MODE` can be one of:
- `put-item`: Overwrites/adds the entire `skip_cla` property. Needs all 4 arguments org, repo, username and email.
- `add-key`: Adds or updates a key/value inside the `skip_cla` map (preserves other keys). Needs all 4 args.
- `delete-key`: Removes a key from the `skip_cla` map. Needs 2 arguments: org and repo.
- `delete-item`: Deletes the entire `skip_cla` item. Needs 1 argument: org.


You can also use AWS CLI to update the `skip_cla` property. Here is an example command:

To add a new `skip_cla` entry:

```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item \
  --table-name "cla-prod-github-orgs" \
  --key '{"organization_name": {"S": "linuxfoundation"}}' \
  --update-expression 'SET skip_cla = :val' \
  --expression-attribute-values '{":val": {"M": {"re:^easycla":{"S":"copilot-swe-agent[bot];*"}}}}'
```

To add a new key to an existing `skip_cla` entry (or replace the existing key):

```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item \
  --table-name "cla-prod-github-orgs" \
  --key '{"organization_name": {"S": "linuxfoundation"}}' \
  --update-expression "SET skip_cla.#repo = :val" \
  --expression-attribute-names '{"#repo": "re:^easycla"}' \
  --expression-attribute-values '{":val": {"S": "copilot-swe-agent[bot];*"}}'
```

To delete a key from an existing `skip_cla` entry:

```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item \
  --table-name "cla-prod-github-orgs" \
  --key '{"organization_name": {"S": "linuxfoundation"}}' \
  --update-expression "REMOVE skip_cla.#repo" \
  --expression-attribute-names '{"#repo": "re:^easycla"}'
```

To delete the entire `skip_cla` entry:

```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item \
  --table-name "cla-prod-github-orgs" \
  --key '{"organization_name": {"S": "linuxfoundation"}}' \
  --update-expression "REMOVE skip_cla"
```

To see given organization's entry: `./utils/scan.sh github-orgs organization_name sun-test-org`.

Or using AWS CLI:

``` 
aws --profile "lfproduct-prod" dynamodb scan --table-name "cla-prod-github-orgs" --filter-expression "contains(organization_name,:v)" --expression-attribute-values "{\":v\":{\"S\":\"linuxfoundation\"}}" --max-items 100 | jq -r '.Items'
```

