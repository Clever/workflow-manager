package embedded

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/Clever/aws-sdk-go-counter/aws"
	"github.com/Clever/aws-sdk-go-counter/aws/awserr"
	"github.com/Clever/workflow-manager/executor/sfnconventions"
	"github.com/Clever/workflow-manager/gen-go/client"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/ghodss/yaml"
	"github.com/hashicorp/go-multierror"
	"github.com/mohae/deepcopy"
)

type embedded struct {
	environment         string
	app                 string
	sfnAccountID        string
	sfnRegion           string
	sfnRoleARN          string
	sfnAPI              sfniface.SFNAPI
	resources           map[string]interface{}
	workflowDefinitions []models.WorkflowDefinition
}

var _ client.Client = &embedded{}

// Config for embedded wfm.
type Config struct {
	Environment         string
	App                 string
	SFNAccountID        string
	SFNRegion           string
	SFNRoleARN          string
	SFNAPI              sfniface.SFNAPI
	Resources           map[string]interface{}
	WorkflowDefinitions []byte
}

func (c Config) validate() error {
	if c.Environment == "" {
		return errors.New("must configure Environment")
	}
	if c.App == "" {
		return errors.New("must configure App")
	}
	if c.SFNAccountID == "" {
		return errors.New("must configure SFNAccountID")
	}
	if c.SFNRegion == "" {
		return errors.New("must configure SFNRegion")
	}
	if c.SFNAPI == nil {
		return errors.New("must configure SFN client")
	}
	if c.Resources == nil {
		return errors.New("must configure resources")
	}
	if c.WorkflowDefinitions == nil {
		return errors.New("must configure workflow definitions")
	}
	return nil
}

// New returns a client to an embedded workflow manager.
func New(config *Config) (client.Client, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	wfdefs, err := parseWorkflowDefinitions(config.WorkflowDefinitions)
	if err != nil {
		return nil, err
	}
	if err := verifyWorkflowDefinitionResources(wfdefs, config.Resources); err != nil {
		return nil, err
	}
	return &embedded{
		environment:         config.Environment,
		app:                 config.App,
		sfnAccountID:        config.SFNAccountID,
		sfnRegion:           config.SFNRegion,
		sfnRoleARN:          config.SFNRoleARN,
		sfnAPI:              config.SFNAPI,
		resources:           config.Resources,
		workflowDefinitions: wfdefs,
	}, nil
}

func parseWorkflowDefinitions(wfdefbs []byte) ([]models.WorkflowDefinition, error) {
	var wfdefs []models.WorkflowDefinition
	if err := yaml.Unmarshal(wfdefbs, &wfdefs); err != nil {
		return nil, err
	}
	uniqName := map[string]struct{}{}
	for _, wfdef := range wfdefs {
		if _, ok := uniqName[wfdef.Name]; ok {
			return nil, fmt.Errorf("duplicate workflow definition found: %s", wfdef.Name)
		}
	}
	return wfdefs, nil
}

func verifyWorkflowDefinitionResources(wfdefs []models.WorkflowDefinition, resources map[string]interface{}) error {
	for _, wfd := range wfdefs {
		for stateName, state := range wfd.StateMachine.States {
			if state.Resource == "" {
				continue
			}
			if _, ok := resources[state.Resource]; !ok {
				return fmt.Errorf("unknown resource '%s' in %s.%s", state.Resource, wfd.Name, stateName)
			}
		}
	}
	return nil
}

func (e embedded) HealthCheck(ctx context.Context) error {
	return nil
}

func (e embedded) GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error) {
	return e.workflowDefinitions, nil
}

func (e embedded) GetWorkflowDefinitionByNameAndVersion(ctx context.Context, i *models.GetWorkflowDefinitionByNameAndVersionInput) (*models.WorkflowDefinition, error) {
	for _, wd := range e.workflowDefinitions {
		if wd.Name == i.Name {
			return &wd, nil
		}
	}
	return nil, models.NotFound{}
}

func (e embedded) GetWorkflows(ctx context.Context, i *models.GetWorkflowsInput) ([]models.Workflow, error) {
	var validation error
	if i.Limit != nil {
		validation = multierror.Append(validation, errors.New("Limit not supported"))
	}
	if i.OldestFirst != nil {
		validation = multierror.Append(validation, errors.New("OldestFirst not supported"))
	}
	if i.PageToken != nil {
		validation = multierror.Append(validation, errors.New("PageToken not supported"))
	}
	if i.Status != nil {
		validation = multierror.Append(validation, errors.New("Status not supported"))
	}
	if i.ResolvedByUser != nil {
		validation = multierror.Append(validation, errors.New("ResolvedByUser not supported"))
	}
	if i.SummaryOnly != nil {
		validation = multierror.Append(validation, errors.New("SummaryOnly not supported"))
	}
	if validation != nil {
		return nil, validation
	}

	//WorkflowDefinitionName string

	panic("TODO")
}

func (e embedded) StartWorkflow(ctx context.Context, i *models.StartWorkflowRequest) (*models.Workflow, error) {
	// generate state machine
	wd, err := e.GetWorkflowDefinitionByNameAndVersion(ctx, &models.GetWorkflowDefinitionByNameAndVersionInput{Name: i.WorkflowDefinition.Name})
	if err != nil {
		return nil, err
	}
	stateMachine := deepcopy.Copy(wd.StateMachine).(models.SLStateMachine)
	for stateName, s := range stateMachine.States {
		state := deepcopy.Copy(s).(models.SLState)
		if state.Type != models.SLStateTypeTask {
			continue
		}
		state.Resource = sfnconventions.EmbeddedResource(state.Resource, e.sfnRegion, e.sfnAccountID, i.Namespace)
		stateMachine.States[stateName] = state
	}
	stateMachineDefBytes, err := json.MarshalIndent(stateMachine, "", "  ")
	if err != nil {
		return nil, err
	}

	// find or create the state machine in AWS
	stateMachineName := sfnconventions.StateMachineName(wd.Name, wd.Version, i.Namespace, wd.StateMachine.StartAt)
	stateMachineARN := sfnconventions.StateMachineARN(e.sfnRegion, e.sfnAccountID, wd.Name, wd.Version, i.Namespace, wd.StateMachine.StartAt)
	out, err := e.sfnAPI.DescribeStateMachineWithContext(ctx, &sfn.DescribeStateMachineInput{
		StateMachineArn: aws.String(stateMachineARN),
	})
	if err != nil {
		// if it doesn't exist, create it
		if awserr, ok := err.(awserr.Error); ok && awserr.Code() == sfn.ErrCodeStateMachineDoesNotExist {
			if _, err := e.sfnAPI.CreateStateMachine(&sfn.CreateStateMachineInput{
				Name:       aws.String(stateMachineName),
				Definition: aws.String(string(stateMachineDefBytes)),
				RoleArn:    aws.String(e.sfnRoleARN),
			}); err != nil {
				return nil, fmt.Errorf("CreateStateMachine error: %s", err.Error())
			}
		} else {
			return nil, err
		}
	} else /* state machine already exists */ {
		// if it exists, verify they're the same--if not, it's user error:
		// state machines are immutable, user should create a workflow def with
		// a new name
		var existingStateMachine models.SLStateMachine
		if err := json.Unmarshal([]byte(aws.StringValue(out.Definition)), &existingStateMachine); err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(existingStateMachine, stateMachine) {
			return nil, fmt.Errorf(`existing state machine differs from new state machine.
State machines are immutable. Please rename the state machine. Existing state machine:
%s
New state machine:
%s`, *out.Definition, string(stateMachineDefBytes))
		}
	}

	// start execution!
	var inputJSON map[string]interface{}
	if err := json.Unmarshal([]byte(i.Input), &inputJSON); err != nil {
		return nil, models.BadRequest{
			Message: fmt.Sprintf("input is not a valid JSON object: %s ", err),
		}
	}
	workflow := resources.NewWorkflow(wd, i.Input, i.Namespace, i.Queue, i.Tags)
	if _, err := e.sfnAPI.StartExecutionWithContext(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(stateMachineARN),
		Input:           aws.String(i.Input),
		Name:            aws.String(workflow.ID),
	}); err != nil {
		return nil, err
	}
	return workflow, nil
}

func (e embedded) CancelWorkflow(ctx context.Context, i *models.CancelWorkflowInput) error {
	panic("TODO")
}

func (e embedded) GetWorkflowByID(ctx context.Context, workflowID string) (*models.Workflow, error) {
	panic("TODO")
}
