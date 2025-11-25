// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import { validate_200_Status, validate_expected_status, getAPIBaseURL, getTokenForV2 } from '../../support/commands';

describe('To Validate & test Repository APIs via API call (V1)', function () {
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
  const validRepositoryID = '550e8400-e29b-41d4-a716-446655440000';

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /repository/{repository_id} - Get repository by ID (Requires authentication)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}repository/${validRepositoryID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
    }).then((response) => {
      return cy.logJson('GET /repository/{repository_id} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return repository data or error object - both are valid
      });
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    it('Returns 401 for Repository APIs that require authentication when called without token', () => {
      const authenticatedEndpoints = [
        {
          title: 'GET /repository/{repository_id} without token',
          method: 'GET',
          url: `${claEndpoint}repository/${validRepositoryID}`,
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

    it('Returns 4xx for missing or malformed parameters for Repository APIs', function () {
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
          title: 'GET /repository with invalid UUID format - API accepts and returns error object',
          method: 'GET',
          url: `${claEndpoint}repository/invalid-uuid`,
          expectedStatus: 200, // V1 API accepts invalid UUID and returns error in response body
          headers: { Authorization: `Bearer ${bearerToken}` },
        },
        {
          title: 'PUT /repository (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}repository/${validRepositoryID}`,
          body: {},
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
