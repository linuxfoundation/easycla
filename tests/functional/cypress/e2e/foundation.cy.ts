import {validateApiResponse,validate_200_Status,getTokenKey} from '../support/commands'
//import {appConfig} from  '../support/config.${Cypress.env("CYPRESS_ENV")}'
describe("To Validate & get list of Foundation ClaGroups via API call", function () {
  
 // Define a variable for the environment
 const environment = Cypress.env("CYPRESS_ENV");

 // Import the appropriate configuration based on the environment
 let appConfig;
 if (environment === 'dev') {
   appConfig = require('../appConfig/config.dev.ts').appConfig;
 } else if (environment === 'production') {
   appConfig = require('../appConfig/config.production.ts').appConfig;
 }
  
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/foundation
    const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/foundation-mapping`;
    const foundationSFID=appConfig.foundationSFID; //project name: easyAutom foundation

    let bearerToken: string = null;
    before(() => { 
        if(bearerToken==null){
        getTokenKey(bearerToken);
        cy.window().then((win) => {
            bearerToken = win.localStorage.getItem('bearerToken');
          });
        }
    });

it("Get CLA Groups under a foundation- Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}?foundationSFID=${foundationSFID}`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
        // Validate specific data in the response
       expect(response.body).to.have.property('list');           
        let list = response.body.list;
        expect(list[0].foundation_sfid).to.eql(foundationSFID)
         // Assert that the response contains an array
        expect(list[0].cla_groups).to.be.an('array');
          // Assert that the array has at least one item
        expect(list[0].cla_groups.length).to.be.greaterThan(0);
          //To validate schema of response
             validateApiResponse("foundation/listFoundationClaGroups.json",response)
        });
  });

})