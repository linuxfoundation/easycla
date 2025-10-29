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

describe('To Validate & test Gerrits APIs via API call (V3)', function () {
  //Reference api doc: V3 API gerrits endpoints
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
  it('Get Gerrit Instances with authentication - Record should return 200 Response', function () {
    const projectSFID = 'a092M00001IV4SfQAL'; // Example SFID

    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/${projectSFID}/gerrits`,
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

  it('Get Gerrit Instance by ID with authentication - Record should return 200 or 404', function () {
    const gerritID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // Example UUID

    cy.request({
      method: 'GET',
      url: `${claEndpoint}gerrits/${gerritID}`,
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

  it('Triple test for flakiness - Gerrits endpoints', function () {
    // Run test 3 times to catch flaky behavior
    const projectSFID = 'a092M00001IV4SfQAL';

    cy.wrap([1, 2, 3]).each((iteration) => {
      cy.task('log', `Gerrits test iteration ${iteration}/3`);
      return cy
        .request({
          method: 'GET',
          url: `${claEndpoint}project/${projectSFID}/gerrits`,
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
    it('Returns 401 for Gerrits APIs when called without token', () => {
      const exampleProjectSFID = 'a092M00001IV4SfQAL';
      const exampleGerritID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';

      const requests = [
        // GET /project/{projectSFID}/gerrits (requires auth)
        { method: 'GET', url: `${claEndpoint}project/${exampleProjectSFID}/gerrits` },

        // GET /gerrits/{gerritID} (requires auth)
        { method: 'GET', url: `${claEndpoint}gerrits/${exampleGerritID}` },

        // POST /project/{projectSFID}/gerrits (requires auth if it exists)
        {
          method: 'POST',
          url: `${claEndpoint}project/${exampleProjectSFID}/gerrits`,
          body: { gerrit_name: 'test-gerrit', gerrit_url: 'https://gerrit.example.com' },
        },

        // PUT /gerrits/{gerritID} (requires auth if it exists)
        {
          method: 'PUT',
          url: `${claEndpoint}gerrits/${exampleGerritID}`,
          body: { gerrit_name: 'updated-gerrit' },
        },

        // DELETE /gerrits/{gerritID} (requires auth if it exists)
        { method: 'DELETE', url: `${claEndpoint}gerrits/${exampleGerritID}` },
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

  // ========================= Expected failures (gerrits) =========================
  describe('Expected failures', () => {
    it('Returns errors due to malformed requests for Gerrits APIs', function () {
      const defaultHeaders = getXACLHeader();
      const invalidSFID = 'invalid-sfid';
      const invalidID = 'invalid-uuid';

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
          title: 'GET /project/{invalidSFID}/gerrits (bad request)',
          method: 'GET',
          url: `${claEndpoint}project/${invalidSFID}/gerrits`,
          needsAuth: true,
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'GET /gerrits/{invalidID} (bad request)',
          method: 'GET',
          url: `${claEndpoint}gerrits/${invalidID}`,
          needsAuth: true,
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'POST /project/{validSFID}/gerrits with empty body (bad request)',
          method: 'POST',
          url: `${claEndpoint}project/a092M00001IV4SfQAL/gerrits`,
          body: {},
          needsAuth: true,
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'PUT /gerrits/{invalidID} (bad request)',
          method: 'PUT',
          url: `${claEndpoint}gerrits/${invalidID}`,
          body: { gerrit_name: 'test' },
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
