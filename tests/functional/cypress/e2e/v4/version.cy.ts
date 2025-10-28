import {
  validateApiResponse,
  validate_200_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
  validate_expected_status,
} from '../../support/commands';

describe('To Validate & check cla version via API call', function () {
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/version
  const claEndpoint = getAPIBaseURL('v4');
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

  it('Returns the application version information- Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}ops/version`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      //To validate schema of response
      validateApiResponse('version/getVersion.json', response);
    });
  });

  it('Version endpoint works without authentication (no token required)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}ops/version`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      // No auth - version endpoint is public
    }).then((response) => {
      validate_200_Status(response);
      //To validate schema of response
      validateApiResponse('version/getVersion.json', response);
    });
  });

  // ========================= Expected failures (version) =========================
  describe('Expected failures', () => {
    it('Returns errors due to malformed requests for Version APIs', function () {
      const defaultHeaders = getXACLHeader();

      const cases: Array<{
        title: string;
        method: 'GET' | 'POST' | 'PUT' | 'DELETE';
        url: string;
        body?: any;
        headers?: any;
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
          title: 'POST /ops/version (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}ops/version`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method POST is not allowed, but [GET] are',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method POST is not allowed, but [GET] are',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'PUT /ops/version (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}ops/version`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method PUT is not allowed, but [GET] are',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method PUT is not allowed, but [GET] are',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'DELETE /ops/version (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}ops/version`,
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method DELETE is not allowed, but [GET] are',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method DELETE is not allowed, but [GET] are',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /ops/version/invalid-path (not found)',
          method: 'GET',
          url: `${claEndpoint}ops/version/invalid-path`,
          expectedStatusLocal: 404,
          expectedMessageLocal: 'path /v4/ops/version/invalid-path was not found',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 404,
          expectedMessageRemote: 'path /v4/ops/version/invalid-path was not found',
          expectedMessageContainsRemote: true,
        },
      ];

      cy.wrap(cases).each((c: any) => {
        return cy
          .request({
            method: c.method,
            url: c.url,
            body: c.body,
            headers: c.headers || defaultHeaders,
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
