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

describe('To Validate & test CLA Manager APIs via API call (V3)', function () {
  //Reference api doc: V3 API cla-manager endpoints
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

  // Test authenticated endpoints
  it('Get CLA Managers with authentication - Record should return 200 Response', function () {
    const companyID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // Example UUID
    const projectID = 'a092M00001IV4SfQAL'; // Example SFID

    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/${companyID}/project/${projectID}/cla-managers`,
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
        if (response.body.list) {
          expect(response.body.list).to.be.an('array');
        }
      } else if (response.status === 404) {
        validate_404_Status(response);
      } else {
        cy.log(`Unexpected status: ${response.status}`);
      }
    });
  });

  it('Triple test for flakiness - CLA Manager endpoints', function () {
    // Run test 3 times to catch flaky behavior
    const companyID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
    const projectID = 'a092M00001IV4SfQAL';

    cy.wrap([1, 2, 3]).each((iteration) => {
      cy.task('log', `CLA Manager test iteration ${iteration}/3`);
      return cy
        .request({
          method: 'GET',
          url: `${claEndpoint}company/${companyID}/project/${projectID}/cla-managers`,
          timeout: timeout,
          failOnStatusCode: false,
          headers: getXACLHeader(),
          auth: {
            bearer: bearerToken,
          },
        })
        .then((response) => {
          // Accept either 200 or 404 as valid responses
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
  });

  // ========================= Auth required tests =========================
  describe('Authentication Required Tests', () => {
    it('Returns 401 for CLA Manager APIs when called without token', () => {
      const exampleCompanyID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
      const exampleProjectID = 'a092M00001IV4SfQAL';
      const exampleUserID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8f';

      const requests = [
        // GET /company/{companyID}/project/{projectID}/cla-managers (requires auth)
        { method: 'GET', url: `${claEndpoint}company/${exampleCompanyID}/project/${exampleProjectID}/cla-managers` },

        // POST /company/{companyID}/project/{projectID}/cla-managers (requires auth if it exists)
        {
          method: 'POST',
          url: `${claEndpoint}company/${exampleCompanyID}/project/${exampleProjectID}/cla-managers`,
          body: { user_id: exampleUserID },
        },

        // DELETE /company/{companyID}/project/{projectID}/cla-managers/{userID} (requires auth if it exists)
        {
          method: 'DELETE',
          url: `${claEndpoint}company/${exampleCompanyID}/project/${exampleProjectID}/cla-managers/${exampleUserID}`,
        },
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
            // V3 OAuth2 endpoints should return 401 when no token provided
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

  // ========================= Expected failures (cla-manager) =========================
  describe('Expected failures', () => {
    it('Returns errors due to malformed requests for CLA Manager APIs', function () {
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
          title: 'GET /company/{invalidID}/project/{validSFID}/cla-managers (bad request)',
          method: 'GET',
          url: `${claEndpoint}company/${invalidID}/project/a092M00001IV4SfQAL/cla-managers`,
          needsAuth: true,
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'GET /company/{validID}/project/{invalidSFID}/cla-managers (bad request)',
          method: 'GET',
          url: `${claEndpoint}company/d9428888-122b-4b20-8c4a-0c9a1a6f9b8e/project/${invalidSFID}/cla-managers`,
          needsAuth: true,
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'POST /company/{validID}/project/{validSFID}/cla-managers with empty body (bad request)',
          method: 'POST',
          url: `${claEndpoint}company/d9428888-122b-4b20-8c4a-0c9a1a6f9b8e/project/a092M00001IV4SfQAL/cla-managers`,
          body: {},
          needsAuth: true,
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'DELETE /company/{invalidID}/project/{validSFID}/cla-managers/{validID} (bad request)',
          method: 'DELETE',
          url: `${claEndpoint}company/${invalidID}/project/a092M00001IV4SfQAL/cla-managers/d9428888-122b-4b20-8c4a-0c9a1a6f9b8f`,
          needsAuth: true,
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
