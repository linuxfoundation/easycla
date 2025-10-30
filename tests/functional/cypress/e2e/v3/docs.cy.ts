import { validate_200_Status, getAPIBaseURL, validate_expected_status } from '../../support/commands';

describe('To Validate & test Documentation APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  it('Get API Documentation - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}api-docs`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.not.be.null;
      });
    });
  });

  it('Get Swagger JSON - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}swagger.json`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body).to.have.property('swagger');
        expect(response.body).to.have.property('info');
        expect(response.body).to.have.property('paths');
      });
    });
  });

  describe('Expected failures', () => {
    it('Returns errors due to malformed requests for Documentation APIs', function () {
      const cases: Array<{
        title: string;
        method: 'GET' | 'POST' | 'PUT' | 'DELETE';
        url: string;
        body?: any;
        expectedStatus?: number;
        expectedCode?: number;
        expectedMessage?: string;
        expectedMessageContains?: boolean;
      }> = [
        {
          title: 'POST /api-docs (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}api-docs`,
          body: {},
          expectedStatus: 405,
          expectedCode: 405,
          expectedMessage: 'method POST is not allowed, but [GET] are',
          expectedMessageContains: true,
        },
        {
          title: 'POST /swagger.json (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}swagger.json`,
          body: {},
          expectedStatus: 405,
          expectedCode: 405,
          expectedMessage: 'method POST is not allowed, but [GET] are',
          expectedMessageContains: true,
        },
      ];

      cy.wrap(cases).each((c: any) => {
        return cy
          .request({
            method: c.method,
            url: c.url,
            body: c.body,
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing: ${c.title}`);
              validate_expected_status(
                response,
                c.expectedStatus,
                c.expectedCode,
                c.expectedMessage,
                c.expectedMessageContains,
              );
            });
          });
      });
    });
  });
});
