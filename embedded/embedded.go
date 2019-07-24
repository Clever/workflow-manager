package embedded

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/Clever/workflow-manager/embedded/sfnfunction"
	"github.com/Clever/workflow-manager/executor/sfnconventions"
	"github.com/Clever/workflow-manager/gen-go/client"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/ghodss/yaml"
	"github.com/go-openapi/strfmt"
	"github.com/hashicorp/go-multierror"
	"github.com/mohae/deepcopy"
	uuid "github.com/satori/go.uuid"
	errors "golang.org/x/xerrors"
)

// Embedded ...
type Embedded struct {
	environment         string
	app                 string
	sfnAccountID        string
	sfnRegion           string
	sfnRoleArn          string
	sfnAPI              sfniface.SFNAPI
	resources           map[string]*sfnfunction.Resource
	workflowDefinitions []models.WorkflowDefinition
	workerName          string
}

var _ client.Client = &Embedded{}

// Config for Embedded wfm.
type Config struct {
	Environment         string
	App                 string
	SFNAccountID        string
	SFNRegion           string
	SFNRoleArn          string
	SFNAPI              sfniface.SFNAPI
	Resources           map[string]interface{}
	WorkflowDefinitions []byte
	WorkerName          string
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
	return nil
}

// New returns a client to an embedded workflow manager.
func New(config *Config) (*Embedded, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	wfdefs, err := parseWorkflowDefinitions(config.WorkflowDefinitions)
	if err != nil {
		return nil, err
	}
	r := map[string]*sfnfunction.Resource{}
	for k := range config.Resources {
		kcopy := k
		var err error
		if r[kcopy], err = sfnfunction.New(kcopy, config.Resources[kcopy]); err != nil {
			return nil, errors.Errorf("function '%s': %s", kcopy, err.Error())
		}
	}
	for _, wfdef := range wfdefs {
		if err := validateWorkflowDefinition(wfdef, r); err != nil {
			return nil, err
		}
	}
	wn := config.WorkerName
	if wn == "" {
		// give it sane default
		an := "wfm-embedded"
		if a := os.Getenv("APP_NAME"); a != "" {
			an = a
		}
		hn, _ := os.Hostname()
		wn = fmt.Sprintf("%s-%s-%s", an, hn, randString(5))
	}
	return &Embedded{
		environment:         config.Environment,
		app:                 config.App,
		sfnAccountID:        config.SFNAccountID,
		sfnRegion:           config.SFNRegion,
		sfnRoleArn:          config.SFNRoleArn,
		sfnAPI:              config.SFNAPI,
		resources:           r,
		workflowDefinitions: wfdefs,
		workerName:          wn,
	}, nil
}

const lettersAndNumbers = "abcdefghijklmnopqrstuvwxyz0123456789"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = lettersAndNumbers[rand.Intn(len(lettersAndNumbers))]
	}
	return string(b)
}

// ParseWorkflowDefinition converts a yaml workflow definition into our model
func ParseWorkflowDefinition(wfdefbs []byte) (models.WorkflowDefinition, error) {
	var wfd models.WorkflowDefinition
	if err := yaml.Unmarshal(wfdefbs, &wfd); err != nil {
		return models.WorkflowDefinition{}, err
	}
	wfd.CreatedAt = strfmt.DateTime(time.Now())
	return wfd, nil
}

func parseWorkflowDefinitions(wfdefbs []byte) ([]models.WorkflowDefinition, error) {
	var wfdefs []models.WorkflowDefinition
	if err := yaml.Unmarshal(wfdefbs, &wfdefs); err != nil {
		return nil, err
	}
	uniqName := map[string]struct{}{}
	for _, wfdef := range wfdefs {
		if _, ok := uniqName[wfdef.Name]; ok {
			return nil, errors.Errorf("duplicate workflow definition found: %s", wfdef.Name)
		}
	}
	return wfdefs, nil
}

func validateWorkflowDefinition(wfdef models.WorkflowDefinition, resources map[string]*sfnfunction.Resource) error {
	if wfdef.Name == "" {
		return errors.Errorf("workflow definition name is required")
	}
	if len(wfdef.StateMachine.States) == 0 {
		return errors.Errorf("must define at least one state in %s", wfdef.Name)
	}

	return validateWorkflowDefinitionStates(wfdef, resources)
}

func validateWorkflowDefinitionStates(wfd models.WorkflowDefinition, resources map[string]*sfnfunction.Resource) error {
	endFound := false
	for stateName, state := range wfd.StateMachine.States {
		checkNextState := false
		switch state.Type {
		case models.SLStateTypeTask:
			checkNextState = true
			if state.Resource == "" {
				return errors.Errorf("must specify resource for task in %s.%s", wfd.Name, stateName)
			}
			if _, ok := resources[state.Resource]; !ok {
				return errors.Errorf("unknown resource '%s' in %s.%s", state.Resource, wfd.Name, stateName)
			}
		case models.SLStateTypePass:
			checkNextState = true
			if state.Result == "" && state.ResultPath == "" {
				return errors.Errorf("must specify results in %s.%s", wfd.Name, stateName)
			}
		case models.SLStateTypeChoice:
			if len(state.Choices) == 0 {
				return errors.Errorf("must specify at one choice in %s.%s", wfd.Name, stateName)
			}
		case models.SLStateTypeWait:
			checkNextState = true
			// technically we could use an absolute timestamp, but at the time of writing we don't
			// want to support that type of workflow
			if state.Seconds <= 0 {
				return errors.Errorf("invalid seconds parameter in wait %s.%s", wfd.Name, stateName)
			}
		case models.SLStateTypeSucceed, models.SLStateTypeFail, models.SLStateTypeParallel:
			// no op
		default:
			return errors.Errorf("invalid state type '%s' in %s.%s", state.Type, wfd.Name, stateName)
		}

		if state.End {
			endFound = true
		} else if checkNextState {
			if state.Next == "" {
				return errors.Errorf("must specify next state in %s.%s", wfd.Name, stateName)
			}
		}
	}

	if !endFound {
		return errors.Errorf("must specify an end state in %s", wfd.Name)
	}

	return nil
}

// HealthCheck ...
func (e *Embedded) HealthCheck(ctx context.Context) error {
	return nil
}

// GetWorkflowDefinitions ...
func (e *Embedded) GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error) {
	return e.workflowDefinitions, nil
}

// GetWorkflowDefinitionByNameAndVersion ...
func (e *Embedded) GetWorkflowDefinitionByNameAndVersion(ctx context.Context, i *models.GetWorkflowDefinitionByNameAndVersionInput) (*models.WorkflowDefinition, error) {
	for _, wd := range e.workflowDefinitions {
		if wd.Name == i.Name {
			return &wd, nil
		}
	}
	return nil, models.NotFound{}
}

// GetWorkflows ...
func (e *Embedded) GetWorkflows(ctx context.Context, i *models.GetWorkflowsInput) ([]models.Workflow, error) {
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

	wd, err := e.GetWorkflowDefinitionByNameAndVersion(ctx, &models.GetWorkflowDefinitionByNameAndVersionInput{Name: i.WorkflowDefinitionName})
	if err != nil {
		return nil, err
	}
	smArn := sfnconventions.StateMachineArn(e.sfnRegion, e.sfnAccountID, wd.Name, wd.Version, e.environment, wd.StateMachine.StartAt)
	out, err := e.sfnAPI.ListExecutionsWithContext(ctx, &sfn.ListExecutionsInput{
		StateMachineArn: aws.String(smArn),
	})
	if err != nil {
		return nil, err
	}
	wfs := []models.Workflow{}
	for _, e := range out.Executions {
		wfs = append(wfs, models.Workflow{
			WorkflowSummary: models.WorkflowSummary{
				ID:                 aws.StringValue(e.Name),
				CreatedAt:          strfmt.DateTime(aws.TimeValue(e.StartDate)),
				Status:             resources.SFNStatusToWorkflowStatus(aws.StringValue(e.Status)),
				StoppedAt:          strfmt.DateTime(aws.TimeValue(e.StopDate)),
				WorkflowDefinition: wd,
			},
		})
	}
	return wfs, nil
}

func (e *Embedded) setStateMachineResources(i *models.StartWorkflowRequest, stateMachine *models.SLStateMachine) {
	for stateName, s := range stateMachine.States {
		state := deepcopy.Copy(s).(models.SLState)
		switch state.Type {
		case models.SLStateTypeTask:
			state.Resource = sfnconventions.EmbeddedResourceArn(state.Resource, e.sfnRegion, e.sfnAccountID, i.Namespace, e.app)
			stateMachine.States[stateName] = state
		case models.SLStateTypeParallel:
			// recurse into sub-state machines to generate resource arns
			for idx := range state.Branches {
				branch := deepcopy.Copy(state.Branches[idx]).(*models.SLStateMachine)
				e.setStateMachineResources(i, branch)
				state.Branches[idx] = branch
			}
			stateMachine.States[stateName] = state
		}
	}
}

// StartWorkflow ...
func (e *Embedded) StartWorkflow(ctx context.Context, i *models.StartWorkflowRequest) (*models.Workflow, error) {
	var validation error
	if i.Namespace == "" {
		i.Namespace = e.environment
	} else if i.Namespace != e.environment {
		// can only submit workflows into the environment that this app exists in
		validation = multierror.Append(validation, errors.Errorf("namespace '%s' must match environment '%s'", i.Namespace, e.environment))
	}
	if i.Queue != "" {
		validation = multierror.Append(validation, errors.New("queue not supported"))
	}
	if i.Tags != nil {
		validation = multierror.Append(validation, errors.New("tags not supported"))
	}
	if validation != nil {
		return nil, validation
	}

	// generate state machine
	wd, err := e.GetWorkflowDefinitionByNameAndVersion(ctx, &models.GetWorkflowDefinitionByNameAndVersionInput{Name: i.WorkflowDefinition.Name})
	if err != nil {
		return nil, errors.Errorf("GetWorkflowDefinitionByNameAndVersion: %s", err.Error())
	}
	stateMachine := deepcopy.Copy(wd.StateMachine).(*models.SLStateMachine)
	e.setStateMachineResources(i, stateMachine)
	stateMachineDefBytes, err := json.MarshalIndent(stateMachine, "", "  ")
	if err != nil {
		return nil, errors.Errorf("json marshal: %s", err.Error())
	}

	// find or create the state machine in AWS
	stateMachineName := sfnconventions.StateMachineName(wd.Name, wd.Version, i.Namespace, wd.StateMachine.StartAt)
	stateMachineArn := sfnconventions.StateMachineArn(e.sfnRegion, e.sfnAccountID, wd.Name, wd.Version, i.Namespace, wd.StateMachine.StartAt)
	out, err := e.sfnAPI.DescribeStateMachineWithContext(ctx, &sfn.DescribeStateMachineInput{
		StateMachineArn: aws.String(stateMachineArn),
	})
	if err != nil {
		// if it doesn't exist, create it
		if awserr, ok := err.(awserr.Error); ok && awserr.Code() == sfn.ErrCodeStateMachineDoesNotExist {
			if _, err := e.sfnAPI.CreateStateMachine(&sfn.CreateStateMachineInput{
				Name:       aws.String(stateMachineName),
				Definition: aws.String(string(stateMachineDefBytes)),
				RoleArn:    aws.String(e.sfnRoleArn),
			}); err != nil {
				return nil, errors.Errorf("CreateStateMachine error: %s", err.Error())
			}
		} else {
			return nil, err
		}
	} else if wd.Version == -1 /* state machine already exists and allows mutation (version == -1) */ {
		if _, err := e.sfnAPI.UpdateStateMachine(&sfn.UpdateStateMachineInput{
			Definition:      aws.String(string(stateMachineDefBytes)),
			RoleArn:         aws.String(e.sfnRoleArn),
			StateMachineArn: out.StateMachineArn,
		}); err != nil {
			return nil, errors.Errorf("UpdateStateMachine: %s", err.Error())
		}
		// Control for "Executions started immediately after calling UpdateStateMachine might use the previous state machine definition and roleArn."
		// https://docs.aws.amazon.com/step-functions/latest/dg/concepts-read-consistency.html
		time.Sleep(5 * time.Second)
	} else {
		// if it exists, verify they're the same--if not, it's user error:
		// state machines are immutable, user should create a workflow def with
		// a new name
		var existingStateMachine models.SLStateMachine
		if err := json.Unmarshal([]byte(aws.StringValue(out.Definition)), &existingStateMachine); err != nil {
			return nil, err
		}
		if *out.Definition != string(stateMachineDefBytes) {
			return nil, errors.Errorf(`existing state machine differs from new state machine.
State machines are immutable. Please rename the state machine or set version to -1 to allow mutation. Existing state machine:
%s
New state machine:
%s`, *out.Definition, string(stateMachineDefBytes))
		}
	}

	// start execution!
	var inputJSON interface{}
	if err := json.Unmarshal([]byte(i.Input), &inputJSON); err != nil {
		return nil, models.BadRequest{
			Message: fmt.Sprintf("input is not a valid JSON object: %s ", err),
		}
	}
	workflow := &models.Workflow{
		WorkflowSummary: models.WorkflowSummary{
			ID:                 workflowID(stateMachineName),
			CreatedAt:          strfmt.DateTime(time.Now()),
			LastUpdated:        strfmt.DateTime(time.Now()),
			WorkflowDefinition: wd,
			Status:             models.WorkflowStatusQueued,
			Namespace:          i.Namespace,
			Input:              i.Input,
		},
	}
	if _, err := e.sfnAPI.StartExecutionWithContext(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(stateMachineArn),
		Input:           aws.String(i.Input),
		Name:            aws.String(workflow.ID),
	}); err != nil {
		return nil, errors.Errorf("StartExecution: %s", err.Error())
	}
	return workflow, nil
}

// CancelWorkflow ...
func (e *Embedded) CancelWorkflow(ctx context.Context, i *models.CancelWorkflowInput) error {
	widParts, err := parseWorkflowID(i.WorkflowID)
	if err != nil {
		return err
	}
	if _, err := e.sfnAPI.StopExecutionWithContext(ctx, &sfn.StopExecutionInput{
		ExecutionArn: aws.String(sfnconventions.ExecutionArn(e.sfnRegion, e.sfnAccountID, widParts.SMName, i.WorkflowID)),
		Cause:        aws.String(i.Reason.Reason),
		Error:        aws.String("CancelWorkflow"),
	}); err != nil {
		return err
	}
	return nil
}

// GetWorkflowByID ...
func (e *Embedded) GetWorkflowByID(ctx context.Context, workflowID string) (*models.Workflow, error) {
	widParts, err := parseWorkflowID(workflowID)
	if err != nil {
		return nil, err
	}
	smName := widParts.SMName
	smNameParts, err := sfnconventions.StateMachineNameParts(smName)
	if err != nil {
		return nil, err
	}
	wd, err := e.GetWorkflowDefinitionByNameAndVersion(ctx, &models.GetWorkflowDefinitionByNameAndVersionInput{
		Name: smNameParts.WDName,
	})
	if err != nil {
		return nil, err
	}
	out, err := e.sfnAPI.DescribeExecutionWithContext(ctx, &sfn.DescribeExecutionInput{
		ExecutionArn: aws.String(sfnconventions.ExecutionArn(e.sfnRegion, e.sfnAccountID, smName, workflowID)),
	})
	if err != nil {
		return nil, err
	}
	return &models.Workflow{
		Output: aws.StringValue(out.Output),
		WorkflowSummary: models.WorkflowSummary{
			ID:                 workflowID,
			CreatedAt:          strfmt.DateTime(aws.TimeValue(out.StartDate)),
			LastUpdated:        strfmt.DateTime(time.Now()),
			WorkflowDefinition: wd,
			Status:             resources.SFNStatusToWorkflowStatus(aws.StringValue(out.Status)),
			Namespace:          smNameParts.Namespace,
			Input:              aws.StringValue(out.Input),
		},
	}, nil
}

// NewWorkflowDefinition creates a new workflow definition.
func (e *Embedded) NewWorkflowDefinition(ctx context.Context, i *models.NewWorkflowDefinitionRequest) (*models.WorkflowDefinition, error) {
	if e.workflowDefinitions == nil {
		e.workflowDefinitions = []models.WorkflowDefinition{}
	}
	replace := false
	var replaceIdx int
	for idx, wfd := range e.workflowDefinitions {
		if wfd.Name == i.Name {
			if wfd.Version == -1 {
				replace = true
				replaceIdx = idx
				break
			}
			return nil, errors.Errorf("%s workflow definition already exists", i.Name)
		}
	}

	wfd := models.WorkflowDefinition{
		CreatedAt:    strfmt.DateTime(time.Now()),
		DefaultTags:  i.DefaultTags,
		Manager:      i.Manager,
		Name:         i.Name,
		StateMachine: i.StateMachine,
		Version:      i.Version,
	}
	if err := validateWorkflowDefinition(wfd, e.resources); err != nil {
		return nil, errors.Errorf("could not validate state machine: %s", err)
	}
	if replace {
		e.workflowDefinitions[replaceIdx] = wfd
	} else {
		e.workflowDefinitions = append(e.workflowDefinitions, wfd)
	}
	return &wfd, nil
}

// workflowID generates a workflow ID.
// The workflow ID will contain the state machine name.
// This is to support the implementation of `GetWorkflowByID`, which requires
// the state machine name in order to call `DescribeExecution` in the SFN API.
func workflowID(smName string) string {
	return fmt.Sprintf("%s--%s", smName, shortUUID())
}

type workflowIDParts struct {
	SMName    string
	ShortUUID string
}

func parseWorkflowID(wid string) (*workflowIDParts, error) {
	s := strings.Split(wid, "--")
	if len(s) < 2 {
		return nil, errors.Errorf("expected workflowID two contain at least two parts: %s", wid)
	}
	return &workflowIDParts{
		SMName:    strings.Join(s[0:len(s)-1], "--"),
		ShortUUID: s[len(s)-1],
	}, nil
}

func shortUUID() string {
	id := uuid.NewV4().String()
	s := strings.Split(id, "-")
	return s[0]
}
