import {
  validate_200_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
  validate_expected_status,
} from '../../support/commands';

describe('To Validate & get API documentation via API call (V3)', function () {
  //Reference api doc: V3 API docs endpoints
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

  it('Returns API documentation HTML- Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}api-docs`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
      // No auth required for docs endpoint
    }).then((response) => {
      validate_200_Status(response);
      // Validate HTML content
      expect(response.body).to.be.a('string');
      expect(response.body.length).to.be.greaterThan(0);
    });
  });

  it('Returns Swagger specification as JSON- Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}swagger.json`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
      // No auth required for swagger.json endpoint
    }).then((response) => {
      validate_200_Status(response);
      // Validate JSON content
      expect(response.body).to.be.an('object');
      expect(response.body.swagger).to.equal('2.0');
      expect(response.body.info).to.be.an('object');
      expect(response.body.info.title).to.equal('CLA API');
      expect(response.body.basePath).to.equal('/v3');
    });
  });

  it('Triple test for flakiness - API docs endpoints', function () {
    // Run test 3 times to catch flaky behavior
    cy.wrap([1, 2, 3]).each((iteration) => {
      cy.task('log', `Docs test iteration ${iteration}/3`);

      // Test API docs HTML
      return cy
        .request({
          method: 'GET',
          url: `${claEndpoint}api-docs`,
          timeout: timeout,
          failOnStatusCode: allowFail,
          headers: getXACLHeader(),
          auth: { bearer: bearerToken },
        })
        .then((response) => {
          validate_200_Status(response);
          expect(response.body).to.be.a('string');
          expect(response.body.length).to.be.greaterThan(0);

          // Test Swagger JSON
          return cy.request({
            method: 'GET',
            url: `${claEndpoint}swagger.json`,
            timeout: timeout,
            failOnStatusCode: allowFail,
            headers: getXACLHeader(),
            auth: { bearer: bearerToken },
          });
        })
        .then((response) => {
          validate_200_Status(response);
          expect(response.body).to.be.an('object');
          expect(response.body.swagger).to.equal('2.0');
        });
    });
  });

  // ========================= Expected failures (docs) =========================
  describe('Expected failures', () => {
    it('Returns errors due to malformed requests for Docs APIs', function () {
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
          title: 'POST /api-docs (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}api-docs`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method POST is not allowed, but [GET] are',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method POST is not allowed, but [GET] are',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'PUT /api-docs (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}api-docs`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method PUT is not allowed, but [GET] are',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method PUT is not allowed, but [GET] are',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'DELETE /api-docs (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}api-docs`,
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method DELETE is not allowed, but [GET] are',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method DELETE is not allowed, but [GET] are',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'POST /swagger.json (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}swagger.json`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method POST is not allowed, but [GET] are',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 405,
          expectedMessageRemote: 'method POST is not allowed, but [GET] are',
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
