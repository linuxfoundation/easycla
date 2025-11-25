// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import { validate_200_Status, validate_expected_status, getAPIBaseURL, getTokenForV2 } from '../../support/commands';

describe('To Validate & test Signature APIs via API call (V1)', function () {
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
  const validSignatureID = '550e8400-e29b-41d4-a716-446655440000';
  const validUserID = '550e8400-e29b-41d4-a716-446655440001';
  const validProjectID = '550e8400-e29b-41d4-a716-446655440002';

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /signature/{signature_id} - Get signature by ID (Requires authentication)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signature/${validSignatureID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
    }).then((response) => {
      return cy.logJson('GET /signature/{signature_id} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return signature data or error object - both are valid
      });
    });
  });

  it('POST /signature - Create signature (Requires authentication)', function () {
    const signatureData = {
      signature_type: 'cla',
      signature_signed: true,
      signature_approved: true,
      signature_embargo_acked: true,
      signature_sign_url: 'http://sign.com/here',
      signature_return_url: 'http://cla-system.com/signed',
      signature_project_id: validProjectID,
      signature_reference_id: validUserID,
      signature_reference_type: 'user', // V1 expects 'user' or 'company', not 'individual'
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}signature`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
      body: signatureData,
    }).then((response) => {
      return cy.logJson('POST /signature response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API returns signature object on successful creation
      });
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    it('Returns 401 for Signature APIs that require authentication when called without token', () => {
      const authenticatedEndpoints = [
        {
          title: 'GET /signature/{signature_id} without token',
          method: 'GET',
          url: `${claEndpoint}signature/${validSignatureID}`,
        },
        {
          title: 'POST /signature without token',
          method: 'POST',
          url: `${claEndpoint}signature`,
          body: {
            signature_type: 'cla',
            signature_reference_type: 'user', // V1 expects 'user' or 'company'
          },
        },
        {
          title: 'DELETE /signature/{signature_id} without token',
          method: 'DELETE',
          url: `${claEndpoint}signature/${validSignatureID}`,
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
        headers?: any;
      }> = [
        {
          title: 'POST /signature with missing parameters',
          method: 'POST',
          url: `${claEndpoint}signature`,
          body: {},
          expectedStatus: 400,
          headers: { Authorization: `Bearer ${bearerToken}` },
        },
        {
          title: 'GET /signature with invalid UUID format',
          method: 'GET',
          url: `${claEndpoint}signature/invalid-uuid`,
          expectedStatus: 400,
          headers: { Authorization: `Bearer ${bearerToken}` },
        },
        {
          title: 'PUT /signature (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}signature/${validSignatureID}`,
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
