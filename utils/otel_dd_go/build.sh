#!/bin/bash
go fmt otel_dd.go && go vet otel_dd.go && go build otel_dd.go
