# Infrastructure DevOps

This folder contains a number of infrastructure support routines for
deploying and managing EasyCLA with various AWS environments.

## Prerequisites

- AWS Account Details and Credentials
- AWS [Command Line Tools](https://aws.amazon.com/cli/)

## Scripts

A few command line helper scripts are provided for assisting with S3 and
DyanamoDB backup and restore functions. In general, run the scripts without any
arguments to see the script usage.

| Script                    | Description |
|:--------------------------|:------------|
| backup-dynamodb-tables.sh | Backups up the DyanamoDB tables (creates a snapshot) |
| backup-s3-buckets.sh      | Backup S3 bucket contents to the local folder |
| restore-s3-buckets.sh     | Restores the S3 bucket contents to the S3 bucket |
| update-s3-permissions.sh  | Updates the S3 bucket permissions - some items are exptected to be public-read |
| logger.sh                 | Helper utility script for output log date/time stamp |
| colors.sh                 | Helper utility script for colorized output |
| utils.sh                  | Helper utility script with a couple convenience routines |

## Backup Files from S3 to Local

Occasionally, there is a need to backup and restore data from a S3 bucket.
This section describes the backup procedure going from S3 to a local folder.

The AWS CLI makes working with files in S3 very easy. However, the file
globbing available on most Unix/Linux systems is not quite as easy to use
with the AWS CLI. S3 doesn’t have folders, but it does use the concept of
folders by using the "/" character in S3 object keys as a folder delimiter.

To copy all objects in an S3 bucket to your local machine simply use the aws
s3 cp command with the `--recursive` option.

For example `aws s3 cp s3://my-s3-bucket/ ./ --recursive` will copy all
files from the "my-s3-bucket" bucket to the current working directory on
your local machine. If there are folders represented in the object keys (keys
containing "/" characters), they will be downloaded as separate directories
in the target location.

1. First, establish your AWS credentials for the environment
2. Then run the copy command with the `--recursive` flag

Example:

```bash
mkdir ./cla-project-logo-staging
aws s3 cp s3://cla-project-logo-staging/ ./cla-project-logo-staging --recursive
mkdir ./cla-signature-files-staging
aws s3 cp s3://cla-signature-files-staging/ ./cla-signature-files-staging --recursive
```

Optionally, we wrote a convenience script:

```bash
# Usage:
backup-s3-buckets.sh <environment>

# Examples:
backup-s3-buckets.sh dev
backup-s3-buckets.sh staging
backup-s3-buckets.sh prod
```

> Note: when using the scripts, you may want to review and adjust the local
> backup folder name.

## Restore Files From Local to S3

Occasionally, there is a need to backup and restore data from a S3 bucket.
This section describes the restore procedure going from a local folder to a
S3 bucket.

1. First, establish your AWS credentials for the environment
1. Then run the copy command with the `--recursive` flag

```bash
aws s3 mb s3://cla-project-logo-staging
aws s3 cp cla-project-logo-staging/ s3://cla-project-logo-staging/ --recursive
aws s3 mb s3://cla-signature-files-staging
aws s3 cp cla-signature-files-staging/ s3://cla-signature-files-staging/ --recursive
```

[Full instructions](https://aws.amazon.com/getting-started/tutorials/backup-to-s3-cli/)

Optionally, we wrote a convenience script:

```bash
# Usage:
restore-s3-buckets.sh <environment>

# Examples:
restore-s3-buckets.sh dev
restore-s3-buckets.sh staging
restore-s3-buckets.sh prod
```

> Note: when using the scripts, you may want to review and adjust the local
> backup folder name.

