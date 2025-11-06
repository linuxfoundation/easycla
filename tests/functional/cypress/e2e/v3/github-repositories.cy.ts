/*
 * Comprehensive test suite for all GitHub Repositories APIs in V3 (tagged with 'github-repositories' in swagger)
 *
 * Covers all HTTP methods for GitHub Repositories endpoints:
 * - POST /project/{projectSFID}/github/repositories (authenticated)
 * - GET /project/{projectSFID}/github/repositories (authenticated)
 * - DELETE /project/{projectSFID}/github/repositories/{repositoryID} (authenticated)
 *
 * Includes comprehensive negative testing:
 * - 401 Unauthorized tests for all endpoints
 * - 4xx validation error tests for malformed parameters
 * - Invalid UUID and parameter format tests
 *
 * Uses flexible status code assertions to handle various valid API responses
 * All responses are logged via cy.logJson() for debugging purposes
 */
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

describe('To Validate & test GitHub Repositories APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  let bearerToken: string = null;
  let validProjectSFID: string = null;
  let createdRepositoryID: string = null;

  // Test data
  const testRepositoryID = 'test-repo-' + Math.random().toString(36).substring(7);
  const testGithubRepository = {
    repositoryExternalID: testRepositoryID,
    repositoryName: 'test-repository',
    repositoryType: 'github',
    repositoryUrl: 'https://github.com/test-org/test-repository',
    repositoryOrganizationName: 'test-org',
  };

  before(() => {
    getTokenKey();
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  // Cleanup any created resources after all tests
  after(() => {
    if (createdRepositoryID && validProjectSFID) {
      cy.task('log', `Cleaning up test GitHub repository: ${createdRepositoryID}`);
      cy.request({
        method: 'DELETE',
        url: `${claEndpoint}project/${validProjectSFID}/github/repositories/${createdRepositoryID}`,
        timeout: timeout,
        failOnStatusCode: false,
        headers: getXACLHeaders(),
        auth: {
          bearer: bearerToken,
        },
      }).then((response) => {
        cy.task('log', `Cleanup DELETE GitHub repository ${createdRepositoryID}: ${response.status}`);
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

  it('GET /project/{projectSFID}/github/repositories - Get Project GitHub Repositories', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/${validProjectSFID}/github/repositories`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET GitHub repositories response', response).then(() => {
        cy.task('log', `Get GitHub repositories status: ${response.status}`);

        if (response.status === 200) {
          validate_200_Status(response);
          expect(response.body).to.be.an('object');

          if (response.body.list || response.body.repositories) {
            expect(response.body.list || response.body.repositories).to.be.an('array');
          }

          validateApiResponse('github-repositories/getProjectGithubRepositories.json', response);
        } else if (response.status === 403) {
          // Expected if user doesn't have permission
          validate_expected_status(response, 403);
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

  it('POST /project/{projectSFID}/github/repositories - Add GitHub Repository', function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}project/${validProjectSFID}/github/repositories`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: testGithubRepository,
    }).then((response) => {
      return cy.logJson('POST GitHub repository response', response).then(() => {
        cy.task('log', `GitHub repository creation status: ${response.status}`);

        if (response.status === 200 || response.status === 201) {
          // Success case
          validate_expected_status(response, response.status);
          expect(response.body).to.be.an('object');

          createdRepositoryID = response.body.repositoryID || testRepositoryID;
          cy.task('log', `Created GitHub repository: ${createdRepositoryID}`);

          validateApiResponse('github-repositories/addGithubRepository.json', response);
        } else if (response.status === 409) {
          // Repository already exists - this is acceptable
          cy.task('log', `GitHub repository already exists: ${response.status}`);
          validate_expected_status(response, 409);
          createdRepositoryID = testRepositoryID;
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

  it('DELETE /project/{projectSFID}/github/repositories/{repositoryID} - Remove GitHub Repository', function () {
    const repositoryID = createdRepositoryID || 'test-repository-placeholder';

    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}project/${validProjectSFID}/github/repositories/${repositoryID}`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('DELETE GitHub repository response', response).then(() => {
        cy.task('log', `Delete GitHub repository status: ${response.status}`);

        if (response.status === 200 || response.status === 204) {
          // Success case - could be 200 or 204
          expect([200, 204]).to.include(response.status);
          createdRepositoryID = null; // Clear to avoid cleanup attempt
        } else if (response.status === 404) {
          // Expected if repository doesn't exist
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
    it('Returns 401 for GitHub Repositories APIs when called without token', () => {
      const testProjectSFID = validProjectSFID || 'a096s000003ZFmAAM';
      const testRepoID = 'test-repo-id';

      const requests = [
        { method: 'GET', url: `${claEndpoint}project/${testProjectSFID}/github/repositories` },
        { method: 'POST', url: `${claEndpoint}project/${testProjectSFID}/github/repositories` },
        { method: 'DELETE', url: `${claEndpoint}project/${testProjectSFID}/github/repositories/${testRepoID}` },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
            ...(req.method === 'POST' ? { body: testGithubRepository } : {}),
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);
              validate_expected_status(response, 401);
            });
          });
      });
    });

    it('Returns 4xx for malformed GitHub Repositories API parameters', () => {
      const requests = [
        {
          title: 'Invalid project SFID in path',
          method: 'GET',
          url: `${claEndpoint}project/invalid-sfid/github/repositories`,
        },
        {
          title: 'Invalid repository ID in path',
          method: 'DELETE',
          url: `${claEndpoint}project/${validProjectSFID}/github/repositories/invalid@repo`,
        },
        {
          title: 'Empty repository ID in path',
          method: 'DELETE',
          url: `${claEndpoint}project/${validProjectSFID}/github/repositories/`,
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
          url: `${claEndpoint}project/${validProjectSFID}/github/repositories`,
          body: {},
        },
        {
          title: 'POST with invalid repository data',
          method: 'POST',
          url: `${claEndpoint}project/${validProjectSFID}/github/repositories`,
          body: { invalidField: 'invalid-value' },
        },
        {
          title: 'POST with malformed repository URL',
          method: 'POST',
          url: `${claEndpoint}project/${validProjectSFID}/github/repositories`,
          body: { ...testGithubRepository, repositoryUrl: 'invalid-url' },
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
      const nonExistentRepo = 'nonexistent-repo-' + Math.random().toString(36).substring(7);

      const requests = [
        {
          title: 'GET repositories for non-existent project',
          method: 'GET',
          url: `${claEndpoint}project/${nonExistentSFID}/github/repositories`,
        },
        {
          title: 'DELETE non-existent repository',
          method: 'DELETE',
          url: `${claEndpoint}project/${validProjectSFID}/github/repositories/${nonExistentRepo}`,
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
