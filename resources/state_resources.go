package resources

import (
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/go-openapi/strfmt"
)

// StateResource maps the Resource URI (e.g. ARN for the JobDefinition
// in batchclient) to a `name` and `namespace`. Each WorkflowDefinition.State defines
// a resource name for that state, and a `namespace` is provided as part of
// when creating a new Workflow. StateResource allows for a dynamic lookup of the
// URI by the `executor` package.
//

func NewBatchResource(name, namespace, arn string) *models.StateResource {
	return &models.StateResource{
		Name:        name,
		Namespace:   namespace,
		URI:         arn,
		Type:        models.StateResourceTypeJobDefinitionARN,
		LastUpdated: strfmt.DateTime(time.Now()),
	}
}
