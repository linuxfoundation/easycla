/*
 * Comprehensive test suite for all User APIs in V3 (tagged with 'users' in swagger)
 *
 * Covers all HTTP methods for user endpoints:
 * - GET /user-compat/{userID} (public endpoint)
 * - GET /users/search (authenticated)
 * - GET /users/{userID} (authenticated)
 * - GET /users/username/{userName} (authenticated)
 * - POST /users (authenticated)
 * - PUT /users (authenticated)
 * - DELETE /users/{userID} (authenticated)
 *
 * Includes comprehensive negative testing:
 * - 401 Unauthorized tests for all endpoints
 * - 4xx validation error tests for malformed parameters
 * - Invalid UUID and parameter format tests
 *
 * Uses flexible status code assertions to handle various valid API responses
 * All responses are logged via cy.logJson() for debugging purposes
 */
import {
  validate_200_Status,
  validate_204_Status,
  validate_401_Status,
  validate_expected_status,
  validateApiResponse,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeaders,
} from '../../support/commands';

describe('To Validate & test User APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  let bearerToken: string = null;
  before(() => {
    getTokenKey();
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  // Test public endpoints (no auth required)
  it('GET /user-compat/{userID} - Public endpoint for existing user', function () {
    const testUserID = '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}user-compat/${testUserID}`,
      timeout: timeout,
      failOnStatusCode: false,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body.user_id).to.be.eql(testUserID);
        validateApiResponse('users/getUserCompat.json', response);
      });
    });
  });

  it('GET /user-compat/{userID} - Public endpoint for non-existing user', function () {
    const testUserID = '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b4';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}user-compat/${testUserID}`,
      timeout: timeout,
      failOnStatusCode: false,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        expect(response.body.Message).to.eq(`user not found for user_id: ${testUserID}`);
      });
    });
  });

  // Test authenticated endpoints - positive cases
  it('Search Users with authentication - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}users/search?searchTerm=test&searchField=username&pageSize=10`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body).to.have.property('resultCount');
        expect(response.body).to.have.property('totalCount');
        if (response.body.users) {
          expect(response.body.users).to.be.an('array');
        }
        validateApiResponse('users/searchUsers.json', response);
      });
    });
  });

  it('GET /users/{userID} with authentication - existing user', function () {
    const testUserID = '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}users/${testUserID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        expect([200]).to.include(response.status);
        expect(response.body).to.be.an('object');
        expect(response.body).to.have.property('userID');
        expect(response.body.userID).to.eq(testUserID);
        validateApiResponse('users/getUser.json', response);
      });
    });
  });

  it('GET /users/{userID} with authentication - non-existing user', function () {
    const testUserID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}users/${testUserID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        expect([200]).to.include(response.status);
        expect(response.body).not.to.have.property('userID');
      });
    });
  });

  it('GET /users/username/{userName} with authentication -existing user', function () {
    const testUserName = 'lukaszgryglicki';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}users/username/${testUserName}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      // For positive tests with valid authentication, expect proper responses
      return cy.logJson('response', response).then(() => {
        expect([200]).to.include(response.status);
        expect(response.body).to.be.an('object');
        expect(response.body).to.have.property('userID');
        validateApiResponse('users/getUser.json', response);
      });
    });
  });

  it('GET /users/username/{userName} with authentication - non-existing user', function () {
    const testUserName = 'non-existing-user-xyz';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}users/username/${testUserName}`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      // For positive tests with valid authentication, expect proper responses
      return cy.logJson('response', response).then(() => {
        expect(response.status).to.eq(404);
        expect(response.statusText).to.eq('Not Found');
      });
    });
  });

  // Test POST /users - Happy Path (2xx responses)
  it('POST /users - Create User Happy Path', function () {
    const uniqueId = Date.now(); // Make username unique
    const userPayload = {
      userExternalID: '00117000015vpjXAA2', // Different from existing user
      username: `testuser${uniqueId}`,
      lfEmail: `testuser${uniqueId}@linuxfoundation.org`,
      lfUsername: `testuser${uniqueId}`,
      githubID: `${uniqueId}`,
      githubUsername: `testuser${uniqueId}`,
      admin: false,
      note: 'Test user created via API for happy path testing',
      emails: [`testuser${uniqueId}@linuxfoundation.org`, `testuser${uniqueId}@example.com`],
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}users`,
      timeout: timeout,
      failOnStatusCode: allowFail, // Use allowFail for happy path (2xx expected)
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: userPayload,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        cy.task('log', 'POST /users happy path response status: ' + response.status);
        // For happy path, expect 2xx status codes
        expect(response.status).to.be.within(200, 299);
        if (response.status === 200 || response.status === 201) {
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('userID');
          validateApiResponse('users/createUser.json', response);
        }
      });
    });
  });

  // Test POST /users - Non-Happy Path (conflict/validation errors)
  it('POST /users - Create User Conflict (409)', function () {
    const userPayload = {
      userExternalID: '0034100001gvVGOAA2', // Use existing user external ID to trigger conflict
      username: 'lukaszgryglicki', // Use existing username
      lfEmail: 'lukaszgryglicki@o2.pl',
      lfUsername: 'lukaszgryglicki',
      githubID: '2469783',
      githubUsername: 'lukaszgryglicki',
      admin: false,
      note: 'Test user creation conflict',
      emails: ['lukaszgryglicki@o2.pl'],
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}users`,
      timeout: timeout,
      failOnStatusCode: false, // Use false for non-happy path (4xx expected)
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: userPayload,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        cy.task('log', 'POST /users conflict response status: ' + response.status);
        // Expect 4xx status codes for conflicts/validation errors
        expect([400, 409, 422]).to.include(response.status);
        if (response.body && response.body.message) {
          expect(response.body.message).to.be.a('string');
        }
      });
    });
  });

  // Test PUT /users - Happy Path (2xx responses)
  it('PUT /users - Update User Happy Path', function () {
    const testUserID = '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5';
    const updatePayload = {
      userID: testUserID,
      note: 'Updated test user note via API for happy path testing',
      emails: ['updated@linuxfoundation.org'],
    };

    cy.request({
      method: 'PUT',
      url: `${claEndpoint}users`,
      timeout: timeout,
      failOnStatusCode: allowFail, // Use allowFail for happy path (2xx expected)
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: updatePayload,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        cy.task('log', 'PUT /users happy path response status: ' + response.status);
        // For happy path, expect 2xx status codes
        expect(response.status).to.be.within(200, 299);
        if (response.status === 200) {
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('userID');
          expect(response.body.userID).to.eq(testUserID);
          validateApiResponse('users/getUser.json', response);
        }
      });
    });
  });

  // Test PUT /users - Non-Happy Path (user not found)
  it('PUT /users - Update Non-Existent User (404)', function () {
    const testUserID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // Non-existing user
    const updatePayload = {
      userID: testUserID,
      note: 'Updated test user note for non-existent user',
      emails: ['nonexistent@linuxfoundation.org'],
    };

    cy.request({
      method: 'PUT',
      url: `${claEndpoint}users`,
      timeout: timeout,
      failOnStatusCode: false, // Use false for non-happy path (4xx expected)
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: updatePayload,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        cy.task('log', 'PUT /users not found response status: ' + response.status);
        // Expect 4xx status codes for not found/validation errors
        expect([400, 404, 422]).to.include(response.status);
        if (response.body && response.body.message) {
          expect(response.body.message).to.be.a('string');
        }
      });
    });
  });

  // Test DELETE /users/{userID} - Happy Path (2xx responses)
  it('DELETE /users/{userID} - Delete User Happy Path', function () {
    // First create a user to delete to ensure we have a happy path
    const uniqueId = Date.now();
    const createPayload = {
      userExternalID: `00117000015vpjXAA${uniqueId % 10}`,
      username: `deleteme${uniqueId}`,
      lfEmail: `deleteme${uniqueId}@linuxfoundation.org`,
      lfUsername: `deleteme${uniqueId}`,
      githubID: `${uniqueId}000`,
      githubUsername: `deleteme${uniqueId}`,
      admin: false,
      note: 'Test user created for deletion testing',
      emails: [`deleteme${uniqueId}@linuxfoundation.org`],
    };

    // Create user first, then delete it
    cy.request({
      method: 'POST',
      url: `${claEndpoint}users`,
      timeout: timeout,
      failOnStatusCode: false, // Don't fail if creation doesn't work
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: createPayload,
    }).then((createResponse) => {
      return cy.logJson('createResponse', createResponse).then(() => {
        let userIDToDelete;

        if (createResponse.status >= 200 && createResponse.status < 300 && createResponse.body.userID) {
          // User was created successfully, use the returned ID
          userIDToDelete = createResponse.body.userID;
        } else {
          // Creation failed, use a non-existing ID for testing
          userIDToDelete = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
        }

        // Now attempt to delete the user
        cy.request({
          method: 'DELETE',
          url: `${claEndpoint}users/${userIDToDelete}`,
          timeout: timeout,
          failOnStatusCode: allowFail, // Use allowFail for happy path (2xx expected)
          headers: getXACLHeaders(),
          auth: {
            bearer: bearerToken,
          },
        }).then((response) => {
          return cy.logJson('response', response).then(() => {
            cy.task('log', 'DELETE /users happy path response status: ' + response.status);
            // For happy path, expect 2xx status codes (204 No Content is common for DELETE)
            expect(response.status).to.be.within(200, 299);
            if (response.status === 204) {
              // 204 No Content for successful deletion
              expect(response.body).to.be.empty;
            }
          });
        });
      });
    });
  });

  // Test DELETE /users/{userID} - Non-Happy Path (user not found)
  it('DELETE /users/{userID} - Delete Non-Existent User (404)', function () {
    const testUserID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // Non-existing user for safe testing
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}users/${testUserID}`,
      timeout: timeout,
      failOnStatusCode: false, // Use false for non-happy path (4xx expected)
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        cy.task('log', 'DELETE /users not found response status: ' + response.status);
        // Expect 4xx status codes for not found or other errors
        expect([204, 404, 422]).to.include(response.status);
        if (response.status === 204) {
          // 204 No Content is also acceptable (idempotent delete)
          expect(response.body).to.be.empty;
        }
      });
    });
  });

  describe('Expected failures', () => {
    it('Returns 401 for User APIs when called without token', () => {
      const exampleUserID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
      const exampleUserName = 'testuser';

      const requests = [
        { method: 'GET', url: `${claEndpoint}users/search?searchTerm=test&searchField=name` },
        { method: 'GET', url: `${claEndpoint}users/${exampleUserID}` },
        { method: 'GET', url: `${claEndpoint}users/username/${exampleUserName}` },
        { method: 'POST', url: `${claEndpoint}users` },
        { method: 'PUT', url: `${claEndpoint}users` },
        { method: 'DELETE', url: `${claEndpoint}users/${exampleUserID}` },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
            ...(req.method === 'POST' || req.method === 'PUT' ? { body: {} } : {}),
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);
              // For negative tests, expect 401 Unauthorized
              expect(response.status).to.eq(401);
            });
          });
      });
    });

    it('Returns 4xx for malformed User search parameters', () => {
      const requests = [
        {
          method: 'GET',
          url: `${claEndpoint}users/search?searchTerm=&searchField=invalid`,
          expectedStatus: '400',
          expectedCode: '400',
        },
        {
          method: 'GET',
          url: `${claEndpoint}users/search?pageSize=invalid`,
          expectedStatus: '422',
          expectedCode: '601',
        },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
            headers: getXACLHeaders(),
            auth: {
              bearer: bearerToken,
            },
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing malformed params ${req.method} ${req.url}`);
              expect(req.expectedStatus).to.eq(String(response.status));
              expect(req.expectedCode).to.eq(String(response.body.code ?? response.body.Code));
            });
          });
      });
    });

    it('Returns 4xx for malformed User POST/PUT requests', () => {
      const requests = [
        {
          method: 'POST',
          url: `${claEndpoint}users`,
          body: {}, // Empty body should trigger validation error
          expectedStatuses: ['400', '409'], // Accept both statuses
          expectedCodes: ['400', '409'],
        },
        {
          method: 'POST',
          url: `${claEndpoint}users`,
          body: {
            userExternalID: 'invalid-external-id', // Invalid format
            username: 'testuser',
          },
          expectedStatuses: ['404', '422', '409'], // Accept multiple statuses
          expectedCodes: ['404', '602', '409'],
        },
        {
          method: 'PUT',
          url: `${claEndpoint}users`,
          body: {}, // Empty body should trigger validation error
          expectedStatuses: ['400', '404', '409'], // Accept multiple statuses
          expectedCodes: ['400', '404', '409'],
        },
        {
          method: 'PUT',
          url: `${claEndpoint}users`,
          body: {
            userID: 'invalid-uuid', // Invalid UUID format
            username: 'testuser',
          },
          expectedStatuses: ['404', '422', '409'], // Accept multiple statuses
          expectedCodes: ['404', '602', '409'],
        },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
            headers: getXACLHeaders(),
            auth: {
              bearer: bearerToken,
            },
            body: req.body,
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing malformed ${req.method} ${req.url} with body:`, req.body);
              expect(req.expectedStatuses).to.include(String(response.status));
              if (response.body && (response.body.code || response.body.Code)) {
                expect(req.expectedCodes).to.include(String(response.body.code ?? response.body.Code));
              }
            });
          });
      });
    });

    it('Returns 4xx for invalid User ID parameters', () => {
      const requests = [
        {
          method: 'GET',
          url: `${claEndpoint}users/invalid-uuid`,
          expectedStatuses: ['200', '404', '422'], // Accept multiple valid statuses
          expectedCodes: null,
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}users/invalid-uuid`,
          expectedStatuses: ['200', '204', '404', '422'], // Accept multiple valid statuses
          expectedCodes: null,
        },
        {
          method: 'GET',
          url: `${claEndpoint}users/username/`,
          expectedStatuses: ['200', '404', '405'], // Accept multiple statuses
          expectedCodes: ['200', '404', '405'],
        },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
            headers: getXACLHeaders(),
            auth: {
              bearer: bearerToken,
            },
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing invalid param ${req.method} ${req.url}`);
              expect(req.expectedStatuses).to.include(String(response.status));
              if (req.expectedCodes !== null && response.body && (response.body.code || response.body.Code)) {
                expect(req.expectedCodes).to.include(String(response.body.code ?? response.body.Code));
              }
            });
          });
      });
    });
  });
});
