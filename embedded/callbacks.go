// TODO
// add polling loop for sqs
// manually test: example workflow def with callback, callback is received in
// polling loop. Print it out. Use CLI to send task success
// Things that could go wrong:
//  - policy on queue could be wrong
package embedded

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"golang.org/x/time/rate"
	errors "golang.org/x/xerrors"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

// CallbackTaskFinalizer finishes tasks asynchronously.
type CallbackTaskFinalizer interface {
	Failure(taskKey string, err error) error
	Success(taskKey string, result interface{}) error
	QueueURL() string
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

type sqsMessageBody struct {
	TaskToken string `json:"TaskToken"`
	TaskKey   string `json:"TaskKey"`
}

func (f *finalizer) poll(ctx context.Context) error {
	consecutiveReceiveErrors := 0
	receiveLimiter := rate.NewLimiter(rate.Every(50*time.Millisecond), 1)
	logger.FromContext(ctx).InfoD("ewfm-sqs-poll-start", logger.M{"queue": f.sqsQueueURL})
	for {
		select {
		case <-ctx.Done():
			logger.FromContext(ctx).Info("ewfm-sqs-poll-done")
			return nil
		default:
			receiveLimiter.Wait(ctx)
			out, err := f.sqsAPI.ReceiveMessageWithContext(ctx, &sqs.ReceiveMessageInput{
				MaxNumberOfMessages: aws.Int64(1),
				QueueUrl:            aws.String(f.sqsQueueURL),
				WaitTimeSeconds:     aws.Int64(10),
			})
			if err != nil {
				err = errors.Errorf("ReceiveMessage: %v", err)
				logger.FromContext(ctx).ErrorD("ewfm-sqs-error", logger.M{
					"error": err,
				})
				consecutiveReceiveErrors++
				// if erroring consistently, a receivemessage error is considered terminal
				if consecutiveReceiveErrors == 10 {
					return err
				}
				continue
			}
			consecutiveReceiveErrors = 0
			if len(out.Messages) > 1 {
				return errors.Errorf("received %d messages, expected at most 1", len(out.Messages))
			} else if len(out.Messages) == 0 {
				continue
			}
			msg := out.Messages[0]
			var msgBody sqsMessageBody
			if err := json.Unmarshal([]byte(aws.StringValue(msg.Body)), &msgBody); err != nil {
				logger.FromContext(ctx).ErrorD("ewfm-sqs-error", logger.M{
					"error": errors.Errorf("Unmarshal: %v", err),
				})
				continue
			}

			// save the message in ddb
			logger.FromContext(ctx).ErrorD("ewfm-TODO-SAVE", logger.M{
				"msg": aws.StringValue(msg.Body),
			})
			//if, err := f.ddbAPI.PutItemWithContext(ctx, dynamodb.PutItemInput{
			//}); err != nil {

			if _, err := f.sqsAPI.DeleteMessageWithContext(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(f.sqsQueueURL),
				ReceiptHandle: msg.ReceiptHandle,
			}); err != nil {
				logger.FromContext(ctx).ErrorD("ewfm-sqs-error", logger.M{
					"body":  msgBody,
					"error": errors.Errorf("DeleteMessage: %v", err),
				})
				continue
			}
		}
	}
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

	go f.poll(ctx)
	return f, nil
}

func sqsQueueName(env, app string) string {
	return fmt.Sprintf("%s--%s-callbacks", env, app)
}

func sqsQueueURL(region, account, queueName string) string {
	return fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s", region, account, queueName)
}

//	ddbTableName := fmt.Sprintf("%s--%s-callbacks", config.Environment, config.App)
func ensureSQSQueueForCallbacks(ctx context.Context, sqsAPI sqsiface.SQSAPI, config *Config) error {
	//sourceArnLike := []string{fmt.Sprintf("arn:aws:sns:*:%s:*", account))}
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
	// if _, err := sqsAPI.SetQueueAttributesWithContext(ctx, &sqs.SetQueueAttributesInput{
	// 	Attributes: map[string]*string{
	// 		"Policy": "",
	// 	},
	// 	QueueUrl *string `type:"string" required:"true"`
	// }); err != nil {

	// }
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
