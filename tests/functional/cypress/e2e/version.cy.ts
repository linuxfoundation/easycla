import {
  validateApiResponse,
  validate_200_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
} from '../support/commands';

describe('To Validate & check cla version via API call', function () {
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/version
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

  it('Returns the application version information- Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}ops/version`,
      timeout: 180000,
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
});
