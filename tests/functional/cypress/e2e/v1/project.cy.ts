// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import {
  validate_200_Status,
  validate_401_Status,
  validate_expected_status,
  getAPIBaseURL,
  getTokenForV2,
} from '../../support/commands';

describe('To Validate & test Project APIs via API call (V1)', function () {
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
  const validProjectID = '550e8400-e29b-41d4-a716-446655440000';
  const validProjectSFDCID = 'a096s00000003ZFmAAM';
  const validExternalProjectID = 'test-project-external';

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /project - Get all projects (Requires authentication)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
    }).then((response) => {
      return cy.logJson('GET /project response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return projects array or error object - both are valid
      });
    });
  });

  it('GET /project/{project_id} - Get project by ID (Requires authentication)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/${validProjectID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
    }).then((response) => {
      return cy.logJson('GET /project/{project_id} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return project data or error object - both are valid
      });
    });
  });

  it('GET /project/external/{project_external_id} - Get project by external ID (Requires authentication)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/external/${validExternalProjectID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: {
        Authorization: `Bearer ${bearerToken}`,
      },
    }).then((response) => {
      return cy.logJson('GET /project/external/{project_external_id} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        // V1 API can return project data or error object - both are valid
      });
    });
  });

  // ============================================================================
  // EXPECTED FAILURES - SEPARATE TESTS FOR 401 AND 4xx VALIDATION ERRORS
  // ============================================================================
  describe('Expected failures', () => {
    it('Returns 401 for Project APIs that require authentication when called without token', () => {
      const authenticatedEndpoints = [
        {
          title: 'GET /project without token',
          method: 'GET',
          url: `${claEndpoint}project`,
        },
        {
          title: 'GET /project/{project_id} without token',
          method: 'GET',
          url: `${claEndpoint}project/${validProjectID}`,
        },
        {
          title: 'GET /project/external/{project_external_id} without token',
          method: 'GET',
          url: `${claEndpoint}project/external/${validExternalProjectID}`,
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
        headers?: any;
      }> = [
        {
          title: 'GET /project with invalid project ID format',
          method: 'GET',
          url: `${claEndpoint}project/invalid-uuid`,
          expectedStatus: 400,
          headers: { Authorization: `Bearer ${bearerToken}` },
        },
        {
          title: 'POST /project (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}project`,
          body: {},
          expectedStatus: 405,
        },
        {
          title: 'PUT /project (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}project`,
          body: {},
          expectedStatus: 405,
        },
        {
          title: 'DELETE /project (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}project`,
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
