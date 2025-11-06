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

describe('To Validate & test Template APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  let bearerToken: string = null;
  let testTemplateID: string = null; // Track existing template for tests
  let testClaGroupID: string = null; // Track existing CLA group for tests

  before(() => {
    getTokenKey();
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /template - Get Templates with authentication', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}template`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /template response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('array');

        if (response.body.length > 0) {
          // Extract test data for other tests
          const template = response.body[0];
          testTemplateID = template.ID;
          cy.task('log', `Found test template - ID: ${testTemplateID}, Name: ${template.Name}`);
        }
        validateApiResponse('template/getTemplates.json', response);
      });
    });
  });

  it('GET /project - Find a CLA group for template testing', function () {
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

        if (response.body.projects && response.body.projects.length > 0) {
          // Use the first project as test CLA group
          const project = response.body.projects[0];
          testClaGroupID = project.projectID;
          cy.task(
            'log',
            `Found test CLA group for template tests - ID: ${testClaGroupID}, Name: ${project.projectName}`,
          );
        }
      });
    });
  });

  // ============================================================================
  // TEMPLATE CREATION TESTS
  // ============================================================================

  it('POST /clagroup/{claGroupId}/template - Create CLA Group Template', function () {
    // Skip if no template ID or CLA group available
    if (!testTemplateID || !testClaGroupID) {
      cy.task('log', 'Skipping template creation - no test template or CLA group available');
      return;
    }

    const createTemplatePayload = {
      TemplateID: testTemplateID,
      MetaFields: [
        {
          name: 'Project Name',
          value: 'Test Project for Template',
          description: 'The project name',
          templateVariable: 'PROJECT_NAME',
        },
        {
          name: 'Project Legal Entity Name',
          value: 'Test Legal Entity',
          description: 'The project legal entity name',
          templateVariable: 'PROJECT_ENTITY_NAME',
        },
        {
          name: 'Project Manager Email',
          value: 'testmanager@example.com',
          description: 'Project manager email address',
          templateVariable: 'PROJECT_MANAGER_EMAIL',
        },
      ],
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}clagroup/${testClaGroupID}/template`,
      timeout: timeout,
      failOnStatusCode: false, // Allow various responses
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: createTemplatePayload,
    }).then((response) => {
      return cy.logJson('POST /clagroup/{claGroupId}/template response', response).then(() => {
        cy.task('log', 'POST template response status: ' + response.status);

        // Never expect 5xx - that would be internal server error
        expect(response.status).to.not.be.within(500, 599);

        if (response.status >= 200 && response.status <= 299) {
          // Success case
          expect(response.body).to.be.an('object');
          expect(response.body).to.have.property('individualPDFURL');
          expect(response.body).to.have.property('corporatePDFURL');

          cy.task('log', `Successfully created template for CLA group: ${testClaGroupID}`);
          validateApiResponse('template/createCLAGroupTemplate.json', response);
        } else {
          // Expected error cases (permissions, conflicts, validation errors, etc.)
          expect([400, 401, 403, 404, 409, 422]).to.include(response.status);
          cy.task('log', `Template creation returned ${response.status} - expected API behavior`);
        }
      });
    });
  });

  // ============================================================================
  // NEGATIVE TEST CASES - EXPECT 4xx ERROR RESPONSES
  // ============================================================================

  it('POST /clagroup/{claGroupId}/template - Create Template with Invalid Data (4xx)', function () {
    if (!testClaGroupID) {
      cy.task('log', 'Skipping invalid template creation test - no test CLA group available');
      return;
    }

    const invalidPayload = {
      TemplateID: '', // Invalid empty template ID
      MetaFields: [], // Empty meta fields
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}clagroup/${testClaGroupID}/template`,
      timeout: timeout,
      failOnStatusCode: false, // Use false for non-happy path (4xx expected)
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: invalidPayload,
    }).then((response) => {
      return cy.logJson('POST template invalid data response', response).then(() => {
        cy.task('log', 'POST template invalid data response status: ' + response.status);
        // Expect 4xx status codes for validation errors
        expect([400, 422]).to.include(response.status);
        if (response.body && response.body.message) {
          expect(response.body.message).to.be.a('string');
        }
      });
    });
  });

  it('POST /clagroup/{claGroupId}/template - Create Template for Non-Existent CLA Group (4xx)', function () {
    if (!testTemplateID) {
      cy.task('log', 'Skipping non-existent CLA group test - no test template available');
      return;
    }

    const nonExistentClaGroupID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // Non-existing CLA group for safe testing

    const validPayload = {
      TemplateID: testTemplateID,
      MetaFields: [
        {
          name: 'Project Name',
          value: 'Test Project',
          description: 'The project name',
          templateVariable: 'PROJECT_NAME',
        },
      ],
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}clagroup/${nonExistentClaGroupID}/template`,
      timeout: timeout,
      failOnStatusCode: false, // Use false for non-happy path (4xx expected)
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: validPayload,
    }).then((response) => {
      return cy.logJson('POST template non-existent CLA group response', response).then(() => {
        cy.task('log', 'POST template non-existent CLA group response status: ' + response.status);
        // Never expect 5xx - that would be internal server error
        expect(response.status).to.not.be.within(500, 599);
        // Allow various expected responses including auth failures
        expect([400, 401, 404, 422]).to.include(response.status);
      });
    });
  });

  describe('Expected failures', () => {
    it('Returns 401 for Template APIs when called without token', () => {
      const exampleClaGroupID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';

      const requests = [
        { method: 'GET', url: `${claEndpoint}template` },
        { method: 'POST', url: `${claEndpoint}clagroup/${exampleClaGroupID}/template` },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
            ...(req.method === 'POST' ? { body: {} } : {}),
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);
              // Never expect 5xx - that would be internal server error
              expect(response.status).to.not.be.within(500, 599);
              // For negative tests, expect 401 Unauthorized
              expect(response.status).to.eq(401);
            });
          });
      });
    });

    it('Returns 4xx for malformed Template POST requests', () => {
      const requests = [
        {
          method: 'POST',
          url: `${claEndpoint}clagroup/invalid-uuid/template`, // Invalid CLA group UUID
          body: {
            TemplateID: 'valid-template-id',
            MetaFields: [],
          },
          expectedStatuses: [400, 404, 422], // Accept multiple statuses
          expectedCodes: ['400', '404', '602', '604'],
        },
        {
          method: 'POST',
          url: `${claEndpoint}clagroup/d9428888-122b-4b20-8c4a-0c9a1a6f9b8e/template`,
          body: {}, // Empty body should trigger validation error
          expectedStatuses: [400, 404, 422], // Accept multiple statuses
          expectedCodes: ['400', '404', '602', '604'],
        },
        {
          method: 'POST',
          url: `${claEndpoint}clagroup/d9428888-122b-4b20-8c4a-0c9a1a6f9b8e/template`,
          body: {
            TemplateID: '', // Invalid empty template ID
            MetaFields: 'invalid-format', // Invalid format for MetaFields
          },
          expectedStatuses: [400, 404, 422], // Accept multiple statuses
          expectedCodes: ['400', '404', '602', '604'],
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
              cy.task('log', `Testing malformed ${req.method} ${req.url}`, req.body);
              // Never expect 5xx - that would be internal server error
              expect(response.status).to.not.be.within(500, 599);
              expect(req.expectedStatuses).to.include(response.status);
              if (response.body && (response.body.code || response.body.Code)) {
                expect(req.expectedCodes).to.include(String(response.body.code ?? response.body.Code));
              }
            });
          });
      });
    });

    it('Returns 4xx for invalid Template path parameters', () => {
      const requests = [
        {
          method: 'POST',
          url: `${claEndpoint}clagroup//template`, // Empty CLA group ID in path
          body: { TemplateID: 'test', MetaFields: [] },
          expectedStatuses: [400, 404, 405], // Accept multiple statuses including method not allowed
          expectedCodes: ['400', '404', '405'],
        },
        {
          method: 'POST',
          url: `${claEndpoint}clagroup/not-a-uuid/template`, // Invalid UUID format
          body: { TemplateID: 'test', MetaFields: [] },
          expectedStatuses: [400, 404, 422], // Accept multiple statuses
          expectedCodes: ['400', '404', '602', '604'],
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
              cy.task('log', `Testing invalid path param ${req.method} ${req.url}`);
              // Never expect 5xx - that would be internal server error
              expect(response.status).to.not.be.within(500, 599);
              expect(req.expectedStatuses).to.include(response.status);
              if (req.expectedCodes !== null && response.body && (response.body.code || response.body.Code)) {
                expect(req.expectedCodes).to.include(String(response.body.code ?? response.body.Code));
              }
            });
          });
      });
    });
  });
});
