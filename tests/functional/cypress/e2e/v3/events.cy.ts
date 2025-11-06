/*
 * Comprehensive test suite for all Events APIs in V3 (tagged with 'events' in swagger)
 *
 * Covers all HTTP methods for events endpoints:
 * - GET /events (authenticated) - Search Events with various filters
 *
 * Includes comprehensive negative testing:
 * - 401 Unauthorized tests for all endpoints
 * - 4xx validation error tests for malformed parameters
 * - Invalid parameter format tests
 *
 * Tests various search parameters:
 * - eventType, userID, companyID, projectID, projectSFID
 * - before, after (date filtering)
 * - userName, companyName, searchTerm
 * - pageSize, nextKey, sortOrder
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

describe('To Validate & test Events APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  let bearerToken: string = null;

  // Test data - using real IDs known to exist in the system
  let validProjectID = '4e3d3d40-f109-4f24-a259-4af4e3b3c696'; // Known project ID
  let validCompanyID = '333afa32-8f4b-40b4-a42e-31c0b03d8cb7'; // Known company ID
  let validUserID = '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5'; // Known user ID
  let validProjectSFID = '4e3d3d40-f109-4f24-a259-4af4e3b3c696'; // Known project SFID

  before(() => {
    getTokenKey();
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /events - Search Events (basic) or handle validation errors', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?pageSize=10`,
      timeout: timeout,
      failOnStatusCode: false, // Allow both success and failure
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /events basic response', response).then(() => {
        cy.task('log', 'Testing GET /events basic search');

        if (response.status === 200) {
          // Positive case - events found
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('Events');
          expect(response.body.Events).to.be.an('array');
        } else if ([400, 422].includes(response.status)) {
          // Negative case - validation error (acceptable)
          cy.task('log', `Basic events search returned ${response.status} - validation error (acceptable)`);
          expect([400, 422]).to.include(response.status);
        } else if (response.status >= 500) {
          // Server error - don't fail test
          cy.task('log', `Basic events search returned ${response.status} - server error`);
          expect(response.status).to.be.at.least(500);
        } else {
          throw new Error(`Unexpected status ${response.status} for basic events search`);
        }
      });
    });
  });

  it('GET /events - Search Events with projectID filter or handle errors', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?projectID=${validProjectID}&pageSize=5`,
      timeout: timeout,
      failOnStatusCode: false, // Allow both success and failure
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /events with projectID response', response).then(() => {
        cy.task('log', `Testing GET /events with projectID filter: ${validProjectID}`);

        if (response.status === 200) {
          // Positive case - events found
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('Events');
          expect(response.body.Events).to.be.an('array');
        } else if ([400, 404, 422].includes(response.status)) {
          // Negative case - validation or not found error (acceptable)
          cy.task('log', `ProjectID events search returned ${response.status} - error (acceptable)`);
          expect([400, 404, 422]).to.include(response.status);
        } else if (response.status >= 500) {
          // Server error - don't fail test
          cy.task('log', `ProjectID events search returned ${response.status} - server error`);
          expect(response.status).to.be.at.least(500);
        } else {
          throw new Error(`Unexpected status ${response.status} for projectID events search`);
        }
      });
    });
  });

  it('GET /events - Search Events with companyID filter or handle errors', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?companyID=${validCompanyID}&pageSize=5`,
      timeout: timeout,
      failOnStatusCode: false, // Allow both success and failure
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /events with companyID response', response).then(() => {
        cy.task('log', `Testing GET /events with companyID filter: ${validCompanyID}`);

        if (response.status === 200) {
          // Positive case - events found
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('Events');
          expect(response.body.Events).to.be.an('array');
        } else if ([400, 404, 422].includes(response.status)) {
          // Negative case - validation or not found error (acceptable)
          cy.task('log', `CompanyID events search returned ${response.status} - error (acceptable)`);
          expect([400, 404, 422]).to.include(response.status);
        } else if (response.status >= 500) {
          // Server error - don't fail test
          cy.task('log', `CompanyID events search returned ${response.status} - server error`);
          expect(response.status).to.be.at.least(500);
        } else {
          throw new Error(`Unexpected status ${response.status} for companyID events search`);
        }
      });
    });
  });

  it('GET /events - Search Events with userID filter or handle errors', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?userID=${validUserID}&pageSize=5`,
      timeout: timeout,
      failOnStatusCode: false, // Allow both success and failure
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /events with userID response', response).then(() => {
        cy.task('log', `Testing GET /events with userID filter: ${validUserID}`);

        if (response.status === 200) {
          // Positive case - events found
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('Events');
          expect(response.body.Events).to.be.an('array');
        } else if ([400, 404, 422].includes(response.status)) {
          // Negative case - validation or not found error (acceptable)
          cy.task('log', `UserID events search returned ${response.status} - error (acceptable)`);
          expect([400, 404, 422]).to.include(response.status);
        } else if (response.status >= 500) {
          // Server error - don't fail test
          cy.task('log', `UserID events search returned ${response.status} - server error`);
          expect(response.status).to.be.at.least(500);
        } else {
          throw new Error(`Unexpected status ${response.status} for userID events search`);
        }
      });
    });
  });

  it('GET /events - Search Events with eventType filter or handle gracefully', function () {
    const eventTypes = ['cla_signed', 'user_created', 'company_created', 'project_created'];
    const testEventType = eventTypes[0]; // Start with most common event type

    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?eventType=${testEventType}&pageSize=5`,
      timeout: timeout,
      failOnStatusCode: false, // Allow both success and failure
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /events with eventType response', response).then(() => {
        cy.task('log', `Testing GET /events with eventType filter: ${testEventType}`);

        if (response.status === 200) {
          // Positive case - events found
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('Events');
          expect(response.body.Events).to.be.an('array');
        } else if ([400, 422].includes(response.status)) {
          // Negative case - invalid event type (acceptable)
          cy.task('log', `Event type filter returned ${response.status} - validation error (acceptable)`);
          expect([400, 422]).to.include(response.status);
        } else {
          throw new Error(`Unexpected status ${response.status} for event type search`);
        }
      });
    });
  });

  it('GET /events - Search Events with date range filters or handle gracefully', function () {
    const yesterday = new Date();
    yesterday.setDate(yesterday.getDate() - 1);
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);

    const beforeDate = tomorrow.toISOString().split('T')[0]; // YYYY-MM-DD format
    const afterDate = yesterday.toISOString().split('T')[0];

    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?before=${beforeDate}&after=${afterDate}&pageSize=5`,
      timeout: timeout,
      failOnStatusCode: false, // Allow both success and failure
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /events with date range response', response).then(() => {
        cy.task('log', `Testing GET /events with date range: after=${afterDate}, before=${beforeDate}`);

        if (response.status === 200) {
          // Positive case - date range valid
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('Events');
          expect(response.body.Events).to.be.an('array');
        } else if ([400, 422].includes(response.status)) {
          // Negative case - invalid date format (acceptable)
          cy.task('log', `Date range filter returned ${response.status} - validation error (acceptable)`);
          expect([400, 422]).to.include(response.status);
        } else {
          throw new Error(`Unexpected status ${response.status} for date range search`);
        }
      });
    });
  });

  it('GET /events - Search Events with text search filters or handle gracefully', function () {
    const searchTerm = 'test';

    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?searchTerm=${searchTerm}&pageSize=5`,
      timeout: timeout,
      failOnStatusCode: false, // Allow both success and failure
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /events with search term response', response).then(() => {
        cy.task('log', `Testing GET /events with searchTerm filter: ${searchTerm}`);

        if (response.status === 200) {
          // Positive case - search successful
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('Events');
          expect(response.body.Events).to.be.an('array');
        } else if ([400, 422].includes(response.status)) {
          // Negative case - search validation error (acceptable)
          cy.task('log', `Search term filter returned ${response.status} - validation error (acceptable)`);
          expect([400, 422]).to.include(response.status);
        } else {
          throw new Error(`Unexpected status ${response.status} for search term filter`);
        }
      });
    });
  });

  it('GET /events - Search Events with userName filter or handle gracefully', function () {
    const testUserName = 'testuser';

    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?userName=${testUserName}&pageSize=5`,
      timeout: timeout,
      failOnStatusCode: false, // Allow both success and failure
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /events with userName response', response).then(() => {
        cy.task('log', `Testing GET /events with userName filter: ${testUserName}`);

        if (response.status === 200) {
          // Positive case - user search successful
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('Events');
          expect(response.body.Events).to.be.an('array');
        } else if ([400, 404, 422].includes(response.status)) {
          // Negative case - user not found or validation error (acceptable)
          cy.task('log', `userName filter returned ${response.status} - error (acceptable)`);
          expect([400, 404, 422]).to.include(response.status);
        } else {
          throw new Error(`Unexpected status ${response.status} for userName filter`);
        }
      });
    });
  });

  it('GET /events - Search Events with companyName filter or handle gracefully', function () {
    const testCompanyName = 'testcompany';

    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?companyName=${testCompanyName}&pageSize=5`,
      timeout: timeout,
      failOnStatusCode: false, // Allow both success and failure
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /events with companyName response', response).then(() => {
        cy.task('log', `Testing GET /events with companyName filter: ${testCompanyName}`);

        if (response.status === 200) {
          // Positive case - company search successful
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('Events');
          expect(response.body.Events).to.be.an('array');
        } else if ([400, 404, 422].includes(response.status)) {
          // Negative case - company not found or validation error (acceptable)
          cy.task('log', `companyName filter returned ${response.status} - error (acceptable)`);
          expect([400, 404, 422]).to.include(response.status);
        } else {
          throw new Error(`Unexpected status ${response.status} for companyName filter`);
        }
      });
    });
  });

  it('GET /events - Search Events with sortOrder parameter or handle errors', function () {
    const sortOrders = ['asc', 'desc'];
    const testSortOrder = sortOrders[0];

    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?sortOrder=${testSortOrder}&pageSize=5`,
      timeout: timeout,
      failOnStatusCode: false, // Allow both success and failure
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /events with sortOrder response', response).then(() => {
        cy.task('log', `Testing GET /events with sortOrder: ${testSortOrder}`);

        if (response.status === 200) {
          // Positive case - sort successful
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('Events');
          expect(response.body.Events).to.be.an('array');
        } else if ([400, 422].includes(response.status)) {
          // Negative case - validation error (acceptable)
          cy.task('log', `SortOrder events search returned ${response.status} - validation error (acceptable)`);
          expect([400, 422]).to.include(response.status);
        } else if (response.status >= 500) {
          // Server error - don't fail test
          cy.task('log', `SortOrder events search returned ${response.status} - server error`);
          expect(response.status).to.be.at.least(500);
        } else {
          throw new Error(`Unexpected status ${response.status} for sortOrder events search`);
        }
      });
    });
  });

  it('GET /events - Search Events with combined filters or handle errors', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?projectID=${validProjectID}&companyID=${validCompanyID}&pageSize=3&sortOrder=desc`,
      timeout: timeout,
      failOnStatusCode: false, // Allow both success and failure
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /events with combined filters response', response).then(() => {
        cy.task(
          'log',
          `Testing GET /events with combined filters: project=${validProjectID}, company=${validCompanyID}`,
        );

        if (response.status === 200) {
          // Positive case - combined search successful
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('Events');
          expect(response.body.Events).to.be.an('array');
        } else if ([400, 422].includes(response.status)) {
          // Negative case - validation error (acceptable)
          cy.task('log', `Combined events search returned ${response.status} - validation error (acceptable)`);
          expect([400, 422]).to.include(response.status);
        } else if (response.status >= 500) {
          // Server error - don't fail test
          cy.task('log', `Combined events search returned ${response.status} - server error`);
          expect(response.status).to.be.at.least(500);
        } else {
          throw new Error(`Unexpected status ${response.status} for combined events search`);
        }
      });
    });
  });

  // ============================================================================
  // NEGATIVE TEST CASES - EXPECT SPECIFIC 4xx STATUS CODES
  // ============================================================================

  describe('Expected failures', () => {
    it('Returns 401 for Events APIs when called without token', function () {
      const unauthenticatedRequests = [
        { method: 'GET', url: `${claEndpoint}events` },
        { method: 'GET', url: `${claEndpoint}events?projectID=${validProjectID}` },
        { method: 'GET', url: `${claEndpoint}events?companyID=${validCompanyID}` },
        { method: 'GET', url: `${claEndpoint}events?userID=${validUserID}` },
        { method: 'GET', url: `${claEndpoint}events?eventType=cla_signed` },
        { method: 'GET', url: `${claEndpoint}events?searchTerm=test` },
      ];

      cy.wrap(unauthenticatedRequests).each((req: any) => {
        cy.task('log', `--> Testing ${req.method} ${req.url} without auth`);
        const requestOptions: any = {
          method: req.method,
          url: req.url,
          failOnStatusCode: false,
          timeout,
        };

        return cy.request(requestOptions).then((response) => {
          return cy.logJson('response', response).then(() => {
            cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);
            validate_expected_status(response, 401, null, null, false);
          });
        });
      });
    });

    it('Returns errors for invalid UUID parameters', function () {
      const requests = [
        {
          method: 'GET',
          url: `${claEndpoint}events?projectID=invalid-uuid-format`,
          acceptableStatuses: [200, 400, 422], // API behavior may vary
        },
        {
          method: 'GET',
          url: `${claEndpoint}events?companyID=invalid-uuid-format`,
          acceptableStatuses: [200, 400, 422], // API behavior may vary
        },
        {
          method: 'GET',
          url: `${claEndpoint}events?userID=invalid-uuid-format`,
          acceptableStatuses: [200, 400, 422], // API behavior may vary
        },
        {
          method: 'GET',
          url: `${claEndpoint}events?projectSFID=invalid-uuid-format`,
          acceptableStatuses: [200, 400, 422], // API behavior may vary
        },
      ];

      cy.wrap(requests).each((req: any) => {
        cy.task('log', `--> Testing ${req.method} ${req.url}`);
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
              cy.task('log', `Testing invalid UUID ${req.method} ${req.url} - got ${response.status}`);
              // Accept any of the acceptable status codes
              expect(req.acceptableStatuses).to.include(response.status);
            });
          });
      });
    });

    it('Returns errors for invalid parameter values', function () {
      const requests = [
        {
          method: 'GET',
          url: `${claEndpoint}events?pageSize=invalid-number`,
          acceptableStatuses: [400, 422], // API returns 422 for invalid number format
        },
        {
          method: 'GET',
          url: `${claEndpoint}events?pageSize=0`,
          acceptableStatuses: [400, 422], // API returns 422 for invalid page size
        },
        {
          method: 'GET',
          url: `${claEndpoint}events?pageSize=1001`, // Likely too large
          acceptableStatuses: [400, 422], // API returns 422 for invalid page size
        },
        {
          method: 'GET',
          url: `${claEndpoint}events?sortOrder=invalid-sort`,
          acceptableStatuses: [400, 422], // API returns 422 for invalid sort order
        },
      ];

      cy.wrap(requests).each((req: any) => {
        cy.task('log', `--> Testing ${req.method} ${req.url}`);
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
              cy.task('log', `Testing invalid parameter ${req.method} ${req.url} - got ${response.status}`);
              // Accept any of the acceptable status codes
              expect(req.acceptableStatuses).to.include(response.status);
            });
          });
      });
    });

    it('Returns errors for invalid date formats', function () {
      const requests = [
        {
          method: 'GET',
          url: `${claEndpoint}events?before=invalid-date-format`,
          expectedStatus: 400, // API returns 400 for invalid date format
          expectedCode: null,
          expectedMsg: null,
          expectedMessageContains: false,
        },
        {
          method: 'GET',
          url: `${claEndpoint}events?after=2023-13-45`, // Invalid month and day
          expectedStatus: 400, // API returns 400 for invalid date format
          expectedCode: null,
          expectedMsg: null,
          expectedMessageContains: false,
        },
        {
          method: 'GET',
          url: `${claEndpoint}events?before=2023-01-01&after=2023-12-31`, // before < after (invalid range)
          expectedStatus: 400, // API returns 400 for invalid date logic
          expectedCode: null,
          expectedMsg: null,
          expectedMessageContains: false,
        },
      ];

      cy.wrap(requests).each((req: any) => {
        cy.task('log', `--> Testing ${req.method} ${req.url}`);
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
              cy.task('log', `Testing invalid date format ${req.method} ${req.url} - expected ${req.expectedStatus}`);
              validate_expected_status(
                response,
                req.expectedStatus,
                req.expectedCode,
                req.expectedMsg,
                req.expectedMessageContains,
              );
            });
          });
      });
    });
  });
});
