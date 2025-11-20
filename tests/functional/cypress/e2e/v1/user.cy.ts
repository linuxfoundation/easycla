// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import {
  validate_200_Status,
  validate_401_Status,
  validate_expected_status,
  getAPIBaseURL,
  getTokenForV2,
} from '../../support/commands';

describe('To Validate & test User APIs via API call (V1)', function () {
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
  const validUserID = '550e8400-e29b-41d4-a716-446655440000';
  const validProjectID = '550e8400-e29b-41d4-a716-446655440001';
  const validCompanyID = '550e8400-e29b-41d4-a716-446655440002';

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('POST /user/gerrit - Create/get Gerrit user (Requires authentication)', function () {
    const gerritUserData = {
      username: 'testuser',
      email: 'testuser@example.com',
      full_name: 'Test User',
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}user/gerrit`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
      body: gerritUserData,
    }).then((response) => {
      return cy.logJson('POST /user/gerrit response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return user data or error object - both are valid
      });
    });
  });

  it('GET /user/{user_id}/signatures - Get user signatures (Requires authentication)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}user/${validUserID}/signatures`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
    }).then((response) => {
      return cy.logJson('GET /user/{user_id}/signatures response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return signatures array or error object - both are valid
      });
    });
  });

  it('GET /users/company/{user_company_id} - Get company users (Requires authentication)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}users/company/${validCompanyID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
    }).then((response) => {
      return cy.logJson('GET /users/company/{user_company_id} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return users array or error object - both are valid
      });
    });
  });

  it('GET /user/{user_id}/project/{project_id}/last-signature/{company_id} - Get last signature (Requires authentication)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}user/${validUserID}/project/${validProjectID}/last-signature/${validCompanyID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
    }).then((response) => {
      return cy.logJson('GET /user/.../last-signature/... response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return signature data or error object - both are valid
      });
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    it('Returns 401 for User APIs that require authentication when called without token', () => {
      const authenticatedEndpoints = [
        {
          title: 'POST /user/gerrit without token',
          method: 'POST',
          url: `${claEndpoint}user/gerrit`,
          body: { username: 'test' },
        },
        {
          title: 'GET /user/{user_id}/signatures without token',
          method: 'GET',
          url: `${claEndpoint}user/${validUserID}/signatures`,
        },
        {
          title: 'GET /users/company/{user_company_id} without token',
          method: 'GET',
          url: `${claEndpoint}users/company/${validCompanyID}`,
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
        headers?: any;
      }> = [
        {
          title: 'POST /user/gerrit with missing parameters',
          method: 'POST',
          url: `${claEndpoint}user/gerrit`,
          body: {},
          expectedStatus: 400,
          headers: { Authorization: `Bearer ${bearerToken}` },
        },
        {
          title: 'GET /user with invalid UUID format',
          method: 'GET',
          url: `${claEndpoint}user/invalid-uuid/signatures`,
          expectedStatus: 400,
          headers: { Authorization: `Bearer ${bearerToken}` },
        },
        {
          title: 'PUT /user/gerrit (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}user/gerrit`,
          body: {},
          expectedStatus: 405,
        },
        {
          title: 'DELETE /user/{user_id}/signatures (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}user/${validUserID}/signatures`,
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
