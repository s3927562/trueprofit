package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"

	"backend/internal/handlers"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("load aws config: %v", err)
	}

	h := handlers.NewAskHandler(cfg)

	_ = os.Getenv("AWS_LAMBDA_FUNCTION_NAME")

	lambda.Start(h.Handle)
}
