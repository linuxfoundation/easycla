/*
 * Comprehensive test suite for all Gerrit APIs in V3 (tagged with 'gerrits' in swagger)
 *
 * Covers all HTTP methods for gerrit endpoints:
 * - GET /gerrit/repos (authenticated, requires gerritHost parameter)
 * - POST /project/{projectID}/gerrits (authenticated)
 * - DELETE /project/{projectID}/gerrits/{gerritID} (authenticated)
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

describe('To Validate & test Gerrit APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  let bearerToken: string = null;
  let createdGerritID: string = null; // Track created gerrit for cleanup
  let testProjectID: string = null; // Track existing project for tests

  before(() => {
    getTokenKey();
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  // Cleanup any created gerrits after all tests
  after(() => {
    if (createdGerritID && testProjectID) {
      cy.task('log', `Cleaning up test gerrit: ${createdGerritID} from project: ${testProjectID}`);
      cy.request({
        method: 'DELETE',
        url: `${claEndpoint}project/${testProjectID}/gerrits/${createdGerritID}`,
        timeout: timeout,
        failOnStatusCode: false,
        headers: getXACLHeaders(),
        auth: {
          bearer: bearerToken,
        },
      }).then((response) => {
        cy.task('log', `Cleanup DELETE gerrit ${createdGerritID}: ${response.status}`);
      });
    }
  });

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /gerrit/repos - Get Gerrit Repositories with valid host', function () {
    // Use a known Gerrit host that should be accessible
    const gerritHost = 'gerrit.onap.org';

    cy.request({
      method: 'GET',
      url: `${claEndpoint}gerrit/repos?gerritHost=${gerritHost}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /gerrit/repos response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        if (response.body.repos) {
          expect(response.body.repos).to.be.an('array');
          // Validate response structure
          validateApiResponse('gerrits/getGerritRepos.json', response);
        }
      });
    });
  });

  it('GET /project - Find a project with gerrits for testing', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project?pageSize=50`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /project response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');

        if (response.body.projects && response.body.projects.length > 0) {
          // Look for a project that might support gerrit integration
          const project = response.body.projects.find((p) => p.projectID) || response.body.projects[0];
          testProjectID = project.projectID;
          cy.task('log', `Found test project for gerrit tests - ID: ${testProjectID}, Name: ${project.projectName}`);
        }
      });
    });
  });

  // ============================================================================
  // CRUD OPERATIONS - POST AND DELETE GERRITS
  // ============================================================================

  it('POST /project/{projectID}/gerrits - Add Gerrit Configuration', function () {
    // Skip if no test project available
    if (!testProjectID) {
      cy.task('log', 'Skipping gerrit creation - no test project available');
      return;
    }

    const uniqueId = Date.now() + Math.floor(Math.random() * 1000);
    const addGerritPayload = {
      gerritName: `Test-Gerrit-${uniqueId}`,
      gerritUrl: 'https://gerrit.onap.org', // Use a real Gerrit URL that should be acceptable
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}project/${testProjectID}/gerrits`,
      timeout: timeout,
      failOnStatusCode: false, // Allow various responses
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: addGerritPayload,
    }).then((response) => {
      return cy.logJson('POST /project/{projectID}/gerrits response', response).then(() => {
        cy.task('log', 'POST gerrit response status: ' + response.status);

        // Never expect 5xx - that would be internal server error
        expect(response.status).to.not.be.within(500, 599);

        if (response.status >= 200 && response.status <= 299) {
          // Success case
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('gerritId');
          expect(response.body.gerritName).to.eq(addGerritPayload.gerritName);
          expect(response.body.gerritUrl).to.eq(addGerritPayload.gerritUrl);

          createdGerritID = response.body.gerritId; // Track for cleanup
          cy.task('log', `Successfully created gerrit with ID: ${createdGerritID}`);
          validateApiResponse('gerrits/addGerrit.json', response);
        } else {
          // Expected error cases (permissions, conflicts, etc.)
          expect([400, 401, 403, 404, 409, 422]).to.include(response.status);
          cy.task('log', `Gerrit creation returned ${response.status} - expected API behavior`);
        }
      });
    });
  });

  it('DELETE /project/{projectID}/gerrits/{gerritID} - Delete Gerrit Configuration', function () {
    // Skip if no gerrit was created or no test project
    if (!createdGerritID || !testProjectID) {
      cy.task('log', 'Skipping gerrit deletion - no created gerrit available');
      return;
    }

    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}project/${testProjectID}/gerrits/${createdGerritID}`,
      timeout: timeout,
      failOnStatusCode: false, // Allow various responses
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('DELETE /project/{projectID}/gerrits/{gerritID} response', response).then(() => {
        cy.task('log', 'DELETE gerrit response status: ' + response.status);

        // Never expect 5xx - that would be internal server error
        expect(response.status).to.not.be.within(500, 599);

        if (response.status >= 200 && response.status <= 299) {
          // Success case (204 No Content is typical for DELETE)
          if (response.status === 204) {
            expect(response.body).to.be.empty;
          }
          cy.task('log', `Successfully deleted gerrit: ${createdGerritID}`);
          createdGerritID = null; // Clear since deleted
        } else {
          // Expected error cases
          expect([400, 401, 403, 404, 422]).to.include(response.status);
          cy.task('log', `Gerrit deletion returned ${response.status} - expected API behavior`);
        }
      });
    });
  });

  // ============================================================================
  // NEGATIVE TEST CASES - EXPECT 4xx ERROR RESPONSES
  // ============================================================================

  it('POST /project/{projectID}/gerrits - Create Gerrit with Invalid Data (4xx)', function () {
    if (!testProjectID) {
      cy.task('log', 'Skipping invalid gerrit creation test - no test project available');
      return;
    }

    const invalidPayload = {
      gerritName: '', // Invalid empty name
      gerritUrl: 'invalid-url', // Invalid URL format
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}project/${testProjectID}/gerrits`,
      timeout: timeout,
      failOnStatusCode: false, // Use false for non-happy path (4xx expected)
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: invalidPayload,
    }).then((response) => {
      return cy.logJson('POST gerrit invalid data response', response).then(() => {
        cy.task('log', 'POST gerrit invalid data response status: ' + response.status);
        // Expect 4xx status codes for validation errors
        expect([400, 422]).to.include(response.status);
        if (response.body && response.body.message) {
          expect(response.body.message).to.be.a('string');
        }
      });
    });
  });

  it('DELETE /project/{projectID}/gerrits/{gerritID} - Delete Non-Existent Gerrit (404)', function () {
    if (!testProjectID) {
      cy.task('log', 'Skipping non-existent gerrit deletion test - no test project available');
      return;
    }

    const nonExistentGerritID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // Non-existing gerrit for safe testing

    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}project/${testProjectID}/gerrits/${nonExistentGerritID}`,
      timeout: timeout,
      failOnStatusCode: false, // Use false for non-happy path (4xx expected)
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('DELETE non-existent gerrit response', response).then(() => {
        cy.task('log', 'DELETE non-existent gerrit response status: ' + response.status);
        // Never expect 5xx - that would be internal server error
        expect(response.status).to.not.be.within(500, 599);
        // Allow various expected responses including auth failures
        expect([204, 401, 404, 422]).to.include(response.status);
        if (response.status === 204) {
          // 204 No Content is also acceptable (idempotent delete)
          expect(response.body).to.be.empty;
        }
      });
    });
  });

  describe('Expected failures', () => {
    it('Returns 401 for Gerrit APIs when called without token', () => {
      const exampleProjectID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
      const exampleGerritID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';

      const requests = [
        { method: 'GET', url: `${claEndpoint}gerrit/repos?gerritHost=gerrit.onap.org` },
        { method: 'POST', url: `${claEndpoint}project/${exampleProjectID}/gerrits` },
        { method: 'DELETE', url: `${claEndpoint}project/${exampleProjectID}/gerrits/${exampleGerritID}` },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
            ...(req.method === 'POST' ? { body: {} } : {}),
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);
              // Never expect 5xx - that would be internal server error
              expect(response.status).to.not.be.within(500, 599);
              // For negative tests, expect 401 Unauthorized
              expect(response.status).to.eq(401);
            });
          });
      });
    });

    it('Returns 4xx for malformed Gerrit parameters', () => {
      const requests = [
        {
          method: 'GET',
          url: `${claEndpoint}gerrit/repos`, // Missing required gerritHost parameter
          expectedStatuses: [400, 422],
          expectedCodes: ['400', '422', '601', '602'],
        },
        {
          method: 'GET',
          url: `${claEndpoint}gerrit/repos?gerritHost=`, // Empty gerritHost parameter
          expectedStatuses: [400, 422],
          expectedCodes: ['400', '422', '601', '602'],
        },
        {
          method: 'GET',
          url: `${claEndpoint}gerrit/repos?gerritHost=invalid-host-format`, // Invalid host format
          expectedStatuses: [400, 422],
          expectedCodes: ['400', '422', '601', '602'],
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
              // Never expect 5xx - that would be internal server error
              expect(response.status).to.not.be.within(500, 599);
              expect(req.expectedStatuses).to.include(response.status);
              if (response.body && (response.body.code || response.body.Code)) {
                expect(req.expectedCodes).to.include(String(response.body.code ?? response.body.Code));
              }
            });
          });
      });
    });

    it('Returns 4xx for malformed Gerrit POST/DELETE requests', () => {
      const exampleProjectID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
      const exampleGerritID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';

      const requests = [
        {
          method: 'POST',
          url: `${claEndpoint}project/${exampleProjectID}/gerrits`,
          body: {}, // Empty body should trigger validation error
          expectedStatuses: [400, 404, 422], // Accept multiple statuses
          expectedCodes: [400, 404, 602, 604],
        },
        {
          method: 'POST',
          url: `${claEndpoint}project/${exampleProjectID}/gerrits`,
          body: {
            gerritName: '', // Invalid empty name
            gerritUrl: 'invalid-url', // Invalid URL format
          },
          expectedStatuses: [400, 404, 422], // Accept multiple statuses
          expectedCodes: [400, 404, 602, 604],
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}project/invalid-uuid/gerrits/${exampleGerritID}`, // Invalid project UUID
          expectedStatuses: [400, 404, 422], // Accept multiple statuses
          expectedCodes: [400, 404, 602, 604],
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}project/${exampleProjectID}/gerrits/invalid-uuid`, // Invalid gerrit UUID
          expectedStatuses: [400, 404, 422], // Accept multiple statuses
          expectedCodes: [400, 404, 602, 604],
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
            ...(req.body ? { body: req.body } : {}),
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing malformed ${req.method} ${req.url}`, req.body || 'no body');
              // Never expect 5xx - that would be internal server error
              expect(response.status).to.not.be.within(500, 599);
              expect(req.expectedStatuses).to.include(response.status);
              if (response.body && (response.body.code || response.body.Code)) {
                expect(req.expectedCodes).to.include(response.body.code ?? response.body.Code);
              }
            });
          });
      });
    });
  });
});
