// Webhook Trigger Lambda entry point
package main

import (
	"github.com/aws/aws-lambda-go/lambda"

	"loan-eligibility-engine/internal/handlers"
	"loan-eligibility-engine/internal/utils"
)

func main() {
	// Initialize logger
	_ = utils.InitLogger("info")
	defer utils.Sync()

	// Create handler
	handler := handlers.NewWebhookTriggerHandler()

	// Start Lambda
	lambda.Start(handler.Handle)
}
