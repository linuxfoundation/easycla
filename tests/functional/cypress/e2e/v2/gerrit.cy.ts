// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import { validate_200_Status, validate_expected_status, getAPIBaseURL } from '../../support/commands';

describe('To Validate & test Gerrit APIs via API call (V2)', function () {
  const claEndpoint = getAPIBaseURL('v2');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;

  // Test data
  const validGerritID = '550e8400-e29b-41d4-a716-446655440000';
  const validContractType = 'individual';

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /gerrit/{gerrit_id} - Get gerrit instance by ID (No authentication required)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}gerrit/${validGerritID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('GET /gerrit/{gerrit_id} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V2 API can return gerrit data or error object - both are valid responses
      });
    });
  });

  it('GET /gerrit/{gerrit_id}/{contract_type}/agreementUrl.html - Get agreement HTML (No authentication required)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}gerrit/${validGerritID}/${validContractType}/agreementUrl.html`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('GET /gerrit/{gerrit_id}/{contract_type}/agreementUrl.html response', response).then(() => {
        validate_200_Status(response);
        // This endpoint returns HTML content
        expect(response.body).to.be.a('string');
      });
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    it('Returns 4xx for missing or malformed parameters for Gerrit APIs', function () {
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
          title: 'GET /gerrit/{gerrit_id} with invalid UUID format',
          method: 'GET',
          url: `${claEndpoint}gerrit/invalid-uuid`,
          expectedStatus: 400,
        },
        {
          title: 'GET /gerrit/{gerrit_id}/{contract_type}/agreementUrl.html with invalid contract type',
          method: 'GET',
          url: `${claEndpoint}gerrit/${validGerritID}/invalid-type/agreementUrl.html`,
          expectedStatus: 400,
        },
        {
          title: 'POST /gerrit (method not allowed in V2)',
          method: 'POST',
          url: `${claEndpoint}gerrit`,
          body: {},
          expectedStatus: 404,
        },
        {
          title: 'DELETE /gerrit/{gerrit_id} (method not allowed in V2)',
          method: 'DELETE',
          url: `${claEndpoint}gerrit/${validGerritID}`,
          expectedStatus: 404,
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
