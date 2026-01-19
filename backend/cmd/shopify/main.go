package main

import (
	"backend/internal/handlers"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handlers.ShopifyHandler)
}
