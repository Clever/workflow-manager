package main

import (
	"context"
	"log"
	"os"

	_ "embed"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

// Handler encapsulates the external dependencies of the lambda function.
type Handler struct {
	launchConfig LaunchConfig
}

// Handle is invoked by the Lambda runtime with the contents of the function input.
func (h Handler) Handle(ctx context.Context, input interface{}) error {
	// create a request-specific logger, attach it to ctx, and add the Lambda request ID.
	ctx = logger.NewContext(ctx, logger.New(os.Getenv("APP_NAME")))
	if lambdaContext, ok := lambdacontext.FromContext(ctx); ok {
		logger.FromContext(ctx).AddContext("aws-request-id", lambdaContext.AwsRequestID)
	}
	logger.FromContext(ctx).InfoD("received", logger.M{
		"foo": "bar",
	})

	return nil
}

func main() {
	handler := Handler{
		launchConfig: InitLaunchConfig(),
	}

	if os.Getenv("IS_LOCAL") == "true" {
		// Update input as needed to debug
		var input interface{}
		log.Printf("Running locally with this input: %+v\n", input)
		err := handler.Handle(context.Background(), input)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		lambda.Start(handler.Handle)
	}
}
