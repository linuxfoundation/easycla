import {
  validateApiResponse,
  validate_200_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
} from '../support/commands';

describe('To Validate & get health status via API call', function () {
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/health/operation/healthCheck
  const claEndpoint = getAPIBaseURL('v4');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);

  let bearerToken: string = null;
  before(() => {
    if (bearerToken == null) {
      getTokenKey(bearerToken);
      cy.window().then((win) => {
        bearerToken = win.localStorage.getItem('bearerToken');
      });
    }
  });

  it('Returns the Health of the application- Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}ops/health`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      //To validate schema of response
      validateApiResponse('health/healthCheck.json', response);
    });
  });
});
