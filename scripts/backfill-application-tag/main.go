package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
)

const (
	sfnRegion = "us-west-2"

	// If empty, tag all state machines
	// If non-empty, before tagging a state machine, make sure its current env tag matches this value
	environmentToTag = "clever-dev"

	// If true, iterate through all state machines and check their tags but don't apply any tags
	// if false, that plus actually tag them.
	dryRun = true
)

func main() {

	sfnsess, err := session.NewSession(aws.NewConfig().WithRegion(sfnRegion))
	if err != nil {
		log.Fatalf("new AWS session: %v", err)
	}
	sfnAPI := sfn.New(sfnsess)

	ctx := context.Background()

	stateMachineArns := []string{}

	err = sfnAPI.ListStateMachinesPagesWithContext(ctx, &sfn.ListStateMachinesInput{}, func(out *sfn.ListStateMachinesOutput, lastPage bool) bool {
		for _, machine := range out.StateMachines {
			stateMachineArns = append(stateMachineArns, *machine.StateMachineArn)
		}
		return true
	})

	// limit ourselves to one state machine every 300ms or less
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for _, machineArn := range stateMachineArns {
		if environmentToTag != "" {
			arnParts := strings.Split(machineArn, ":")
			machineName := arnParts[len(arnParts)-1]

			// all workflow-manager state machines are named <env>--<other stuff>
			dashParts := strings.Split(machineName, "--")
			if dashParts[0] != environmentToTag {
				continue
			}
		}

		<-ticker.C
		tagOutput, err := retryThrottles(func() (*sfn.ListTagsForResourceOutput, error) {
			return sfnAPI.ListTagsForResource(&sfn.ListTagsForResourceInput{
				ResourceArn: &machineArn,
			})
		}, 5*time.Second)

		if err != nil {
			fmt.Printf("got error listing tags for %s: %v\n", machineArn, err)
		}

		tagMap := map[string]string{}
		for _, tag := range tagOutput.Tags {
			tagMap[*tag.Key] = *tag.Value
		}
		if _, found := tagMap["application"]; found {
			continue
		}
		if environmentToTag != "" && tagMap["environment"] != environmentToTag {
			continue
		}
		if workflowName, found := tagMap["workflow-definition-name"]; found {
			if dryRun {
				fmt.Printf("would tag %s\n", machineArn)
				continue
			} else {
				fmt.Printf("tagging %s\n", machineArn)
			}

			_, err := retryThrottles(func() (struct{}, error) {
				_, err := sfnAPI.TagResource(&sfn.TagResourceInput{
					ResourceArn: &machineArn,
					Tags: []*sfn.Tag{
						{Key: aws.String("application"), Value: &workflowName},
					},
				})
				return struct{}{}, err
			}, 5*time.Second)

			if err != nil {
				log.Fatalf("error adding tags for %s: %v\n", machineArn, err)

			}
		}
	}

	if err != nil {
		log.Fatalf("iterating over state machines: %v", err)
	}
}

func retryThrottles[T any](f func() (T, error), waitTime time.Duration) (T, error) {
	t, err := f()
	if aerr, ok := err.(awserr.Error); ok && request.IsErrorThrottle(aerr) {
		log.Printf("sleeping...\n")
		time.Sleep(waitTime)
		return retryThrottles(f, waitTime)

	}
	return t, err
}
