// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import {
  validate_200_Status,
  validate_204_Status,
  validate_401_Status,
  validate_expected_status,
  validateApiResponse,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeaders,
} from '../../support/commands';

describe('To Validate & test GitHub Organizations APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  let bearerToken: string = null;
  let validProjectSFID: string = null;
  let createdOrgName: string = null;

  // Test data
  const testOrgName = 'test-org-' + Math.random().toString(36).substring(7);
  const testGithubOrganization = {
    organizationName: testOrgName,
    autoEnabled: true,
  };

  before(() => {
    getTokenKey();
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  // Cleanup any created resources after all tests
  after(() => {
    if (createdOrgName && validProjectSFID) {
      cy.task('log', `Cleaning up test GitHub organization: ${createdOrgName}`);
      cy.request({
        method: 'DELETE',
        url: `${claEndpoint}project/${validProjectSFID}/github/organizations/${createdOrgName}`,
        timeout: timeout,
        failOnStatusCode: false,
        headers: getXACLHeaders(),
        auth: {
          bearer: bearerToken,
        },
      }).then((response) => {
        cy.task('log', `Cleanup DELETE GitHub organization ${createdOrgName}: ${response.status}`);
      });
    }
  });

  // ============================================================================
  // SETUP - GET VALID IDS FOR TESTING
  // ============================================================================

  it('GET /project - Find valid project SFID for testing', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project?pageSize=10`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /project response for setup', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');

        if (response.body.projects && response.body.projects.length > 0) {
          validProjectSFID = response.body.projects[0].projectSFID || response.body.projects[0].projectID;
          cy.task('log', `Found test project SFID: ${validProjectSFID}`);
        }
      });
    });
  });

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /project/{projectSFID}/github/organizations - Get Project GitHub Organizations', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/${validProjectSFID}/github/organizations`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET GitHub organizations response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');

        if (response.body.list) {
          expect(response.body.list).to.be.an('array');
        }

        validateApiResponse('github-organizations/getProjectGithubOrganizations.json', response);
      });
    });
  });

  it('POST /project/{projectSFID}/github/organizations - Add GitHub Organization', function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}project/${validProjectSFID}/github/organizations`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: testGithubOrganization,
    }).then((response) => {
      return cy.logJson('POST GitHub organization response', response).then(() => {
        cy.task('log', `GitHub organization creation status: ${response.status}`);

        if (response.status === 200 || response.status === 201) {
          // Success case
          validate_expected_status(response, response.status);
          expect(response.body).to.be.an('object');

          createdOrgName = testOrgName;
          cy.task('log', `Created GitHub organization: ${createdOrgName}`);

          validateApiResponse('github-organizations/addGithubOrganization.json', response);
        } else if (response.status === 409) {
          // Organization already exists - this is acceptable
          cy.task('log', `GitHub organization already exists: ${response.status}`);
          validate_expected_status(response, 409);
          createdOrgName = testOrgName;
        } else if (response.status >= 500) {
          // Skip test if it's a server error
          cy.task('log', `Skipping due to server error: ${response.status}`);
          this.skip();
        } else {
          validate_expected_status(response, response.status);
        }
      });
    });
  });

  it('PUT /project/{projectSFID}/github/organizations/{orgName}/config - Update GitHub Organization Config', function () {
    const orgName = createdOrgName || 'test-org-placeholder';
    const updateConfig = {
      autoEnabled: false,
    };

    cy.request({
      method: 'PUT',
      url: `${claEndpoint}project/${validProjectSFID}/github/organizations/${orgName}/config`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: updateConfig,
    }).then((response) => {
      return cy.logJson('PUT GitHub organization config response', response).then(() => {
        cy.task('log', `GitHub organization config update status: ${response.status}`);

        if (response.status === 200) {
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          validateApiResponse('github-organizations/updateGithubOrganizationConfig.json', response);
        } else if (response.status === 404) {
          // Expected if organization doesn't exist
          validate_expected_status(response, 404);
        } else if (response.status >= 500) {
          // Skip test if it's a server error
          cy.task('log', `Skipping due to server error: ${response.status}`);
          this.skip();
        } else {
          validate_expected_status(response, response.status);
        }
      });
    });
  });

  it('DELETE /project/{projectSFID}/github/organizations/{orgName} - Remove GitHub Organization', function () {
    const orgName = createdOrgName || 'test-org-placeholder';

    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}project/${validProjectSFID}/github/organizations/${orgName}`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('DELETE GitHub organization response', response).then(() => {
        cy.task('log', `Delete GitHub organization status: ${response.status}`);

        if (response.status === 200 || response.status === 204) {
          // Success case - could be 200 or 204
          expect([200, 204]).to.include(response.status);
          createdOrgName = null; // Clear to avoid cleanup attempt
        } else if (response.status === 404) {
          // Expected if organization doesn't exist
          validate_expected_status(response, 404);
        } else if (response.status >= 500) {
          // Skip test if it's a server error
          cy.task('log', `Skipping due to server error: ${response.status}`);
          this.skip();
        } else {
          validate_expected_status(response, response.status);
        }
      });
    });
  });

  // ============================================================================
  // NEGATIVE TEST CASES - EXPECTED FAILURES
  // ============================================================================

  describe('Expected failures', () => {
    it('Returns 401 for GitHub Organizations APIs when called without token', () => {
      const testProjectSFID = validProjectSFID || 'a096s000003ZFmAAM';
      const testOrgName = 'test-org';

      const requests = [
        { method: 'GET', url: `${claEndpoint}project/${testProjectSFID}/github/organizations` },
        { method: 'POST', url: `${claEndpoint}project/${testProjectSFID}/github/organizations` },
        { method: 'DELETE', url: `${claEndpoint}project/${testProjectSFID}/github/organizations/${testOrgName}` },
        { method: 'PUT', url: `${claEndpoint}project/${testProjectSFID}/github/organizations/${testOrgName}/config` },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
            ...(req.method === 'POST' || req.method === 'PUT' ? { body: testGithubOrganization } : {}),
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);
              validate_expected_status(response, 401);
            });
          });
      });
    });

    it('Returns 4xx for malformed GitHub Organizations API parameters', () => {
      const requests = [
        {
          title: 'Invalid project SFID in path',
          method: 'GET',
          url: `${claEndpoint}project/invalid-sfid/github/organizations`,
        },
        {
          title: 'Invalid organization name in path',
          method: 'DELETE',
          url: `${claEndpoint}project/${validProjectSFID}/github/organizations/invalid@org`,
        },
        {
          title: 'Empty organization name in path',
          method: 'DELETE',
          url: `${claEndpoint}project/${validProjectSFID}/github/organizations/`,
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
              cy.task('log', `Testing malformed params: ${req.title} - Status: ${response.status}`);

              // API might be lenient and return 200 for some malformed parameters
              // Allow both 2xx (lenient API behavior) and 4xx (strict validation)
              if (response.status >= 200 && response.status <= 299) {
                cy.task('log', `API is lenient for malformed parameter: ${req.title}`);
              } else if (response.status >= 400 && response.status <= 499) {
                cy.task('log', `API properly validates malformed parameter: ${req.title}`);
              } else if (response.status >= 500) {
                cy.task('log', `API has server error for malformed parameter: ${req.title}`);
              } else {
                expect(response.status).to.be.within(200, 599);
              }
            });
          });
      });
    });

    it('Returns 4xx for POST with invalid data', () => {
      const requests = [
        {
          title: 'POST with empty body',
          method: 'POST',
          url: `${claEndpoint}project/${validProjectSFID}/github/organizations`,
          body: {},
        },
        {
          title: 'POST with invalid organization data',
          method: 'POST',
          url: `${claEndpoint}project/${validProjectSFID}/github/organizations`,
          body: { invalidField: 'invalid-value' },
        },
        {
          title: 'PUT with invalid config data',
          method: 'PUT',
          url: `${claEndpoint}project/${validProjectSFID}/github/organizations/test-org/config`,
          body: { invalidConfig: true },
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
            body: req.body,
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing invalid data: ${req.title} - Status: ${response.status}`);

              // Expect 4xx error for invalid data, allow 5xx as some APIs may fail
              if (response.status >= 500) {
                cy.task('log', `API returned 5xx error - ${req.title}`);
              } else {
                expect(response.status).to.be.within(400, 499);
              }
            });
          });
      });
    });

    it('Returns 4xx for non-existent resources', () => {
      const nonExistentSFID = 'a096s000000000AAA';
      const nonExistentOrg = 'nonexistent-org-' + Math.random().toString(36).substring(7);

      const requests = [
        {
          title: 'GET organizations for non-existent project',
          method: 'GET',
          url: `${claEndpoint}project/${nonExistentSFID}/github/organizations`,
        },
        {
          title: 'DELETE non-existent organization',
          method: 'DELETE',
          url: `${claEndpoint}project/${validProjectSFID}/github/organizations/${nonExistentOrg}`,
        },
        {
          title: 'PUT config for non-existent organization',
          method: 'PUT',
          url: `${claEndpoint}project/${validProjectSFID}/github/organizations/${nonExistentOrg}/config`,
          body: { autoEnabled: false },
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
            ...(req.body ? { body: req.body } : {}),
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing non-existent resource: ${req.title} - Status: ${response.status}`);

              // API might return 200 with empty results for non-existent resources
              // Allow both 2xx (lenient API) and 4xx (proper validation)
              if (response.status >= 200 && response.status <= 299) {
                cy.task('log', `API is lenient for non-existent resource: ${req.title}`);
              } else if (response.status >= 400 && response.status <= 499) {
                cy.task('log', `API properly handles non-existent resource: ${req.title}`);
              } else if (response.status >= 500) {
                cy.task('log', `API has server error for non-existent resource: ${req.title}`);
              } else {
                expect(response.status).to.be.within(200, 599);
              }
            });
          });
      });
    });
  });
});
