import {validateApiResponse,validate_200_Status,getTokenKey} from '../support/commands'

describe("To Validate & get api-docs via API call", function () {
  
  
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/docs
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

it("Endpoint to render the API documentation- Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/api-docs`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
        });
  });

it("Returns the Swagger specification as a JSON document- Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/swagger.json`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
        });
  });

})