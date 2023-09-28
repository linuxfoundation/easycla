import {validateApiResponse,validate_200_Status,getTokenKey} from '../support/commands'

describe("To Validate & check cla version via API call", function () {
   
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/version
     const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4`;
   
    let bearerToken: string = null;
    before(() => { 
        if(bearerToken==null){
        getTokenKey(bearerToken);
        cy.window().then((win) => {
            bearerToken = win.localStorage.getItem('bearerToken');
          });
        }
    });

it("Returns the application version information- Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/ops/version`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
          //To validate schema of response
             validateApiResponse("version/getVersion.json",response)
        });
  });

})