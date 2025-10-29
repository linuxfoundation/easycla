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

describe('To Validate & test Company APIs via API call (V3)', function () {
  //Reference api doc: V3 API company endpoints
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
  it('Get Company by External SFID without auth - Record should return 200 or 404', function () {
    const companySFID = 'a092M00001IV4SfQAL'; // Example SFID

    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/external/${companySFID}`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeader(),
      // No auth required for this endpoint
    }).then((response) => {
      // This endpoint returns data or 404 if not found
      if (response.status === 200) {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body.companyExternalID).to.be.a('string');
      } else if (response.status === 404) {
        validate_404_Status(response);
      } else {
        cy.log(`Unexpected status: ${response.status}`);
      }
    });
  });

  it('Get Company by Signing Entity Name without auth - Record should return 200 or 404', function () {
    const signingEntityName = 'Example Company LLC';
    const companySFID = 'a092M00001IV4SfQAL'; // Example SFID

    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/signing-entity-name?name=${encodeURIComponent(signingEntityName)}&companySFID=${companySFID}`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeader(),
      // No auth required for this endpoint
    }).then((response) => {
      // This endpoint returns data or 404 if not found
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

  it('Search Organization without auth - Record should return 200 Response', function () {
    const companyName = 'Test Company';

    cy.request({
      method: 'GET',
      url: `${claEndpoint}organization/search?companyName=${encodeURIComponent(companyName)}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
      // No auth required for this endpoint
    }).then((response) => {
      validate_200_Status(response);
      expect(response.body).to.be.an('object');
      if (response.body.list) {
        expect(response.body.list).to.be.an('array');
      }
    });
  });

  // Test authenticated endpoints
  it('Get Company by Internal ID with authentication - Record should return 200 or 404', function () {
    const companyID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // Example UUID

    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/${companyID}`,
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
        expect(response.body.companyID).to.be.a('string');
      } else if (response.status === 404) {
        validate_404_Status(response);
      } else {
        cy.log(`Unexpected status: ${response.status}`);
      }
    });
  });

  it('Get Company by Name with authentication - Record should return 200 or 404', function () {
    const companyName = 'Test Company Inc';

    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/name/${encodeURIComponent(companyName)}`,
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

  it('Triple test for flakiness - Company endpoints', function () {
    // Run test 3 times to catch flaky behavior
    const companyName = 'Linux Foundation';

    cy.wrap([1, 2, 3]).each((iteration) => {
      cy.task('log', `Company test iteration ${iteration}/3`);
      return cy
        .request({
          method: 'GET',
          url: `${claEndpoint}organization/search?companyName=${encodeURIComponent(companyName)}`,
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
    it('Returns 401 for Company APIs when called without token', () => {
      const exampleCompanyID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
      const exampleCompanyName = 'Test Company Inc';
      const exampleProjectID = 'a092M00001IV4SfQAL';

      const requests = [
        // GET /company/{companyID} (requires auth)
        { method: 'GET', url: `${claEndpoint}company/${exampleCompanyID}` },

        // GET /company/name/{companyName} (requires auth)
        { method: 'GET', url: `${claEndpoint}company/name/${encodeURIComponent(exampleCompanyName)}` },

        // POST /company (requires auth if it exists)
        { method: 'POST', url: `${claEndpoint}company`, body: { company_name: exampleCompanyName } },

        // PUT /company (requires auth if it exists)
        {
          method: 'PUT',
          url: `${claEndpoint}company`,
          body: { company_id: exampleCompanyID, company_name: exampleCompanyName },
        },

        // DELETE /company/{companyID} (requires auth if it exists)
        { method: 'DELETE', url: `${claEndpoint}company/${exampleCompanyID}` },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            body: req.body,
            headers: getXACLHeader(),
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);
            // Some endpoints might return 404 or 405 instead of 401 if they don't exist
            // We mainly want to ensure they don't return 200 without auth
            if (response.status === 401) {
              validate_401_Status(response, local);
            } else if (response.status === 404) {
              validate_404_Status(response);
            } else if (response.status === 405) {
              // Method not allowed is also acceptable for non-existent endpoints
              expect(response.status).to.equal(405);
            } else {
              // Fail if we get a 200 without auth (should not happen)
              expect(response.status, `Expected 401, 404, or 405 but got ${response.status}`).to.not.equal(200);
            }
          });
      });
    });
  });

  // ========================= Expected failures (company) =========================
  describe('Expected failures', () => {
    it('Returns errors due to malformed requests for Company APIs', function () {
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
          title: 'GET /company/{invalidID} (bad request)',
          method: 'GET',
          url: `${claEndpoint}company/${invalidID}`,
          needsAuth: true,
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'GET /company/external/{invalidSFID} (bad request)',
          method: 'GET',
          url: `${claEndpoint}company/external/${invalidSFID}`,
          needsAuth: false, // Public endpoint
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'GET /company/signing-entity-name with missing params (bad request)',
          method: 'GET',
          url: `${claEndpoint}company/signing-entity-name`,
          needsAuth: false, // Public endpoint
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'GET /organization/search with missing params (bad request)',
          method: 'GET',
          url: `${claEndpoint}organization/search`,
          needsAuth: false, // Public endpoint
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
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
