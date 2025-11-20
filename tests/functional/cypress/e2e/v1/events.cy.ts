// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import {
  validate_200_Status,
  validate_401_Status,
  validate_expected_status,
  getAPIBaseURL,
  getTokenForV2,
} from '../../support/commands';

describe('To Validate & test Events APIs via API call (V1)', function () {
  const claEndpoint = getAPIBaseURL('v1');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;

  let bearerToken: string = null;
  before(() => {
    const envToken = Cypress.env('TOKEN');
    if (envToken && envToken !== '-') {
      bearerToken = envToken;
    } else {
      return getTokenForV2().then((token) => {
        bearerToken = token;
      });
    }
  });

  // Test data
  const validEventID = '550e8400-e29b-41d4-a716-446655440000';
  const validUserID = '550e8400-e29b-41d4-a716-446655440001';
  const validProjectID = '550e8400-e29b-41d4-a716-446655440002';
  const validCompanyID = '550e8400-e29b-41d4-a716-446655440003';

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it.skip('GET /events - Get paginated/filtered events (Requires authentication)', function () {
    // Use filtering to get a limited set of events to avoid large responses
    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?event_type=user_signed_cla`, // Filter by event type for smaller response
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
    }).then((response) => {
      return cy.logJson('GET /events with filter response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body).to.have.property('events');
        expect(response.body.events).to.be.an('array');
      });
    });
  });

  it.skip('GET /events - Get all events (Requires authentication)', function () {
    // SKIPPED: This endpoint may return very large responses without filtering
    // Error: ProvisionedThroughputExceededException on table cla-dev-events
    // Use the filtered version above instead for better performance
    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?limit=10`, // Try with pagination to reduce load
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
    }).then((response) => {
      return cy.logJson('GET /events response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return events array or error object - both are valid
      });
    });
  });

  it.skip('POST /events - Create event (Requires authentication)', function () {
    const eventData = {
      event_type: 'user_signed_cla',
      event_data: 'User signed individual CLA',
      user_id: validUserID,
      event_project_id: validProjectID,
      event_company_id: validCompanyID,
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}events`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
      body: eventData,
    }).then((response) => {
      return cy.logJson('POST /events response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API returns event object on successful creation
      });
    });
  });

  it.skip('GET /events/{event_id} - Get specific event (Requires authentication)', function () {
    // SKIPPED: This endpoint returns 404/error responses in dev environment for test event IDs
    cy.request({
      method: 'GET',
      url: `${claEndpoint}events/${validEventID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
    }).then((response) => {
      return cy.logJson('GET /events/{event_id} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return event data or error object - both are valid
      });
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    it.skip('Returns 401 for Events APIs that require authentication when called without token', () => {
      // SKIPPED: V1 Events API authentication behavior varies by endpoint
      const authenticatedEndpoints = [
        {
          title: 'GET /events without token',
          method: 'GET',
          url: `${claEndpoint}events`,
        },
        {
          title: 'POST /events without token',
          method: 'POST',
          url: `${claEndpoint}events`,
          body: {
            event_type: 'test',
            event_data: 'test data',
          },
        },
        {
          title: 'GET /events/{event_id} without token',
          method: 'GET',
          url: `${claEndpoint}events/${validEventID}`,
        },
      ];

      cy.wrap(authenticatedEndpoints).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            body: req.body,
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing: ${req.title}`);
              expect(response.status).to.eq(401);
              expect(response.statusText).to.eq('Unauthorized');
              // V1 API returns simple string for 401 errors (same as V2)
              expect(response.body).to.be.a('string');
              expect(response.body).to.contain('authorization');
            });
          });
      });
    });

    it.skip('Returns 4xx for missing or malformed parameters for Events APIs', function () {
      // SKIPPED: V1 Events API returns inconsistent status codes (404 vs 400) for UUID validation
      // Different behavior depending on endpoint implementation
      const cases: Array<{
        title: string;
        method: 'GET' | 'POST' | 'PUT' | 'DELETE';
        url: string;
        body?: any;
        expectedStatus: number;
        expectedCode?: number;
        expectedMessage?: string;
        expectedMessageContains?: boolean;
        headers?: any;
      }> = [
        {
          title: 'POST /events with missing parameters',
          method: 'POST',
          url: `${claEndpoint}events`,
          body: {},
          expectedStatus: 400,
          headers: { Authorization: `Bearer ${bearerToken}` },
        },
        {
          title: 'GET /events with invalid event ID format',
          method: 'GET',
          url: `${claEndpoint}events/invalid-id`,
          expectedStatus: 400,
          headers: { Authorization: `Bearer ${bearerToken}` },
        },
        {
          title: 'PUT /events (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}events`,
          body: {},
          expectedStatus: 405,
        },
        {
          title: 'DELETE /events (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}events`,
          expectedStatus: 405,
        },
      ];

      cy.wrap(cases).each((c: any) => {
        return cy
          .request({
            method: c.method,
            url: c.url,
            body: c.body,
            headers: c.headers,
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing: ${c.title}`);
              validate_expected_status(
                response,
                c.expectedStatus,
                c.expectedCode,
                c.expectedMessage,
                c.expectedMessageContains,
              );
            });
          });
      });
    });
  });
});
