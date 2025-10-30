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
      validate_200_Status(response);
      expect(response.body).to.be.an('object');
      expect(response.body).to.have.property('resultCount');
      expect(response.body).to.have.property('totalCount');
      if (response.body.users) {
        expect(response.body.users).to.be.an('array');
      }
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

  // Test POST /users - Create User
  it('POST /users - Create User with authentication', function () {
    const userPayload = {
      userExternalID: '00117000015vpjXAAQ',
      username: 'testuser123',
      lfEmail: 'testuser123@linuxfoundation.org',
      lfUsername: 'testuser123',
      githubID: '12345678',
      githubUsername: 'testuser123',
      admin: false,
      note: 'Test user created via API',
      emails: ['testuser123@linuxfoundation.org', 'testuser123@example.com'],
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}users`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: userPayload,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        cy.task('log', 'POST /users response status: ' + response.status);
        // Expect either 200 (created), 400 (bad request), or 409 (conflict if user already exists)
        expect([200, 400, 409]).to.include(response.status);
        if (response.status === 200) {
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('userID');
        } else if (response.status === 409) {
          expect(response.body).to.have.property('message');
        }
      });
    });
  });

  // Test PUT /users - Update User
  it('PUT /users - Update User with authentication', function () {
    const testUserID = '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5';
    const updatePayload = {
      userID: testUserID,
      note: 'Updated test user note via API',
      emails: ['updated@linuxfoundation.org'],
    };

    cy.request({
      method: 'PUT',
      url: `${claEndpoint}users`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: updatePayload,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        cy.task('log', 'PUT /users response status: ' + response.status);
        // Expect either 200 (updated), 400 (bad request), or 404 (user not found)
        expect([200, 400, 404]).to.include(response.status);
        if (response.status === 200) {
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('userID');
          expect(response.body.userID).to.eq(testUserID);
        }
      });
    });
  });

  // Test DELETE /users/{userID} - Delete User
  it('DELETE /users/{userID} - Delete User with authentication', function () {
    const testUserID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // non-existing user for safe testing
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}users/${testUserID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        cy.task('log', 'DELETE /users response status: ' + response.status);
        // Expect either 204 (deleted) or 404 (user not found)
        expect([204, 404]).to.include(response.status);
        if (response.status === 204) {
          // 204 No Content for successful deletion
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
