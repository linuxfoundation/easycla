// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import { validate_200_Status, validate_expected_status, getAPIBaseURL } from '../../support/commands';

describe('To Validate & test Health APIs via API call (V2)', function () {
  const claEndpoint = getAPIBaseURL('v2');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /health - Returns the Health of the application', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}health`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('GET /health response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V2 health endpoint returns request headers as response body
        expect(response.body).to.have.property('HOST');
      });
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    it('Returns 4xx for missing or malformed parameters for Health APIs', function () {
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
          title: 'POST /health (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}health`,
          body: {},
          expectedStatus: 405,
          expectedMessage: '405 Method Not Allowed',
        },
        {
          title: 'PUT /health (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}health`,
          body: {},
          expectedStatus: 405,
          expectedMessage: '405 Method Not Allowed',
        },
        {
          title: 'DELETE /health (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}health`,
          expectedStatus: 405,
          expectedMessage: '405 Method Not Allowed',
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
