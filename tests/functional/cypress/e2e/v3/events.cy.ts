import {
  validateApiResponse,
  validate_200_Status,
  validate_401_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
  validate_expected_status,
} from '../../support/commands';

describe('To Validate & test Events APIs via API call (V3)', function () {
  //Reference api doc: V3 API events endpoints
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
  it('Get Events with authentication - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}events?pageSize=10`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      expect(response.body).to.be.an('object');
      if (response.body.list) {
        expect(response.body.list).to.be.an('array');
      }
    });
  });

  it('Get Company Events with authentication - Record should return 200 Response', function () {
    const companyID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // Example UUID

    cy.request({
      method: 'GET',
      url: `${claEndpoint}events/company/${companyID}?pageSize=10`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      expect(response.body).to.be.an('object');
      if (response.body.list) {
        expect(response.body.list).to.be.an('array');
      }
    });
  });

  it('Triple test for flakiness - Events endpoints', function () {
    // Run test 3 times to catch flaky behavior
    cy.wrap([1, 2, 3]).each((iteration) => {
      cy.task('log', `Events test iteration ${iteration}/3`);
      return cy
        .request({
          method: 'GET',
          url: `${claEndpoint}events?pageSize=5`,
          timeout: timeout,
          failOnStatusCode: allowFail,
          headers: getXACLHeader(),
          auth: { bearer: bearerToken },
          auth: {
            bearer: bearerToken,
          },
        })
        .then((response) => {
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
        });
    });
  });

  // ========================= Auth required tests =========================
  describe('Authentication Required Tests', () => {
    it('Returns 401 for Events APIs when called without token', () => {
      const exampleCompanyID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
      const exampleUserID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8f';

      const requests = [
        // GET /events (requires auth)
        { method: 'GET', url: `${claEndpoint}events` },

        // GET /events/company/{companyID} (requires auth)
        { method: 'GET', url: `${claEndpoint}events/company/${exampleCompanyID}` },

        // GET /events/user/{userID} (requires auth if it exists)
        { method: 'GET', url: `${claEndpoint}events/user/${exampleUserID}` },
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

  // ========================= Expected failures (events) =========================
  describe('Expected failures', () => {
    it('Returns errors due to malformed requests for Events APIs', function () {
      const defaultHeaders = getXACLHeader();
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
          title: 'GET /events/company/{invalidID} (bad request)',
          method: 'GET',
          url: `${claEndpoint}events/company/${invalidID}`,
          needsAuth: true,
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'POST /events (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}events`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method POST is not allowed',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method POST is not allowed',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'PUT /events (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}events`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method PUT is not allowed',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method PUT is not allowed',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'DELETE /events (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}events`,
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method DELETE is not allowed',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method DELETE is not allowed',
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
