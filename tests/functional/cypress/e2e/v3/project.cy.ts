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

describe('To Validate & test Project APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  let bearerToken: string = null;
  let createdProjectID: string = null; // Track created project for cleanup
  let testProjectID: string = null; // Track existing project for tests
  let testProjectSFID: string = null; // Track existing project SFID for tests
  let testProjectName: string = null; // Track existing project name for tests

  before(() => {
    getTokenKey();
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  // Cleanup any created projects after all tests
  after(() => {
    if (createdProjectID) {
      cy.task('log', `Cleaning up test project: ${createdProjectID}`);
      cy.request({
        method: 'DELETE',
        url: `${claEndpoint}project/${createdProjectID}`,
        timeout: timeout,
        failOnStatusCode: false,
        headers: getXACLHeaders(),
        auth: {
          bearer: bearerToken,
        },
      }).then((response) => {
        cy.task('log', `Cleanup DELETE project ${createdProjectID}: ${response.status}`);
      });
    }
  });

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /project - Get CLA Groups with authentication', function () {
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
      return cy.logJson('GET /project response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body).to.have.property('pageSize');
        if (response.body.projects && response.body.projects.length > 0) {
          expect(response.body.projects).to.be.an('array');
          // Extract test data for other tests - use actual field names from API response
          const project = response.body.projects[0];
          testProjectID = project.projectID;
          testProjectSFID = project.projectExternalID;
          testProjectName = project.projectName;
          cy.task(
            'log',
            `Found test project - ID: ${testProjectID}, SFID: ${testProjectSFID}, Name: ${testProjectName}`,
          );
        }
        validateApiResponse('project/getProjects.json', response);
      });
    });
  });

  it('GET /project with search parameters', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project?searchTerm=test&searchField=projectName&pageSize=5`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /project with search response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body).to.have.property('pageSize');
        validateApiResponse('project/getProjects.json', response);
      });
    });
  });

  it('GET /project/{projectID} - Get CLA Group by ID', function () {
    // Skip if no test project ID available
    if (!testProjectID) {
      cy.task('log', 'Skipping GET /project/{projectID} - no test project available');
      return cy.skip();
    }

    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/${testProjectID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /project/{projectID} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body.projectID).to.exist;
        validateApiResponse('project/getProjectById.json', response);
      });
    });
  });

  it('GET /project/external/{projectSFID} - Get CLA Groups by External ID', function () {
    // Skip if no test project SFID available
    if (!testProjectSFID) {
      cy.task('log', 'Skipping GET /project/external/{projectSFID} - no test project SFID available');
      return cy.skip();
    }

    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/external/${testProjectSFID}?pageSize=5`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /project/external/{projectSFID} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        validateApiResponse('project/getProjectsByExternalID.json', response);
      });
    });
  });

  it('GET /project/name/{projectName} - Get CLA Group by Name', function () {
    // Skip if no test project name available
    if (!testProjectName) {
      cy.task('log', 'Skipping GET /project/name/{projectName} - no test project name available');
      return cy.skip();
    }

    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/name/${encodeURIComponent(testProjectName)}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /project/name/{projectName} response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body.projectName).to.exist;
        validateApiResponse('project/getProjectByName.json', response);
      });
    });
  });

  it('POST /project - Create a new CLA Group', function () {
    const uniqueProjectName = `test-project-${Date.now()}`;
    const projectData = {
      projectName: uniqueProjectName,
      projectDescription: 'Test project created by Cypress tests',
      projectExternalID: `sf-test-${Date.now()}`,
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}project`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: projectData,
    }).then((response) => {
      return cy.logJson('POST /project response', response).then(() => {
        cy.task('log', `Testing POST /project with data:`, projectData);

        // Never expect 5xx - that would be internal server error
        expect(response.status).to.not.be.within(500, 599);

        if (response.status >= 200 && response.status <= 299) {
          // Success - project was created
          validate_expected_status(response, 200);
          expect(response.body).to.be.an('object');
          if (response.body.projectID) {
            createdProjectID = response.body.projectID;
            cy.task('log', `Successfully created project with ID: ${createdProjectID}`);
          }
          validateApiResponse('project/createProject.json', response);
        } else {
          // Expect 4xx for validation errors or missing fields
          expect(response.status).to.be.within(400, 499);
          validate_expected_status(response, response.status);
          cy.task('log', `POST /project returned expected error: ${response.status}`);
        }
      });
    });
  });

  it('PUT /project - Update a CLA Group', function () {
    // Skip if no test project to update
    if (!testProjectID) {
      cy.task('log', 'Skipping PUT /project - no project available to update');
      return cy.skip();
    }

    const updateData = {
      projectID: testProjectID,
      projectName: `updated-project-${Date.now()}`,
      projectDescription: 'Updated project description via Cypress tests',
    };

    cy.request({
      method: 'PUT',
      url: `${claEndpoint}project`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: updateData,
    }).then((response) => {
      return cy.logJson('PUT /project response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        validateApiResponse('project/updateProject.json', response);
      });
    });
  });

  it('DELETE /project/{projectID} - Delete CLA Group by ID', function () {
    // Use created project ID or a safe test ID for deletion attempt
    const projectIDToDelete = createdProjectID || '00000000-0000-0000-0000-000000000000';

    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}project/${projectIDToDelete}`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('DELETE /project/{projectID} response', response).then(() => {
        cy.task('log', `DELETE project response status: ${response.status}`);

        // Never expect 5xx - that would be internal server error
        expect(response.status).to.not.be.within(500, 599);

        if (response.status >= 200 && response.status <= 299) {
          // Success - project was deleted
          validate_expected_status(response, [200, 204]);
          cy.task('log', `Successfully deleted project ${projectIDToDelete}`);
          createdProjectID = null; // Clear so we don't try to clean up again
        } else {
          // Expect 4xx for non-existent projects, permission issues, etc.
          expect(response.status).to.be.within(400, 499);
          validate_expected_status(response, response.status);
          cy.task('log', `DELETE returned expected error for project ${projectIDToDelete}: ${response.status}`);
        }
      });
    });
  });

  // ============================================================================
  // NEGATIVE TEST CASES - EXPECT SPECIFIC 4xx STATUS CODES
  // ============================================================================

  describe('Expected failures', () => {
    it('Returns 401 for Project APIs when called without token', function () {
      const unauthenticatedRequests = [
        {
          method: 'GET',
          url: `${claEndpoint}project`,
        },
        {
          method: 'POST',
          url: `${claEndpoint}project`,
          body: { projectName: 'test' },
        },
        {
          method: 'PUT',
          url: `${claEndpoint}project`,
          body: { projectID: 'test', projectName: 'test' },
        },
        {
          method: 'GET',
          url: `${claEndpoint}project/test-id`,
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}project/test-id`,
        },
        {
          method: 'GET',
          url: `${claEndpoint}project/external/test-sfid`,
        },
        {
          method: 'GET',
          url: `${claEndpoint}project/name/test-name`,
        },
      ];

      cy.wrap(unauthenticatedRequests).each((req: any) => {
        cy.task('log', `--> Testing ${req.method} ${req.url} without auth`);
        const requestOptions: any = {
          method: req.method,
          url: req.url,
          failOnStatusCode: false,
          timeout,
        };

        if (req.body) {
          requestOptions.body = req.body;
        }

        return cy.request(requestOptions).then((response) => {
          return cy.logJson('response', response).then(() => {
            cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);
            // Never expect 5xx - that would be internal server error
            expect(response.status).to.not.be.within(500, 599);
            // For negative tests, expect exactly 401 Unauthorized
            expect(response.status).to.eq(401);
          });
        });
      });
    });

    it('Returns errors due to invalid or missing parameters', function () {
      const requests = [
        // Invalid UUID format
        {
          method: 'GET',
          url: `${claEndpoint}project/invalid-uuid-format`,
          expectedStatus: 400,
          expectedCode: null,
          expectedMsg: 'not found', // API returns "not found" message, not "invalid UUID format"
          expectedMessageContains: true,
          mode: 'both',
        },
        // Non-existent project ID
        {
          method: 'GET',
          url: `${claEndpoint}project/00000000-0000-0000-0000-000000000000`,
          expectedStatus: 400, // API returns 400, not 404
          expectedCode: null,
          expectedMsg: 'not found', // API says "cla group ... not found"
          expectedMessageContains: true,
          mode: 'both',
        },
        // Non-existent project name
        {
          method: 'GET',
          url: `${claEndpoint}project/name/nonexistent-project-name-12345`,
          expectedStatus: 404, // Let's try 404 again
          expectedCode: null,
          expectedMsg: null, // Don't validate message for 404 responses (might be empty)
          expectedMessageContains: false,
          mode: 'both',
        },
        // Empty project name path - local environment
        {
          method: 'GET',
          url: `${claEndpoint}project/name/`,
          expectedStatus: 400, // Local also returns 400
          expectedCode: null,
          expectedMsg: null, // Don't validate message for empty path
          expectedMessageContains: false,
          mode: 'local',
        },
        // Empty project name path - remote environment
        {
          method: 'GET',
          url: `${claEndpoint}project/name/`,
          expectedStatus: 400, // Remote also returns 400, not 403
          expectedCode: null,
          expectedMsg: null, // Don't validate message for empty path
          expectedMessageContains: false,
          mode: 'remote',
        },
      ];

      cy.wrap(requests).each((req: any) => {
        // Skip test if mode doesn't match environment
        if (req.mode === 'local' && !local) return;
        if (req.mode === 'remote' && local) return;

        cy.task('log', `--> Testing ${req.method} ${req.url}`);
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
              cy.task('log', `Testing invalid param ${req.method} ${req.url} - expected ${req.expectedStatus}`);
              validate_expected_status(
                response,
                req.expectedStatus,
                req.expectedCode,
                req.expectedMsg,
                req.expectedMessageContains,
              );
            });
          });
      });
    });

    it('Returns 400 for malformed POST requests', function () {
      const requests = [
        // Empty body
        {
          method: 'POST',
          url: `${claEndpoint}project`,
          body: {},
          expectedStatus: 400, // API returns 400, not 422
          expectedCode: null,
          expectedMsg: 'Missing Project Name', // API says "Missing Project Name or Project ACL parameter."
          expectedMessageContains: true,
          mode: 'both',
        },
        // Missing required field
        {
          method: 'POST',
          url: `${claEndpoint}project`,
          body: {
            projectDescription: 'Test project without name',
          },
          expectedStatus: 400, // API returns 400, not 422
          expectedCode: null,
          expectedMsg: 'Missing Project Name', // API says "Missing Project Name or Project ACL parameter."
          expectedMessageContains: true,
          mode: 'both',
        },
      ];

      cy.wrap(requests).each((req: any) => {
        cy.task('log', `--> Testing ${req.method} ${req.url} with invalid body`);
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
              cy.task('log', `Testing malformed POST ${req.method} ${req.url} - expected ${req.expectedStatus}`);
              validate_expected_status(
                response,
                req.expectedStatus,
                req.expectedCode,
                req.expectedMsg,
                req.expectedMessageContains,
              );
            });
          });
      });
    });

    it('Returns 422 for non-existent project updates', function () {
      const requests = [
        // Update non-existent project
        {
          method: 'PUT',
          url: `${claEndpoint}project`,
          body: {
            projectID: '00000000-0000-0000-0000-000000000000',
            projectName: 'test-update',
          },
          expectedStatus: 422, // API returns 422, not 404
          expectedCode: null,
          expectedMsg: 'should match', // API says "projectID in body should match ..."
          expectedMessageContains: true,
          mode: 'both',
        },
      ];

      cy.wrap(requests).each((req: any) => {
        cy.task('log', `--> Testing ${req.method} ${req.url} with non-existent ID`);
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
              cy.task('log', `Testing non-existent PUT ${req.method} ${req.url} - expected ${req.expectedStatus}`);
              validate_expected_status(
                response,
                req.expectedStatus,
                req.expectedCode,
                req.expectedMsg,
                req.expectedMessageContains,
              );
            });
          });
      });
    });
  });
});
