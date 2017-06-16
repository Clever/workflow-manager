package resources

import "time"

const (
	AWSBatchJobDefinition = "JobDefinitionArn"
)

// StateResource contains a lookup from a Resource name to
// a `namespace`, `version` and an URI for reference by
// the exectuor (e.g. ARN for the JobDefinition in the batchclient?)
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
