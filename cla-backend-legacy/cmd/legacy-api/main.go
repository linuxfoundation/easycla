package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/server"
)

func main() {
	h := server.NewHTTPHandler()
	adapter := httpadapter.New(h)

	// API Gateway (REST / v1) proxy integration
	lambda.Start(adapter.Proxy)
}
