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
              rejecter(new Error("Recieved unexpected statusCode " + response.statusCode));
              return;
          }
        });
      }());
    });
  }

  /**
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
  newWorkflow(options, cb) {
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
              rejecter(new Error("Recieved unexpected statusCode " + response.statusCode));
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
