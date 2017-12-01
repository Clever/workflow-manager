module.exports.Errors = {};

/**
 * BadRequest
 * @extends Error
 * @memberof module:workflow-manager
 * @alias module:workflow-manager.Errors.BadRequest
 * @property {string} message
 */
module.exports.Errors.BadRequest = class extends Error {
  constructor(body) {
    super(body.message);
    for (const k of Object.keys(body)) {
      this[k] = body[k];
    }
  }
};

/**
 * InternalError
 * @extends Error
 * @memberof module:workflow-manager
 * @alias module:workflow-manager.Errors.InternalError
 * @property {string} message
 */
module.exports.Errors.InternalError = class extends Error {
  constructor(body) {
    super(body.message);
    for (const k of Object.keys(body)) {
      this[k] = body[k];
    }
  }
};

/**
 * NotFound
 * @extends Error
 * @memberof module:workflow-manager
 * @alias module:workflow-manager.Errors.NotFound
 * @property {string} message
 */
module.exports.Errors.NotFound = class extends Error {
  constructor(body) {
    super(body.message);
    for (const k of Object.keys(body)) {
      this[k] = body[k];
    }
  }
};

/**
 * Conflict
 * @extends Error
 * @memberof module:workflow-manager
 * @alias module:workflow-manager.Errors.Conflict
 * @property {string} message
 */
module.exports.Errors.Conflict = class extends Error {
  constructor(body) {
    super(body.message);
    for (const k of Object.keys(body)) {
      this[k] = body[k];
    }
  }
};

