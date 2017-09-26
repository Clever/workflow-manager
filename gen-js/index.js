const async = require("async");
const discovery = require("clever-discovery");
const kayvee = require("kayvee");
const request = require("request");
const opentracing = require("opentracing");
const {commandFactory} = require("hystrixjs");
const RollingNumberEvent = require("hystrixjs/lib/metrics/RollingNumberEvent");

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
 * Request status log is used to
 * to output the status of a request returned
 * by the client.
 */
function responseLog(logger, req, res, err) {
  var res = res || { };
  var req = req || { };
  var logData = {
	"backend": "workflow-manager",
	"method": req.method || "",
	"uri": req.uri || "",
    "message": err || (res.statusMessage || ""),
    "status_code": res.statusCode || 0,
  };

  if (err) {
    logger.errorD("client-request-finished", logData);
  } else {
    logger.infoD("client-request-finished", logData);
  }
}

/**
 * Default circuit breaker options.
 * @alias module:workflow-manager.DefaultCircuitOptions
 */
const defaultCircuitOptions = {
  forceClosed:            true,
  requestVolumeThreshold: 20,
  maxConcurrentRequests:  100,
  requestVolumeThreshold: 20,
  sleepWindow:            5000,
  errorPercentThreshold:  90,
  logIntervalMs:          30000
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
   * @param {module:kayvee.Logger} [options.logger=logger.New("workflow-manager-wagclient")] - The Kayvee 
   * logger to use in the client.
   * @param {Object} [options.circuit] - Options for constructing the client's circuit breaker.
   * @param {bool} [options.circuit.forceClosed] - When set to true the circuit will always be closed. Default: true.
   * @param {number} [options.circuit.maxConcurrentRequests] - the maximum number of concurrent requests
   * the client can make at the same time. Default: 100.
   * @param {number} [options.circuit.requestVolumeThreshold] - The minimum number of requests needed
   * before a circuit can be tripped due to health. Default: 20.
   * @param {number} [options.circuit.sleepWindow] - how long, in milliseconds, to wait after a circuit opens
   * before testing for recovery. Default: 5000.
   * @param {number} [options.circuit.errorPercentThreshold] - the threshold to place on the rolling error
   * rate. Once the error rate exceeds this percentage, the circuit opens.
   * Default: 90.
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
    if (options.logger) {
      this.logger = options.logger;
    } else {
      this.logger =  new kayvee.logger("workflow-manager-wagclient");
    }

    const circuitOptions = Object.assign({}, defaultCircuitOptions, options.circuit);
    this._hystrixCommand = commandFactory.getOrCreate("workflow-manager").
      errorHandler(this._hystrixCommandErrorHandler).
      circuitBreakerForceClosed(circuitOptions.forceClosed).
      requestVolumeRejectionThreshold(circuitOptions.maxConcurrentRequests).
      circuitBreakerRequestVolumeThreshold(circuitOptions.requestVolumeThreshold).
      circuitBreakerSleepWindowInMilliseconds(circuitOptions.sleepWindow).
      circuitBreakerErrorThresholdPercentage(circuitOptions.errorPercentThreshold).
      timeout(0).
      statisticalWindowLength(10000).
      statisticalWindowNumberOfBuckets(10).
      run(this._hystrixCommandRun).
      context(this).
      build();

    setInterval(() => this._logCircuitState(), circuitOptions.logIntervalMs);
  }

  _hystrixCommandErrorHandler(err) {
    // to avoid counting 4XXs as errors, only count an error if it comes from the request library
    if (err._fromRequest === true) {
      return err;
    }
    return false;
  }

  _hystrixCommandRun(method, args) {
    return method.apply(this, args);
  }

  _logCircuitState(logger) {
    // code below heavily borrows from hystrix's internal HystrixSSEStream.js logic
    const metrics = this._hystrixCommand.metrics;
    const healthCounts = metrics.getHealthCounts()
    const circuitBreaker = this._hystrixCommand.circuitBreaker;
    this.logger.infoD("workflow-manager", {
      "requestCount":                    healthCounts.totalCount,
      "errorCount":                      healthCounts.errorCount,
      "errorPercentage":                 healthCounts.errorPercentage,
      "isCircuitBreakerOpen":            circuitBreaker.isOpen(),
      "rollingCountFailure":             metrics.getRollingCount(RollingNumberEvent.FAILURE),
      "rollingCountShortCircuited":      metrics.getRollingCount(RollingNumberEvent.SHORT_CIRCUITED),
      "rollingCountSuccess":             metrics.getRollingCount(RollingNumberEvent.SUCCESS),
      "rollingCountTimeout":             metrics.getRollingCount(RollingNumberEvent.TIMEOUT),
      "currentConcurrentExecutionCount": metrics.getCurrentExecutionCount(),
      "latencyTotalMean":                metrics.getExecutionTime("mean") || 0,
    });
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
    return this._hystrixCommand.execute(this._healthCheck, arguments);
  }
  _healthCheck(options, cb) {
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
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver();
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param NewStateResource
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
  postStateResource(NewStateResource, options, cb) {
    return this._hystrixCommand.execute(this._postStateResource, arguments);
  }
  _postStateResource(NewStateResource, options, cb) {
    const params = {};
    params["NewStateResource"] = NewStateResource;

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
        span.logEvent("POST /state-resources");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "POST",
        uri: this.address + "/state-resources",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  
      requestOptions.body = params.NewStateResource;
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 201:
              resolver(body);
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param {string} params.namespace
   * @param {string} params.name
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
  deleteStateResource(params, options, cb) {
    return this._hystrixCommand.execute(this._deleteStateResource, arguments);
  }
  _deleteStateResource(params, options, cb) {
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
      if (!params.namespace) {
        rejecter(new Error("namespace must be non-empty because it's a path parameter"));
        return;
      }
      if (!params.name) {
        rejecter(new Error("name must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("DELETE /state-resources/{namespace}/{name}");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "DELETE",
        uri: this.address + "/state-resources/" + params.namespace + "/" + params.name + "",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver();
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param {string} params.namespace
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
  getStateResource(params, options, cb) {
    return this._hystrixCommand.execute(this._getStateResource, arguments);
  }
  _getStateResource(params, options, cb) {
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
      if (!params.namespace) {
        rejecter(new Error("namespace must be non-empty because it's a path parameter"));
        return;
      }
      if (!params.name) {
        rejecter(new Error("name must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("GET /state-resources/{namespace}/{name}");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "GET",
        uri: this.address + "/state-resources/" + params.namespace + "/" + params.name + "",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver(body);
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param {string} params.namespace
   * @param {string} params.name
   * @param [params.NewStateResource]
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
  putStateResource(params, options, cb) {
    return this._hystrixCommand.execute(this._putStateResource, arguments);
  }
  _putStateResource(params, options, cb) {
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
      if (!params.namespace) {
        rejecter(new Error("namespace must be non-empty because it's a path parameter"));
        return;
      }
      if (!params.name) {
        rejecter(new Error("name must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("PUT /state-resources/{namespace}/{name}");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "PUT",
        uri: this.address + "/state-resources/" + params.namespace + "/" + params.name + "",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  
      requestOptions.body = params.NewStateResource;
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 201:
              resolver(body);
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * Get the latest versions of all available WorkflowDefinitions
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
  getWorkflowDefinitions(options, cb) {
    return this._hystrixCommand.execute(this._getWorkflowDefinitions, arguments);
  }
  _getWorkflowDefinitions(options, cb) {
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
        span.logEvent("GET /workflow-definitions");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "GET",
        uri: this.address + "/workflow-definitions",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver(body);
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param NewWorkflowDefinitionRequest
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
  newWorkflowDefinition(NewWorkflowDefinitionRequest, options, cb) {
    return this._hystrixCommand.execute(this._newWorkflowDefinition, arguments);
  }
  _newWorkflowDefinition(NewWorkflowDefinitionRequest, options, cb) {
    const params = {};
    params["NewWorkflowDefinitionRequest"] = NewWorkflowDefinitionRequest;

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
        span.logEvent("POST /workflow-definitions");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "POST",
        uri: this.address + "/workflow-definitions",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  
      requestOptions.body = params.NewWorkflowDefinitionRequest;
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 201:
              resolver(body);
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param {string} params.name
   * @param {boolean} [params.latest=true]
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
  getWorkflowDefinitionVersionsByName(params, options, cb) {
    return this._hystrixCommand.execute(this._getWorkflowDefinitionVersionsByName, arguments);
  }
  _getWorkflowDefinitionVersionsByName(params, options, cb) {
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
      if (typeof params.latest !== "undefined") {
        query["latest"] = params.latest;
      }
  

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("GET /workflow-definitions/{name}");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "GET",
        uri: this.address + "/workflow-definitions/" + params.name + "",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver(body);
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param [params.NewWorkflowDefinitionRequest]
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
  updateWorkflowDefinition(params, options, cb) {
    return this._hystrixCommand.execute(this._updateWorkflowDefinition, arguments);
  }
  _updateWorkflowDefinition(params, options, cb) {
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
        span.logEvent("PUT /workflow-definitions/{name}");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "PUT",
        uri: this.address + "/workflow-definitions/" + params.name + "",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  
      requestOptions.body = params.NewWorkflowDefinitionRequest;
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 201:
              resolver(body);
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param {string} params.name
   * @param {number} params.version
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
  getWorkflowDefinitionByNameAndVersion(params, options, cb) {
    return this._hystrixCommand.execute(this._getWorkflowDefinitionByNameAndVersion, arguments);
  }
  _getWorkflowDefinitionByNameAndVersion(params, options, cb) {
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
      if (!params.version) {
        rejecter(new Error("version must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("GET /workflow-definitions/{name}/{version}");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "GET",
        uri: this.address + "/workflow-definitions/" + params.name + "/" + params.version + "",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver(body);
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param {number} [params.limit]
   * @param {boolean} [params.oldestFirst]
   * @param {string} [params.pageToken]
   * @param {string} [params.status]
   * @param {string} params.workflowDefinitionName
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
  getWorkflows(params, options, cb) {
    return this._hystrixCommand.execute(this._getWorkflows, arguments);
  }
  _getWorkflows(params, options, cb) {
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
      if (typeof params.limit !== "undefined") {
        query["limit"] = params.limit;
      }
  
      if (typeof params.oldestFirst !== "undefined") {
        query["oldestFirst"] = params.oldestFirst;
      }
  
      if (typeof params.pageToken !== "undefined") {
        query["pageToken"] = params.pageToken;
      }
  
      if (typeof params.status !== "undefined") {
        query["status"] = params.status;
      }
  
      query["workflowDefinitionName"] = params.workflowDefinitionName;
  

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
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver(body);
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }


  /**
   * @param {Object} params
   * @param {number} [params.limit]
   * @param {boolean} [params.oldestFirst]
   * @param {string} [params.pageToken]
   * @param {string} [params.status]
   * @param {string} params.workflowDefinitionName
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {external:Span} [options.span] - An OpenTracing span - For example from the parent request
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @returns {Object} iter
   * @returns {function} iter.map - takes in a function, applies it to each resource, and returns a promise to the result as an array
   * @returns {function} iter.toArray - returns a promise to the resources as an array
   * @returns {function} iter.forEach - takes in a function, applies it to each resource
   */
  getWorkflowsIter(params, options) {
    const it = (f, saveResults, cb) => new Promise((resolve, reject) => {
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
      if (typeof params.limit !== "undefined") {
        query["limit"] = params.limit;
      }
  
      if (typeof params.oldestFirst !== "undefined") {
        query["oldestFirst"] = params.oldestFirst;
      }
  
      if (typeof params.pageToken !== "undefined") {
        query["pageToken"] = params.pageToken;
      }
  
      if (typeof params.status !== "undefined") {
        query["status"] = params.status;
      }
  
      query["workflowDefinitionName"] = params.workflowDefinitionName;
  

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
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
      const logger = this.logger;
  
      let results = [];
      async.whilst(
        () => requestOptions.uri !== "",
        cbW => {
          if (span) {
            span.logEvent("GET /workflows");
          }
      const address = this.address;
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            cbW(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              if (saveResults) {
                results = results.concat(body.map(f));
              } else {
                body.forEach(f);
              }
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              cbW(err);
              return;
            
            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              cbW(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              cbW(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              cbW(err);
              return;
          }

          requestOptions.qs = null;
          requestOptions.useQuerystring = false;
          requestOptions.uri = "";
          if (response.headers["x-next-page-path"]) {
            requestOptions.uri = address + response.headers["x-next-page-path"];
          }
          cbW();
        });
      }());
        },
        err => {
          if (err) {
            rejecter(err);
            return;
          }
          if (saveResults) {
            resolver(results);
          } else {
            resolver();
          }
        }
      );
    });

    return {
      map: (f, cb) => this._hystrixCommand.execute(it, [f, true, cb]),
      toArray: cb => this._hystrixCommand.execute(it, [x => x, true, cb]),
      forEach: (f, cb) => this._hystrixCommand.execute(it, [f, false, cb]),
    };
  }

  /**
   * @param StartWorkflowRequest - Parameters for starting a workflow (workflow definition, input, and optionally namespace and queue)
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
  startWorkflow(StartWorkflowRequest, options, cb) {
    return this._hystrixCommand.execute(this._startWorkflow, arguments);
  }
  _startWorkflow(StartWorkflowRequest, options, cb) {
    const params = {};
    params["StartWorkflowRequest"] = StartWorkflowRequest;

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
  
      requestOptions.body = params.StartWorkflowRequest;
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver(body);
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param {string} params.workflowId
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
  CancelWorkflow(params, options, cb) {
    return this._hystrixCommand.execute(this._CancelWorkflow, arguments);
  }
  _CancelWorkflow(params, options, cb) {
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
      if (!params.workflowId) {
        rejecter(new Error("workflowId must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("DELETE /workflows/{workflowId}");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "DELETE",
        uri: this.address + "/workflows/" + params.workflowId + "",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  
      requestOptions.body = params.reason;
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver();
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {string} workflowId
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
  getWorkflowByID(workflowId, options, cb) {
    return this._hystrixCommand.execute(this._getWorkflowByID, arguments);
  }
  _getWorkflowByID(workflowId, options, cb) {
    const params = {};
    params["workflowId"] = workflowId;

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
      if (!params.workflowId) {
        rejecter(new Error("workflowId must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      if (span) {
        opentracing.inject(span, opentracing.FORMAT_TEXT_MAP, headers);
        span.logEvent("GET /workflows/{workflowId}");
        span.setTag("span.kind", "client");
      }

      const requestOptions = {
        method: "GET",
        uri: this.address + "/workflows/" + params.workflowId + "",
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
  

      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;
  
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
            err._fromRequest = true;
            responseLog(logger, requestOptions, response, err)
            rejecter(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolver(body);
              break;
            
            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
              return;
            
            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              rejecter(err);
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

module.exports.DefaultCircuitOptions = defaultCircuitOptions;
