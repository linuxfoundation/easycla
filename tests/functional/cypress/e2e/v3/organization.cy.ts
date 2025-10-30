import {
  validate_200_Status,
  validate_401_Status,
  validateApiResponse,
  getAPIBaseURL,
  validate_expected_status,
} from '../../support/commands';

describe('To Validate & test Organization APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  it('Search Organization by company name - Record should return 200 Response', function () {
    const companyName = 'Linux Foundation';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}organization/search?companyName=${encodeURIComponent(companyName)}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body.list).to.be.an('array');
        expect(response.body.list.length).to.be.greaterThan(0);
        validateApiResponse('organization/searchOrganization.json', response);
      });
    });
  });

  it('Search Organization by website name - Record should return 200 Response', function () {
    const websiteName = 'linuxfoundation.org';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}organization/search?websiteName=${encodeURIComponent(websiteName)}`,
      timeout: timeout,
      failOnStatusCode: false, // Handle connection issues for local
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body.list).to.be.an('array');
        expect(response.body.list.length).to.be.greaterThan(0);
      });
    });
  });

  it('Search Organization with both company name and website - Record should return 200 Response', function () {
    const companyName = 'Linux Foundation';
    const websiteName = 'linuxfoundation.org';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}organization/search?companyName=${encodeURIComponent(companyName)}&websiteName=${encodeURIComponent(websiteName)}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body.list).to.be.an('array');
        expect(response.body.list.length).to.be.greaterThan(0);
      });
    });
  });

  it('Search Organization by non-existing company name - Record should return 200 Response', function () {
    const companyName = 'Non-existing XYZ';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}organization/search?companyName=${encodeURIComponent(companyName)}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body.list).to.be.an('array');
        expect(response.body.list.length).to.eq(0);
      });
    });
  });

  describe('Expected failures', () => {
    it('Returns errors due to missing or malformed parameters for Organization APIs', function () {
      const cases: Array<{
        title: string;
        method: 'GET';
        url: string;
        expectedStatus: number;
        expectedCode?: number;
        expectedMessage?: string;
        expectedMessageContains?: boolean;
      }> = [
        {
          title: 'GET /organization/search with neither companyName nor websiteName',
          method: 'GET',
          url: `${claEndpoint}organization/search`,
          expectedStatus: 400,
          expectedMessage: 'companyName or websiteName or filter at least one required',
          expectedMessageContains: true,
        },
        {
          title: 'GET /organization/search with malformed websiteName',
          method: 'GET',
          url: `${claEndpoint}organization/search?websiteName=not-a-valid-url`,
          expectedStatus: 422,
          expectedMessage: 'websiteName in query should match',
          expectedMessageContains: true,
          expectedCode: 605,
        },
      ];

      cy.wrap(cases).each((c: any) => {
        cy.task('log', `--> ${c.title} | ${c.method} ${c.url}`);
        return cy
          .request({
            method: c.method,
            url: c.url,
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
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
