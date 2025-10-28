import { getAPIBaseURL, validateApiResponse, validate_200_Status } from '../../support/commands';

describe('To Validate & get api-docs via API call', function () {
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/docs
  const claEndpoint = getAPIBaseURL('v4');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;

  it('Endpoint to render the API documentation - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}api-docs`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it('Returns the Swagger specification as a JSON document - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}swagger.json`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      validate_200_Status(response);
    });
  });
});
