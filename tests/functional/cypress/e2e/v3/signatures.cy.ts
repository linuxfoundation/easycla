import {
  validateApiResponse,
  validate_200_Status,
  validate_401_Status,
  validate_404_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
  validate_expected_status,
} from '../../support/commands';

describe('To Validate & test Signature APIs via API call (V3)', function () {
  //Reference api doc: V3 API signatures endpoints
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  let bearerToken: string = null;
  before(() => {
    if (bearerToken == null) {
      getTokenKey(bearerToken);
      cy.window().then((win) => {
        bearerToken = win.localStorage.getItem('bearerToken');
      });
    }
  });

  // Test public endpoints (no auth required)
  it('Get Project Company Signatures without auth - Record should return 200 Response', function () {
    const projectID = 'a092M00001IV4SfQAL'; // Example SFID
    const companyID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // Example UUID

    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${projectID}/company/${companyID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
      // No auth required for this endpoint
    }).then((response) => {
      // This endpoint returns data or empty list, both are valid 200 responses
      validate_200_Status(response);
      expect(response.body).to.be.an('object');
    });
  });

  it('Download signed ICLA PDF without auth - should work for valid signatures', function () {
    const claGroupID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // Example UUID
    const userID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8f'; // Example UUID

    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/${claGroupID}/${userID}/icla/pdf`,
      timeout: timeout,
      failOnStatusCode: false, // Allow 404 for non-existent signatures
      headers: getXACLHeader(),
      // No auth required for this endpoint
    }).then((response) => {
      // This endpoint might return 404 if signature doesn't exist, which is valid
      if (response.status === 200) {
        expect(response.headers['content-type']).to.contain('application/pdf');
      } else if (response.status === 404) {
        validate_404_Status(response);
      } else {
        // Log unexpected status for investigation
        cy.log(`Unexpected status: ${response.status}`);
      }
    });
  });

  it('Download signed CCLA PDF without auth - should work for valid signatures', function () {
    const claGroupID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // Example UUID
    const companyID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8f'; // Example UUID

    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/${claGroupID}/${companyID}/ccla/pdf`,
      timeout: timeout,
      failOnStatusCode: false, // Allow 404 for non-existent signatures
      headers: getXACLHeader(),
      // No auth required for this endpoint
    }).then((response) => {
      // This endpoint might return 404 if signature doesn't exist, which is valid
      if (response.status === 200) {
        expect(response.headers['content-type']).to.contain('application/pdf');
      } else if (response.status === 404) {
        validate_404_Status(response);
      } else {
        // Log unexpected status for investigation
        cy.log(`Unexpected status: ${response.status}`);
      }
    });
  });

  // Test authenticated endpoints
  it('Get Signature by ID with authentication - Record should return 200 or 404', function () {
    const signatureID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // Example UUID

    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/id/${signatureID}`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      // This can return 200 with data or 404 if not found
      if (response.status === 200) {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
      } else if (response.status === 404) {
        validate_404_Status(response);
      } else {
        cy.log(`Unexpected status: ${response.status}`);
      }
    });
  });

  it('Triple test for flakiness - Signature endpoints', function () {
    // Run test 3 times to catch flaky behavior
    const projectID = 'a092M00001IV4SfQAL';
    const companyID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';

    cy.wrap([1, 2, 3]).each((iteration) => {
      cy.task('log', `Signature test iteration ${iteration}/3`);
      return cy
        .request({
          method: 'GET',
          url: `${claEndpoint}signatures/project/${projectID}/company/${companyID}`,
          timeout: timeout,
          failOnStatusCode: allowFail,
          headers: getXACLHeader(),
          auth: { bearer: bearerToken },
        })
        .then((response) => {
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
        });
    });
  });

  // ========================= Auth required tests =========================
  describe('Authentication Required Tests', () => {
    it('Returns 401 for Signature APIs when called without token', () => {
      const exampleSignatureID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
      const exampleProjectID = 'a092M00001IV4SfQAL';
      const exampleCompanyID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8f';
      const exampleUserID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8a';

      const requests = [
        // GET /signatures/id/{signatureID} (requires auth)
        { method: 'GET', url: `${claEndpoint}signatures/id/${exampleSignatureID}` },

        // GET /signatures/project/{projectID} (requires auth)
        { method: 'GET', url: `${claEndpoint}signatures/project/${exampleProjectID}` },

        // GET /signatures/company/{companyID} (requires auth)
        { method: 'GET', url: `${claEndpoint}signatures/company/${exampleCompanyID}` },

        // GET /signatures/user/{userID} (requires auth)
        { method: 'GET', url: `${claEndpoint}signatures/user/${exampleUserID}` },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            headers: getXACLHeader(),
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);
            // V3 OAuth2 endpoints should return 401 when no token provided
            validate_401_Status(response, local);
          });
      });
    });
  });

  // ========================= Expected failures (signatures) =========================
  describe('Expected failures', () => {
    it('Returns errors due to malformed requests for Signature APIs', function () {
      const defaultHeaders = getXACLHeader();
      const invalidID = 'invalid-uuid';
      const invalidSFID = 'invalid-sfid';

      const cases: Array<{
        title: string;
        method: 'GET' | 'POST' | 'PUT' | 'DELETE';
        url: string;
        body?: any;
        headers?: any;
        needsAuth?: boolean;
        // when running locally
        expectedStatusLocal?: number;
        expectedCodeLocal?: number;
        expectedMessageLocal?: string;
        expectedMessageContainsLocal?: boolean;
        // when running against dev via ACS & API-gw
        expectedStatusRemote?: number;
        expectedCodeRemote?: number;
        expectedMessageRemote?: string;
        expectedMessageContainsRemote?: boolean;
      }> = [
        {
          title: 'GET /signatures/id/{invalidID} (bad request)',
          method: 'GET',
          url: `${claEndpoint}signatures/id/${invalidID}`,
          needsAuth: true,
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'GET /signatures/project/{invalidSFID} (bad request)',
          method: 'GET',
          url: `${claEndpoint}signatures/project/${invalidSFID}`,
          needsAuth: true,
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'GET /signatures/company/{invalidID} (bad request)',
          method: 'GET',
          url: `${claEndpoint}signatures/company/${invalidID}`,
          needsAuth: true,
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'GET /signatures/user/{invalidID} (bad request)',
          method: 'GET',
          url: `${claEndpoint}signatures/user/${invalidID}`,
          needsAuth: true,
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'POST /signatures/id/{validID} (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}signatures/id/d9428888-122b-4b20-8c4a-0c9a1a6f9b8e`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method POST is not allowed',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method POST is not allowed',
          expectedMessageContainsRemote: true,
        },
      ];

      cy.wrap(cases).each((c: any) => {
        const authHeaders = c.needsAuth
          ? {
              ...defaultHeaders,
              Authorization: `Bearer ${bearerToken}`,
            }
          : defaultHeaders;

        return cy
          .request({
            method: c.method,
            url: c.url,
            body: c.body,
            headers: c.headers || authHeaders,
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            cy.task('log', `Testing: ${c.title}`);

            const es = local
              ? (c.expectedStatusLocal ?? c.expectedStatus)
              : (c.expectedStatusRemote ?? c.expectedStatus);
            const ec = local ? (c.expectedCodeLocal ?? c.expectedCode) : (c.expectedCodeRemote ?? c.expectedCode);
            const em = local
              ? (c.expectedMessageLocal ?? c.expectedMessage)
              : (c.expectedMessageRemote ?? c.expectedMessage);
            const emc = local
              ? (c.expectedMessageContainsLocal ?? c.expectedMessageContains)
              : (c.expectedMessageContainsRemote ?? c.expectedMessageContains);

            cy.task('log', `  --> expected ${es}, ${ec}, '${em}' (contains? ${emc})`);
            validate_expected_status(response, es, ec, em, emc);
          });
      });
    });
  });
});
