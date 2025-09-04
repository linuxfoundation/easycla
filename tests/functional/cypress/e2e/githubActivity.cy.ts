import { validate_200_Status, getTokenKey, getAPIBaseURL, getXACLHeader } from '../support/commands';
describe('To Validate & get GitHub Activity Callback via API call', function () {
  //Reference api doc:  https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/github-activity
  const claEndpoint = getAPIBaseURL('v4') + `github/activity`;
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

  it('GitHub Activity Callback Handler reacts to GitHub events emmited.', function () {
    const headers = {
      ...getXACLHeader(),
      'x-github-event': 'pull_request',
      'x-hub-signature': 'sha1=deadbeef',
    };
    cy.request({
      method: 'POST',
      url: `${claEndpoint}`,
      failOnStatusCode: allowFail,
      auth: {
        bearer: bearerToken,
      },
      headers: headers,
      body: {
        action: 'requested_action',
      },
    }).then((response) => {
      // return cy.logJson('response', response).then(() => validate_200_Status(response));
      validate_200_Status(response);
    });
  });
});
