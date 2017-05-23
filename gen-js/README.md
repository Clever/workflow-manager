<a name="module_workflow-manager"></a>

## workflow-manager
workflow-manager client library.


* [workflow-manager](#module_workflow-manager)
    * [WorkflowManager](#exp_module_workflow-manager--WorkflowManager) ⏏
        * [new WorkflowManager(options)](#new_module_workflow-manager--WorkflowManager_new)
        * _instance_
            * [.healthCheck([options], [cb])](#module_workflow-manager--WorkflowManager+healthCheck) ⇒ <code>Promise</code>
            * [.getJobsForWorkflow(workflowName, [options], [cb])](#module_workflow-manager--WorkflowManager+getJobsForWorkflow) ⇒ <code>Promise</code>
            * [.startJobForWorkflow(input, [options], [cb])](#module_workflow-manager--WorkflowManager+startJobForWorkflow) ⇒ <code>Promise</code>
            * [.CancelJob(params, [options], [cb])](#module_workflow-manager--WorkflowManager+CancelJob) ⇒ <code>Promise</code>
            * [.GetJob(jobId, [options], [cb])](#module_workflow-manager--WorkflowManager+GetJob) ⇒ <code>Promise</code>
            * [.getWorkflows([options], [cb])](#module_workflow-manager--WorkflowManager+getWorkflows) ⇒ <code>Promise</code>
            * [.newWorkflow(NewWorkflowRequest, [options], [cb])](#module_workflow-manager--WorkflowManager+newWorkflow) ⇒ <code>Promise</code>
            * [.getWorkflowByName(name, [options], [cb])](#module_workflow-manager--WorkflowManager+getWorkflowByName) ⇒ <code>Promise</code>
            * [.updateWorkflow(params, [options], [cb])](#module_workflow-manager--WorkflowManager+updateWorkflow) ⇒ <code>Promise</code>
        * _static_
            * [.RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)
                * [.Exponential](#module_workflow-manager--WorkflowManager.RetryPolicies.Exponential)
                * [.Single](#module_workflow-manager--WorkflowManager.RetryPolicies.Single)
                * [.None](#module_workflow-manager--WorkflowManager.RetryPolicies.None)
            * [.Errors](#module_workflow-manager--WorkflowManager.Errors)
                * [.BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest) ⇐ <code>Error</code>
                * [.InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError) ⇐ <code>Error</code>
                * [.NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound) ⇐ <code>Error</code>

<a name="exp_module_workflow-manager--WorkflowManager"></a>

### WorkflowManager ⏏
workflow-manager client

**Kind**: Exported class  
<a name="new_module_workflow-manager--WorkflowManager_new"></a>

#### new WorkflowManager(options)
Create a new client object.


| Param | Type | Default | Description |
| --- | --- | --- | --- |
| options | <code>Object</code> |  | Options for constructing a client object. |
| [options.address] | <code>string</code> |  | URL where the server is located. Must provide this or the discovery argument |
| [options.discovery] | <code>bool</code> |  | Use clever-discovery to locate the server. Must provide this or the address argument |
| [options.timeout] | <code>number</code> |  | The timeout to use for all client requests, in milliseconds. This can be overridden on a per-request basis. |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | <code>RetryPolicies.Single</code> | The logic to determine which requests to retry, as well as how many times to retry. |

<a name="module_workflow-manager--WorkflowManager+healthCheck"></a>

#### workflowManager.healthCheck([options], [cb]) ⇒ <code>Promise</code>
Checks if the service is healthy

**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>undefined</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+getJobsForWorkflow"></a>

#### workflowManager.getJobsForWorkflow(workflowName, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object[]</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| workflowName | <code>string</code> |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+startJobForWorkflow"></a>

#### workflowManager.startJobForWorkflow(input, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| input |  |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+CancelJob"></a>

#### workflowManager.CancelJob(params, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>undefined</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| params | <code>Object</code> |  |
| params.jobId | <code>string</code> |  |
| params.reason |  |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+GetJob"></a>

#### workflowManager.GetJob(jobId, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| jobId | <code>string</code> |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+getWorkflows"></a>

#### workflowManager.getWorkflows([options], [cb]) ⇒ <code>Promise</code>
Get the latest versions of all available workflows

**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object[]</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+newWorkflow"></a>

#### workflowManager.newWorkflow(NewWorkflowRequest, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| NewWorkflowRequest |  |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+getWorkflowByName"></a>

#### workflowManager.getWorkflowByName(name, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| name | <code>string</code> |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+updateWorkflow"></a>

#### workflowManager.updateWorkflow(params, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| params | <code>Object</code> |  |
| [params.NewWorkflowRequest] |  |  |
| params.name | <code>string</code> |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager.RetryPolicies"></a>

#### WorkflowManager.RetryPolicies
Retry policies available to use.

**Kind**: static property of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  

* [.RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)
    * [.Exponential](#module_workflow-manager--WorkflowManager.RetryPolicies.Exponential)
    * [.Single](#module_workflow-manager--WorkflowManager.RetryPolicies.Single)
    * [.None](#module_workflow-manager--WorkflowManager.RetryPolicies.None)

<a name="module_workflow-manager--WorkflowManager.RetryPolicies.Exponential"></a>

##### RetryPolicies.Exponential
The exponential retry policy will retry five times with an exponential backoff.

**Kind**: static constant of <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code>  
<a name="module_workflow-manager--WorkflowManager.RetryPolicies.Single"></a>

##### RetryPolicies.Single
Use this retry policy to retry a request once.

**Kind**: static constant of <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code>  
<a name="module_workflow-manager--WorkflowManager.RetryPolicies.None"></a>

##### RetryPolicies.None
Use this retry policy to turn off retries.

**Kind**: static constant of <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code>  
<a name="module_workflow-manager--WorkflowManager.Errors"></a>

#### WorkflowManager.Errors
Errors returned by methods.

**Kind**: static property of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  

* [.Errors](#module_workflow-manager--WorkflowManager.Errors)
    * [.BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest) ⇐ <code>Error</code>
    * [.InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError) ⇐ <code>Error</code>
    * [.NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound) ⇐ <code>Error</code>

<a name="module_workflow-manager--WorkflowManager.Errors.BadRequest"></a>

##### Errors.BadRequest ⇐ <code>Error</code>
BadRequest

**Kind**: static class of <code>[Errors](#module_workflow-manager--WorkflowManager.Errors)</code>  
**Extends:** <code>Error</code>  
**Properties**

| Name | Type |
| --- | --- |
| message | <code>string</code> | 

<a name="module_workflow-manager--WorkflowManager.Errors.InternalError"></a>

##### Errors.InternalError ⇐ <code>Error</code>
InternalError

**Kind**: static class of <code>[Errors](#module_workflow-manager--WorkflowManager.Errors)</code>  
**Extends:** <code>Error</code>  
**Properties**

| Name | Type |
| --- | --- |
| message | <code>string</code> | 

<a name="module_workflow-manager--WorkflowManager.Errors.NotFound"></a>

##### Errors.NotFound ⇐ <code>Error</code>
NotFound

**Kind**: static class of <code>[Errors](#module_workflow-manager--WorkflowManager.Errors)</code>  
**Extends:** <code>Error</code>  
**Properties**

| Name | Type |
| --- | --- |
| message | <code>string</code> | 

