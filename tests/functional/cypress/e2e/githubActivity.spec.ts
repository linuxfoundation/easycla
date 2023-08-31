import {validate_200_Status,getTokenKey} from '../support/commands'
describe("To Validate & get GitHub Activity Callback via API call", function () {
    //Reference api doc:  https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/github-activity
    const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/github/activity`;

    let bearerToken: string = null;
    before(() => { 
        if(bearerToken==null){
        getTokenKey(bearerToken);
        cy.window().then((win) => {
            bearerToken = win.localStorage.getItem('bearerToken');
          });
        }
    });

it("GitHub Activity Callback Handler reacts to GitHub events emmited.", function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}`,
      auth: {
        'bearer': bearerToken,
      },
      body:{

        "action": "requested_action"
      }
    }).then((response) => {
      validate_200_Status(response);
        });
  });

})