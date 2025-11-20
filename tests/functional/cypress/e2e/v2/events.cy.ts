// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import { validate_200_Status, validate_expected_status, getAPIBaseURL, getTokenForV2 } from '../../support/commands';

describe('To Validate & test Events APIs via API call (V2)', function () {
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
  const validCompanyName = 'Test Company';
  const validProjectName = 'Test Project';
  const validAuthorityName = 'John Doe';
  const validAuthorityEmail = 'john.doe@example.com';

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('POST /send-authority-email - Send authority email (Requires authentication)', function () {
    const emailData = {
      company_name: validCompanyName,
      project_name: validProjectName,
      authority_name: validAuthorityName,
      authority_email: validAuthorityEmail,
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}send-authority-email`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
      body: emailData,
    }).then((response) => {
      return cy.logJson('POST /send-authority-email response', response).then(() => {
        validate_200_Status(response);
        // V2 API returns null body for successful POST requests
        expect(response.body).to.be.null;
      });
    });
  });

  it('POST /clear-cache - Clear cache (Requires authentication)', function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}clear-cache`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
    }).then((response) => {
      return cy.logJson('POST /clear-cache response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V2 API returns {status: 'OK'} for successful clear-cache requests
        expect(response.body).to.have.property('status', 'OK');
      });
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    it('Returns 401 for Events APIs that require authentication when called without token', () => {
      const authenticatedEndpoints = [
        { method: 'POST', url: `${claEndpoint}send-authority-email`, body: {} },
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

    it('Returns 4xx for missing or malformed parameters for Events APIs', function () {
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
          title: 'POST /send-authority-email with missing parameters',
          method: 'POST',
          url: `${claEndpoint}send-authority-email`,
          body: {},
          expectedStatus: 400,
          headers: { Authorization: `Bearer ${bearerToken}` },
        },
        {
          title: 'GET /send-authority-email (method not allowed)',
          method: 'GET',
          url: `${claEndpoint}send-authority-email`,
          expectedStatus: 405,
        },
        {
          title: 'DELETE /clear-cache (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}clear-cache`,
          expectedStatus: 405, // API returns 405 for method not allowed before checking auth
        },
        {
          title: 'GET /clear-cache (method not allowed)',
          method: 'GET',
          url: `${claEndpoint}clear-cache`,
          expectedStatus: 405, // API returns 405 for method not allowed before checking auth
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
