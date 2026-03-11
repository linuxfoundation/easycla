module github.com/linuxfoundation/easycla/cla-backend-legacy

go 1.25

require (
	github.com/aws/aws-lambda-go v1.47.0
	github.com/aws/aws-sdk-go-v2 v1.37.0
	github.com/aws/aws-sdk-go-v2/config v1.28.0
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.15.0
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.34.6
	github.com/aws/aws-sdk-go-v2/service/s3 v1.58.0
	github.com/aws/aws-sdk-go-v2/service/ses v1.31.0
	github.com/aws/aws-sdk-go-v2/service/sns v1.31.0
	github.com/aws/aws-sdk-go-v2/service/ssm v1.36.0
	github.com/awslabs/aws-lambda-go-api-proxy v0.16.0
	github.com/go-chi/chi/v5 v5.0.12
	github.com/golang-jwt/jwt/v4 v4.5.1
	github.com/google/uuid v1.6.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.52.0
	go.opentelemetry.io/otel v1.27.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.27.0
	go.opentelemetry.io/otel/sdk v1.27.0
	go.opentelemetry.io/otel/trace v1.27.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.3 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.41 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.22.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.3.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.9.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.17.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.24.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.28.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.32.2 // indirect
	github.com/aws/smithy-go v1.22.5 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.27.0 // indirect
	go.opentelemetry.io/otel/metric v1.27.0 // indirect
	go.opentelemetry.io/proto/otlp v1.2.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240520151616-dc85e6b867a5 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240515191416-fc5f0ca64291 // indirect
	google.golang.org/grpc v1.64.0 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
)
