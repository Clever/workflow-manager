package embedded

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Clever/workflow-manager/embedded/sfnfunction"
	"github.com/Clever/workflow-manager/executor/sfnconventions"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

var log = logger.New("wfm-embedded")

// PollForWork begins polling for work. It stops when the context is canceled
// or an un-recoverable error is encountered.
func (e *Embedded) PollForWork(ctx context.Context) error {
	// register activities with AWS and spawn GetActivityTask polling loops
	g, ctx := errgroup.WithContext(ctx)
	for resourceName, resource := range e.resources {
		activityArn := sfnconventions.EmbeddedResourceArn(resourceName, e.sfnRegion, e.sfnAccountID, e.environment, e.app)
		activityArnParts := strings.Split(activityArn, ":")
		activityName := activityArnParts[len(activityArnParts)-1]
		createOutput, err := e.sfnAPI.CreateActivityWithContext(ctx, &sfn.CreateActivityInput{
			Name: aws.String(activityName),
		})
		if err != nil {
			return fmt.Errorf("creating activity: %s", err)
		}
		log.InfoD("startup", logger.M{"activity": *createOutput.ActivityArn})
		r := resource
		g.Go(func() error {
			return e.pollGetActivityTask(ctx, r, *createOutput.ActivityArn)
		})
	}
	return g.Wait()
}

func (e *Embedded) pollGetActivityTask(ctx context.Context, resource *sfnfunction.Resource, activityArn string) error {
	// allow one GetActivityTask per second, max 1 at a time
	limiter := rate.NewLimiter(rate.Every(1*time.Second), 1)
	for ctx.Err() == nil {
		if err := limiter.Wait(ctx); err != nil {
			continue
		}
		select {
		case <-ctx.Done():
			log.Info("getactivitytask-stop")
		default:
			log.TraceD("getactivitytask-start", logger.M{"activity-arn": activityArn, "worker-name": e.workerName})
			out, err := e.sfnAPI.GetActivityTaskWithContext(ctx, &sfn.GetActivityTaskInput{
				ActivityArn: aws.String(activityArn),
				WorkerName:  aws.String(e.workerName),
			})
			if err != nil {
				if err == context.Canceled || awsErr(err, request.CanceledErrorCode) {
					log.Info("getactivitytask-stop")
					continue
				}
				log.ErrorD("getactivitytask-error", logger.M{"error": err.Error()})
				continue
			}
			if out.TaskToken == nil {
				continue
			}
			input := *out.Input
			token := *out.TaskToken
			log.TraceD("getactivitytask", logger.M{"input": input, "token": shortToken(token)})
			e.handleTask(ctx, resource, token, input)
		}
	}
	return nil
}

func shortToken(token string) string {
	shasum := fmt.Sprintf("%x", md5.Sum([]byte(token)))
	if len(shasum) > 5 {
		return shasum[0:5]
	}
	return shasum
}

// handleTask sends heartbeats to SFN, invokes the resource function, and
// reports to SFN the result.
func (e *Embedded) handleTask(ctx context.Context, resource *sfnfunction.Resource, token, input string) {
	// Create a context to run the heartbeat and the function in parallel.
	// Add the token as an identifier in the logger attached to the ctx.
	c, cancel := context.WithCancel(ctx)
	g, ctx := errgroup.WithContext(c)
	ctx = logger.NewContext(ctx, logger.New("wfm-embedded"))
	logger.FromContext(ctx).AddContext("token", shortToken(token))

	// Send heartbeats on an interval
	g.Go(func() error {
		defer func() {
			logger.FromContext(ctx).Trace("heartbeat-end")
		}()
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-heartbeat.C:
				if err := sendTaskHeartbeat(ctx, e.sfnAPI, token); err != nil {
					return err
				}
			}
		}
	})

	// Pass the input to the function.
	g.Go(func() error {
		defer func() {
			cancel() // this will kill the heartbeat goroutine
		}()
		result := resource.Call(ctx, input)
		if result.Failure != nil {
			return sendTaskFailure(ctx, e.sfnAPI,
				aws.StringValue(result.Failure.Error),
				aws.StringValue(result.Failure.Cause),
				token)
		} else if result.Success != nil {
			return sendTaskSuccess(ctx, e.sfnAPI,
				aws.StringValue(result.Success.Output),
				token)
		}
		return errors.New("unexpected result")
	})

	if err := g.Wait(); err != nil {
		logger.FromContext(ctx).ErrorD("internal-error", logger.M{"error": err.Error()})
		sendTaskFailure(ctx, e.sfnAPI, "InternalError", err.Error(), token)
		return
	}
}

func sendTaskHeartbeat(ctx context.Context, sfnAPI sfniface.SFNAPI, token string) error {
	if _, err := sfnAPI.SendTaskHeartbeatWithContext(ctx, &sfn.SendTaskHeartbeatInput{
		TaskToken: aws.String(token),
	}); err != nil {
		if awsErr(err, sfn.ErrCodeInvalidToken, sfn.ErrCodeTaskDoesNotExist, sfn.ErrCodeTaskTimedOut) {
			return err
		}
		if err == context.Canceled || awsErr(err, request.CanceledErrorCode) {
			// context was canceled while sending heartbeat, this is a signal to shut it down
			return nil
		}
		log.ErrorD("heartbeat-error-unknown", logger.M{"error": err.Error()}) // should investigate unknown/unclassified errors
	}
	logger.FromContext(ctx).Trace("send-task-heartbeat")
	return nil
}

func sendTaskFailure(ctx context.Context, sfnAPI sfniface.SFNAPI, errorName, cause, taskToken string) error {
	// don't use SendTaskFailureWithContext, since the failure itself could
	// be from the context timing out, etc., and we still want to report
	// to AWS the failure
	_, err := sfnAPI.SendTaskFailure(&sfn.SendTaskFailureInput{
		Error:     aws.String(errorName),
		Cause:     aws.String(cause),
		TaskToken: aws.String(taskToken),
	})
	if err != nil {
		logger.FromContext(ctx).ErrorD("send-task-failure-error", logger.M{"error": err.Error()})
	}
	logger.FromContext(ctx).ErrorD("send-task-failure", logger.M{
		"error": errorName,
		"cause": cause,
	})
	return err
}

func sendTaskSuccess(ctx context.Context, sfnAPI sfniface.SFNAPI, output string, taskToken string) error {
	if _, err := sfnAPI.SendTaskSuccessWithContext(ctx, &sfn.SendTaskSuccessInput{
		Output:    aws.String(output),
		TaskToken: aws.String(taskToken),
	}); err != nil {
		logger.FromContext(ctx).ErrorD("send-task-success-error", logger.M{"error": err.Error()})
		return err
	}
	logger.FromContext(ctx).DebugD("send-task-success", logger.M{"output": output})
	return nil
}

func awsErr(err error, codes ...string) bool {
	if err == nil {
		return false
	}
	if aerr, ok := err.(awserr.Error); ok {
		for _, code := range codes {
			if aerr.Code() == code {
				return true
			}
		}
	}
	return false
}
