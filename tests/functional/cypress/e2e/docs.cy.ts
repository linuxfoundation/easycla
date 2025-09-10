import {
  validateApiResponse,
  validate_200_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
} from '../support/commands';

describe('To Validate & get api-docs via API call', function () {
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/docs
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

  it('Endpoint to render the API documentation- Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}api-docs`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it('Returns the Swagger specification as a JSON document- Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}swagger.json`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });
});
