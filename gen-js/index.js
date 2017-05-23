const async = require("async");
const discovery = require("clever-discovery");
const request = require("request");
const opentracing = require("opentracing");

/**
 * @external Span
 * @see {@link https://doc.esdoc.org/github.com/opentracing/opentracing-javascript/class/src/span.js~Span.html}
 */

const { Errors } = require("./types");

/**
 * The exponential retry policy will retry five times with an exponential backoff.
 * @alias module:workflow-manager.RetryPolicies.Exponential
 */
const exponentialRetryPolicy = {
  backoffs() {
    const ret = [];
    let next = 100.0; // milliseconds
    const e = 0.05; // +/- 5% jitter
    while (ret.length < 5) {
      const jitter = ((Math.random() * 2) - 1) * e * next;
      ret.push(next + jitter);
      next *= 2;
    }
    return ret;
  },
  retry(requestOptions, err, res) {
    if (err || requestOptions.method === "POST" ||
        requestOptions.method === "PATCH" ||
        res.statusCode < 500) {
      return false;
    }
    return true;
  },
};

/**
 * Use this retry policy to retry a request once.
 * @alias module:workflow-manager.RetryPolicies.Single
 */
const singleRetryPolicy = {
  backoffs() {
    return [1000];
  },
  retry(requestOptions, err, res) {
    if (err || requestOptions.method === "POST" ||
        requestOptions.method === "PATCH" ||
        res.statusCode < 500) {
      return false;
    }
    return true;
  },
};

/**
 * Use this retry policy to turn off retries.
 * @alias module:workflow-manager.RetryPolicies.None
 */
const noRetryPolicy = {
  backoffs() {
    return [];
  },
  retry() {
    return false;
  },
};

/**
 * workflow-manager client library.
 * @module workflow-manager
 * @typicalname WorkflowManager
 */

/**
 * workflow-manager client
 * @alias module:workflow-manager
 */
class WorkflowManager {

  /**
   * Create a new client object.
   * @param {Object} options - Options for constructing a client object.
   * @param {string} [options.address] - URL where the server is located. Must provide
   * this or the discovery argument
   * @param {bool} [options.discovery] - Use clever-discovery to locate the server. Must provide
   * this or the address argument
   * @param {number} [options.timeout] - The timeout to use for all client requests,
   * in milliseconds. This can be overridden on a per-request basis.
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy=RetryPolicies.Single] - The logic to
   * determine which requests to retry, as well as how many times to retry.
   */
  constructor(options) {
    options = options || {};

    if (options.discovery) {
      try {
        this.address = discovery("workflow-manager", "http").url();
      } catch (e) {
        this.address = discovery("workflow-manager", "default").url();
      }
    } else if (options.address) {
      this.address = options.address;
    } else {
      throw new Error("Cannot initialize workflow-manager without discovery or address");
    }
    if (options.timeout) {
      this.timeout = options.timeout;
    }
    if (options.retryPolicy) {
      this.retryPolicy = options.retryPolicy;
    }
  }

  /**
   * Checks if the service is healthy
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {external:Span} [options.span] - An OpenTracing span - For example from the parent request
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {undefined}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  healthCheck(options, cb) {
    const params = {};

    if (!cb && typeof options === "function") {
      cb = options;
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      const rejecter = (err) => {
        reject(err);
        if (cb) {
          cb(err);
        }
      };
      const resolver = (data) => {
        resolve(data);
        if (cb) {
          cb(null, data);
        }
      };


      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;
      const span = options.span;

      const headers = {};

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("GET /_health");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "GET",
        uri: this.address + "/_health",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
  
      let retries = 0;
      (function requestOnce() {
        request(requestOptions, (err, response, body) => {
          if (retries < backoffs.length && retryPolicy.retry(requestOptions, err, response, body)) {
            const backoff = backoffs[retries];
            retries += 1;
            setTimeout(requestOnce, backoff);
            return;
          }
          if (err) {
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver();
              break;
            
            case 400:
              rejecter(new Errors.BadRequest(body || {}));
              return;
            
            case 500:
              rejecter(new Errors.InternalError(body || {}));
              return;
            
            default:
              rejecter(new Error("Received unexpected statusCode " + response.statusCode));
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {string} workflowName
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {external:Span} [options.span] - An OpenTracing span - For example from the parent request
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object[]}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.NotFound}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  getJobsForWorkflow(workflowName, options, cb) {
    const params = {};
    params["workflowName"] = workflowName;

    if (!cb && typeof options === "function") {
      cb = options;
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      const rejecter = (err) => {
        reject(err);
        if (cb) {
          cb(err);
        }
      };
      const resolver = (data) => {
        resolve(data);
        if (cb) {
          cb(null, data);
        }
      };


      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;
      const span = options.span;

      const headers = {};

      const query = {};
      query["workflowName"] = params.workflowName;
  

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("GET /jobs");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "GET",
        uri: this.address + "/jobs",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
  
      let retries = 0;
      (function requestOnce() {
        request(requestOptions, (err, response, body) => {
          if (retries < backoffs.length && retryPolicy.retry(requestOptions, err, response, body)) {
            const backoff = backoffs[retries];
            retries += 1;
            setTimeout(requestOnce, backoff);
            return;
          }
          if (err) {
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver(body);
              break;
            
            case 400:
              rejecter(new Errors.BadRequest(body || {}));
              return;
            
            case 404:
              rejecter(new Errors.NotFound(body || {}));
              return;
            
            case 500:
              rejecter(new Errors.InternalError(body || {}));
              return;
            
            default:
              rejecter(new Error("Received unexpected statusCode " + response.statusCode));
              return;
          }
        });
      }());
    });
  }

  /**
   * @param input
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {external:Span} [options.span] - An OpenTracing span - For example from the parent request
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.NotFound}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  startJobForWorkflow(input, options, cb) {
    const params = {};
    params["input"] = input;

    if (!cb && typeof options === "function") {
      cb = options;
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      const rejecter = (err) => {
        reject(err);
        if (cb) {
          cb(err);
        }
      };
      const resolver = (data) => {
        resolve(data);
        if (cb) {
          cb(null, data);
        }
      };


      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;
      const span = options.span;

      const headers = {};

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("POST /jobs");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "POST",
        uri: this.address + "/jobs",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  
      requestOptions.body = params.input;
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
  
      let retries = 0;
      (function requestOnce() {
        request(requestOptions, (err, response, body) => {
          if (retries < backoffs.length && retryPolicy.retry(requestOptions, err, response, body)) {
            const backoff = backoffs[retries];
            retries += 1;
            setTimeout(requestOnce, backoff);
            return;
          }
          if (err) {
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver(body);
              break;
            
            case 400:
              rejecter(new Errors.BadRequest(body || {}));
              return;
            
            case 404:
              rejecter(new Errors.NotFound(body || {}));
              return;
            
            case 500:
              rejecter(new Errors.InternalError(body || {}));
              return;
            
            default:
              rejecter(new Error("Received unexpected statusCode " + response.statusCode));
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param {string} params.jobId
   * @param params.reason
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {external:Span} [options.span] - An OpenTracing span - For example from the parent request
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {undefined}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.NotFound}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  CancelJob(params, options, cb) {
    if (!cb && typeof options === "function") {
      cb = options;
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      const rejecter = (err) => {
        reject(err);
        if (cb) {
          cb(err);
        }
      };
      const resolver = (data) => {
        resolve(data);
        if (cb) {
          cb(null, data);
        }
      };


      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;
      const span = options.span;

      const headers = {};
      if (!params.jobId) {
        rejecter(new Error("jobId must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("DELETE /jobs/{jobId}");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "DELETE",
        uri: this.address + "/jobs/" + params.jobId + "",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  
      requestOptions.body = params.reason;
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
  
      let retries = 0;
      (function requestOnce() {
        request(requestOptions, (err, response, body) => {
          if (retries < backoffs.length && retryPolicy.retry(requestOptions, err, response, body)) {
            const backoff = backoffs[retries];
            retries += 1;
            setTimeout(requestOnce, backoff);
            return;
          }
          if (err) {
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver();
              break;
            
            case 400:
              rejecter(new Errors.BadRequest(body || {}));
              return;
            
            case 404:
              rejecter(new Errors.NotFound(body || {}));
              return;
            
            case 500:
              rejecter(new Errors.InternalError(body || {}));
              return;
            
            default:
              rejecter(new Error("Received unexpected statusCode " + response.statusCode));
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {string} jobId
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {external:Span} [options.span] - An OpenTracing span - For example from the parent request
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.NotFound}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  GetJob(jobId, options, cb) {
    const params = {};
    params["jobId"] = jobId;

    if (!cb && typeof options === "function") {
      cb = options;
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      const rejecter = (err) => {
        reject(err);
        if (cb) {
          cb(err);
        }
      };
      const resolver = (data) => {
        resolve(data);
        if (cb) {
          cb(null, data);
        }
      };


      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;
      const span = options.span;

      const headers = {};
      if (!params.jobId) {
        rejecter(new Error("jobId must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("GET /jobs/{jobId}");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "GET",
        uri: this.address + "/jobs/" + params.jobId + "",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
  
      let retries = 0;
      (function requestOnce() {
        request(requestOptions, (err, response, body) => {
          if (retries < backoffs.length && retryPolicy.retry(requestOptions, err, response, body)) {
            const backoff = backoffs[retries];
            retries += 1;
            setTimeout(requestOnce, backoff);
            return;
          }
          if (err) {
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver(body);
              break;
            
            case 400:
              rejecter(new Errors.BadRequest(body || {}));
              return;
            
            case 404:
              rejecter(new Errors.NotFound(body || {}));
              return;
            
            case 500:
              rejecter(new Errors.InternalError(body || {}));
              return;
            
            default:
              rejecter(new Error("Received unexpected statusCode " + response.statusCode));
              return;
          }
        });
      }());
    });
  }

  /**
   * Get the latest versions of all available workflows
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {external:Span} [options.span] - An OpenTracing span - For example from the parent request
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object[]}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  getWorkflows(options, cb) {
    const params = {};

    if (!cb && typeof options === "function") {
      cb = options;
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      const rejecter = (err) => {
        reject(err);
        if (cb) {
          cb(err);
        }
      };
      const resolver = (data) => {
        resolve(data);
        if (cb) {
          cb(null, data);
        }
      };


      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;
      const span = options.span;

      const headers = {};

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("GET /workflows");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "GET",
        uri: this.address + "/workflows",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
  
      let retries = 0;
      (function requestOnce() {
        request(requestOptions, (err, response, body) => {
          if (retries < backoffs.length && retryPolicy.retry(requestOptions, err, response, body)) {
            const backoff = backoffs[retries];
            retries += 1;
            setTimeout(requestOnce, backoff);
            return;
          }
          if (err) {
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver(body);
              break;
            
            case 400:
              rejecter(new Errors.BadRequest(body || {}));
              return;
            
            case 500:
              rejecter(new Errors.InternalError(body || {}));
              return;
            
            default:
              rejecter(new Error("Received unexpected statusCode " + response.statusCode));
              return;
          }
        });
      }());
    });
  }

  /**
   * @param NewWorkflowRequest
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {external:Span} [options.span] - An OpenTracing span - For example from the parent request
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  newWorkflow(NewWorkflowRequest, options, cb) {
    const params = {};
    params["NewWorkflowRequest"] = NewWorkflowRequest;

    if (!cb && typeof options === "function") {
      cb = options;
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      const rejecter = (err) => {
        reject(err);
        if (cb) {
          cb(err);
        }
      };
      const resolver = (data) => {
        resolve(data);
        if (cb) {
          cb(null, data);
        }
      };


      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;
      const span = options.span;

      const headers = {};

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("POST /workflows");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "POST",
        uri: this.address + "/workflows",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  
      requestOptions.body = params.NewWorkflowRequest;
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
  
      let retries = 0;
      (function requestOnce() {
        request(requestOptions, (err, response, body) => {
          if (retries < backoffs.length && retryPolicy.retry(requestOptions, err, response, body)) {
            const backoff = backoffs[retries];
            retries += 1;
            setTimeout(requestOnce, backoff);
            return;
          }
          if (err) {
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 201:
              resolver(body);
              break;
            
            case 400:
              rejecter(new Errors.BadRequest(body || {}));
              return;
            
            case 500:
              rejecter(new Errors.InternalError(body || {}));
              return;
            
            default:
              rejecter(new Error("Received unexpected statusCode " + response.statusCode));
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {string} name
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {external:Span} [options.span] - An OpenTracing span - For example from the parent request
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.NotFound}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  getWorkflowByName(name, options, cb) {
    const params = {};
    params["name"] = name;

    if (!cb && typeof options === "function") {
      cb = options;
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      const rejecter = (err) => {
        reject(err);
        if (cb) {
          cb(err);
        }
      };
      const resolver = (data) => {
        resolve(data);
        if (cb) {
          cb(null, data);
        }
      };


      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;
      const span = options.span;

      const headers = {};
      if (!params.name) {
        rejecter(new Error("name must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("GET /workflows/{name}");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "GET",
        uri: this.address + "/workflows/" + params.name + "",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
  
      let retries = 0;
      (function requestOnce() {
        request(requestOptions, (err, response, body) => {
          if (retries < backoffs.length && retryPolicy.retry(requestOptions, err, response, body)) {
            const backoff = backoffs[retries];
            retries += 1;
            setTimeout(requestOnce, backoff);
            return;
          }
          if (err) {
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver(body);
              break;
            
            case 400:
              rejecter(new Errors.BadRequest(body || {}));
              return;
            
            case 404:
              rejecter(new Errors.NotFound(body || {}));
              return;
            
            case 500:
              rejecter(new Errors.InternalError(body || {}));
              return;
            
            default:
              rejecter(new Error("Received unexpected statusCode " + response.statusCode));
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param [params.NewWorkflowRequest]
   * @param {string} params.name
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {external:Span} [options.span] - An OpenTracing span - For example from the parent request
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.NotFound}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  updateWorkflow(params, options, cb) {
    if (!cb && typeof options === "function") {
      cb = options;
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      const rejecter = (err) => {
        reject(err);
        if (cb) {
          cb(err);
        }
      };
      const resolver = (data) => {
        resolve(data);
        if (cb) {
          cb(null, data);
        }
      };


      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;
      const span = options.span;

      const headers = {};
      if (!params.name) {
        rejecter(new Error("name must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("PUT /workflows/{name}");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "PUT",
        uri: this.address + "/workflows/" + params.name + "",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  
      requestOptions.body = params.NewWorkflowRequest;
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
  
      let retries = 0;
      (function requestOnce() {
        request(requestOptions, (err, response, body) => {
          if (retries < backoffs.length && retryPolicy.retry(requestOptions, err, response, body)) {
            const backoff = backoffs[retries];
            retries += 1;
            setTimeout(requestOnce, backoff);
            return;
          }
          if (err) {
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 201:
              resolver(body);
              break;
            
            case 400:
              rejecter(new Errors.BadRequest(body || {}));
              return;
            
            case 404:
              rejecter(new Errors.NotFound(body || {}));
              return;
            
            case 500:
              rejecter(new Errors.InternalError(body || {}));
              return;
            
            default:
              rejecter(new Error("Received unexpected statusCode " + response.statusCode));
              return;
          }
        });
      }());
    });
  }
};

module.exports = WorkflowManager;

/**
 * Retry policies available to use.
 * @alias module:workflow-manager.RetryPolicies
 */
module.exports.RetryPolicies = {
  Single: singleRetryPolicy,
  Exponential: exponentialRetryPolicy,
  None: noRetryPolicy,
};

/**
 * Errors returned by methods.
 * @alias module:workflow-manager.Errors
 */
module.exports.Errors = Errors;
