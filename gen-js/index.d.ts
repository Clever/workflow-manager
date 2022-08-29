import { Logger } from "kayvee";

type Callback<R> = (err: Error, result: R) => void;
type ArrayInner<R> = R extends (infer T)[] ? T : never;

interface RetryPolicy {
  backoffs(): number[];
  retry(requestOptions: {method: string}, err: Error, res: {statusCode: number}): boolean;
}

interface RequestOptions {
  timeout?: number;
  retryPolicy?: RetryPolicy;
}

interface IterResult<R> {
  map<T>(f: (r: R) => T, cb?: Callback<T[]>): Promise<T[]>;
  toArray(cb?: Callback<R[]>): Promise<R[]>;
  forEach(f: (r: R) => void, cb?: Callback<void>): Promise<void>;
  forEachAsync(f: (r: R) => void, cb?: Callback<void>): Promise<void>;
}

interface CircuitOptions {
  forceClosed?: boolean;
  maxConcurrentRequests?: number;
  requestVolumeThreshold?: number;
  sleepWindow?: number;
  errorPercentThreshold?: number;
}

interface GenericOptions {
  timeout?: number;
  keepalive?: boolean;
  retryPolicy?: RetryPolicy;
  logger?: Logger;
  circuit?: CircuitOptions;
  serviceName?: string;
}

interface DiscoveryOptions {
  discovery: true;
  address?: undefined;
}

interface AddressOptions {
  discovery?: false;
  address: string;
}

type WorkflowManagerOptions = (DiscoveryOptions | AddressOptions) & GenericOptions;

import models = WorkflowManager.Models

declare class WorkflowManager {
  constructor(options: WorkflowManagerOptions);

  close(): void;
  
  healthCheck(options?: RequestOptions, cb?: Callback<void>): Promise<void>
  
  postStateResource(NewStateResource?: models.NewStateResource, options?: RequestOptions, cb?: Callback<models.StateResource>): Promise<models.StateResource>
  
  deleteStateResource(params: models.DeleteStateResourceParams, options?: RequestOptions, cb?: Callback<void>): Promise<void>
  
  getStateResource(params: models.GetStateResourceParams, options?: RequestOptions, cb?: Callback<models.StateResource>): Promise<models.StateResource>
  
  putStateResource(params: models.PutStateResourceParams, options?: RequestOptions, cb?: Callback<models.StateResource>): Promise<models.StateResource>
  
  getWorkflowDefinitions(options?: RequestOptions, cb?: Callback<models.WorkflowDefinition[]>): Promise<models.WorkflowDefinition[]>
  
  newWorkflowDefinition(NewWorkflowDefinitionRequest?: models.NewWorkflowDefinitionRequest, options?: RequestOptions, cb?: Callback<models.WorkflowDefinition>): Promise<models.WorkflowDefinition>
  
  getWorkflowDefinitionVersionsByName(params: models.GetWorkflowDefinitionVersionsByNameParams, options?: RequestOptions, cb?: Callback<models.WorkflowDefinition[]>): Promise<models.WorkflowDefinition[]>
  
  updateWorkflowDefinition(params: models.UpdateWorkflowDefinitionParams, options?: RequestOptions, cb?: Callback<models.WorkflowDefinition>): Promise<models.WorkflowDefinition>
  
  getWorkflowDefinitionByNameAndVersion(params: models.GetWorkflowDefinitionByNameAndVersionParams, options?: RequestOptions, cb?: Callback<models.WorkflowDefinition>): Promise<models.WorkflowDefinition>
  
  getWorkflows(params: models.GetWorkflowsParams, options?: RequestOptions, cb?: Callback<models.Workflow[]>): Promise<models.Workflow[]>
  getWorkflowsIter(params: models.GetWorkflowsParams, options?: RequestOptions): IterResult<ArrayInner<models.Workflow[]>>
  
  startWorkflow(StartWorkflowRequest?: models.StartWorkflowRequest, options?: RequestOptions, cb?: Callback<models.Workflow>): Promise<models.Workflow>
  
  CancelWorkflow(params: models.CancelWorkflowParams, options?: RequestOptions, cb?: Callback<void>): Promise<void>
  
  getWorkflowByID(workflowID: string, options?: RequestOptions, cb?: Callback<models.Workflow>): Promise<models.Workflow>
  
  resumeWorkflowByID(params: models.ResumeWorkflowByIDParams, options?: RequestOptions, cb?: Callback<models.Workflow>): Promise<models.Workflow>
  
  resolveWorkflowByID(workflowID: string, options?: RequestOptions, cb?: Callback<void>): Promise<void>
  
}

declare namespace WorkflowManager {
  const RetryPolicies: {
    Single: RetryPolicy;
    Exponential: RetryPolicy;
    None: RetryPolicy;
  }

  const DefaultCircuitOptions: CircuitOptions;

  namespace Errors {
    interface ErrorBody {
      message: string;
      [key: string]: any;
    }

    
    class BadRequest {
  message?: string;

  constructor(body: ErrorBody);
}
    
    class InternalError {
  message?: string;

  constructor(body: ErrorBody);
}
    
    class NotFound {
  message?: string;

  constructor(body: ErrorBody);
}
    
    class Conflict {
  message?: string;

  constructor(body: ErrorBody);
}
    
  }

  namespace Models {
    
    type CancelReason = {
  reason?: string;
};
    
    type CancelWorkflowParams = {
  workflowID: string;
  reason: CancelReason;
};
    
    type Conflict = {
  message?: string;
};
    
    type DeleteStateResourceParams = {
  namespace: string;
  name: string;
};
    
    type GetStateResourceParams = {
  namespace: string;
  name: string;
};
    
    type GetWorkflowDefinitionByNameAndVersionParams = {
  name: string;
  version: number;
};
    
    type GetWorkflowDefinitionVersionsByNameParams = {
  name: string;
  latest?: boolean;
};
    
    type GetWorkflowsParams = {
  limit?: number;
  oldestFirst?: boolean;
  pageToken?: string;
  status?: string;
  resolvedByUser?: boolean;
  summaryOnly?: boolean;
  workflowDefinitionName: string;
};
    
    type Job = {
  attempts?: JobAttempt[];
  container?: string;
  createdAt?: string;
  id?: string;
  input?: string;
  name?: string;
  output?: string;
  queue?: string;
  startedAt?: string;
  state?: string;
  stateResource?: StateResource;
  status?: JobStatus;
  statusReason?: string;
  stoppedAt?: string;
};
    
    type JobAttempt = {
  containerInstanceARN?: string;
  createdAt?: string;
  exitCode?: number;
  reason?: string;
  startedAt?: string;
  stoppedAt?: string;
  taskARN?: string;
};
    
    type JobStatus = ("created" | "queued" | "waiting_for_deps" | "running" | "succeeded" | "failed" | "aborted_deps_failed" | "aborted_by_user");
    
    type Manager = ("step-functions");
    
    type NewStateResource = {
  name?: string;
  namespace?: string;
  uri?: string;
};
    
    type NewWorkflowDefinitionRequest = {
  defaultTags?: { [key: string]: {
  
} };
  manager?: Manager;
  name?: string;
  stateMachine?: SLStateMachine;
  version?: number;
};
    
    type PutStateResourceParams = {
  namespace: string;
  name: string;
  NewStateResource?: NewStateResource;
};
    
    type ResolvedByUserWrapper = {
  isSet?: boolean;
  value?: boolean;
};
    
    type ResumeWorkflowByIDParams = {
  workflowID: string;
  overrides: WorkflowDefinitionOverrides;
};
    
    type SLCatcher = {
  ErrorEquals?: SLErrorEquals[];
  Next?: string;
  ResultPath?: string;
};
    
    type SLChoice = {
  And?: SLChoice[];
  BooleanEquals?: boolean;
  Next?: string;
  Not?: SLChoice;
  NumericEquals?: number;
  NumericGreaterThan?: number;
  NumericGreaterThanEquals?: number;
  NumericLessThan?: number;
  NumericLessThanEquals?: number;
  Or?: SLChoice[];
  StringEquals?: string;
  StringGreaterThan?: string;
  StringGreaterThanEquals?: string;
  StringLessThan?: string;
  StringLessThanEquals?: string;
  TimestampEquals?: string;
  TimestampGreaterThan?: string;
  TimestampGreaterThanEquals?: string;
  TimestampLessThan?: string;
  TimestampLessThanEquals?: string;
  Variable?: string;
};
    
    type SLErrorEquals = string;
    
    type SLRetrier = {
  BackoffRate?: number;
  ErrorEquals?: SLErrorEquals[];
  IntervalSeconds?: number;
  MaxAttempts?: number;
};
    
    type SLState = {
  Branches?: SLStateMachine[];
  Catch?: SLCatcher[];
  Cause?: string;
  Choices?: SLChoice[];
  Comment?: string;
  Default?: string;
  End?: boolean;
  Error?: string;
  HeartbeatSeconds?: number;
  InputPath?: string;
  ItemsPath?: string;
  Iterator?: SLStateMachine;
  MaxConcurrency?: number;
  Next?: string;
  OutputPath?: string;
  Parameters?: { [key: string]: {
  
} };
  Resource?: string;
  Result?: string;
  ResultPath?: string;
  ResultSelector?: { [key: string]: string };
  Retry?: SLRetrier[];
  Seconds?: number;
  SecondsPath?: string;
  TimeoutSeconds?: number;
  Timestamp?: string;
  TimestampPath?: string;
  Type?: SLStateType;
};
    
    type SLStateMachine = {
  Comment?: string;
  StartAt?: string;
  States?: { [key: string]: SLState };
  TimeoutSeconds?: number;
  Version?: ("1.0");
};
    
    type SLStateType = ("Pass" | "Task" | "Choice" | "Wait" | "Succeed" | "Fail" | "Parallel" | "Map");
    
    type StartWorkflowRequest = {
  idSuffix?: string;
  input?: string;
  namespace?: string;
  queue?: string;
  tags?: { [key: string]: {
  
} };
  workflowDefinition?: WorkflowDefinitionRef;
};
    
    type StateResource = {
  lastUpdated?: string;
  name?: string;
  namespace?: string;
  type?: StateResourceType;
  uri?: string;
};
    
    type StateResourceType = ("JobDefinitionARN" | "ActivityARN" | "LambdaFunctionARN" | "TaskARN");
    
    type UpdateWorkflowDefinitionParams = {
  NewWorkflowDefinitionRequest?: NewWorkflowDefinitionRequest;
  name: string;
};
    
    type Workflow = WorkflowSummary & {
  jobs?: Job[];
  output?: string;
  statusReason?: string;
};
    
    type WorkflowDefinition = {
  createdAt?: string;
  defaultTags?: { [key: string]: {
  
} };
  id?: string;
  manager?: Manager;
  name?: string;
  stateMachine?: SLStateMachine;
  version?: number;
};
    
    type WorkflowDefinitionOverrides = {
  StartAt?: string;
};
    
    type WorkflowDefinitionRef = {
  name?: string;
  version?: number;
};
    
    type WorkflowQuery = {
  limit?: number;
  oldestFirst?: boolean;
  pageToken?: string;
  resolvedByUserWrapper?: ResolvedByUserWrapper;
  status?: WorkflowStatus;
  summaryOnly?: boolean;
  workflowDefinitionName: string;
};
    
    type WorkflowStatus = ("queued" | "running" | "failed" | "succeeded" | "cancelled");
    
    type WorkflowSummary = {
  createdAt?: string;
  id?: string;
  input?: string;
  lastJob?: Job;
  lastUpdated?: string;
  namespace?: string;
  queue?: string;
  resolvedByUser?: boolean;
  retries?: string[];
  retryFor?: string;
  status?: WorkflowStatus;
  stoppedAt?: string;
  tags?: { [key: string]: {
  
} };
  workflowDefinition?: WorkflowDefinition;
};
    
  }
}

export = WorkflowManager;
