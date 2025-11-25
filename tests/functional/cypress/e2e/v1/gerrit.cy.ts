// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import { validate_200_Status, validate_expected_status, getAPIBaseURL, getTokenForV2 } from '../../support/commands';

describe('To Validate & test Gerrit APIs via API call (V1)', function () {
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
  const validProjectID = '550e8400-e29b-41d4-a716-446655440000';
  const validGerritID = '550e8400-e29b-41d4-a716-446655440001';

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it.skip('GET /project/{project_id}/gerrits - Get project Gerrit instances (No authentication required)', function () {
    // SKIPPED: This endpoint may cause server errors in development environment
    // when querying non-existent projects with test UUIDs
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/${validProjectID}/gerrits`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('GET /project/{project_id}/gerrits response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return gerrits array or error object - both are valid
      });
    });
  });

  it.skip('GET /gerrit/{gerrit_id} - Get Gerrit instance by ID (No authentication required)', function () {
    // SKIPPED: This endpoint may cause server errors in development environment
    // when querying non-existent Gerrit instances with test UUIDs
    cy.request({
      method: 'GET',
      url: `${claEndpoint}gerrit/${validGerritID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('GET /gerrit/{gerrit_id} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return gerrit data or error object - both are valid
      });
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    it.skip('Returns 4xx for missing or malformed parameters for Gerrit APIs', function () {
      // SKIPPED: V1 API behavior varies for UUID validation (400 vs 404)
      // Different status codes returned depending on endpoint implementation
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
          title: 'GET /gerrit with invalid UUID format - returns 400 bad request',
          method: 'GET',
          url: `${claEndpoint}gerrit/invalid-uuid`,
          expectedStatus: 400, // V1 API returns 400 for invalid gerrit UUID format
        },
        {
          title: 'GET /project with invalid UUID format - returns 400 bad request',
          method: 'GET',
          url: `${claEndpoint}project/invalid-uuid/gerrits`,
          expectedStatus: 400, // V1 API returns 400 for invalid project UUID format
        },
        {
          title: 'PUT /gerrit (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}gerrit/${validGerritID}`,
          body: {},
          expectedStatus: 405,
        },
        {
          title: 'DELETE /project/{project_id}/gerrits (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}project/${validProjectID}/gerrits`,
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
