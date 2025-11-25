// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import { validate_200_Status, validate_expected_status, getAPIBaseURL } from '../../support/commands';

describe('To Validate & test Project APIs via API call (V2)', function () {
  const claEndpoint = getAPIBaseURL('v2');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;

  // Test data
  const validProjectID = '550e8400-e29b-41d4-a716-446655440000';
  const validUserID = '550e8400-e29b-41d4-a716-446655440001';
  const validCompanyID = '550e8400-e29b-41d4-a716-446655440002';

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /project/{project_id} - Get project by ID (No authentication required)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/${validProjectID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('GET /project/{project_id} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V2 API can return project data or error object
        if (response.body.errors) {
          // API returned error (project not found), which is valid
          expect(response.body).to.have.property('errors');
        } else {
          // API returned project data
          expect(response.body).to.have.property('project_id');
        }
      });
    });
  });

  it('GET /project/{project_id}/document/{document_type} - Get project document (No authentication required)', function () {
    const documentType = 'individual'; // or 'corporate'
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/${validProjectID}/document/${documentType}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('GET /project/{project_id}/document/{document_type} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V2 API can return document data or error object - both are valid
      });
    });
  });

  it('GET /project/{project_id}/companies - Get project companies (No authentication required)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/${validProjectID}/companies`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('GET /project/{project_id}/companies response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V2 API returns error object when project not found, array when project exists
        if (response.body.errors) {
          expect(response.body).to.have.property('errors');
        } else {
          expect(response.body).to.be.an('array');
        }
      });
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    it('Returns 4xx for missing or malformed parameters for Project APIs', function () {
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
          title: 'GET /project/{project_id} with invalid UUID format',
          method: 'GET',
          url: `${claEndpoint}project/invalid-uuid`,
          expectedStatus: 400,
        },
        {
          title: 'GET /project/{project_id}/document/{document_type} with invalid document type',
          method: 'GET',
          url: `${claEndpoint}project/${validProjectID}/document/invalid-type`,
          expectedStatus: 400,
        },
        {
          title: 'POST /project (method not allowed in V2)',
          method: 'POST',
          url: `${claEndpoint}project`,
          body: {},
          expectedStatus: 404,
        },
        {
          title: 'DELETE /project/{project_id} (method not allowed in V2)',
          method: 'DELETE',
          url: `${claEndpoint}project/${validProjectID}`,
          expectedStatus: 404,
        },
        {
          title: 'POST /user/{user_id}/request-company-whitelist/{company_id} with missing required fields',
          method: 'POST',
          url: `${claEndpoint}user/${validUserID}/request-company-whitelist/${validCompanyID}`,
          body: {},
          expectedStatus: 400,
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
