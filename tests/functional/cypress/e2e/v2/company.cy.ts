// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import { validate_200_Status, validate_expected_status, getAPIBaseURL } from '../../support/commands';

describe('To Validate & test Company APIs via API call (V2)', function () {
  const claEndpoint = getAPIBaseURL('v2');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;

  // Test data - using UUIDs that might exist
  let validCompanyID = '550e8400-e29b-41d4-a716-446655440000';

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /company - Get all companies (No authentication required)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}company`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('GET /company response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('array');
        // V2 API returns companies as an array
        if (response.body.length > 0) {
          expect(response.body[0]).to.have.property('company_id');
        }
      });
    });
  });

  it('GET /company/{company_id} - Get company by ID (No authentication required)', function () {
    // First get companies to find a valid ID
    cy.request({
      method: 'GET',
      url: `${claEndpoint}company`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((companiesResponse) => {
      validate_200_Status(companiesResponse);
      expect(companiesResponse.body).to.be.an('array');

      if (companiesResponse.body.length > 0) {
        // Use the first company ID that has a company_name
        const companyWithName = companiesResponse.body.find((c) => c.company_name !== null);
        if (companyWithName) {
          const companyId = companyWithName.company_id;
          cy.request({
            method: 'GET',
            url: `${claEndpoint}company/${companyId}`,
            timeout: timeout,
            failOnStatusCode: allowFail,
          }).then((response) => {
            return cy.logJson('GET /company/{company_id} response', response).then(() => {
              validate_200_Status(response);
              expect(response.body).to.be.an('object');
              expect(response.body).to.have.property('company_id');
            });
          });
        } else {
          // Use any company ID to test the endpoint
          const companyId = companiesResponse.body[0].company_id;
          cy.request({
            method: 'GET',
            url: `${claEndpoint}company/${companyId}`,
            timeout: timeout,
            failOnStatusCode: allowFail,
          }).then((response) => {
            return cy.logJson('GET /company/{company_id} response', response).then(() => {
              validate_200_Status(response);
              expect(response.body).to.be.an('object');
              expect(response.body).to.have.property('company_id');
            });
          });
        }
      } else {
        // Skip test if no companies exist - use a known UUID for testing
        cy.request({
          method: 'GET',
          url: `${claEndpoint}company/${validCompanyID}`,
          timeout: timeout,
          failOnStatusCode: false,
        }).then((response) => {
          // V2 API returns 200 even for invalid UUIDs, but may return different data structure
          expect(response.status).to.eq(200);
        });
      }
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    // V2 Company endpoints do NOT require authentication, so no 401 tests needed

    it('Returns 4xx for missing or malformed parameters for Company APIs', function () {
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
          title: 'POST /company (not defined in V2)',
          method: 'POST',
          url: `${claEndpoint}company`,
          body: {},
          expectedStatus: 404,
          expectedMessage: 'not defined',
          expectedMessageContains: true,
        },
        {
          title: 'PUT /company (not defined in V2)',
          method: 'PUT',
          url: `${claEndpoint}company`,
          body: {},
          expectedStatus: 404,
          expectedMessage: 'not defined',
          expectedMessageContains: true,
        },
        {
          title: 'DELETE /company/{company_id} (not defined in V2)',
          method: 'DELETE',
          url: `${claEndpoint}company/550e8400-e29b-41d4-a716-446655440000`,
          expectedStatus: 404,
          expectedMessage: 'not defined',
          expectedMessageContains: true,
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
