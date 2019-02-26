package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/Clever/workflow-manager/embedded"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
)

const sfnRegion = "us-east-2"

var sfnAccountID = strings.Replace(os.Getenv("AWS_ACCOUNT_NUMBER"), "-", "", -1)

// handlers for state transitions follow some rules (copied from aws-lambda-go)
// Rules:
//
// 	* handler must be a function
// 	* handler may take between 0 and two arguments.
// 	* if there are two arguments, the first argument must implement "context.Context".
// 	* handler may return between 0 and two arguments.
// 	* if there are two return values, the second argument must implement "error".
// 	* if there is one return value it must implement "error".
//
// Valid function signatures:
//
// 	func ()
// 	func () error
// 	func (TIn) error
// 	func () (TOut, error)
// 	func (TIn) (TOut, error)
// 	func (context.Context) error
// 	func (context.Context, TIn) error
// 	func (context.Context) (TOut, error)
// 	func (context.Context, TIn) (TOut, error)
//
// Where "TIn" and "TOut" are types compatible with the "encoding/json" standard library.

type Foo struct {
	Bar string
}

func first(ctx context.Context) (Foo, error) {
	return Foo{Bar: "world"}, nil
}

func second(ctx context.Context, input Foo) (string, error) {
	return fmt.Sprintf("Hello, %s!", input.Bar), nil
}

func main() {
	wfdefs, err := ioutil.ReadFile("./workflowdefinitions.yml")
	if err != nil {
		log.Fatal(err)
	}
	_, err = embedded.New(&embedded.Config{
		Environment:  "clever-dev",
		App:          "example",
		SFNAccountID: sfnAccountID,
		SFNRegion:    sfnRegion,
		SFNAPI: sfn.New(session.New(&aws.Config{
			Region: aws.String(sfnRegion),
		})),
		Resources: map[string]interface{}{
			"first":  first,
			"second": second,
		},
		WorkflowDefinitions: wfdefs,
	})
	if err != nil {
		log.Fatal(err)
	}
}
