// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import { validate_200_Status, validate_expected_status, getAPIBaseURL, getTokenForV2 } from '../../support/commands';

describe('To Validate & test User APIs via API call (V2)', function () {
  const claEndpoint = getAPIBaseURL('v2');
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
  const validUserID = '550e8400-e29b-41d4-a716-446655440000';
  const validProjectID = '550e8400-e29b-41d4-a716-446655440001';
  const validCompanyID = '550e8400-e29b-41d4-a716-446655440002';

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /user/{user_id} - Get user by ID (No authentication required)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}user/${validUserID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('GET /user/{user_id} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V2 API can return user data or error object
        if (response.body.errors) {
          // API returned error (user not found), which is valid
          expect(response.body).to.have.property('errors');
        } else {
          // API returned user data
          expect(response.body).to.have.property('user_id');
        }
      });
    });
  });

  it('GET /user/{user_id}/active-signature - Get user active signature (No authentication required)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}user/${validUserID}/active-signature`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('GET /user/{user_id}/active-signature response', response).then(() => {
        validate_200_Status(response);
        // API returns null if no active signature, which is valid
        expect(response.body).to.satisfy((body) => body === null || typeof body === 'object');
      });
    });
  });

  it('GET /user/{user_id}/project/{project_id}/last-signature - Get user project last signature (No authentication required)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}user/${validUserID}/project/${validProjectID}/last-signature`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('GET /user/{user_id}/project/{project_id}/last-signature response', response).then(() => {
        validate_200_Status(response);
        // API returns null if no signature found, which is valid
        expect(response.body).to.satisfy((body) => body === null || typeof body === 'object');
      });
    });
  });

  it('GET /user-from-session - Get user from session (No token required)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}user-from-session`,
      timeout: timeout,
      failOnStatusCode: false, // Allow 302 redirect
    }).then((response) => {
      return cy.logJson('GET /user-from-session response', response).then(() => {
        // V2 API returns 302 redirect when no session exists
        expect([200, 302]).to.include(response.status);
      });
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    it('Returns 401 for User APIs that require authentication when called without token', () => {
      const authenticatedEndpoints = [
        { method: 'GET', url: `${claEndpoint}user-from-token` },
        { method: 'POST', url: `${claEndpoint}clear-cache` },
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
              cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);
              expect(response.status).to.eq(401);
            });
          });
      });
    });

    it('Returns 4xx for missing or malformed parameters for User APIs', function () {
      const cases: Array<{
        title: string;
        method: 'GET' | 'POST' | 'PUT' | 'DELETE';
        url: string;
        body?: any;
        expectedStatus: number;
        expectedCode?: number;
        expectedMessage?: string;
        expectedMessageContains?: boolean;
      }> = [
        {
          title: 'GET /user/{user_id} with invalid UUID format',
          method: 'GET',
          url: `${claEndpoint}user/invalid-uuid`,
          expectedStatus: 400,
        },
        {
          title: 'POST /user/{user_id}/request-company-whitelist/{company_id} with missing parameters',
          method: 'POST',
          url: `${claEndpoint}user/${validUserID}/request-company-whitelist/${validCompanyID}`,
          body: {},
          expectedStatus: 400,
        },
        {
          title: 'POST /user/{user_id}/invite-company-admin with missing parameters',
          method: 'POST',
          url: `${claEndpoint}user/${validUserID}/invite-company-admin`,
          body: {},
          expectedStatus: 400,
        },
        {
          title: 'POST /user/{user_id}/request-company-ccla with missing parameters',
          method: 'POST',
          url: `${claEndpoint}user/${validUserID}/request-company-ccla`,
          body: {},
          expectedStatus: 400,
        },
        {
          title: 'DELETE /user/{user_id}/active-signature (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}user/${validUserID}/active-signature`,
          expectedStatus: 405,
        },
        {
          title: 'DELETE /user/{user_id} (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}user/${validUserID}`,
          expectedStatus: 405,
        },
      ];

      cy.wrap(cases).each((c: any) => {
        return cy
          .request({
            method: c.method,
            url: c.url,
            body: c.body,
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
