package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

// SFNWorkflowManager manages workflows run through AWS Step Functions.
type SFNWorkflowManager struct {
	sfnapi    sfniface.SFNAPI
	store     store.Store
	region    string
	roleARN   string
	accountID string
}

func NewSFNWorkflowManager(sfnapi sfniface.SFNAPI, store store.Store, roleARN, region, accountID string) WorkflowManager {
	return &SFNWorkflowManager{
		sfnapi:    sfnapi,
		store:     store,
		roleARN:   roleARN,
		region:    region,
		accountID: accountID,
	}
}

func wdTypeToSLStateType(wdType string) models.SLStateType {
	switch wdType {
	case "WORKER":
		return models.SLStateTypeTask
	default:
		return models.SLStateTypeTask
	}
}

func wdRetryToSLRetry(wdRetry []*models.Retrier) []*models.SLRetrier {
	retriers := []*models.SLRetrier{}
	for _, wdRetrier := range wdRetry {
		retriers = append(retriers, &models.SLRetrier{
			ErrorEquals: wdRetrier.ErrorEquals,
			MaxAttempts: wdRetrier.MaxAttempts,
		})
	}
	return retriers
}

func wdResourceToSLResource(wdResource, region, accountID, namespace string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:activity:%s-%s", region, accountID, namespace, wdResource)
}

func wdToStateMachine(wd resources.WorkflowDefinition, region, accountID, namespace string) *models.SLStateMachine {
	states := map[string]models.SLState{}
	for _, state := range wd.StatesMap {
		states[state.Name()] = models.SLState{
			Type:     wdTypeToSLStateType(state.Type()),
			Next:     state.Next(),
			End:      state.IsEnd(),
			Retry:    wdRetryToSLRetry(state.Retry()),
			Resource: wdResourceToSLResource(state.Resource(), region, accountID, namespace),
		}
	}

	return &models.SLStateMachine{
		Comment: wd.Description,
		StartAt: wd.StartAtStr,
		States:  states,
		// TimeoutSeconds: not supported in wd
		Version: "1.0",
	}
}

func stateMachineName(wdName string, wdVersion int, namespace string, queue string) string {
	return fmt.Sprintf("%s-%s-%d-%s", namespace, wdName, wdVersion, queue)
}

func stateMachineARN(region, accountID, wdName string, wdVersion int, namespace string, queue string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s", region, accountID, stateMachineName(wdName, wdVersion, namespace, queue))
}

func (wm *SFNWorkflowManager) describeOrCreateStateMachine(wd resources.WorkflowDefinition, namespace, queue string) (*sfn.DescribeStateMachineOutput, error) {
	describeOutput, err := wm.sfnapi.DescribeStateMachine(&sfn.DescribeStateMachineInput{
		StateMachineArn: aws.String(stateMachineARN(wm.region, wm.accountID, wd.NameStr, wd.VersionInt, namespace, queue)),
	})
	if err == nil {
		return describeOutput, nil
	}
	awserr, ok := err.(awserr.Error)
	if !ok {
		return nil, fmt.Errorf("non-AWS error in findOrCreateStateMachine: %s", err)
	}
	if awserr.Code() != sfn.ErrCodeStateMachineDoesNotExist {
		return nil, fmt.Errorf("unexpected AWS error in findOrCreateStateMachine: %s", awserr)
	}

	// state machine doesn't exist, create it
	// the name must be unique. Use definition name+version, namespace, and queue
	stateMachineJSON, err := json.MarshalIndent(wdToStateMachine(wd, wm.region, wm.accountID, namespace), "", "  ")
	if err != nil {
		return nil, err
	}
	definition := string(stateMachineJSON)
	name := stateMachineName(wd.NameStr, wd.VersionInt, namespace, queue)
	log.InfoD("create-state-machine", logger.M{"definition": definition, "name": name})
	_, err = wm.sfnapi.CreateStateMachine(&sfn.CreateStateMachineInput{
		Name:       aws.String(name),
		Definition: aws.String(definition),
		RoleArn:    aws.String(wm.roleARN),
	})
	if err != nil {
		return nil, fmt.Errorf("CreateStateMachine error: %s", err.Error())
	}

	return wm.describeOrCreateStateMachine(wd, namespace, queue)
}

func (wm *SFNWorkflowManager) CreateWorkflow(wd resources.WorkflowDefinition, input []string, namespace string, queue string) (*resources.Workflow, error) {
	describeOutput, err := wm.describeOrCreateStateMachine(wd, namespace, queue)
	if err != nil {
		return nil, err
	}

	// submit an execution using input, set execution name == our workflow GUID
	workflow := resources.NewWorkflow(wd, input)
	workflow.Namespace = namespace
	workflow.Queue = queue
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	_, err = wm.sfnapi.StartExecution(&sfn.StartExecutionInput{
		StateMachineArn: describeOutput.StateMachineArn,
		Input:           aws.String(string(inputJSON)),
		Name:            aws.String(workflow.ID),
	})
	if err != nil {
		return nil, err
	}

	return workflow, wm.store.SaveWorkflow(*workflow)
}

func (wm *SFNWorkflowManager) CancelWorkflow(workflow *resources.Workflow, reason string) error {
	// cancel execution
	return errors.New("TODO")
}

func executionARN(region, accountID, stateMachineName, executionName string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:execution:%s:%s", region, accountID, stateMachineName, executionName)
}

func sfnStatusToWorkflowStatus(sfnStatus string) resources.WorkflowStatus {
	switch sfnStatus {
	case sfn.ExecutionStatusRunning:
		return resources.Running
	case sfn.ExecutionStatusSucceeded:
		return resources.Succeeded
	case sfn.ExecutionStatusFailed:
		return resources.Failed
	case sfn.ExecutionStatusTimedOut:
		return resources.Failed
	case sfn.ExecutionStatusAborted:
		return resources.Cancelled
	default:
		return resources.Queued // this should never happen, since all cases are covered above
	}
}

func (wm *SFNWorkflowManager) UpdateWorkflowStatus(workflow *resources.Workflow) error {
	// get execution from AWS, pull in all the data into the workflow object
	wd := workflow.WorkflowDefinition
	execARN := executionARN(wm.region, wm.accountID, stateMachineName(wd.NameStr, wd.VersionInt, workflow.Namespace, workflow.Queue), workflow.ID)
	describeOutput, err := wm.sfnapi.DescribeExecution(&sfn.DescribeExecutionInput{
		ExecutionArn: aws.String(execARN),
	})
	if err != nil {
		return err
	}

	workflow.LastUpdated = time.Now()
	workflow.Status = sfnStatusToWorkflowStatus(*describeOutput.Status)

	// TODO: pull in execution history to populate jobs array
	// err := wm.sfnapi.GetExecutionHistoryPages(&sfn.GetExecutionHistoryInput{
	// 	ExecutionArn: aws.String(execARN),
	// }, func(historyOutput *sfn.GetExecutionHistoryOutput, lastPage bool) bool {
	// 	// NOTE: if pulling the entire execution history becomes infeasible, we can:
	// 	// 1) limit the results with `maxResults`
	// 	// 2) set `reverseOrder` to true to get most recent events first
	// 	// 3) store the last event we processed, and stop paging once we reach it
	// 	for _, evt := range historyOutput.Events {
	// 		switch *evt.Type {
	// 		case sfn.HistoryEventTypeTaskStateEntered:
	// 			job := &resources.Job{}
	// 		}
	// 	}
	// 	return true
	// })
	// if err != nil {
	// 	return err
	// }

	return wm.store.UpdateWorkflow(*workflow)
}
