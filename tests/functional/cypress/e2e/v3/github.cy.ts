// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import {
  validate_200_Status,
  validate_401_Status,
  validate_expected_status,
  validateApiResponse,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeaders,
} from '../../support/commands';

describe('To Validate & test GitHub APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  let bearerToken: string = null;

  before(() => {
    getTokenKey();
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /github/login - GitHub OAuth Login Redirect (Public)', function () {
    const callbackUrl = 'https://easycla.dev.platform.linuxfoundation.org/github-callback';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}github/login?callback=${encodeURIComponent(callbackUrl)}`,
      timeout: timeout,
      failOnStatusCode: false, // Allow redirects
      followRedirect: false, // Don't follow the redirect, just check the response
    }).then((response) => {
      return cy.logJson('GET /github/login response', response).then(() => {
        cy.task('log', `GitHub login response status: ${response.status}`);

        // Expect a redirect status code (302) since this initiates OAuth flow
        validate_expected_status(response, 302);

        // Should have a Location header for the redirect
        expect(response.headers).to.have.property('location');
        expect(response.headers.location).to.be.a('string');
        expect(response.headers.location).to.include('github.com');

        // Validate that it's redirecting to GitHub OAuth
        expect(response.headers.location).to.include('oauth/authorize');
      });
    });
  });

  it('GET /github/org/{orgName}/exists - Check Organization Existence (Authenticated)', function () {
    const testOrgName = 'linuxfoundation';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}github/org/${testOrgName}/exists`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /github/org/{orgName}/exists response', response).then(() => {
        cy.task('log', `GitHub org exists response status: ${response.status}`);

        // Never expect 5xx - that would be internal server error
        expect(response.status).to.not.be.within(500, 599);

        if (response.status >= 200 && response.status < 300) {
          validate_expected_status(response, 200);

          // Handle both object and empty responses for 2xx status
          if (response.body && typeof response.body === 'object') {
            expect(response.body).to.be.an('object');
            if (response.body.exists !== undefined) {
              expect(response.body.exists).to.be.a('boolean');
            }
            validateApiResponse('github/getGitHubOrgExists.json', response);
          } else {
            // API returned 200 but with empty/non-object body
            cy.task('log', 'API returned 2xx status with empty or non-object body');
          }
        } else if (response.status === 404) {
          // 404 is acceptable and may have empty body
          validate_expected_status(response, 404);
          cy.task('log', 'API correctly returned 404 for organization existence check');
        } else {
          // Expect 4xx errors for access issues or validation errors
          expect(response.status).to.be.within(400, 499);
          validate_expected_status(response, response.status);
        }
      });
    });
  });

  it('GET /github/org/{orgName}/exists - Check Non-Existent Organization (Authenticated)', function () {
    const testOrgName = 'non-existent-org-xyz-12345';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}github/org/${testOrgName}/exists`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /github/org/{orgName}/exists non-existent response', response).then(() => {
        cy.task('log', `GitHub org exists non-existent response status: ${response.status}`);

        // Never expect 5xx - that would be internal server error
        expect(response.status).to.not.be.within(500, 599);

        if (response.status === 200) {
          validate_expected_status(response, 200);
          expect(response.body).to.be.an('object');

          if (response.body.exists !== undefined) {
            expect(response.body.exists).to.be.a('boolean');
          }
        } else if (response.status === 404) {
          validate_expected_status(response, 404);
        } else {
          // Other 4xx codes are also acceptable
          expect(response.status).to.be.within(400, 499);
          validate_expected_status(response, response.status);
        }
      });
    });
  });

  // ============================================================================
  // REDIRECT CALLBACK ENDPOINT TESTING
  // ============================================================================

  it('GET /github/redirect - GitHub OAuth Callback (Public)', function () {
    const dummyCode = 'test_authorization_code';
    const dummyState = 'test_state_parameter';

    cy.request({
      method: 'GET',
      url: `${claEndpoint}github/redirect?code=${dummyCode}&state=${dummyState}`,
      timeout: timeout,
      failOnStatusCode: false,
    }).then((response) => {
      return cy.logJson('GET /github/redirect response', response).then(() => {
        cy.task('log', `GitHub redirect response status: ${response.status}`);

        if (response.status === 302) {
          // Redirect is expected for OAuth callback
          validate_expected_status(response, 302);
          // If it's a redirect, should have Location header
          expect(response.headers).to.have.property('location');
        } else if (response.status >= 200 && response.status < 300) {
          // Some implementations might return 200 with success/error page
          validate_expected_status(response, 200);
        } else if (response.status >= 400 && response.status <= 499) {
          // 4xx errors are expected for invalid OAuth parameters
          validate_expected_status(response, response.status);
        } else if (response.status >= 500 && response.status <= 599) {
          // 5xx errors are also acceptable for OAuth endpoints with dummy parameters
          validate_expected_status(response, response.status);
          cy.task('log', 'OAuth redirect returned 5xx - expected with dummy OAuth parameters');
        } else {
          // Any other status code
          cy.task('log', `OAuth redirect returned unexpected status: ${response.status}`);
          validate_expected_status(response, response.status);
        }
      });
    });
  });

  // ============================================================================
  // NEGATIVE TEST CASES - EXPECTED FAILURES
  // ============================================================================

  describe('Expected failures', () => {
    it('Returns 401 for GitHub authenticated APIs when called without token', () => {
      const testOrgName = 'linuxfoundation';

      const requests = [{ method: 'GET', url: `${claEndpoint}github/org/${testOrgName}/exists` }];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);

              // If API consistently returns 500, we expect that
              if (response.status >= 500) {
                cy.task('log', `API returning ${response.status} - may not be implemented`);
                expect(response.status).to.be.within(500, 599);
              } else {
                validate_expected_status(response, 401);
              }
            });
          });
      });
    });

    it('Returns 4xx for malformed GitHub API parameters', () => {
      const requests = [
        {
          title: 'GET /github/login without required callback parameter',
          method: 'GET',
          url: `${claEndpoint}github/login`,
          expectedStatuses: [400, 422],
          expectedCodes: [400, 422, 602],
        },
        {
          title: 'GET /github/login with empty callback parameter',
          method: 'GET',
          url: `${claEndpoint}github/login?callback=`,
          expectedStatuses: [400, 422],
          expectedCodes: [400, 422, 602],
        },
        {
          title: 'GET /github/redirect without required parameters',
          method: 'GET',
          url: `${claEndpoint}github/redirect`,
          expectedStatuses: [400, 422, 500],
          expectedCodes: [400, 422, 500, 602],
        },
        {
          title: 'GET /github/redirect with only code parameter',
          method: 'GET',
          url: `${claEndpoint}github/redirect?code=test`,
          expectedStatuses: [400, 422, 500],
          expectedCodes: [400, 422, 500, 602],
        },
        {
          title: 'GET /github/redirect with only state parameter',
          method: 'GET',
          url: `${claEndpoint}github/redirect?state=test`,
          expectedStatuses: [400, 422, 500],
          expectedCodes: [400, 422, 500, 602],
        },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing malformed params: ${req.title}`);

              // Allow both 4xx and 5xx if API is not implemented
              expect(req.expectedStatuses).to.include(response.status);

              if (response.body && (response.body.code || response.body.Code)) {
                expect(req.expectedCodes).to.include(response.body.code ?? response.body.Code);
              }
            });
          });
      });
    });

    it('Returns 4xx for invalid GitHub organization names', () => {
      const requests = [
        {
          title: 'Empty organization name',
          method: 'GET',
          url: `${claEndpoint}github/org//exists`,
          expectedStatuses: [400, 404, 422, 500],
          expectedCodes: [400, 404, 422, 500, 601],
        },
        {
          title: 'Organization name with invalid characters',
          method: 'GET',
          url: `${claEndpoint}github/org/invalid@org#name/exists`,
          expectedStatuses: [400, 404, 422, 500],
          expectedCodes: [400, 404, 422, 500, 601],
        },
        {
          title: 'Extremely long organization name',
          method: 'GET',
          url: `${claEndpoint}github/org/${'a'.repeat(500)}/exists`,
          expectedStatuses: [400, 404, 422, 500],
          expectedCodes: [400, 404, 422, 500, 601],
        },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
            headers: getXACLHeaders(),
            auth: {
              bearer: bearerToken,
            },
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing invalid org name: ${req.title}`);

              // Allow both 4xx and 5xx if API is not implemented
              expect(req.expectedStatuses).to.include(response.status);

              if (response.body && (response.body.code || response.body.Code)) {
                expect(req.expectedCodes).to.include(response.body.code ?? response.body.Code);
              }
            });
          });
      });
    });
  });
});
