# DEV

## CLA Local Development

Copyright The Linux Foundation and each contributor to CommunityBridge.

SPDX-License-Identifier: CC-BY-4.0

## Prerequisites

- Go 1.14.x
- Python 3
- Node 8/12+

## Node Setup

NodeJS is used by [serverless](https://www.serverless.com/) to facilitate
deployment and location testing. In all but few occasions, we use NodeJS v12+.

The frontend UI still requires an older version of NodeJS - version 8. This is
due to compatibility issues with the Angular and Ionic toolchain for the legacy
UI. Use node version 8.x for the frontend source package installation
(typically the `src` folder) and build/deploy commands.
For all other folders which use node, use version 12.x+.

In order to quickly switch between node versions, we recommend you use
[Node Version Manager - nvm](https://github.com/nvm-sh/nvm). The CircleCI
[build configuration](https://github.com/communitybridge/easycla/blob/master/.circleci/config.yml)
uses this approach to switch between node versions within the build andx
deployment.

## Building and Running the Python Backend

These are the steps for setting up a local dev environment for the python
backend (work in progress). The legacy Python backend is used for
the Gerrit and GitHub interaction, Docusign workflows, and a few other API
calls.

Historical and Current Endpoints:

- /v1 (us-east-1)
- /v2 (us-east-1)
- /v1/salesforce/projects - handles project management console project listing queries (us-east-1)
- /v1/salesforce/project - handles project management console project queries (us-east-1)
- /v2/github/installation - handles github bot installation (us-east-1)
- /v2/github/activity - handles github activity callbacks to the product (us-east-1)

### Install Python

You will need to install a local Python 3.6+ runtime - Python 3.7 will work now
that we've updated the `hug` library. 

One easy way to install a Python runtime on a Mac is via the `pipenv` tool.
Here, we'll use it to install a specific version of Python.  There are other
approaches to installing Python. Feel free to add your preferred approach to the
information below.

```bash
brew install pipenv
```

To create a new Python installation using `pipenv`, specify the specific version
using the `--python VERSION` flag, like so:

```bash
pipenv --python 3.7
```

When given a Python version, like this, Pipenv will automatically scan your
system for a Python that matches that given version and download one if
necessary.

### Configure the Virtual Environment

You can use `pipenv` or any other tool to configure a local sandbox for testing.
We prefer to use `virtualenv`.

```bash
pip3 install virtualenv
```

Now setup the virtual env using python 3.6+:

```bash
# Check your version and grab the appropriate path
python --version

which python

# Setup the virtual env
cd cla-backend
virtualenv --python=/usr/local/Cellar/python3/3.7.4/bin/python3 .venv
```

This will create a `.venv` folder in the `cla-backend` project folder. We'll
want to load the sandbox environment by using:

```bash
source .venv/bin/activate
```

### Install the Requirements

If not already done, load the virtual environment. Then we can install the
required modules:

```bash
source .venv/bin/activate
pip3 install -r requirements.txt
```

This will install the dependencies in the `.venv` path.

### Setup Environment Variables

We'll need a few run-time environment variables to communicate with AWS when
running locally.

- `AWS_REGION` - the AWS region, normally this should be set to `us-east-1`
- `AWS_ACCESS_KEY_ID` - AWS key, used to authenticate to AWS for DynamoDB and SSM
- `AWS_SECRET_ACCESS_KEY` - AWS secret key, used to authenticate to AWS for DynamoDB and SSM
- `PRODUCT_DOMAIN` - e.g. `export PRODUCT_DOMAIN=dev.lfcla.com`
- `ROOT_DOMAIN` - e.g. `export ROOT_DOMAIN=lfcla.dev.platform.linuxfoundation.org`

Optional environment variables:

- `PORT` - optional, the HTTP port when running in the local mode. The default port is 5000.
- `STAGE` - optional, specifies the environment stage. The default is typically `dev`.

For testing locally, you'll want to point to the dev environment or stand up a
local DynamoDB and S3 instance. Generally, we run the backend services and UI
locally and simply point to the DEV environment. The `STAGE` environment
variable controls where we point. Make sure you export/provide/setup the AWS
properties in order to connect.


When running on Linux it looks like `.venv` sets $HOME to /tmp, and then python backend is looking for the AWS config file in `~/.aws/config`
This means it ends up in `/tmp/.aws/config`. You can use the following scritp to activate your environment (`setenv.secret`) via: `source setenv.secret`:
```
#!/bin/bash
rm -rf /tmp/aws
cp -R ~/.aws /tmp/.aws
export AWS_SDK_LOAD_CONFIG=1
export AWS_PROFILE='lfproduct-dev'
export AWS_REGION='us-east-1'
export AWS_ACCESS_KEY_ID='[redacted]'
export AWS_SECRET_ACCESS_KEY='[redacted]'
export PRODUCT_DOMAIN='dev.lfcla.com'
export ROOT_DOMAIN='lfcla.dev.platform.linuxfoundation.org'
export PORT='5000'
export STAGE='dev'
```

And the following one to unset the environment:
```
#!/bin/bash
rm -rf /tmp/.aws
unset AWS_PROFILE
unset AWS_REGION
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY
unset PRODUCT_DOMAIN
unset ROOT_DOMAIN
unset PORT
unset STAGE
```

## Run the Python Backend

```bash
# ok to use node v12+
cd cla-backend
yarn install
```

This will install the `serverless` tool and plugins.

```bash
# You can simply run this launcher:
# ok to use node v12+
yarn serve:dev

# Or run it using this approach - points to the DEV database/environment
node_modules/serverless/bin/serverless wsgi serve -s 'dev' 
```

To test:

```bash
# Health Check
open http://localhost:5000/v2/health

# Get User by ID
open http://localhost:5000/v2/user/<some_uuid_from_users_table>
```

## Building and Running the Go Backend

Current Endpoints:

- /v3 (us-east-1) - considered part of EasyCLA v1 which leverages the older (legacy) UI
- /v4 (us-east-2) - considered EasyCLA v2 which leverages the newer LFX UI and LFX Admin Consoles - includes integration with other platform services
- plus a number of support lambdas

### Install Go

Follow the [Go Getting Started](https://golang.org/doc/install) guide to install the tool.

After installation, you should have something similar to:

```bash
# After the GO installation, you should have GOROOT set
echo $GOROOT
/usr/local/opt/go/libexec

# And go should be in your path
go version
go version go1.14.6 darwin/amd64
```

### Setup Project Folders

To build the go stuff, the go tool is very specific on paths. If this isn't
correct, some of the build targets, such as `swagger` will fail with a
nondescript error message. Generally:

```bash
# GOPATH also should be set, depends on where your code would generally be located
echo $GOPATH
# response would be something like this:
/Users/<my-os-user-id>/projects/go

# if `GOPATH` is not set, you will need to explictly set it:
export GOPATH=$HOME/projects/go # or wherever your go projects will be located
# most sane people set this in their `~/.bashrc` or `~/.zshrc` file so they
# don't have to remember to set it again

# If the `GOPATH` variable is NOT set, run: export GOPATH=<your_go_path>
# Inside of this folder you should traditionally have `src`, `bin`, and `pkg` folders
# To recreate them, run (needed for the first-time setup only):
pushd $GOPATH && mkdir -p src bin pkg && popd

# If you have code checked out in a different path than the GOPATH (as in the
# case of CLA, probably), you will need to establish a soft link to point where
# you checked out the public easycla source - this must follow the EasyCLA go path
# structure (e.g. the directory path of the source code on disk must match the
# code import paths). To create the path structure, run:
mkdir -p $GOPATH/src/github.com/communitybridge
pushd $GOPATH/src/github.com/communitybridge
ln -s <path_to_where_you_really_have_easycla_checked_out>/easycla easycla
popd

# Confirm everything:
ls -la $GOPATH/src/github.com/communitybridge/easycla
# Should see something like:
lrwxr-xr-x  1 ddeal  staff  29 Jul  7 14:48 /Users/ddeal/projects/go/src/github.com/communitybridge/easycla@ -> /Users/ddeal/projects/easycla
```

### Configure Git

EasyCLA currently relies on several private libraries for user authorization and
common data models and other libraries:

- github.com/LF-Engineering/lfx-kit (authorization library)
- github.com/LF-Engineering/lfx-models (common data models)
- github.com/LF-Engineering/aws-lambda-go-api-proxy (forked library which includes query parameters fixes)

The near-term plan is to migrate some of these private libraries to the 
github.com/communitybridge organization as public repositories.

Until that time, each user needs to request access to these repositories 
and create/update the following file: `~/.gitconfig` to include the following:

```code
[url "ssh://git@github.com/"]
  insteadOf = https://github.com/
```

This GoLang build file (Makefile) will handle pulling dependencies from the private
repositories.

### Building Go Source

```bash
# Navigate to the go source code and build it - you must be in this path/directory
pushd $GOPATH/src/github.com/communitybridge/easycla/cla-backend-go

# First time only, you will need to install the dev tools.  Simply run:
make setup
```

Once the tools are installed, you can run a clean build with all
the bells and whistles. This will:

- remove the old binaries and support lambdas
- combine the swagger specification fragments into a single compiled swagger file (one for v1, one for v2) - python doesn't generate APIs from a swagger
- Runs swagger-go to generate the API and data models for both v1 and v2 APIs - validates swagger spec
- Downloads Org Service, Project Service, User Service and ACS Service swagger - generates the client stubs
- Downloads any external dependencies
- Builds API server and support lambdas
- formats the code (should be already formatted) - run this before you checkin code - the linter will catch violations
- build the code - use `build` or `build-mac` based on your platform
- test will run unit tests
- lint will run lint checks (do this before checking in code to avoid CI/CD catching the violations later)

Mac:

```bash
make all
# or 
make all-mac

# or everything individually - including the extra lambdas
make clean swagger deps fmt build-mac build-aws-lambda-mac build-metrics-lambda-mac build-dynamo-events-lambda-mac build-zipbuilder-scheduler-lambda-mac build-zipbuilder-lambda-mac test lint
```

Linux:
```bash
make all-linux
# or everything individually - including the extra lambdas
make clean swagger deps fmt build-linux build-aws-lambda-linux build-metrics-lambda-linux build-dynamo-events-lambda-linux build-zipbuilder-scheduler-lambda-linux build-zipbuilder-lambda-linux test lint
```

After the above, you should have the binary now (Mac example):

```bash
ls -lhF cla
-rwxr-xr-x  1 ddeal  staff    36M Jul 18 10:57 cla-mac*
```

The binary is based on your OS, Mac example:

```bash
file cla-mac
cla-mac: Mach-O 64-bit executable x86_64
```

Linux example:

```bash
file cla
cla: ELF 64-bit LSB executable, x86-64, version 1 (SYSV), statically linked, Go BuildID=9KJXkKLbz8QXVJiIStug/13hRaqNdkbT0_CJAC_96/pqAvGfgbdkRS-xcMCFwk/PEhC2JMjFKBawWbErcth, stripped
```

### Setup the Environment

To run the Go backend locally, it requires a few environment variables to be set:

- `AWS_REGION` - the AWS region, normally this should be set to `us-east-1`
- `AWS_ACCESS_KEY_ID` - AWS key, used to authenticate to AWS for DynamoDB and SSM
- `AWS_SECRET_ACCESS_KEY` - AWS secret key, used to authenticate to AWS for DynamoDB and SSM

Optional environment settings:

- `PORT` - optional, the HTTP port when running in local mode. The default port is 8080.
- `STAGE` - optional, specifies the environment stage. The default is `dev`.
- `GH_ORG_VALIDATION` - set to `false` to test locally which will by-pass the GH auth checks and
   allow local functional tests (e.g. with cURL or Postman) - default is enabled/true

### Running

First build and setup the environment.  Then simply run it:

```bash
# Mac
./bin/cla-mac
# or linux
./bin/cla
```

You should see the typical diagnostic details on startup indicating that it
started without errors including build information with successful load of 
configuration parameters. To confirm the service is up and running locally,
connect to the health service using a browser, curl or PostMan:

```bash
# Endpoints
open http://localhost:8080/v3/ops/health
open http://localhost:8080/v4/ops/health
```

## Testing the UI Locally

If testing in local mode, set the `USE_LOCAL_SERVICES=true` environment variable
and review the localhost values in the `cla.service.ts` implementation for each
console. Update the ports as necessary.

For example, to test the corporate console:

```bash
cd cla-frontend-corporate-console
# use node 8
yarn install-frontend

# move to the source folder
cd src

# Ensure the AWS environment is setup
export AWS_PROFILE=cla-dev-profile
export AWS_REGION=us-east-1

# To use local services, set the following value.
# You will need to have the go and python backend up and running locally,
# otherwise it will use the DEV, STAGING, or PROD environment services based on
# the STAGE environment variable and the specific yarn target that you invoke.
export USE_LOCAL_SERVICES=true

# Run locally (code changes auto-deploy) - dev environment settings
# use node 8
yarn serve:dev
```
