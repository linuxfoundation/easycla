// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import { validate_200_Status, validate_expected_status, getAPIBaseURL } from '../../support/commands';

describe('To Validate & test Signature APIs via API call (V2)', function () {
  const claEndpoint = getAPIBaseURL('v2');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;

  // Test data
  const validProjectID = '550e8400-e29b-41d4-a716-446655440000';
  const validUserID = '550e8400-e29b-41d4-a716-446655440001';
  const validCompanyID = '550e8400-e29b-41d4-a716-446655440002';
  const validSignatureID = '550e8400-e29b-41d4-a716-446655440003';

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('POST /request-individual-signature - Request individual signature (No authentication required)', function () {
    const requestData = {
      project_id: validProjectID,
      user_id: validUserID,
      return_url_type: 'Github',
      return_url: 'https://github.com/test/repo',
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}request-individual-signature`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      body: requestData,
    }).then((response) => {
      return cy.logJson('POST /request-individual-signature response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V2 API can return signature data or error object - both are valid
      });
    });
  });

  it('POST /request-employee-signature - Request employee signature (No authentication required)', function () {
    const requestData = {
      project_id: validProjectID,
      company_id: validCompanyID,
      user_id: validUserID,
      return_url_type: 'Github',
      return_url: 'https://github.com/test/repo',
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}request-employee-signature`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      body: requestData,
    }).then((response) => {
      return cy.logJson('POST /request-employee-signature response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V2 API can return signature data or error object - both are valid
      });
    });
  });

  it('POST /check-prepare-employee-signature - Check employee signature readiness (No authentication required)', function () {
    const requestData = {
      project_id: validProjectID,
      company_id: validCompanyID,
      user_id: validUserID,
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}check-prepare-employee-signature`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      body: requestData,
    }).then((response) => {
      return cy.logJson('POST /check-prepare-employee-signature response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V2 API can return check data or error object - both are valid
      });
    });
  });

  it('GET /return-url/{signature_id} - Get signature return URL (No authentication required)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}return-url/${validSignatureID}`,
      timeout: timeout,
      failOnStatusCode: false, // May redirect
    }).then((response) => {
      return cy.logJson('GET /return-url/{signature_id} response', response).then(() => {
        // Can return 200 (HTML content) or 302 (redirect)
        expect([200, 302]).to.include(response.status);
      });
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    it('Returns 4xx for missing or malformed parameters for Signature APIs', function () {
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
          title: 'POST /request-individual-signature with missing parameters',
          method: 'POST',
          url: `${claEndpoint}request-individual-signature`,
          body: {},
          expectedStatus: 400,
        },
        {
          title: 'POST /request-employee-signature with missing parameters',
          method: 'POST',
          url: `${claEndpoint}request-employee-signature`,
          body: {},
          expectedStatus: 400,
        },
        {
          title: 'POST /check-prepare-employee-signature with missing parameters',
          method: 'POST',
          url: `${claEndpoint}check-prepare-employee-signature`,
          body: {},
          expectedStatus: 400,
        },
        {
          title: 'GET /return-url/{signature_id} with invalid UUID',
          method: 'GET',
          url: `${claEndpoint}return-url/invalid-uuid`,
          expectedStatus: 400,
        },
        {
          title: 'DELETE /request-individual-signature (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}request-individual-signature`,
          expectedStatus: 405,
        },
        {
          title: 'PUT /request-employee-signature (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}request-employee-signature`,
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
