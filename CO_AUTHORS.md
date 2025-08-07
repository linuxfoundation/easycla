## Co-Authors support

You can allow co-authors support on a specific repository, set of repositories matching regular expression pattern, or for all repositories in a GitHub organization.

This can be done on the GitHub organization level by setting the `enable_co_authors` property on `cla-{stage}-github-orgs` DynamoDB table.

Replace `{stage}` with either `dev` or `prod`.

This property is a map attribute that contains mapping from repository pattern to co-authors support enabled or disabled.

So the format is like `"repository_pattern": true|false`.

There can be multiple entries under one Github Organization DynamoDB entry.

Example:
```
{
(...)
    "organization_name": {
      "S": "linuxfoundation"
    },
    "enable_co_authors": {
      "M": {
        "*": {
          "BOOL": true,
        },
        "re:(?i)^repo[0-9]+$": {
          "BOOL": false,
        }
      }
    },
(...)
}
```

This enables co-authors support on all repos under "linuxfoundation", except for those matching the regular expression `re:(?i)^repo[0-9]+$`.

Algorithm to match pattern is as follows:
- First we check repository name for exact match. Repository name is without the organization name, so for `https://github.com/linuxfoundation/easycla` it is just `easycla`. If we find an entry in `enable_co_authors` for `easycla` that entry is used and we stop searching.
- If no exact match is found, we check for regular expression match. Only keys starting with `re:` are considered. If we find a match, we use that entry and stop searching.
- If no match is found, we check for `*` entry. If it exists, we use that entry and stop searching.
- If no match is found, we don't support co-authors for that repository. Default is no co-author support.


There is a script that allows you to update the `enable_co_authors` property in the DynamoDB table. It is located in `utils/enable_co_authors_entry.sh`. You can run it like this:
- `` MODE=mode ./utils/enable_co_authors_entry.sh 'org-name' 'repo-pattern' t ``.
- `` MODE=add-key ./utils/enable_co_authors_entry.sh 'sun-test-org' '*' f ``.

`MODE` can be one of:
- `put-item`: Overwrites/adds the entire `enable_co_authors` property. Needs all 3 arguments org, repo, and pattern.
- `add-key`: Adds or updates a key/value inside the `enable_co_authors` map (preserves other keys). Needs all 3 args.
- `delete-key`: Removes a key from the `enable_co_authors` map. Needs 2 arguments: org and repo.
- `delete-item`: Deletes the entire `enable_co_authors` from the item. Needs 1 argument: org.


You can also use AWS CLI to update the `enable_co_authors` property. Here is an example command:

To add a new `enable_co_authors` entry:

```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item \
  --table-name "cla-prod-github-orgs" \
  --key '{"organization_name": {"S": "linuxfoundation"}}' \
  --update-expression 'SET enable_co_authors = :val' \
  --expression-attribute-values '{":val": {"M": {"re:^easycla":{"BOOL": true}}}}'
```

To add a new key to an existing `enable_co_authors` entry (or replace the existing key):

```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item \
  --table-name "cla-prod-github-orgs" \
  --key '{"organization_name": {"S": "linuxfoundation"}}' \
  --update-expression "SET enable_co_authors.#repo = :val" \
  --expression-attribute-names '{"#repo": "re:^easycla"}' \
  --expression-attribute-values '{":val": {"BOOL": false}}'
```

To delete a key from an existing `enable_co_authors` entry:

```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item \
  --table-name "cla-prod-github-orgs" \
  --key '{"organization_name": {"S": "linuxfoundation"}}' \
  --update-expression "REMOVE enable_co_authors.#repo" \
  --expression-attribute-names '{"#repo": "re:^easycla"}'
```

To delete the entire `enable_co_authors` entry:

```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item \
  --table-name "cla-prod-github-orgs" \
  --key '{"organization_name": {"S": "linuxfoundation"}}' \
  --update-expression "REMOVE enable_co_authors"
```

To see given organization's entry: `./utils/scan.sh github-orgs organization_name sun-test-org`.

Or using AWS CLI:

```
aws --profile "lfproduct-prod" dynamodb scan --table-name "cla-prod-github-orgs" --filter-expression "contains(organization_name,:v)" --expression-attribute-values "{\":v\":{\"S\":\"linuxfoundation\"}}" --max-items 100 | jq -r '.Items'
```

To check for log entries related to skipping CLA check, you can use the following command: `` STAGE=dev DTFROM='1 hour ago' DTTO='1 second ago' ./utils/search_aws_log_group.sh 'cla-backend-dev-githubactivity' 'enable_co_authors' ``.

# Example setup on prod

To add first `enable_co_authors` value for an organization:
```
aws --profile lfproduct-prod --region us-east-1 dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "open-telemetry"}}' --update-expression 'SET enable_co_authors = :val' --expression-attribute-values '{":val": {"M": {"otel-arrow":{"BOOL":true}}}}'
aws --profile lfproduct-prod --region us-east-1 dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "openfga"}}' --update-expression 'SET enable_co_authors = :val' --expression-attribute-values '{":val": {"M": {"vscode-ext":{"BOOL":true}}}}'
```

To add additional repositories entries without overwriting the existing `enable_co_authors` value:
```
aws --profile lfproduct-prod --region us-east-1 dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "open-telemetry"}}' --update-expression 'SET enable_co_authors.#repo = :val' --expression-attribute-names '{"#repo": "*"}' --expression-attribute-values '{":val": {"BOOL": true}}'
aws --profile lfproduct-prod --region us-east-1 dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "openfga"}}' --update-expression 'SET enable_co_authors.#repo = :val' --expression-attribute-names '{"#repo": "*"}' --expression-attribute-values '{":val": {"BOOL": true}}'
```

To delete a specific repo entry from `enable_co_authors`:
```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "open-telemetry"}}' --update-expression 'REMOVE enable_co_authors.#repo' --expression-attribute-names '{"#repo": "*"}'
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "openfga"}}' --update-expression 'REMOVE enable_co_authors.#repo' --expression-attribute-names '{"#repo": "*"}'
```

To delete the entire `enable_co_authors` attribute:
```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "open-telemetry"}}' --update-expression 'REMOVE enable_co_authors'
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "openfga"}}' --update-expression 'REMOVE enable_co_authors'
```

To check values:
```
aws --profile "lfproduct-prod" dynamodb scan --table-name "cla-prod-github-orgs" --filter-expression "contains(organization_name,:v)" --expression-attribute-values "{\":v\":{\"S\":\"open-telemetry\"}}" --max-items 100 | jq -r '.Items'
aws --profile "lfproduct-prod" dynamodb scan --table-name "cla-prod-github-orgs" --filter-expression "contains(organization_name,:v)" --expression-attribute-values "{\":v\":{\"S\":\"openfga\"}}" --max-items 100 | jq -r '.Items'
aws --profile "lfproduct-prod" dynamodb scan --table-name "cla-prod-github-orgs" --filter-expression "contains(organization_name,:v)" --expression-attribute-values "{\":v\":{\"S\":\"open-telemetry\"}}" --max-items 100 | jq -r '.Items[0].enable_co_authors.M["otel-arrow"]["BOOL"]'
aws --profile "lfproduct-prod" dynamodb scan --table-name "cla-prod-github-orgs" --filter-expression "contains(organization_name,:v)" --expression-attribute-values "{\":v\":{\"S\":\"openfga\"}}" --max-items 100 | jq -r '.Items[0].enable_co_authors.M["vscode-ext"]["BOOL"]'
```

Typical adding a new entry for an organization:
```
STAGE=prod MODE=add-key DEBUG=1 ./utils/enable_co_authors_entry.sh 'open-telemetry' 'opentelemetry-rust' t
```


# How co-authors are processed

When a commit is made to a repository that has co-authors support enabled, the backend will check if the commit message contains `co-authored-by:` lines/commit trailers (case insensitive).

If it does, the backend will process the co-authors as follows, assume trailer value is `name <email>` like `Lukasz Gryglicki <lukaszgryglicki@o2.pl>`:

- First we check if email is in format `nuber+username@users.noreply.github.com`. If it is we use number part as GitHub user ID and fetch the user from GitHub API. If the user is found, we use that user as co-author.

- Second we check if email is in format `username@users.noreply.github.com`. If it is we use username part as GitHub username/login and fetch the user from GitHub API. If the user is found, we use that user as co-author.

- Thirs we lookup for email using GitHub API. If the user is found, we use that user as co-author.

- Finally we use the name part for `name <email>` and lookup using GitHub API assuming that this name is GitHub username/login (this is the case for some bots). If the user is found, we use that user as co-author.

We use internal caching while doing all those lookups with cache key `name` and `email` and TTL 24 hours. We even cache by `(name, email)` when nothing is found because this is the most time consuming option. It will have a chance to be found in the future (up to 24 hours from lookup).
