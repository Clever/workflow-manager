const async = require("async");
const discovery = require("clever-discovery");
const kayvee = require("kayvee");
const request = require("request");
const {commandFactory} = require("hystrixjs");
const RollingNumberEvent = require("hystrixjs/lib/metrics/RollingNumberEvent");

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
 * @private
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
 * Takes a promise and uses the provided callback (if any) to handle promise
 * resolutions and rejections
 * @private
 */
function applyCallback(promise, cb) {
  if (!cb) {
    return promise;
  }
  return promise.then((result) => {
    cb(null, result);
  }).catch((err) => {
    cb(err);
  });
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
   * in milliseconds. This can be overridden on a per-request basis. Default is 5000ms.
   * @param {bool} [options.keepalive] - Set keepalive to true for client requests. This sets the
   * forever: true attribute in request. Defaults to true.
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
        this.address = discovery(options.serviceName || "workflow-manager", "http").url();
      } catch (e) {
        this.address = discovery(options.serviceName || "workflow-manager", "default").url();
      }
    } else if (options.address) {
      this.address = options.address;
    } else {
      throw new Error("Cannot initialize workflow-manager without discovery or address");
    }
    if (options.keepalive !== undefined) {
      this.keepalive = options.keepalive;
    } else {
      this.keepalive = true;
    }
    if (options.timeout) {
      this.timeout = options.timeout;
    } else {
      this.timeout = 5000;
    }
    if (options.retryPolicy) {
      this.retryPolicy = options.retryPolicy;
    }
    if (options.logger) {
      this.logger = options.logger;
    } else {
      this.logger = new kayvee.logger((options.serviceName || "workflow-manager") + "-wagclient");
    }

    const circuitOptions = Object.assign({}, defaultCircuitOptions, options.circuit);
    this._hystrixCommand = commandFactory.getOrCreate(options.serviceName || "workflow-manager").
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

    this._logCircuitStateInterval = setInterval(() => this._logCircuitState(), circuitOptions.logIntervalMs);
  }

  /**
  * Releases handles used in client
  */
  close() {
    clearInterval(this._logCircuitStateInterval);
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
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {undefined}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  healthCheck(options, cb) {
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._healthCheck, arguments), callback);
  }

  _healthCheck(options, cb) {
    const params = {};

    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "healthCheck";
      headers[versionHeader] = version;

      const query = {};

      const requestOptions = {
        method: "GET",
        uri: this.address + "/_health",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }


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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolve();
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
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
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  postStateResource(NewStateResource, options, cb) {
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._postStateResource, arguments), callback);
  }

  _postStateResource(NewStateResource, options, cb) {
    const params = {};
    params["NewStateResource"] = NewStateResource;

    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "postStateResource";
      headers[versionHeader] = version;

      const query = {};

      const requestOptions = {
        method: "POST",
        uri: this.address + "/state-resources",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }

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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 201:
              resolve(body);
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
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
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._deleteStateResource, arguments), callback);
  }

  _deleteStateResource(params, options, cb) {
    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "deleteStateResource";
      headers[versionHeader] = version;
      if (!params.namespace) {
        reject(new Error("namespace must be non-empty because it's a path parameter"));
        return;
      }
      if (!params.name) {
        reject(new Error("name must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      const requestOptions = {
        method: "DELETE",
        uri: this.address + "/state-resources/" + params.namespace + "/" + params.name + "",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }


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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolve();
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
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
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._getStateResource, arguments), callback);
  }

  _getStateResource(params, options, cb) {
    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "getStateResource";
      headers[versionHeader] = version;
      if (!params.namespace) {
        reject(new Error("namespace must be non-empty because it's a path parameter"));
        return;
      }
      if (!params.name) {
        reject(new Error("name must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      const requestOptions = {
        method: "GET",
        uri: this.address + "/state-resources/" + params.namespace + "/" + params.name + "",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }


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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolve(body);
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
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
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  putStateResource(params, options, cb) {
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._putStateResource, arguments), callback);
  }

  _putStateResource(params, options, cb) {
    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "putStateResource";
      headers[versionHeader] = version;
      if (!params.namespace) {
        reject(new Error("namespace must be non-empty because it's a path parameter"));
        return;
      }
      if (!params.name) {
        reject(new Error("name must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      const requestOptions = {
        method: "PUT",
        uri: this.address + "/state-resources/" + params.namespace + "/" + params.name + "",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }

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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 201:
              resolve(body);
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
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
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object[]}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  getWorkflowDefinitions(options, cb) {
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._getWorkflowDefinitions, arguments), callback);
  }

  _getWorkflowDefinitions(options, cb) {
    const params = {};

    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "getWorkflowDefinitions";
      headers[versionHeader] = version;

      const query = {};

      const requestOptions = {
        method: "GET",
        uri: this.address + "/workflow-definitions",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }


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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolve(body);
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
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
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  newWorkflowDefinition(NewWorkflowDefinitionRequest, options, cb) {
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._newWorkflowDefinition, arguments), callback);
  }

  _newWorkflowDefinition(NewWorkflowDefinitionRequest, options, cb) {
    const params = {};
    params["NewWorkflowDefinitionRequest"] = NewWorkflowDefinitionRequest;

    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "newWorkflowDefinition";
      headers[versionHeader] = version;

      const query = {};

      const requestOptions = {
        method: "POST",
        uri: this.address + "/workflow-definitions",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }

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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 201:
              resolve(body);
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
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
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._getWorkflowDefinitionVersionsByName, arguments), callback);
  }

  _getWorkflowDefinitionVersionsByName(params, options, cb) {
    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "getWorkflowDefinitionVersionsByName";
      headers[versionHeader] = version;
      if (!params.name) {
        reject(new Error("name must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};
      if (typeof params.latest !== "undefined") {
        query["latest"] = params.latest;
      }


      const requestOptions = {
        method: "GET",
        uri: this.address + "/workflow-definitions/" + params.name + "",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }


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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolve(body);
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
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
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._updateWorkflowDefinition, arguments), callback);
  }

  _updateWorkflowDefinition(params, options, cb) {
    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "updateWorkflowDefinition";
      headers[versionHeader] = version;
      if (!params.name) {
        reject(new Error("name must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      const requestOptions = {
        method: "PUT",
        uri: this.address + "/workflow-definitions/" + params.name + "",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }

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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 201:
              resolve(body);
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
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
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._getWorkflowDefinitionByNameAndVersion, arguments), callback);
  }

  _getWorkflowDefinitionByNameAndVersion(params, options, cb) {
    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "getWorkflowDefinitionByNameAndVersion";
      headers[versionHeader] = version;
      if (!params.name) {
        reject(new Error("name must be non-empty because it's a path parameter"));
        return;
      }
      if (!params.version) {
        reject(new Error("version must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      const requestOptions = {
        method: "GET",
        uri: this.address + "/workflow-definitions/" + params.name + "/" + params.version + "",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }


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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolve(body);
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param {number} [params.limit=10] - Maximum number of workflows to return. Defaults to 10. Restricted to a max of 10,000.
   * @param {boolean} [params.oldestFirst]
   * @param {string} [params.pageToken]
   * @param {string} [params.status] - The status of the workflow (queued, running, etc.).
   * @param {boolean} [params.resolvedByUser] - A flag that indicates whether the workflow has been marked resolved by a user.
   * @param {boolean} [params.summaryOnly] - Limits workflow data to the bare minimum - omits the full workflow definition and job data.
   * @param {string} params.workflowDefinitionName
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
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
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._getWorkflows, arguments), callback);
  }

  _getWorkflows(params, options, cb) {
    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "getWorkflows";
      headers[versionHeader] = version;

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

      if (typeof params.resolvedByUser !== "undefined") {
        query["resolvedByUser"] = params.resolvedByUser;
      }

      if (typeof params.summaryOnly !== "undefined") {
        query["summaryOnly"] = params.summaryOnly;
      }

      query["workflowDefinitionName"] = params.workflowDefinitionName;


      const requestOptions = {
        method: "GET",
        uri: this.address + "/workflows",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }


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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolve(body);
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;
          }
        });
      }());
    });
  }


  /**
   * @param {Object} params
   * @param {number} [params.limit=10] - Maximum number of workflows to return. Defaults to 10. Restricted to a max of 10,000.
   * @param {boolean} [params.oldestFirst]
   * @param {string} [params.pageToken]
   * @param {string} [params.status] - The status of the workflow (queued, running, etc.).
   * @param {boolean} [params.resolvedByUser] - A flag that indicates whether the workflow has been marked resolved by a user.
   * @param {boolean} [params.summaryOnly] - Limits workflow data to the bare minimum - omits the full workflow definition and job data.
   * @param {string} params.workflowDefinitionName
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @returns {Object} iter
   * @returns {function} iter.map - takes in a function, applies it to each resource, and returns a promise to the result as an array
   * @returns {function} iter.toArray - returns a promise to the resources as an array
   * @returns {function} iter.forEach - takes in a function, applies it to each resource
   * @returns {function} iter.forEachAsync - takes in an async function, applies it to each resource
   */
  getWorkflowsIter(params, options) {
    const it = (f, saveResults, isAsync) => new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "getWorkflows";
      headers[versionHeader] = version;

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

      if (typeof params.resolvedByUser !== "undefined") {
        query["resolvedByUser"] = params.resolvedByUser;
      }

      if (typeof params.summaryOnly !== "undefined") {
        query["summaryOnly"] = params.summaryOnly;
      }

      query["workflowDefinitionName"] = params.workflowDefinitionName;


      const requestOptions = {
        method: "GET",
        uri: this.address + "/workflows",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }


      const retryPolicy = options.retryPolicy || this.retryPolicy || singleRetryPolicy;
      const backoffs = retryPolicy.backoffs();
      const logger = this.logger;

      let results = [];
      async.whilst(
        () => requestOptions.uri !== "",
        cbW => {
      const address = this.address;
      let retries = 0;
      (function requestOnce() {
        request(requestOptions, async (err, response, body) => {
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
                if (isAsync) {
                  for (let i = 0; i < body.length; i++) {
                    try {
                      await f(body[i], i, body);
                    } catch(err) {
                      reject(err);
                    }
                  }
                } else {
                  body.forEach(f)
                }
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
            reject(err);
            return;
          }
          if (saveResults) {
            resolve(results);
          } else {
            resolve();
          }
        }
      );
    });

    return {
      map: (f, cb) => applyCallback(this._hystrixCommand.execute(it, [f, true, false]), cb),
      toArray: cb => applyCallback(this._hystrixCommand.execute(it, [x => x, true, false]), cb),
      forEach: (f, cb) => applyCallback(this._hystrixCommand.execute(it, [f, false, false]), cb),
      forEachAsync: (f, cb) => applyCallback(this._hystrixCommand.execute(it, [f, false, true]), cb),
    };
  }

  /**
   * @param StartWorkflowRequest - Parameters for starting a workflow (workflow definition, input, and optionally namespace, queue, and tags)
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
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
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._startWorkflow, arguments), callback);
  }

  _startWorkflow(StartWorkflowRequest, options, cb) {
    const params = {};
    params["StartWorkflowRequest"] = StartWorkflowRequest;

    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "startWorkflow";
      headers[versionHeader] = version;

      const query = {};

      const requestOptions = {
        method: "POST",
        uri: this.address + "/workflows",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }

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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolve(body);
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param {string} params.workflowID
   * @param params.reason
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
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
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._CancelWorkflow, arguments), callback);
  }

  _CancelWorkflow(params, options, cb) {
    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "CancelWorkflow";
      headers[versionHeader] = version;
      if (!params.workflowID) {
        reject(new Error("workflowID must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      const requestOptions = {
        method: "DELETE",
        uri: this.address + "/workflows/" + params.workflowID + "",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }

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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolve();
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {string} workflowID
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.NotFound}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  getWorkflowByID(workflowID, options, cb) {
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._getWorkflowByID, arguments), callback);
  }

  _getWorkflowByID(workflowID, options, cb) {
    const params = {};
    params["workflowID"] = workflowID;

    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "getWorkflowByID";
      headers[versionHeader] = version;
      if (!params.workflowID) {
        reject(new Error("workflowID must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      const requestOptions = {
        method: "GET",
        uri: this.address + "/workflows/" + params.workflowID + "",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }


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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolve(body);
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {Object} params
   * @param {string} params.workflowID
   * @param params.overrides
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {Object}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.NotFound}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  resumeWorkflowByID(params, options, cb) {
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._resumeWorkflowByID, arguments), callback);
  }

  _resumeWorkflowByID(params, options, cb) {
    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "resumeWorkflowByID";
      headers[versionHeader] = version;
      if (!params.workflowID) {
        reject(new Error("workflowID must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      const requestOptions = {
        method: "POST",
        uri: this.address + "/workflows/" + params.workflowID + "",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }

      requestOptions.body = params.overrides;


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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 200:
              resolve(body);
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;
          }
        });
      }());
    });
  }

  /**
   * @param {string} workflowID
   * @param {object} [options]
   * @param {number} [options.timeout] - A request specific timeout
   * @param {module:workflow-manager.RetryPolicies} [options.retryPolicy] - A request specific retryPolicy
   * @param {function} [cb]
   * @returns {Promise}
   * @fulfill {undefined}
   * @reject {module:workflow-manager.Errors.BadRequest}
   * @reject {module:workflow-manager.Errors.NotFound}
   * @reject {module:workflow-manager.Errors.Conflict}
   * @reject {module:workflow-manager.Errors.InternalError}
   * @reject {Error}
   */
  resolveWorkflowByID(workflowID, options, cb) {
    let callback = cb;
    if (!cb && typeof options === "function") {
      callback = options;
    }
    return applyCallback(this._hystrixCommand.execute(this._resolveWorkflowByID, arguments), callback);
  }

  _resolveWorkflowByID(workflowID, options, cb) {
    const params = {};
    params["workflowID"] = workflowID;

    if (!cb && typeof options === "function") {
      options = undefined;
    }

    return new Promise((resolve, reject) => {
      if (!options) {
        options = {};
      }

      const timeout = options.timeout || this.timeout;

      const headers = {};
      headers["Canonical-Resource"] = "resolveWorkflowByID";
      headers[versionHeader] = version;
      if (!params.workflowID) {
        reject(new Error("workflowID must be non-empty because it's a path parameter"));
        return;
      }

      const query = {};

      const requestOptions = {
        method: "POST",
        uri: this.address + "/workflows/" + params.workflowID + "/resolved",
        gzip: true,
        json: true,
        timeout,
        headers,
        qs: query,
        useQuerystring: true,
      };
      if (this.keepalive) {
        requestOptions.forever = true;
      }


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
            reject(err);
            return;
          }

          switch (response.statusCode) {
            case 201:
              resolve();
              break;

            case 400:
              var err = new Errors.BadRequest(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 404:
              var err = new Errors.NotFound(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 409:
              var err = new Errors.Conflict(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            case 500:
              var err = new Errors.InternalError(body || {});
              responseLog(logger, requestOptions, response, err);
              reject(err);
              return;

            default:
              var err = new Error("Received unexpected statusCode " + response.statusCode);
              responseLog(logger, requestOptions, response, err);
              reject(err);
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

const version = "0.15.0";
const versionHeader = "X-Client-Version";
module.exports.Version = version;
module.exports.VersionHeader = versionHeader;
