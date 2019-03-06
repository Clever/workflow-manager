package sfnconventions

import (
	"fmt"
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

// StateMachineARN constructs a state machine ARN.
func StateMachineARN(region, accountID, wdName string, wdVersion int64, namespace string, startAt string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s", region, accountID, StateMachineName(wdName, wdVersion, namespace, startAt))
}

// SFNCLIResource is the activity ARN registered by SFNCLI.
func SFNCLIResource(wdResource, region, accountID, namespace string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:activity:%s--%s", region, accountID, namespace, wdResource)
}

// LambdaResource is the lambda function ARN as deployed by catapult.
func LambdaResource(wdResource, region, accountID, namespace string) string {
	return fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s--%s", region, accountID, namespace, strings.TrimPrefix(wdResource, "lambda:"))
}

// EmbeddedResourceArn is the activity ARN registered by embedded WFM.
func EmbeddedResourceArn(wdResource, region, accountID, namespace string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:activity:%s--embedded-%s", region, accountID, namespace, wdResource)
}
