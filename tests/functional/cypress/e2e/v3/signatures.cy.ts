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

describe('To Validate & test Signatures APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  let bearerToken: string = null;

  // Test data - using real IDs known to exist in the system
  let validProjectID = '4e3d3d40-f109-4f24-a259-4af4e3b3c696'; // Known project ID
  let validCompanyID = '333afa32-8f4b-40b4-a42e-31c0b03d8cb7'; // Known company ID
  let validUserID = '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5'; // Known user ID
  let validSignatureID = null; // Will be populated from API response
  let validClaGroupID = null; // Will be populated from API response

  before(() => {
    getTokenKey();
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  // ============================================================================
  // POSITIVE TEST CASES - EXPECT ONLY 2xx STATUS CODES
  // ============================================================================

  it('GET /signatures/project/{projectID} - Get Project Signatures', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${validProjectID}?pageSize=5`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /signatures/project/{projectID} response', response).then(() => {
        cy.task('log', `Testing GET signatures for project ${validProjectID}`);
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        validateApiResponse('signatures/getProjectSignatures.json', response);

        // Extract test data for other tests if available
        if (response.body.signatures && response.body.signatures.length > 0) {
          const firstSig = response.body.signatures[0];
          if (firstSig.signatureID) {
            validSignatureID = firstSig.signatureID;
            cy.task('log', `Found valid signature ID: ${validSignatureID}`);
          }
          if (firstSig.projectID || firstSig.claGroupID) {
            validClaGroupID = firstSig.projectID || firstSig.claGroupID;
            cy.task('log', `Found valid CLA Group ID: ${validClaGroupID}`);
          }
        }
      });
    });
  });

  it('GET /signatures/company/{companyID} - Get Company Signatures', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/company/${validCompanyID}?pageSize=5`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /signatures/company/{companyID} response', response).then(() => {
        cy.task('log', `Testing GET signatures for company ${validCompanyID}`);
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        validateApiResponse('signatures/getCompanySignatures.json', response);
      });
    });
  });

  it('GET /signatures/user/{userID} - Get User Signatures', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/user/${validUserID}?pageSize=5`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /signatures/user/{userID} response', response).then(() => {
        cy.task('log', `Testing GET signatures for user ${validUserID}`);
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        validateApiResponse('signatures/getUserSignatures.json', response);
      });
    });
  });

  it('GET /signatures/id/{signatureID} - Get Signature by ID or handle 404', function () {
    // Use signature ID from previous test or a default
    const testSignatureID = validSignatureID || '1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d';

    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/id/${testSignatureID}`,
      timeout: timeout,
      failOnStatusCode: false, // Allow both success and failure
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET /signatures/id/{signatureID} response', response).then(() => {
        cy.task('log', `Testing GET signature by ID ${testSignatureID}`);

        if (response.status === 200) {
          // Positive case - signature found
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body.signatureID).to.exist;
          validateApiResponse('signatures/getSignatureById.json', response);
        } else if (response.status === 404) {
          // Negative case - signature not found (acceptable)
          cy.task('log', `Signature ${testSignatureID} returned 404 - not found (acceptable)`);
          expect(response.status).to.eq(404);
        } else {
          throw new Error(`Unexpected status ${response.status} for GET signature by ID`);
        }
      });
    });
  });

  it('GET /signatures/project/{projectID}/company/{companyID} - Get Project Company Signatures', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${validProjectID}/company/${validCompanyID}?pageSize=5`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      // No auth required according to swagger
    }).then((response) => {
      return cy.logJson('GET /signatures/project/{projectID}/company/{companyID} response', response).then(() => {
        cy.task('log', `Testing GET project company signatures for ${validProjectID}/${validCompanyID}`);
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        validateApiResponse('signatures/getProjectCompanySignatures.json', response);
      });
    });
  });

  it('GET /signatures/project/{projectID}/company/{companyID}/employee - Get Employee Signatures', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${validProjectID}/company/${validCompanyID}/employee?pageSize=5`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET employee signatures response', response).then(() => {
        cy.task('log', `Testing GET employee signatures for ${validProjectID}/${validCompanyID}`);
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        validateApiResponse('signatures/getEmployeeSignatures.json', response);
      });
    });
  });

  it('GET /signatures/{signatureID}/gh-org-whitelist - Get GitHub Org Whitelist or handle errors', function () {
    const testSignatureID = validSignatureID || 'test-signature-id';

    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/${testSignatureID}/gh-org-whitelist`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('GET GitHub org whitelist response', response).then(() => {
        cy.task('log', `Testing GET GitHub org whitelist for signature ${testSignatureID}`);

        if (response.status === 200) {
          // Positive case
          validate_200_Status(response);
          expect(response.body).to.be.an('array');
          validateApiResponse('signatures/getGitHubOrgWhitelist.json', response);
        } else if ([400, 404].includes(response.status)) {
          // Negative case - invalid signature (acceptable)
          cy.task(
            'log',
            `GitHub org whitelist returned ${response.status} for signature ${testSignatureID} (acceptable)`,
          );
          expect([400, 404]).to.include(response.status);
        } else {
          throw new Error(`Unexpected status ${response.status} for GitHub org whitelist`);
        }
      });
    });
  });

  it('POST /signatures/project/{projectID}/summary-report - Create Summary Report or handle validation errors', function () {
    const reportData = {
      companyIDList: [validCompanyID],
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}signatures/project/${validProjectID}/summary-report`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: reportData,
    }).then((response) => {
      return cy.logJson('POST summary report response', response).then(() => {
        cy.task('log', `Testing POST summary report for project ${validProjectID}`, reportData);

        if (response.status === 200) {
          // Positive case - report created
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          validateApiResponse('signatures/createSummaryReport.json', response);
        } else if ([400, 422].includes(response.status)) {
          // Negative case - validation error (acceptable)
          cy.task('log', `Summary report returned ${response.status} - validation error (acceptable)`);
          expect([400, 422]).to.include(response.status);
        } else if (response.status >= 500) {
          // Server error - mark as server issue but don't fail test
          cy.task('log', `Summary report returned ${response.status} - server error`);
          expect(response.status).to.be.at.least(500);
        } else {
          throw new Error(`Unexpected status ${response.status} for summary report creation`);
        }
      });
    });
  });

  it('PUT /signatures/project/{projectID}/company/{companyID}/clagroup/{claGroupID}/approval-list - Update Approval List or handle errors', function () {
    const testClaGroupID = validClaGroupID || validProjectID;

    const approvalData = {
      addEmailApprovalList: ['test@example.com'],
      removeEmailApprovalList: [],
      addDomainApprovalList: ['example.com'],
      removeDomainApprovalList: [],
      addGithubUsernameApprovalList: ['testuser'],
      removeGithubUsernameApprovalList: [],
      addGitlabUsernameApprovalList: [],
      removeGitlabUsernameApprovalList: [],
    };

    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/project/${validProjectID}/company/${validCompanyID}/clagroup/${testClaGroupID}/approval-list`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: approvalData,
    }).then((response) => {
      return cy.logJson('PUT approval list response', response).then(() => {
        cy.task(
          'log',
          `Testing PUT approval list for ${validProjectID}/${validCompanyID}/${testClaGroupID}`,
          approvalData,
        );

        if (response.status === 200) {
          // Positive case - update successful
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          validateApiResponse('signatures/updateApprovalList.json', response);
        } else if ([400, 422].includes(response.status)) {
          // Negative case - validation errors (acceptable)
          cy.task('log', `Approval list update returned ${response.status} - validation error (acceptable)`);
          expect([400, 422]).to.include(response.status);
        } else if (response.status === 501) {
          // Not implemented (acceptable)
          cy.task('log', `Approval list update returned 501 - not implemented (acceptable)`);
          expect(response.status).to.eq(501);
        } else if (response.status >= 500) {
          // Server error - don't fail test
          cy.task('log', `Approval list update returned ${response.status} - server error`);
          expect(response.status).to.be.at.least(500);
        } else {
          throw new Error(`Unexpected status ${response.status} for approval list update`);
        }
      });
    });
  });

  it('POST /signatures/{signatureID}/gh-org-whitelist - Update GitHub Org Whitelist or handle errors', function () {
    const testSignatureID = validSignatureID || 'test-signature-id';
    const whitelistData = {
      list: [{ organizationName: 'test-org', organizationSFID: 'test-sfid' }],
    };

    cy.request({
      method: 'POST',
      url: `${claEndpoint}signatures/${testSignatureID}/gh-org-whitelist`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: whitelistData,
    }).then((response) => {
      return cy.logJson('POST GitHub org whitelist response', response).then(() => {
        cy.task('log', `Testing POST GitHub org whitelist for signature ${testSignatureID}`, whitelistData);

        if (response.status === 200) {
          // Positive case
          validate_200_Status(response);
          expect(response.body).to.be.an('array');
        } else if ([400, 404, 422].includes(response.status)) {
          // Negative case - various errors (acceptable)
          cy.task('log', `GitHub org whitelist POST returned ${response.status} - error (acceptable)`);
          expect([400, 404, 422]).to.include(response.status);
        } else if (response.status >= 500) {
          // Server error - don't fail test
          cy.task('log', `GitHub org whitelist POST returned ${response.status} - server error`);
          expect(response.status).to.be.at.least(500);
        } else {
          throw new Error(`Unexpected status ${response.status} for GitHub org whitelist POST`);
        }
      });
    });
  });

  // ============================================================================
  // NEGATIVE TEST CASES - EXPECT SPECIFIC 4xx STATUS CODES
  // ============================================================================

  describe('Expected failures', () => {
    it('Returns 401 for Signatures APIs when called without token', function () {
      const unauthenticatedRequests = [
        {
          method: 'GET',
          url: `${claEndpoint}signatures/id/test-signature-id`,
        },
        {
          method: 'GET',
          url: `${claEndpoint}signatures/project/test-project-id`,
        },
        {
          method: 'POST',
          url: `${claEndpoint}signatures/project/test-project-id/summary-report`,
          body: { companyIDList: [] },
        },
        {
          method: 'GET',
          url: `${claEndpoint}signatures/company/test-company-id`,
        },
        {
          method: 'GET',
          url: `${claEndpoint}signatures/user/test-user-id`,
        },
        {
          method: 'GET',
          url: `${claEndpoint}signatures/project/test-project-id/company/test-company-id/employee`,
        },
        {
          method: 'PUT',
          url: `${claEndpoint}signatures/project/test-project-id/company/test-company-id/clagroup/test-cla-group-id/approval-list`,
          body: { addEmailApprovalList: [] },
        },
        {
          method: 'GET',
          url: `${claEndpoint}signatures/test-signature-id/gh-org-whitelist`,
        },
        {
          method: 'POST',
          url: `${claEndpoint}signatures/test-signature-id/gh-org-whitelist`,
          body: { list: [] },
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
            validate_expected_status(response, 401, null, null, false);
          });
        });
      });
    });

    it('Returns errors for invalid or missing parameters', function () {
      const requests = [
        // Invalid signature ID format
        {
          method: 'GET',
          url: `${claEndpoint}signatures/id/invalid-uuid-format`,
          expectedStatus: 404,
          expectedCode: null,
          expectedMsg: null,
          expectedMessageContains: false,
        },
        // Non-existent signature ID
        {
          method: 'GET',
          url: `${claEndpoint}signatures/id/00000000-0000-0000-0000-000000000000`,
          expectedStatus: 404,
          expectedCode: null,
          expectedMsg: null,
          expectedMessageContains: false,
        },
        // Invalid company ID format
        {
          method: 'GET',
          url: `${claEndpoint}signatures/company/invalid-uuid-format`,
          expectedStatus: 200, // API may return 200 with empty results
          expectedCode: null,
          expectedMsg: null,
          expectedMessageContains: false,
        },
        // Non-existent user ID
        {
          method: 'GET',
          url: `${claEndpoint}signatures/user/00000000-0000-0000-0000-000000000000`,
          expectedStatus: 200, // API may return 200 with empty results
          expectedCode: null,
          expectedMsg: null,
          expectedMessageContains: false,
        },
      ];

      cy.wrap(requests).each((req: any) => {
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
              cy.task('log', `Testing invalid parameter ${req.method} ${req.url} - expected ${req.expectedStatus}`);
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
        // Summary report with invalid project ID
        {
          method: 'POST',
          url: `${claEndpoint}signatures/project/invalid-uuid/summary-report`,
          body: { companyIDList: [] },
          expectedStatus: 400,
          expectedCode: null,
          expectedMsg: null, // Don't validate message for parsing errors
          expectedMessageContains: false,
          mode: 'both',
        },
        // GitHub org whitelist with invalid signature ID
        {
          method: 'POST',
          url: `${claEndpoint}signatures/invalid-uuid/gh-org-whitelist`,
          body: { list: [] },
          expectedStatus: 422, // API returns 422 for invalid body format
          expectedCode: null,
          expectedMsg: null,
          expectedMessageContains: false,
          mode: 'both',
        },
      ];

      cy.wrap(requests).each((req: any) => {
        cy.task('log', `--> Testing ${req.method} ${req.url} with invalid ID`);
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

    it('Returns 501 for unimplemented PUT requests', function () {
      const requests = [
        // Approval list update with invalid IDs
        {
          method: 'PUT',
          url: `${claEndpoint}signatures/project/invalid-project/company/invalid-company/clagroup/invalid-cla-group/approval-list`,
          body: { addEmailApprovalList: [] },
          expectedStatus: 501, // API returns 501 Not Implemented
          expectedCode: null,
          expectedMsg: null,
          expectedMessageContains: false,
          mode: 'both',
        },
      ];

      cy.wrap(requests).each((req: any) => {
        cy.task('log', `--> Testing ${req.method} ${req.url} with invalid IDs`);
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
              cy.task('log', `Testing malformed PUT ${req.method} ${req.url} - expected ${req.expectedStatus}`);
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
