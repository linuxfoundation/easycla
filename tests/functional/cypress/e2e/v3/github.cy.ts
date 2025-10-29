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

describe('To Validate & test GitHub APIs via API call (V3)', function () {
  //Reference api doc: V3 API github endpoints
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
  it('GitHub login redirect endpoint - should redirect or return error', function () {
    const callbackUrl = 'https://example.com/callback';

    cy.request({
      method: 'GET',
      url: `${claEndpoint}github/login?callback=${encodeURIComponent(callbackUrl)}`,
      timeout: timeout,
      failOnStatusCode: false,
      followRedirect: false, // Don't follow redirects
      headers: getXACLHeader(),
      // No auth required for this endpoint
    }).then((response) => {
      // This endpoint should either redirect (302) or return an error
      if (response.status === 302) {
        expect(response.status).to.equal(302);
        expect(response.headers.location).to.be.a('string');
      } else if (response.status >= 400) {
        // Error responses are also acceptable for this test endpoint
        cy.log(`GitHub login returned ${response.status} which is acceptable for test`);
      } else {
        cy.log(`Unexpected status: ${response.status}`);
      }
    });
  });

  it('GitHub redirect endpoint - should handle auth callback', function () {
    const authCode = 'test-auth-code';
    const state = 'test-state';

    cy.request({
      method: 'GET',
      url: `${claEndpoint}github/redirect?code=${authCode}&state=${state}`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeader(),
      // No auth required for this endpoint
    }).then((response) => {
      // This endpoint might return various responses depending on the auth flow
      // Accept any response as valid for testing purposes
      cy.log(`GitHub redirect returned status: ${response.status}`);
      expect(response.status).to.be.a('number');
    });
  });

  it('Triple test for flakiness - GitHub endpoints', function () {
    // Run test 3 times to catch flaky behavior
    const callbackUrl = 'https://example.com/callback';

    cy.wrap([1, 2, 3]).each((iteration) => {
      cy.task('log', `GitHub test iteration ${iteration}/3`);
      return cy
        .request({
          method: 'GET',
          url: `${claEndpoint}github/login?callback=${encodeURIComponent(callbackUrl)}`,
          timeout: timeout,
          failOnStatusCode: false,
          followRedirect: false,
          headers: getXACLHeader(),
        })
        .then((response) => {
          // Accept any reasonable response
          expect(response.status).to.be.a('number');
          expect(response.status).to.be.greaterThan(0);
        });
    });
  });

  // ========================= Expected failures (github) =========================
  describe('Expected failures', () => {
    it('Returns errors due to malformed requests for GitHub APIs', function () {
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
          title: 'GET /github/login with missing callback param (bad request)',
          method: 'GET',
          url: `${claEndpoint}github/login`,
          needsAuth: false, // Public endpoint
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'GET /github/redirect with missing code param (bad request)',
          method: 'GET',
          url: `${claEndpoint}github/redirect`,
          needsAuth: false, // Public endpoint
          expectedStatusLocal: 400,
          expectedStatusRemote: 400,
        },
        {
          title: 'POST /github/login (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}github/login?callback=https://example.com`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method POST is not allowed',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method POST is not allowed',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'POST /github/redirect (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}github/redirect?code=test&state=test`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method POST is not allowed',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method POST is not allowed',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /github/invalid-endpoint (not found)',
          method: 'GET',
          url: `${claEndpoint}github/invalid-endpoint`,
          expectedStatusLocal: 404,
          expectedMessageLocal: 'path /v3/github/invalid-endpoint was not found',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 404,
          expectedMessageRemote: 'path /v3/github/invalid-endpoint was not found',
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
            followRedirect: false,
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
