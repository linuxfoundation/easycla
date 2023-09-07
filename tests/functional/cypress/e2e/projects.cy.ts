import {validateApiResponse,validate_200_Status,getTokenKey} from '../support/commands'
describe("To Validate & get projects Activity Callback via API call", function () {

 // Define a variable for the environment
 const environment = Cypress.env("CYPRESS_ENV");

 // Import the appropriate configuration based on the environment
 let appConfig;
 if (environment === 'dev') {
   appConfig = require('../appConfig/config.dev.ts').appConfig;
 } else if (environment === 'production') {
   appConfig = require('../appConfig/config.production.ts').appConfig;
 }

  //Reference api doc:  https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/project
    const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/project`;
    
    let foundationSFID=appConfig.foundationSFID ; //project name: easyAutom foundation
    let bearerToken: string = null;
    let projectSfid=appConfig.foundationSFID ; //project name: easyAutom foundation
    let externalID=appConfig.foundationSFID ; //project name: easyAutom foundation
    let projectName=appConfig.projectName;
   
    before(() => { 
        if(bearerToken==null){
        getTokenKey(bearerToken);
        cy.window().then((win) => {
            bearerToken = win.localStorage.getItem('bearerToken');
          });
        }
    });

it("Endpoint to fetch the project list", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}`,
      auth: {
        'bearer': bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse("projects/getProjects.json",response)
        });
  });

it("Get CLA enabled projects", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/enabled/${foundationSFID}`,
      auth: {
        'bearer': bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      let list=response.body.list;
       projectSfid=list[0].project_sfid;
       externalID=projectSfid;
       projectName=list[0].project_name;
      validateApiResponse("projects/getCLAProjectsByID.json",response)      
        });
  });

it("Get CLA Groups By SFDC ID", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/external/${externalID}}`,
      auth: {
        'bearer': bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse("projects/getCLAProjectsByID.json",response)      
        });
  });

it("Get Project By Name", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/name/${projectName}`,
      auth: {
        'bearer': bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);    
        });
  });

it("Get Project by ID", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/${projectSfid}`,
      auth: {
        'bearer': bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);    
        });
  });

it("Get SF Project Info by ID", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}-info/${projectSfid}`,
      auth: {
        'bearer': bearerToken,
      },
      failOnStatusCode: false

    }).then((response) => {
      // validate_200_Status(response);    
      const jsonResponse = JSON.stringify(response.body, null, 2);
  cy.log(jsonResponse);
        });
  });

it.skip("Delete Project by ID", function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}/${projectSfid}`,
      auth: {
        'bearer': bearerToken,
      },
      failOnStatusCode: false

    }).then((response) => {
      // validate_200_Status(response);    
      const jsonResponse = JSON.stringify(response.body, null, 2);
  cy.log(jsonResponse);
        });
  });

})