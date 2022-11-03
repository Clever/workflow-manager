package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "embed"

	"github.com/Clever/kayvee-go/v7/logger"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/go-openapi/strfmt"

	"github.com/Clever/workflow-manager/gen-go/models"
	dynamodbgen "github.com/Clever/workflow-manager/gen-go/server/db/dynamodb"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	dynamodbstore "github.com/Clever/workflow-manager/store/dynamodb"
	"github.com/Clever/workflow-manager/wfupdater"
)

var dynamoMaxRetries int = 4

// Handler encapsulates the external dependencies of the lambda function.
type Handler struct {
	store  store.Store
	sfnapi sfniface.SFNAPI
}

func NewHandler(thestore store.Store, sfnapi sfniface.SFNAPI) Handler {
	return Handler{
		store:  thestore,
		sfnapi: sfnapi,
	}
}

// Handle is invoked by the Lambda runtime with the contents of the function input.
func (h Handler) Handle(ctx context.Context, input events.KinesisEvent) error {
	// create a request-specific logger, attach it to ctx, and add the Lambda request ID.
	ctx = logger.NewContext(ctx, logger.New(os.Getenv("APP_NAME")))
	if lambdaContext, ok := lambdacontext.FromContext(ctx); ok {
		logger.FromContext(ctx).AddContext("aws-request-id", lambdaContext.AwsRequestID)
	}
	if err := h.handle(ctx, input); err != nil {
		logger.FromContext(ctx).ErrorD("error", logger.M{
			"error": err.Error(),
		})
		// do not return the error, as we need to guarantee that the lambda continues to process events
	}
	return nil
}

// isGzipped returns whether or not data is Gzipped
func isGzipped(b []byte) bool {
	return b[0] == 0x1f && b[1] == 0x8b
}

func (h Handler) handle(ctx context.Context, input events.KinesisEvent) error {
	for _, rec := range input.Records {
		if err := h.handleRecord(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

func (h Handler) handleRecord(ctx context.Context, rec events.KinesisEventRecord) error {
	data := rec.Kinesis.Data
	if !isGzipped(data) {
		return fmt.Errorf("unexpected data format: %s", base64.StdEncoding.EncodeToString(data))
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	byt, err := ioutil.ReadAll(gzipReader)
	if err != nil {
		return err
	}
	var d events.CloudwatchLogsData
	if err := json.Unmarshal(byt, &d); err != nil {
		return err
	}
	if !strings.HasPrefix(d.LogStream, "states/") {
		logger.FromContext(ctx).InfoD("skipped", logger.M{"group": d.LogGroup, "stream": d.LogStream, "count": len(d.LogEvents)})
		return nil
	}
	logger.FromContext(ctx).AddContext("log-group", d.LogGroup)
	logger.FromContext(ctx).AddContext("log-stream", d.LogStream)
	logger.FromContext(ctx).AddContext("kinesis-seq", rec.Kinesis.SequenceNumber)
	logger.FromContext(ctx).InfoD("received", logger.M{"count": len(d.LogEvents)})
	for _, evt := range d.LogEvents {
		var historyEvent HistoryEvent
		if err := json.Unmarshal([]byte(evt.Message), &historyEvent); err != nil {
			return fmt.Errorf("error decoding message as JSON, message='%s' error='%s'", evt.Message, err)
		}
		if err := h.handleHistoryEvent(ctx, historyEvent); err != nil {
			return err
		}
	}
	return nil
}

// HistoryEvent is the data that SFN delivers to CW Logs.
type HistoryEvent struct {
	// ExecutionARN is the ARN of the execution that the event is related to.
	// Example: arn:aws:states:us-west-2:589690932525:execution:clever-dev--gcr-archive-pipeline-master--0--list_courses_input:91278914-4af8-4fed-ad0d-f79dc86a4c15
	ExecutionARN string `json:"execution_arn"`

	// The id of the event. Events are numbered sequentially, starting at one.
	ID string `json:"id"`

	// Type of the event.
	Type string `json:"type"`

	// Details of the event. Different for each type.
	// We only use this field to capture the output of a workflow when we see an ExecutionSucceeded event, so that is the schema defined here.
	Details struct {
		Output *string `json:"output"`
	} `json:"details"`

	// Timestamp the date and time the event occurred, as a (string) unix millisecond timestamp.
	Timestamp string `json:"event_timestamp"`

	// unused but present in the data delivered to CW Logs:
	/*
		// The id of the previous event.
		PreviousEventID int64 `json:"previous_event_id"`

	*/
}

// execIDFromExecutionARN extracts the execution ID (i.e. our workflow ID) from the execution ARN
// e.g. arn:aws:states:us-west-2:589690932525:execution:clever-dev--sfncli-dummy-worker-master--2--start:cdc2f7f2-787f-4d0e-be4a-e32f427bc824
// -> cdc2f7f2-787f-4d0e-be4a-e32f427bc824
func execIDFromExecutionARN(arn string) string {
	parts := strings.Split(arn, ":")
	return parts[len(parts)-1]
}

// stateMachineFromExecutionARN extracts the state machine name from the execution ARN
// e.g. arn:aws:states:us-west-2:589690932525:execution:clever-dev--sfncli-dummy-worker-master--2--start:cdc2f7f2-787f-4d0e-be4a-e32f427bc824
// -> clever-dev--sfncli-dummy-worker-master--2--start
func stateMachineFromExecutionARN(arn string) string {
	parts := strings.Split(arn, ":")
	return parts[len(parts)-2]
}

// unixMilli returns the local Time corresponding to the given Unix time, msec milliseconds since January 1, 1970 UTC.
// todo: available in go 1.17 as time.UnixMilli
func unixMilli(msec int64) time.Time {
	return time.Unix(msec/1e3, (msec%1e3)*1e6)
}

func ptrStatus(s models.WorkflowStatus) *models.WorkflowStatus {
	return &s
}

func swallowOutOfOrderStateError(ctx context.Context, err error) error {
	if errors.Is(err, store.ErrUpdatingWorkflowFromTerminalToNonTerminalState) {
		logger.FromContext(ctx).WarnD("workflow-already-terminal", logger.M{"err": err.Error()})
		return nil
	}
	return err
}

func (h Handler) handleHistoryEvent(ctx context.Context, evt HistoryEvent) error {
	execID := execIDFromExecutionARN(evt.ExecutionARN)
	smName := stateMachineFromExecutionARN(evt.ExecutionARN)
	logger.FromContext(ctx).AddContext("execution-id", execID)
	logger.FromContext(ctx).AddContext("state-machine-name", smName)
	logger.FromContext(ctx).AddContext("aws-event-type", evt.Type)
	var update store.UpdateWorkflowAttributesInput
	// This special case prevents slamming DDB with requests that would update the state from
	// running => running. Event ID #2 may be a number of different SFN events, but regardless
	// of the specific event type, we consider them all to put the workflow into a 'running'
	// state according to our own internal definition that we want displayed in hubble. Acting
	// only on event ID #2 and terminal events (below in switch statements) means that we only
	// attempt to put the workflow into each state once and therefore save on DDB requests.
	if evt.ID == "2" {
		update.Status = ptrStatus(models.WorkflowStatusRunning)
		logger.FromContext(ctx).InfoD("update-workflow", logger.M(update.Map()))
		return swallowOutOfOrderStateError(ctx, h.store.UpdateWorkflowAttributes(ctx, execID, update))
	}

	// on terminal events, update StoppedAt
	switch evt.Type {
	case "ExecutionAborted", "ExecutionFailed", "ExecutionTimedOut", "ExecutionSucceeded":
		msec, err := strconv.ParseInt(evt.Timestamp, 10, 64)
		if err != nil {
			return fmt.Errorf("could not parse unix millisecond timestamp '%s': %s", evt.Timestamp, err)
		}
		stoppedAt := strfmt.DateTime(unixMilli(msec))
		update.StoppedAt = &stoppedAt
	}
	switch evt.Type {
	case "ExecutionAborted":
		update.Status = ptrStatus(models.WorkflowStatusCancelled)
	case "ExecutionFailed":
		update.Status = ptrStatus(models.WorkflowStatusFailed)
	case "ExecutionTimedOut":
		update.Status = ptrStatus(models.WorkflowStatusFailed)
		update.StatusReason = aws.String(resources.StatusReasonWorkflowTimedOut)
	case "ExecutionSucceeded":
		update.Status = ptrStatus(models.WorkflowStatusSucceeded)
		if evt.Details.Output == nil {
			return fmt.Errorf("unexpected no output for succeeded event. ARN %s", evt.ExecutionARN)
		} else {
			update.Output = evt.Details.Output
		}
	}

	// we consider any successful or canceled workflow as "resolved"
	if update.Status != nil && (*update.Status == models.WorkflowStatusSucceeded || *update.Status == models.WorkflowStatusCancelled) {
		update.ResolvedByUser = aws.Bool(true)
	}

	// Populate the last job within WorkflowSummary on failures so that workflows can be
	// more easily searched for and bucketed by failure state.
	if update.Status != nil && (*update.Status == models.WorkflowStatusFailed) {
		workflow, err := h.store.GetWorkflowByID(ctx, execID)
		if err != nil {
			return err
		}
		if err := wfupdater.UpdateWorkflowLastJob(ctx, h.sfnapi, evt.ExecutionARN, &workflow); err != nil {
			return err
		}
		failedJob := ""
		failedJobResource := ""
		if workflow.LastJob != nil {
			failedJob = workflow.LastJob.State
			if workflow.LastJob.StateResource != nil {
				failedJobResource = workflow.LastJob.StateResource.Name
			}
			update.LastJob = workflow.LastJob
		}
		logger.FromContext(ctx).InfoD("workflow-failed", logger.M{
			"workflow-name":       workflow.WorkflowDefinition.Name,
			"workflow-version":    workflow.WorkflowDefinition.Version,
			"workflow-id":         workflow.ID,
			"failed-job-name":     failedJob,
			"failed-job-resource": failedJobResource,
		})
	}

	if update.ZeroValue() {
		return nil // no updates to perform
	}
	logger.FromContext(ctx).InfoD("update-workflow", logger.M(update.Map()))
	return swallowOutOfOrderStateError(ctx, h.store.UpdateWorkflowAttributes(ctx, execID, update))
}

func main() {
	lc := InitLaunchConfig()

	sfnapi := sfn.New(session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(lc.Env.AwsSfnRegion),
		},
	})))
	svc := dynamodb.New(session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region:     aws.String(lc.Env.AwsDynamoRegion),
			MaxRetries: &dynamoMaxRetries,
		},
	})))
	db := dynamodbstore.New(svc, dynamodbstore.TableConfig{
		PrefixStateResources:      lc.Env.AwsDynamoPrefixStateResources,
		PrefixWorkflowDefinitions: lc.Env.AwsDynamoPrefixWorkflowDefinitions,
		PrefixWorkflows:           lc.Env.AwsDynamoPrefixWorkflows,
	})
	var err error
	db.Future, err = dynamodbgen.New(dynamodbgen.Config{
		DynamoDBAPI:   svc,
		DefaultPrefix: lc.Env.AwsDynamoPrefixWorkflowDefinitions,
		WorkflowDefinitionTable: dynamodbgen.WorkflowDefinitionTable{
			Prefix: lc.Env.AwsDynamoPrefixWorkflowDefinitions,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	handler := NewHandler(
		db,
		sfnapi,
	)
	if os.Getenv("IS_LOCAL") == "true" {
		// Update input as needed to debug
		var input events.KinesisEvent
		log.Printf("Running locally with this input: %+v\n", input)
		err := handler.Handle(context.Background(), input)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		lambda.Start(handler.Handle)
	}
}
