## Allowlisting Bots

You can allow specific bot users to automatically pass the CLA check.

This can be done on the GitHub organization level by setting the `skip_cla` property on `cla-{stage}-github-orgs` DynamoDB table.

Replace `{stage}` with either `dev` or `prod`.

This property is a map attribute that contains mapping from repository pattern to bot GitHub login, email and name pattern.

Example `login` is `lukaszgryglicki` (like any `login` that can be accessed via `https://github.com/login`).

This is sometimes called `username` but we use `login` to avoid confusion with the `name` attribute.

Example name is `"Lukasz Gryglicki"`.

Email pattern and name pattern are optional and `""` (empty) is assumed for them if not specified.

Each pattern is a string and can be one of three possible types (and are checked tin this order):
- `"name"` - exact match for repository name, GitHub login, email address, GitHub name.
- `""` - (empty string) pattern is special and it matches missing property, property with null value or property with empty string value.
- `"re:regexp"` - regular expression match for repository name, GitHub login, name, or email address.
- `"*"` - matches all.

So the format is like `"repository_pattern": "login_pattern;email_pattern;name_pattern"`. `;` is used as a separator.

You can also specify multiple patterns so different set is used for multiple users - in such case configuration must start with `[`, end with `]` and be `||` separated.

For example: `"[;*;copilot-swe-agent[bot];||re:(?i)^l(ukasz)?gryglicki$;*;re:Gryglicki]"`.

Full format is like `"repository_pattern": "[login_pattern;email_pattern;name_pattern||..]"`.

Other complex example: `"re:(?i)^repo\d*$": "[veerendra||re:(?i)^l(ukasz)?gryglicki$;lukaszgryglicki@o2.pl||*;*;Lukasz Gryglicki]"`.

This matches one of:
- GitHub login `veerendra` no matter the email and name.
- GitHub login like lgryglicki, LukaszGryglicki and similar with email lukaszgryglicki@o2.pl, name doesn't matter.
- GitHub name "Lukasz Gryglicki" email and login doesn't matter.

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
          "S": ";re:^\\d+\\+Copilot@users\\.noreply\\.github\\.com$;copilot-swe-agent[bot]"
        },
        "re:(?i)^repo[0-9]+$": {
          "S": "re:vee?rendra;*;*"
        }
      }
    },
(...)
}
```

For example for `copilot-swe-agent[bot]` GitHub bot the exact values returned by GitHub are: id, login, name are all nulls, email is like this `198982749+Copilot@users.noreply.github.com`.

Algorithm to match pattern is as follows:
- First we check repository name for exact match. Repository name is without the organization name, so for `https://github.com/linuxfoundation/easycla` it is just `easycla`. If we find an entry in `skip_cla` for `easycla` that entry is used and we stop searching.
- If no exact match is found, we check for regular expression match. Only keys starting with `re:` are considered. If we find a match, we use that entry and stop searching.
- If no match is found, we check for `*` entry. If it exists, we use that entry and stop searching.
- If no match is found, we don't skip CLA check for any author.
- Now when we have the entry, it is in the following format: `login_pattern;email_pattern;name_pattern` or `"[login_pattern;email_pattern;name_pattern||...]" (array)`.
- We check GitHub login, email address and name against the patterns. Algorithm is the same - login, email and name patterns can be either direct match ("" is a special case that also matches missing or null) or `re:regexp` or `*`.
- If login, email and name match the patterns, we skip CLA check. If login, email or name is not set but the pattern is `*` it means hit.
- So setting pattern to `login_pattern;*;*` means that we only check for login match and assume all emails and names are valid.
- Any actor that matches any of the entries in the array will be skipped (logical OR).
- If we set `repo_pattern` to `*` it means that this configuration applies to all repositories in the organization.
- If there are also specific repository patterns, they will be used instead of `*` (fallback for all).


There is a script that allows you to update the `skip_cla` property in the DynamoDB table. It is located in `utils/skip_cla_entry.sh`. You can run it like this:
- `` MODE=mode ./utils/skip_cla_entry.sh 'org-name' 'repo-pattern' 'login-pattern;email-pattern;name_pattern' ``.
- `` MODE=add-key ./utils/skip_cla_entry.sh 'sun-test-org' '*' ';*;copilot-swe-agent[bot]' ``.
- Complex example: `` MODE=add-key ./utils/skip_cla_entry.sh 'sun-test-org' 're:(?i)^repo[0-9]+$' '[re:(?i)^l(ukasz)?gryglicki$;re:(?i)^l(ukasz)?gryglicki@;*||copilot-swe-agent[bot]]' ``.

`MODE` can be one of:
- `put-item`: Overwrites/adds the entire `skip_cla` property. Needs all 3 arguments org, repo, and pattern.
- `add-key`: Adds or updates a key/value inside the `skip_cla` map (preserves other keys). Needs all 3 args.
- `delete-key`: Removes a key from the `skip_cla` map. Needs 2 arguments: org and repo.
- `delete-item`: Deletes the entire `skip_cla` from the item. Needs 1 argument: org.


You can also use AWS CLI to update the `skip_cla` property. Here is an example command:

To add a new `skip_cla` entry:

```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item \
  --table-name "cla-prod-github-orgs" \
  --key '{"organization_name": {"S": "linuxfoundation"}}' \
  --update-expression 'SET skip_cla = :val' \
  --expression-attribute-values '{":val": {"M": {"re:^easycla":{"S":"some-github-login;*;*"}}}}'
```

To add a new key to an existing `skip_cla` entry (or replace the existing key):

```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item \
  --table-name "cla-prod-github-orgs" \
  --key '{"organization_name": {"S": "linuxfoundation"}}' \
  --update-expression "SET skip_cla.#repo = :val" \
  --expression-attribute-names '{"#repo": "re:^easycla"}' \
  --expression-attribute-values '{":val": {"S": "some-github-login;*;*"}}'
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

To check for log entries related to skipping CLA check, you can use the following command: `` STAGE=dev DTFROM='1 hour ago' DTTO='1 second ago' ./utils/search_aws_log_group.sh 'cla-backend-dev-githubactivity' 'skip_cla' ``.

# Example setup on prod

To add first `skip_cla` value for an organization:
```
aws --profile lfproduct-prod --region us-east-1 dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "open-telemetry"}}' --update-expression 'SET skip_cla = :val' --expression-attribute-values '{":val": {"M": {"otel-arrow":{"S":";re:^\\d+\\+Copilot@users\\.noreply\\.github\\.com$;copilot-swe-agent[bot]"}}}}'
aws --profile lfproduct-prod --region us-east-1 dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "openfga"}}' --update-expression 'SET skip_cla = :val' --expression-attribute-values '{":val": {"M": {"vscode-ext":{"S":";re:^\\d+\\+Copilot@users\\.noreply\\.github\\.com$;copilot-swe-agent[bot]"}}}}'
```

To add additional repositories entries without overwriting the existing `skip_cla` value:
```
aws --profile lfproduct-prod --region us-east-1 dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "open-telemetry"}}' --update-expression 'SET skip_cla.#repo = :val' --expression-attribute-names '{"#repo": "*"}' --expression-attribute-values '{":val": {"S": ";re:^\\d+\\+Copilot@users\\.noreply\\.github\\.com$;copilot-swe-agent[bot]"}}'
aws --profile lfproduct-prod --region us-east-1 dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "openfga"}}' --update-expression 'SET skip_cla.#repo = :val' --expression-attribute-names '{"#repo": "*"}' --expression-attribute-values '{":val": {"S": ";re:^\\d+\\+Copilot@users\\.noreply\\.github\\.com$;copilot-swe-agent[bot]"}}'
```

To delete a specific repo entry from `skip_cla`:
```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "open-telemetry"}}' --update-expression 'REMOVE skip_cla.#repo' --expression-attribute-names '{"#repo": "*"}'
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "openfga"}}' --update-expression 'REMOVE skip_cla.#repo' --expression-attribute-names '{"#repo": "*"}'
```

To delete the entire `skip_cla` attribute:
```
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "open-telemetry"}}' --update-expression 'REMOVE skip_cla'
aws --profile "lfproduct-prod" --region "us-east-1" dynamodb update-item --table-name "cla-prod-github-orgs" --key '{"organization_name": {"S": "openfga"}}' --update-expression 'REMOVE skip_cla'
```

To check values:
```
aws --profile "lfproduct-prod" dynamodb scan --table-name "cla-prod-github-orgs" --filter-expression "contains(organization_name,:v)" --expression-attribute-values "{\":v\":{\"S\":\"open-telemetry\"}}" --max-items 100 | jq -r '.Items'
aws --profile "lfproduct-prod" dynamodb scan --table-name "cla-prod-github-orgs" --filter-expression "contains(organization_name,:v)" --expression-attribute-values "{\":v\":{\"S\":\"openfga\"}}" --max-items 100 | jq -r '.Items'
aws --profile "lfproduct-prod" dynamodb scan --table-name "cla-prod-github-orgs" --filter-expression "contains(organization_name,:v)" --expression-attribute-values "{\":v\":{\"S\":\"open-telemetry\"}}" --max-items 100 | jq -r '.Items[0].skip_cla.M["otel-arrow"]["S"]'
aws --profile "lfproduct-prod" dynamodb scan --table-name "cla-prod-github-orgs" --filter-expression "contains(organization_name,:v)" --expression-attribute-values "{\":v\":{\"S\":\"openfga\"}}" --max-items 100 | jq -r '.Items[0].skip_cla.M["vscode-ext"]["S"]'
```

Typical adding a new entry for an organization:
```
STAGE=prod MODE=add-key DEBUG=1 ./utils/skip_cla_entry.sh 'open-telemetry' 'opentelemetry-rust' ';re:^\d+\+Copilot@users\.noreply\.github\.com$;copilot-swe-agent[bot]'
```

