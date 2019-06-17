// TODO
// add polling loop for sqs
// manually test: example workflow def with callback, callback is received in
// polling loop. Print it out. Use CLI to send task success
// Things that could go wrong:
//  - policy on queue could be wrong
package embedded

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	errors "golang.org/x/xerrors"
)

// CallbackTaskFinalizer finishes tasks asynchronously.
type CallbackTaskFinalizer interface {
	Failure(taskKey string, err error) error
	Success(taskKey string, result interface{}) error
	QueueURL() string
}

// NewWithCallbacks returns a client to an embedded workflow manager.
// The context passed will be used to poll SQS.
func NewWithCallbacks(ctx context.Context, config *Config) (*Embedded, CallbackTaskFinalizer, error) {
	e, err := New(config)
	if err != nil {
		return nil, nil, errors.Errorf("New: %v", err)
	}

	// ensure sqs queue and dynamodb table for capturing task tokens
	f, err := newFinalizer(ctx, config)
	if err != nil {
		return nil, nil, errors.Errorf("newFinalizer: %v", err)
	}
	return e, f, nil
}

func newFinalizer(
	ctx context.Context,
	config *Config,
) (*finalizer, error) {
	sess := session.New(&aws.Config{Region: &config.SFNRegion})
	sqsAPI := sqs.New(sess)
	ddbAPI := dynamodb.New(sess)
	sfnAPI := sfn.New(sess)
	if err := ensureSQSQueueForCallbacks(ctx, sqsAPI, config); err != nil {
		return nil, errors.Errorf("ensureSQSQueueForCallbacks: %v", err)
	} /*else if err := ensureDynamoDBTableForCallbacks(ctx, ddbAPI, ddbTableName); err != nil {
		return nil, nil, errors.Errorf("ensureDynamoDBTableForCallbacks: %v", err)
	}*/

	f := &finalizer{
		sqsAPI:      sqsAPI,
		sqsQueueURL: sqsQueueURL(config.SFNRegion, config.SFNAccountID, sqsQueueName(config.Environment, config.App)),
		ddbAPI:      ddbAPI,
		//ddbTable:    ddbTable,
		sfnAPI: sfnAPI,
	}

	//go f.poll(ctx)
	return f, nil
}

func sqsQueueName(env, app string) string {
	return fmt.Sprintf("%s--%s-callbacks", env, app)
}

func sqsQueueURL(region, account, queueName string) string {
	return fmt.Sprintf("https://%s.queue.amazonaws.com/%s/%s", region, account, queueName)
}

//	ddbTableName := fmt.Sprintf("%s--%s-callbacks", config.Environment, config.App)
func ensureSQSQueueForCallbacks(ctx context.Context, sqsAPI sqsiface.SQSAPI, config *Config) error {
	// 	sourceArnLike := []string{}
	// 	for _, account := range accounts {
	// 		sourceArnLike = append(sourceArnLike, fmt.Sprintf("arn:aws:sns:*:%s:*", account))
	// 	}
	// 	sourceArnLikeBs, _ := json.Marshal(sourceArnLike)
	// 	policy := func(arn string) string {
	// 		return fmt.Sprintf(`
	// {
	//   "Version": "2012-10-17",
	//   "Id": "queue-policy",
	//   "Statement": [
	//     {
	//       "Sid": "allow-sns-to-publish-from-my-accounts",
	//       "Effect": "Allow",
	//       "Principal": "*",
	//       "Action": "sqs:SendMessage",
	//       "Condition": {
	//         "ArnLike": {
	//           "aws:SourceArn": %s
	//         }
	//       },
	//       "Resource": "%s"
	//     }
	//   ]
	// }`, string(sourceArnLikeBs), arn)
	// 	}
	// 	ps := strings.Split(sqsArn, ":")
	// 	if len(ps) != 6 {
	// 		return nil, errors.Errorf("unexpected ARN: %s", sqsArn)
	// 	}
	// 	account := ps[4]
	queueName := fmt.Sprintf("%s--%s-callbacks", config.Environment, config.App)
	dlQueueName := queueName + "-dlq"
	if _, err := sqsAPI.CreateQueueWithContext(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(dlQueueName),
	}); err != nil && !awsErr(err, sqs.ErrCodeQueueNameExists) {
		return errors.Errorf("CreateQueue(%s): %v", dlQueueName, err)
	}
	// queue exists, get its ARN
	out, err := sqsAPI.GetQueueAttributesWithContext(ctx, &sqs.GetQueueAttributesInput{
		AttributeNames: []*string{aws.String("QueueArn")},
		QueueUrl:       aws.String(sqsQueueURL(config.SFNRegion, config.SFNAccountID, dlQueueName)),
	})
	if err != nil {
		return errors.Errorf("GetQueueAttributes: %v", err)
	}
	dlqARN := aws.StringValue(out.Attributes["QueueArn"])
	if _, err := sqsAPI.CreateQueueWithContext(ctx, &sqs.CreateQueueInput{
		Attributes: map[string]*string{
			"RedrivePolicy": aws.String(fmt.Sprintf(`{"deadLetterTargetArn":"%s","maxReceiveCount":10}`, dlqARN)),
		},
		QueueName: aws.String(queueName),
	}); err != nil && !awsErr(err, sqs.ErrCodeQueueNameExists) {
		return errors.Errorf("CreateQueue(%s): %v", queueName, err)
	}

	return nil
}

type finalizer struct {
	sqsAPI      sqsiface.SQSAPI
	sqsQueueURL string
	ddbAPI      dynamodbiface.DynamoDBAPI
	ddbTable    string
	sfnAPI      sfniface.SFNAPI
}

var _ CallbackTaskFinalizer = &finalizer{}

func (f *finalizer) Failure(taskKey string, err error) error {
	return nil
}
func (f *finalizer) Success(taskKey string, result interface{}) error {
	return nil
}
func (f *finalizer) QueueURL() string {
	return f.sqsQueueURL
}
