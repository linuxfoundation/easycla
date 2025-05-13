// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
module github.com/communitybridge/easycla/cla-backend-go

go 1.20

replace github.com/awslabs/aws-lambda-go-api-proxy => github.com/LF-Engineering/aws-lambda-go-api-proxy v0.3.2

require (
	github.com/LF-Engineering/aws-lambda-go-api-proxy v0.3.2
	github.com/LF-Engineering/lfx-kit v0.1.39
	github.com/LF-Engineering/lfx-models v0.7.9
	github.com/aws/aws-lambda-go v1.22.0
	github.com/aws/aws-sdk-go v1.36.27
	github.com/aymerick/raymond v2.0.2+incompatible
	github.com/bitly/go-simplejson v0.5.0 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869
	github.com/davecgh/go-spew v1.1.1
	github.com/gin-gonic/gin v1.7.7
	github.com/go-openapi/errors v0.20.2
	github.com/go-openapi/loads v0.21.0
	github.com/go-openapi/runtime v0.21.1
	github.com/go-openapi/spec v0.20.6
	github.com/go-openapi/strfmt v0.21.3
	github.com/go-openapi/swag v0.21.1
	github.com/go-openapi/validate v0.20.3
	github.com/go-playground/validator/v10 v10.7.0 // indirect
	github.com/go-resty/resty/v2 v2.3.0
	github.com/gofrs/uuid v4.0.0+incompatible
	github.com/golang/mock v1.6.0
	github.com/google/go-github/v37 v37.0.0
	github.com/google/uuid v1.1.4
	github.com/gorilla/sessions v1.2.1 // indirect
	github.com/imroc/req v0.3.0
	github.com/jessevdk/go-flags v1.4.0
	github.com/jinzhu/copier v0.0.0-20190924061706-b57f9002281a
	github.com/jmoiron/sqlx v1.2.0
	github.com/juju/mempool v0.0.0-20160205104927-24974d6c264f // indirect
	github.com/juju/zip v0.0.0-20160205105221-f6b1e93fa2e2
	github.com/kr/pretty v0.3.0 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/mitchellh/mapstructure v1.5.0
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/mozillazg/request v0.8.0 // indirect
	github.com/pdfcpu/pdfcpu v0.3.5-0.20200802160406-be1e0eb55afc
	github.com/rs/cors v1.7.0
	github.com/savaki/dynastore v0.0.0-20171109173440-28d8558bb429
	github.com/shurcooL/githubv4 v0.0.0-20201206200315-234843c633fa
	github.com/shurcooL/graphql v0.0.0-20200928012149-18c5c3165e3a // indirect
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.7.0
	github.com/spf13/viper v1.12.0
	github.com/stretchr/testify v1.8.4
	github.com/verdverm/frisby v0.0.0-20170604211311-b16556248a9a
	github.com/xanzy/go-gitlab v0.50.1
	go.uber.org/ratelimit v0.1.0
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d // indirect
	golang.org/x/net v0.8.0
	golang.org/x/oauth2 v0.6.0
	golang.org/x/sync v0.2.0
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	google.golang.org/protobuf v1.30.0 // indirect
)

require (
	github.com/bradleyfalzon/ghinstallation/v2 v2.2.0
	github.com/golang-jwt/jwt/v4 v4.5.0
)

require (
	github.com/ProtonMail/go-crypto v0.0.0-20230321155629-9a39f2531310 // indirect
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d // indirect
	github.com/aws/aws-sdk-go-v2/service/s3 v1.53.1 // indirect
	github.com/aws/smithy-go v1.20.2 // indirect
	github.com/cloudflare/circl v1.3.2 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-openapi/analysis v0.21.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-playground/locales v0.13.0 // indirect
	github.com/go-playground/universal-translator v0.17.0 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-github/v50 v50.2.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/gorilla/securecookie v1.1.1 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.6.8 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hhrutter/lzw v0.0.0-20190829144645-6f07a24e8650 // indirect
	github.com/hhrutter/tiff v0.0.0-20190829141212-736cae8d0bc7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pelletier/go-toml/v2 v2.0.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.6.1 // indirect
	github.com/spf13/afero v1.9.5 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.4.1 // indirect
	github.com/ugorji/go/codec v1.2.6 // indirect
	go.mongodb.org/mongo-driver v1.10.1 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
