import {validateApiResponse,validate_200_Status,getTokenKey} from '../support/commands'

describe("To Validate & get health status via API call", function () {
   
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/health/operation/healthCheck
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

it("Returns the Health of the application- Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/ops/health`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
          //To validate schema of response
             validateApiResponse("health/healthCheck.json",response)
        });
  });

})