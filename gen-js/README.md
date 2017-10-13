## Modules

<dl>
<dt><a href="#module_workflow-manager">workflow-manager</a></dt>
<dd><p>workflow-manager client library.</p>
</dd>
</dl>

## Functions

<dl>
<dt><a href="#responseLog">responseLog()</a></dt>
<dd><p>Request status log is used to
to output the status of a request returned
by the client.</p>
</dd>
</dl>

<a name="module_workflow-manager"></a>

## workflow-manager
workflow-manager client library.


* [workflow-manager](#module_workflow-manager)
    * [WorkflowManager](#exp_module_workflow-manager--WorkflowManager) ⏏
        * [new WorkflowManager(options)](#new_module_workflow-manager--WorkflowManager_new)
        * _instance_
            * [.healthCheck([options], [cb])](#module_workflow-manager--WorkflowManager+healthCheck) ⇒ <code>Promise</code>
            * [.postStateResource(NewStateResource, [options], [cb])](#module_workflow-manager--WorkflowManager+postStateResource) ⇒ <code>Promise</code>
            * [.deleteStateResource(params, [options], [cb])](#module_workflow-manager--WorkflowManager+deleteStateResource) ⇒ <code>Promise</code>
            * [.getStateResource(params, [options], [cb])](#module_workflow-manager--WorkflowManager+getStateResource) ⇒ <code>Promise</code>
            * [.putStateResource(params, [options], [cb])](#module_workflow-manager--WorkflowManager+putStateResource) ⇒ <code>Promise</code>
            * [.getWorkflowDefinitions([options], [cb])](#module_workflow-manager--WorkflowManager+getWorkflowDefinitions) ⇒ <code>Promise</code>
            * [.newWorkflowDefinition(NewWorkflowDefinitionRequest, [options], [cb])](#module_workflow-manager--WorkflowManager+newWorkflowDefinition) ⇒ <code>Promise</code>
            * [.getWorkflowDefinitionVersionsByName(params, [options], [cb])](#module_workflow-manager--WorkflowManager+getWorkflowDefinitionVersionsByName) ⇒ <code>Promise</code>
            * [.updateWorkflowDefinition(params, [options], [cb])](#module_workflow-manager--WorkflowManager+updateWorkflowDefinition) ⇒ <code>Promise</code>
            * [.getWorkflowDefinitionByNameAndVersion(params, [options], [cb])](#module_workflow-manager--WorkflowManager+getWorkflowDefinitionByNameAndVersion) ⇒ <code>Promise</code>
            * [.getWorkflows(params, [options], [cb])](#module_workflow-manager--WorkflowManager+getWorkflows) ⇒ <code>Promise</code>
            * [.getWorkflowsIter(params, [options])](#module_workflow-manager--WorkflowManager+getWorkflowsIter) ⇒ <code>Object</code> &#124; <code>function</code> &#124; <code>function</code> &#124; <code>function</code>
            * [.startWorkflow(StartWorkflowRequest, [options], [cb])](#module_workflow-manager--WorkflowManager+startWorkflow) ⇒ <code>Promise</code>
            * [.CancelWorkflow(params, [options], [cb])](#module_workflow-manager--WorkflowManager+CancelWorkflow) ⇒ <code>Promise</code>
            * [.getWorkflowByID(workflowID, [options], [cb])](#module_workflow-manager--WorkflowManager+getWorkflowByID) ⇒ <code>Promise</code>
            * [.resumeWorkflowByID(params, [options], [cb])](#module_workflow-manager--WorkflowManager+resumeWorkflowByID) ⇒ <code>Promise</code>
        * _static_
            * [.RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)
                * [.Exponential](#module_workflow-manager--WorkflowManager.RetryPolicies.Exponential)
                * [.Single](#module_workflow-manager--WorkflowManager.RetryPolicies.Single)
                * [.None](#module_workflow-manager--WorkflowManager.RetryPolicies.None)
            * [.Errors](#module_workflow-manager--WorkflowManager.Errors)
                * [.BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest) ⇐ <code>Error</code>
                * [.InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError) ⇐ <code>Error</code>
                * [.NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound) ⇐ <code>Error</code>
            * [.DefaultCircuitOptions](#module_workflow-manager--WorkflowManager.DefaultCircuitOptions)

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
| [options.logger] | <code>module:kayvee.Logger</code> | <code>logger.New(&quot;workflow-manager-wagclient&quot;)</code> | The Kayvee  logger to use in the client. |
| [options.circuit] | <code>Object</code> |  | Options for constructing the client's circuit breaker. |
| [options.circuit.forceClosed] | <code>bool</code> |  | When set to true the circuit will always be closed. Default: true. |
| [options.circuit.maxConcurrentRequests] | <code>number</code> |  | the maximum number of concurrent requests the client can make at the same time. Default: 100. |
| [options.circuit.requestVolumeThreshold] | <code>number</code> |  | The minimum number of requests needed before a circuit can be tripped due to health. Default: 20. |
| [options.circuit.sleepWindow] | <code>number</code> |  | how long, in milliseconds, to wait after a circuit opens before testing for recovery. Default: 5000. |
| [options.circuit.errorPercentThreshold] | <code>number</code> |  | the threshold to place on the rolling error rate. Once the error rate exceeds this percentage, the circuit opens. Default: 90. |

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

<a name="module_workflow-manager--WorkflowManager+postStateResource"></a>

#### workflowManager.postStateResource(NewStateResource, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| NewStateResource |  |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+deleteStateResource"></a>

#### workflowManager.deleteStateResource(params, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>undefined</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| params | <code>Object</code> |  |
| params.namespace | <code>string</code> |  |
| params.name | <code>string</code> |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+getStateResource"></a>

#### workflowManager.getStateResource(params, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| params | <code>Object</code> |  |
| params.namespace | <code>string</code> |  |
| params.name | <code>string</code> |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+putStateResource"></a>

#### workflowManager.putStateResource(params, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| params | <code>Object</code> |  |
| params.namespace | <code>string</code> |  |
| params.name | <code>string</code> |  |
| [params.NewStateResource] |  |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+getWorkflowDefinitions"></a>

#### workflowManager.getWorkflowDefinitions([options], [cb]) ⇒ <code>Promise</code>
Get the latest versions of all available WorkflowDefinitions

**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object[]</code>  
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

<a name="module_workflow-manager--WorkflowManager+newWorkflowDefinition"></a>

#### workflowManager.newWorkflowDefinition(NewWorkflowDefinitionRequest, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| NewWorkflowDefinitionRequest |  |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+getWorkflowDefinitionVersionsByName"></a>

#### workflowManager.getWorkflowDefinitionVersionsByName(params, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object[]</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| params | <code>Object</code> |  |  |
| params.name | <code>string</code> |  |  |
| [params.latest] | <code>boolean</code> | <code>true</code> |  |
| [options] | <code>object</code> |  |  |
| [options.timeout] | <code>number</code> |  | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> |  | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> |  | A request specific retryPolicy |
| [cb] | <code>function</code> |  |  |

<a name="module_workflow-manager--WorkflowManager+updateWorkflowDefinition"></a>

#### workflowManager.updateWorkflowDefinition(params, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| params | <code>Object</code> |  |
| [params.NewWorkflowDefinitionRequest] |  |  |
| params.name | <code>string</code> |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+getWorkflowDefinitionByNameAndVersion"></a>

#### workflowManager.getWorkflowDefinitionByNameAndVersion(params, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| params | <code>Object</code> |  |
| params.name | <code>string</code> |  |
| params.version | <code>number</code> |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+getWorkflows"></a>

#### workflowManager.getWorkflows(params, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object[]</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| params | <code>Object</code> |  |  |
| [params.limit] | <code>number</code> | <code>10</code> | Maximum number of workflows to return. Defaults to 10. Restricted to a max of 10,000. |
| [params.oldestFirst] | <code>boolean</code> |  |  |
| [params.pageToken] | <code>string</code> |  |  |
| [params.status] | <code>string</code> |  |  |
| [params.summaryOnly] | <code>boolean</code> |  | Limits workflow data to the bare minimum - omits the full workflow definition and job data. |
| params.workflowDefinitionName | <code>string</code> |  |  |
| [options] | <code>object</code> |  |  |
| [options.timeout] | <code>number</code> |  | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> |  | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> |  | A request specific retryPolicy |
| [cb] | <code>function</code> |  |  |

<a name="module_workflow-manager--WorkflowManager+getWorkflowsIter"></a>

#### workflowManager.getWorkflowsIter(params, [options]) ⇒ <code>Object</code> &#124; <code>function</code> &#124; <code>function</code> &#124; <code>function</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Returns**: <code>Object</code> - iter<code>function</code> - iter.map - takes in a function, applies it to each resource, and returns a promise to the result as an array<code>function</code> - iter.toArray - returns a promise to the resources as an array<code>function</code> - iter.forEach - takes in a function, applies it to each resource  

| Param | Type | Default | Description |
| --- | --- | --- | --- |
| params | <code>Object</code> |  |  |
| [params.limit] | <code>number</code> | <code>10</code> | Maximum number of workflows to return. Defaults to 10. Restricted to a max of 10,000. |
| [params.oldestFirst] | <code>boolean</code> |  |  |
| [params.pageToken] | <code>string</code> |  |  |
| [params.status] | <code>string</code> |  |  |
| [params.summaryOnly] | <code>boolean</code> |  | Limits workflow data to the bare minimum - omits the full workflow definition and job data. |
| params.workflowDefinitionName | <code>string</code> |  |  |
| [options] | <code>object</code> |  |  |
| [options.timeout] | <code>number</code> |  | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> |  | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> |  | A request specific retryPolicy |

<a name="module_workflow-manager--WorkflowManager+startWorkflow"></a>

#### workflowManager.startWorkflow(StartWorkflowRequest, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| StartWorkflowRequest |  | Parameters for starting a workflow (workflow definition, input, and optionally namespace, queue, and tags) |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+CancelWorkflow"></a>

#### workflowManager.CancelWorkflow(params, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>undefined</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| params | <code>Object</code> |  |
| params.workflowID | <code>string</code> |  |
| params.reason |  |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+getWorkflowByID"></a>

#### workflowManager.getWorkflowByID(workflowID, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| workflowID | <code>string</code> |  |
| [options] | <code>object</code> |  |
| [options.timeout] | <code>number</code> | A request specific timeout |
| [options.span] | <code>[Span](https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html)</code> | An OpenTracing span - For example from the parent request |
| [options.retryPolicy] | <code>[RetryPolicies](#module_workflow-manager--WorkflowManager.RetryPolicies)</code> | A request specific retryPolicy |
| [cb] | <code>function</code> |  |

<a name="module_workflow-manager--WorkflowManager+resumeWorkflowByID"></a>

#### workflowManager.resumeWorkflowByID(params, [options], [cb]) ⇒ <code>Promise</code>
**Kind**: instance method of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
**Fulfill**: <code>Object</code>  
**Reject**: <code>[BadRequest](#module_workflow-manager--WorkflowManager.Errors.BadRequest)</code>  
**Reject**: <code>[NotFound](#module_workflow-manager--WorkflowManager.Errors.NotFound)</code>  
**Reject**: <code>[InternalError](#module_workflow-manager--WorkflowManager.Errors.InternalError)</code>  
**Reject**: <code>Error</code>  

| Param | Type | Description |
| --- | --- | --- |
| params | <code>Object</code> |  |
| params.workflowID | <code>string</code> |  |
| params.overrides |  |  |
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

<a name="module_workflow-manager--WorkflowManager.DefaultCircuitOptions"></a>

#### WorkflowManager.DefaultCircuitOptions
Default circuit breaker options.

**Kind**: static constant of <code>[WorkflowManager](#exp_module_workflow-manager--WorkflowManager)</code>  
<a name="responseLog"></a>

## responseLog()
Request status log is used to
to output the status of a request returned
by the client.

**Kind**: global function  
