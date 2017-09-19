package resources

import "time"

const (
	AWSBatchJobDefinition = "JobDefinitionArn"
	SFNActivity           = "ActivityARN"
)

// StateResource maps the Resource URI (e.g. ARN for the JobDefinition
// in batchclient) to a `name` and `namespace`. Each Workflow.State defines
// a resource name for that state, and a `namespace` is provided as part of
// when creating a new Job. StateResource allows for a dynamic lookup of the
// URI by the `executor` package.
//
type StateResource struct {
	Name        string
	Namespace   string
	URI         string
	Type        string
	LastUpdated time.Time
}

func NewBatchResource(name, namespace, arn string) StateResource {
	return StateResource{
		Name:        name,
		Namespace:   namespace,
		URI:         arn,
		Type:        AWSBatchJobDefinition,
		LastUpdated: time.Now(),
	}
}
