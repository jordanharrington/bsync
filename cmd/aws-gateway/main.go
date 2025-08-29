package main

import (
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/core"
	"github.com/awslabs/aws-lambda-go-api-proxy/gorillamux"
	"github.com/jordanharrington/bsync/api/v1"
	"github.com/jordanharrington/bsync/internal/handlers"
	"github.com/jordanharrington/bsync/internal/server"
	"log"
)

func main() {
	h, err := handlers.Create(context.Background(), v1.ProviderAWS)
	if err != nil {
		log.Fatalf("failed to create handler: %v", err)
	}

	adapter := gorillamux.New(server.NewRouter(h))
	lambda.Start(func(ctx context.Context, req core.SwitchableAPIGatewayRequest) (interface{}, error) {
		return adapter.ProxyWithContext(ctx, req)
	})
}
