package sfnconventions

import (
	"fmt"
	"strconv"
	"strings"
)

// https://docs.aws.amazon.com/step-functions/latest/apireference/API_CreateStateMachine.html#StepFunctions-CreateStateMachine-request-name
var stateMachineNameBadChars = []byte{' ', '<', '>', '{', '}', '[', ']', '?', '*', '"', '#', '%', '\\', '^', '|', '~', '`', '$', '&', ',', ';', ':', '/'}

// StateMachineName is a combination of the workflow definition namesapce, name, version, and the state you'd like to start at.
func StateMachineName(wdName string, wdVersion int64, namespace string, startAt string) string {
	name := fmt.Sprintf("%s--%s--%d--%s", namespace, wdName, wdVersion, startAt)
	for _, badchar := range stateMachineNameBadChars {
		name = strings.Replace(name, string(badchar), "-", -1)
	}
	return name
}

// SMParts are the parts of the state machine name.
type SMParts struct {
	WDName    string
	WDVersion int64
	Namespace string
	StartAt   string
}

// StateMachineNameParts is the reverse of StateMachineName.
func StateMachineNameParts(stateMachineName string) (*SMParts, error) {
	parts := strings.Split(stateMachineName, "--")
	if len(parts) != 4 {
		return nil, fmt.Errorf("expected four parts in sm name: %s", stateMachineName)
	}
	version, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("sm version '%s' not an int: %s", parts[2], err.Error())
	}
	return &SMParts{
		WDName:    parts[1],
		Namespace: parts[0],
		WDVersion: int64(version),
		StartAt:   parts[3],
	}, nil
}

// StateMachineTags returns the tags we place on state machine resources.
func StateMachineTags(namespace, wdName string, wdVersion int64, startAt string, defaultTags map[string]string) map[string]string {
	tags := map[string]string{
		"environment":                  namespace,
		"workflow-definition-name":     wdName,
		"workflow-definition-version":  fmt.Sprintf("%d", wdVersion),
		"workflow-definition-start-at": wdName,
	}
	for k, v := range defaultTags {
		tags[k] = v
	}
	return tags
}

// StateMachineArn constructs a state machine ARN.
func StateMachineArn(region, accountID, wdName string, wdVersion int64, namespace string, startAt string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s", region, accountID, StateMachineName(wdName, wdVersion, namespace, startAt))
}

// LogGroupArn constructs the Cloudwatch Logs log group name for a state machine.
func LogGroupArn(region, accountID, stateMachineName string) string {
	return fmt.Sprintf("arn:aws:logs:%s:%s:log-group:/sfn/%s:*", region, accountID, stateMachineName)
}

// SFNCLIResource is the activity ARN registered by SFNCLI.
func SFNCLIResource(wdResource, region, accountID, namespace string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:activity:%s--%s", region, accountID, namespace, wdResource)
}

// LambdaResource is the lambda function ARN as deployed by catapult.
func LambdaResource(wdResource, region, accountID, namespace string) string {
	return fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s--%s", region, accountID, namespace, strings.TrimPrefix(wdResource, "lambda:"))
}

// GlueState returns the resource name and JobName parameter used for running a Glue job.
func GlueResourceAndJobName(wdResource, namespace string) (string, string) {
	return "arn:aws:states:::glue:startJobRun.sync", fmt.Sprintf("%s--%s", namespace, strings.TrimPrefix(wdResource, "glue:"))
}

// EmbeddedResourceArn is the activity ARN registered by embedded WFM.
func EmbeddedResourceArn(wdResource, region, accountID, namespace string, app string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:activity:%s--%s-%s", region, accountID, namespace, app, wdResource)
}

// ExecutionArn generates the execution ARN format used by SFN.
func ExecutionArn(region, accountID, stateMachineName, executionName string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:execution:%s:%s", region, accountID, stateMachineName, executionName)
}
