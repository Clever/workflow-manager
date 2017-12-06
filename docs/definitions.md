
<a name="definitions"></a>
## Definitions

<a name="badrequest"></a>
### BadRequest

|Name|Schema|
|---|---|
|**message**  <br>*optional*|string|


<a name="cancelreason"></a>
### CancelReason

|Name|Schema|
|---|---|
|**reason**  <br>*optional*|string|


<a name="conflict"></a>
### Conflict

|Name|Schema|
|---|---|
|**message**  <br>*optional*|string|


<a name="internalerror"></a>
### InternalError

|Name|Schema|
|---|---|
|**message**  <br>*optional*|string|


<a name="job"></a>
### Job

|Name|Schema|
|---|---|
|**attempts**  <br>*optional*|< [JobAttempt](#jobattempt) > array|
|**container**  <br>*optional*|string|
|**createdAt**  <br>*optional*|string (date-time)|
|**id**  <br>*optional*|string|
|**input**  <br>*optional*|string|
|**name**  <br>*optional*|string|
|**output**  <br>*optional*|string|
|**queue**  <br>*optional*|string|
|**startedAt**  <br>*optional*|string (date-time)|
|**state**  <br>*optional*|string|
|**stateResource**  <br>*optional*|[StateResource](#stateresource)|
|**status**  <br>*optional*|[JobStatus](#jobstatus)|
|**statusReason**  <br>*optional*|string|
|**stoppedAt**  <br>*optional*|string (date-time)|


<a name="jobattempt"></a>
### JobAttempt

|Name|Schema|
|---|---|
|**containerInstanceARN**  <br>*optional*|string|
|**createdAt**  <br>*optional*|string (date-time)|
|**exitCode**  <br>*optional*|integer|
|**reason**  <br>*optional*|string|
|**startedAt**  <br>*optional*|string (date-time)|
|**stoppedAt**  <br>*optional*|string (date-time)|
|**taskARN**  <br>*optional*|string|


<a name="jobstatus"></a>
### JobStatus
*Type* : enum (created, queued, waiting_for_deps, running, succeeded, failed, aborted_deps_failed, aborted_by_user)


<a name="manager"></a>
### Manager
*Type* : enum (step-functions)


<a name="newstateresource"></a>
### NewStateResource

|Name|Schema|
|---|---|
|**name**  <br>*optional*|string|
|**namespace**  <br>*optional*|string|
|**uri**  <br>*optional*|string|


<a name="newworkflowdefinitionrequest"></a>
### NewWorkflowDefinitionRequest

|Name|Schema|
|---|---|
|**manager**  <br>*optional*|[Manager](#manager)|
|**name**  <br>*optional*|string|
|**stateMachine**  <br>*optional*|[SLStateMachine](#slstatemachine)|


<a name="notfound"></a>
### NotFound

|Name|Schema|
|---|---|
|**message**  <br>*optional*|string|


<a name="resolvedbyuserwrapper"></a>
### ResolvedByUserWrapper

|Name|Schema|
|---|---|
|**isSet**  <br>*optional*|boolean|
|**value**  <br>*optional*|boolean|


<a name="slcatcher"></a>
### SLCatcher

|Name|Schema|
|---|---|
|**ErrorEquals**  <br>*optional*|< [SLErrorEquals](#slerrorequals) > array|
|**Next**  <br>*optional*|string|
|**ResultPath**  <br>*optional*|string|


<a name="slchoice"></a>
### SLChoice

|Name|Schema|
|---|---|
|**And**  <br>*optional*|< [SLChoice](#slchoice) > array|
|**BooleanEquals**  <br>*optional*|boolean|
|**Next**  <br>*optional*|string|
|**Not**  <br>*optional*|[SLChoice](#slchoice)|
|**NumericEquals**  <br>*optional*|integer|
|**NumericGreaterThan**  <br>*optional*|number|
|**NumericGreaterThanEquals**  <br>*optional*|integer|
|**NumericLessThan**  <br>*optional*|number|
|**NumericLessThanEquals**  <br>*optional*|integer|
|**Or**  <br>*optional*|< [SLChoice](#slchoice) > array|
|**StringEquals**  <br>*optional*|string|
|**StringGreaterThan**  <br>*optional*|string|
|**StringGreaterThanEquals**  <br>*optional*|string|
|**StringLessThan**  <br>*optional*|string|
|**StringLessThanEquals**  <br>*optional*|string|
|**TimestampEquals**  <br>*optional*|string (date-time)|
|**TimestampGreaterThan**  <br>*optional*|string (date-time)|
|**TimestampGreaterThanEquals**  <br>*optional*|string (date-time)|
|**TimestampLessThan**  <br>*optional*|string (date-time)|
|**TimestampLessThanEquals**  <br>*optional*|string (date-time)|
|**Variable**  <br>*optional*|string|


<a name="slerrorequals"></a>
### SLErrorEquals
*Type* : string


<a name="slretrier"></a>
### SLRetrier

|Name|Description|Schema|
|---|---|---|
|**BackoffRate**  <br>*optional*||number|
|**ErrorEquals**  <br>*optional*||< [SLErrorEquals](#slerrorequals) > array|
|**IntervalSeconds**  <br>*optional*||integer|
|**MaxAttempts**  <br>*optional*|**Minimum value** : `0`  <br>**Maximum value** : `10`|integer|


<a name="slstate"></a>
### SLState

|Name|Schema|
|---|---|
|**Catch**  <br>*optional*|< [SLCatcher](#slcatcher) > array|
|**Cause**  <br>*optional*|string|
|**Choices**  <br>*optional*|< [SLChoice](#slchoice) > array|
|**Comment**  <br>*optional*|string|
|**Default**  <br>*optional*|string|
|**End**  <br>*optional*|boolean|
|**Error**  <br>*optional*|string|
|**HeartbeatSeconds**  <br>*optional*|integer|
|**InputPath**  <br>*optional*|string|
|**Next**  <br>*optional*|string|
|**OutputPath**  <br>*optional*|string|
|**Resource**  <br>*optional*|string|
|**Result**  <br>*optional*|string|
|**ResultPath**  <br>*optional*|string|
|**Retry**  <br>*optional*|< [SLRetrier](#slretrier) > array|
|**TimeoutSeconds**  <br>*optional*|integer|
|**Type**  <br>*optional*|[SLStateType](#slstatetype)|


<a name="slstatemachine"></a>
### SLStateMachine

|Name|Schema|
|---|---|
|**Comment**  <br>*optional*|string|
|**StartAt**  <br>*optional*|string|
|**TimeoutSeconds**  <br>*optional*|integer|
|**Version**  <br>*optional*|enum (1.0)|


<a name="slstatetype"></a>
### SLStateType
*Type* : enum (Pass, Task, Choice, Wait, Succeed, Fail, Parallel)


<a name="startworkflowrequest"></a>
### StartWorkflowRequest

|Name|Schema|
|---|---|
|**input**  <br>*optional*|string|
|**namespace**  <br>*optional*|string|
|**queue**  <br>*optional*|string|
|**workflowDefinition**  <br>*optional*|[WorkflowDefinitionRef](#workflowdefinitionref)|


<a name="stateresource"></a>
### StateResource

|Name|Schema|
|---|---|
|**lastUpdated**  <br>*optional*|string (date-time)|
|**name**  <br>*optional*|string|
|**namespace**  <br>*optional*|string|
|**type**  <br>*optional*|[StateResourceType](#stateresourcetype)|
|**uri**  <br>*optional*|string|


<a name="stateresourcetype"></a>
### StateResourceType
*Type* : enum (JobDefinitionARN, ActivityARN)


<a name="workflow"></a>
### Workflow
*Polymorphism* : Composition


|Name|Description|Schema|
|---|---|---|
|**createdAt**  <br>*optional*||string (date-time)|
|**id**  <br>*optional*||string|
|**input**  <br>*optional*||string|
|**jobs**  <br>*optional*||< [Job](#job) > array|
|**lastUpdated**  <br>*optional*||string (date-time)|
|**namespace**  <br>*optional*||string|
|**output**  <br>*optional*||string|
|**queue**  <br>*optional*||string|
|**resolvedByUser**  <br>*optional*||boolean|
|**retries**  <br>*optional*|workflow-id's of workflows created as retries for this workflow|< string > array|
|**retryFor**  <br>*optional*|workflow-id of original workflow in case this is a retry|string|
|**status**  <br>*optional*||[WorkflowStatus](#workflowstatus)|
|**statusReason**  <br>*optional*||string|
|**stoppedAt**  <br>*optional*||string (date-time)|
|**workflowDefinition**  <br>*optional*||[WorkflowDefinition](#workflowdefinition)|


<a name="workflowdefinition"></a>
### WorkflowDefinition

|Name|Schema|
|---|---|
|**createdAt**  <br>*optional*|string (date-time)|
|**id**  <br>*optional*|string|
|**manager**  <br>*optional*|[Manager](#manager)|
|**name**  <br>*optional*|string|
|**stateMachine**  <br>*optional*|[SLStateMachine](#slstatemachine)|
|**version**  <br>*optional*|integer|


<a name="workflowdefinitionoverrides"></a>
### WorkflowDefinitionOverrides

|Name|Schema|
|---|---|
|**StartAt**  <br>*optional*|string|


<a name="workflowdefinitionref"></a>
### WorkflowDefinitionRef

|Name|Schema|
|---|---|
|**name**  <br>*optional*|string|
|**version**  <br>*optional*|integer|


<a name="workflowquery"></a>
### WorkflowQuery

|Name|Description|Schema|
|---|---|---|
|**limit**  <br>*optional*|**Maximum value** : `10000`|integer|
|**oldestFirst**  <br>*optional*||boolean|
|**pageToken**  <br>*optional*||string|
|**resolvedByUserWrapper**  <br>*optional*|Tracks whether the resolvedByUser query parameter was sent or omitted in the request.|[ResolvedByUserWrapper](#resolvedbyuserwrapper)|
|**status**  <br>*optional*||[WorkflowStatus](#workflowstatus)|
|**summaryOnly**  <br>*optional*|**Default** : `false`|boolean|
|**workflowDefinitionName**  <br>*required*||string|


<a name="workflowstatus"></a>
### WorkflowStatus
*Type* : enum (queued, running, failed, succeeded, cancelled)


<a name="workflowsummary"></a>
### WorkflowSummary

|Name|Description|Schema|
|---|---|---|
|**createdAt**  <br>*optional*||string (date-time)|
|**id**  <br>*optional*||string|
|**input**  <br>*optional*||string|
|**lastUpdated**  <br>*optional*||string (date-time)|
|**namespace**  <br>*optional*||string|
|**queue**  <br>*optional*||string|
|**resolvedByUser**  <br>*optional*||boolean|
|**retries**  <br>*optional*|workflow-id's of workflows created as retries for this workflow|< string > array|
|**retryFor**  <br>*optional*|workflow-id of original workflow in case this is a retry|string|
|**status**  <br>*optional*||[WorkflowStatus](#workflowstatus)|
|**stoppedAt**  <br>*optional*||string (date-time)|
|**workflowDefinition**  <br>*optional*||[WorkflowDefinition](#workflowdefinition)|



