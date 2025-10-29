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

describe('To Validate & test Organization APIs via API call (V3)', function () {
  //Reference api doc: V3 API organization endpoints
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

  // Test public endpoints (no auth required) - already covered in company.cy.ts
  // These tests focus on authenticated organization endpoints if they exist

  it('Search Organizations - already tested in company.cy.ts', function () {
    // This test is covered in company.cy.ts since /organization/search is there
    const companyName = 'Linux Foundation';

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

  it('Triple test for flakiness - Organization endpoints', function () {
    // Run test 3 times to catch flaky behavior
    const companyName = 'Test Company';

    cy.wrap([1, 2, 3]).each((iteration) => {
      cy.task('log', `Organization test iteration ${iteration}/3`);
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

  // ========================= Expected failures (organization) =========================
  describe('Expected failures', () => {
    it('Returns errors due to malformed requests for Organization APIs', function () {
      const defaultHeaders = getXACLHeader();

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
          title: 'GET /organization/search with missing companyName param (bad request)',
          method: 'GET',
          url: `${claEndpoint}organization/search`,
          needsAuth: false, // Public endpoint
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'POST /organization/search (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}organization/search?companyName=Test`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method POST is not allowed',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method POST is not allowed',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'PUT /organization/search (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}organization/search?companyName=Test`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method PUT is not allowed',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method PUT is not allowed',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'DELETE /organization/search (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}organization/search?companyName=Test`,
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method DELETE is not allowed',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method DELETE is not allowed',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /organization/invalid-path (not found)',
          method: 'GET',
          url: `${claEndpoint}organization/invalid-path`,
          expectedStatusLocal: 404,
          expectedMessageLocal: 'path /v3/organization/invalid-path was not found',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 404,
          expectedMessageRemote: 'path /v3/organization/invalid-path was not found',
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
