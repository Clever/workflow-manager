
<a name="paths"></a>
## Paths

<a name="healthcheck"></a>
### GET /_health

#### Description
Checks if the service is healthy


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**200**|OK response|No Content|


<a name="poststateresource"></a>
### Create or Update a StateResource
```
POST /state-resources
```


#### Parameters

|Type|Name|Schema|
|---|---|---|
|**Body**|**NewStateResource**  <br>*optional*|[NewStateResource](#newstateresource)|


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**201**|StateResource Successfully saved|[StateResource](#stateresource)|


<a name="getstateresource"></a>
### Get details about StateResource given the namespace and name
```
GET /state-resources/{namespace}/{name}
```


#### Parameters

|Type|Name|Schema|
|---|---|---|
|**Path**|**name**  <br>*required*|string|
|**Path**|**namespace**  <br>*required*|string|


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**200**|StateResource|[StateResource](#stateresource)|
|**404**|Entity Not Found|[NotFound](#notfound)|


<a name="putstateresource"></a>
### Create or Update a StateResource for the name and namespace
```
PUT /state-resources/{namespace}/{name}
```


#### Parameters

|Type|Name|Schema|
|---|---|---|
|**Path**|**name**  <br>*required*|string|
|**Path**|**namespace**  <br>*required*|string|
|**Body**|**NewStateResource**  <br>*optional*|[NewStateResource](#newstateresource)|


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**201**|StateResource Successfully saved|[StateResource](#stateresource)|
|**400**|Bad Request|[BadRequest](#badrequest)|


<a name="deletestateresource"></a>
### Delete an existing StateResource
```
DELETE /state-resources/{namespace}/{name}
```


#### Parameters

|Type|Name|Schema|
|---|---|---|
|**Path**|**name**  <br>*required*|string|
|**Path**|**namespace**  <br>*required*|string|


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**200**|StateResource deleted successfully|No Content|
|**404**|Entity Not Found|[NotFound](#notfound)|


<a name="newworkflowdefinition"></a>
### Create a new WorkflowDefinition
```
POST /workflow-definitions
```


#### Parameters

|Type|Name|Schema|
|---|---|---|
|**Body**|**NewWorkflowDefinitionRequest**  <br>*optional*|[NewWorkflowDefinitionRequest](#newworkflowdefinitionrequest)|


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**201**|Successful creation of a new WorkflowDefinition|[WorkflowDefinition](#workflowdefinition)|


<a name="getworkflowdefinitions"></a>
### GET /workflow-definitions

#### Description
Get the latest versions of all available WorkflowDefinitions


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**200**|Successfully fetched all WorkflowDefinitions|< [WorkflowDefinition](#workflowdefinition) > array|


<a name="getworkflowdefinitionversionsbyname"></a>
### List WorkflowDefinition Versions by Name
```
GET /workflow-definitions/{name}
```


#### Parameters

|Type|Name|Schema|Default|
|---|---|---|---|
|**Path**|**name**  <br>*required*|string||
|**Query**|**latest**  <br>*optional*|boolean|`"true"`|


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**200**|WorkflowDefinitions|< [WorkflowDefinition](#workflowdefinition) > array|
|**404**|Entity Not Found|[NotFound](#notfound)|


<a name="updateworkflowdefinition"></a>
### Update an exiting WorkflowDefinition
```
PUT /workflow-definitions/{name}
```


#### Parameters

|Type|Name|Schema|
|---|---|---|
|**Path**|**name**  <br>*required*|string|
|**Body**|**NewWorkflowDefinitionRequest**  <br>*optional*|[NewWorkflowDefinitionRequest](#newworkflowdefinitionrequest)|


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**201**|Successful creation of a new WorkflowDefinition|[WorkflowDefinition](#workflowdefinition)|
|**404**|Entity Not Found|[NotFound](#notfound)|


<a name="getworkflowdefinitionbynameandversion"></a>
### Get a WorkflowDefinition by Name and Version
```
GET /workflow-definitions/{name}/{version}
```


#### Parameters

|Type|Name|Schema|
|---|---|---|
|**Path**|**name**  <br>*required*|string|
|**Path**|**version**  <br>*required*|integer|


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**200**|WorkflowDefinition|[WorkflowDefinition](#workflowdefinition)|
|**404**|Entity Not Found|[NotFound](#notfound)|


<a name="startworkflow"></a>
### Start a Workflow
```
POST /workflows
```


#### Parameters

|Type|Name|Description|Schema|
|---|---|---|---|
|**Body**|**StartWorkflowRequest**  <br>*optional*|Parameters for starting a workflow (workflow definition, input, and optionally namespace, queue, and tags)|[StartWorkflowRequest](#startworkflowrequest)|


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**200**|Responds with Workflow details including Id, Status, Jobs, Input, Namespace, and WorkflowDefinition|[Workflow](#workflow)|
|**404**|Entity Not Found|[NotFound](#notfound)|


<a name="getworkflows"></a>
### Get summary of all active Workflows for a given WorkflowDefinition
```
GET /workflows
```


#### Parameters

|Type|Name|Description|Schema|Default|
|---|---|---|---|---|
|**Query**|**limit**  <br>*optional*|Maximum number of workflows to return. Defaults to 10. Restricted to a max of 10,000.|integer|`10`|
|**Query**|**oldestFirst**  <br>*optional*||boolean||
|**Query**|**pageToken**  <br>*optional*||string||
|**Query**|**status**  <br>*optional*||string||
|**Query**|**summaryOnly**  <br>*optional*|Limits workflow data to the bare minimum - omits the full workflow definition and job data.|boolean|`"false"`|
|**Query**|**workflowDefinitionName**  <br>*required*||string||


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**200**|Workflow|< [Workflow](#workflow) > array|
|**404**|Entity Not Found|[NotFound](#notfound)|


<a name="resumeworkflowbyid"></a>
### Start a new Workflow using job outputs of a completed Workflow from the provided position
```
POST /workflows/{workflowID}
```


#### Parameters

|Type|Name|Schema|
|---|---|---|
|**Path**|**workflowID**  <br>*required*|string|
|**Body**|**overrides**  <br>*required*|[WorkflowDefinitionOverrides](#workflowdefinitionoverrides)|


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**200**|Workflow|[Workflow](#workflow)|
|**404**|Entity Not Found|[NotFound](#notfound)|


<a name="getworkflowbyid"></a>
### Get details for a single Workflow, given a workflowID
```
GET /workflows/{workflowID}
```


#### Parameters

|Type|Name|Schema|
|---|---|---|
|**Path**|**workflowID**  <br>*required*|string|


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**200**|Workflow|[Workflow](#workflow)|
|**404**|Entity Not Found|[NotFound](#notfound)|


<a name="cancelworkflow"></a>
### Cancel workflow with the given workflowID
```
DELETE /workflows/{workflowID}
```


#### Parameters

|Type|Name|Schema|
|---|---|---|
|**Path**|**workflowID**  <br>*required*|string|
|**Body**|**reason**  <br>*required*|[CancelReason](#cancelreason)|


#### Responses

|HTTP Code|Description|Schema|
|---|---|---|
|**200**|Workflow cancelled|No Content|
|**404**|Entity Not Found|[NotFound](#notfound)|



