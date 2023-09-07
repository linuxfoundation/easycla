import { isNull } from 'cypress/types/lodash';
import {validateApiResponse,validate_200_Status,getTokenKey} from '../support/commands'
describe("To Validate cla-manager API call", function () {
  
 // Define a variable for the environment
 const environment = Cypress.env("CYPRESS_ENV");

 // Import the appropriate configuration based on the environment
 let appConfig;
 if (environment === 'dev') {
   appConfig = require('../appConfig/config.dev.ts').appConfig;
 } else if (environment === 'production') {
   appConfig = require('../appConfig/config.production.ts').appConfig;
 }
  
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/metrics
    const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/metrics/`;
    const companyID=appConfig.companyID;//infosys limited
    const companyName=appConfig.companyName;//Infosys Limited
    const projectSFID=appConfig.projectSFID;//SUN
    let projectID=appConfig.projectID;
    let claEndpointForNextKey="";
    let bearerToken: string = null;
   
    before(() => { 
        if(bearerToken==null){
        getTokenKey(bearerToken);
        cy.window().then((win) => {
            bearerToken = win.localStorage.getItem('bearerToken');
          });
        }
    });

    it("Get CLA manager distribution for EasyCLA - Record should Returns 200 Response", function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}cla-manager-distribution`,       
        auth: {
          'bearer': bearerToken,
        }
      }).then((response) => {
        validate_200_Status(response);
        validateApiResponse("metrics/getClaManagerDistribution.json",response)
            });      
        });
      
    it("Get & Returns metrics of company - Record should Returns 200 Response", function () {
          cy.request({
            method: 'GET',
            url: `${claEndpoint}company/${companyID}`,       
            auth: {
              'bearer': bearerToken,
            }
          }).then((response) => {
            validate_200_Status(response);
            expect(response.body.companyName).to.eql(companyName) 
            expect(response.body.id).to.eql(companyID) 
            validateApiResponse("metrics/getCompanyMetric.json",response)
                });      
        });

    it("Get & Returns metrics of company - Record should Returns 200 Response", function () {
          cy.request({
            method: 'GET',
            url: `${claEndpoint}company/${companyID}/project/${projectSFID}`,       
            auth: {
              'bearer': bearerToken,
            }
          }).then((response) => {
            validate_200_Status(response);
            let list=response.body.list;
            expect(list[1].companyName).to.eql(companyName) 
            expect(list[1].companyID).to.eql(companyID) 
            expect(list[1].projectSFID).to.eql(projectSFID)
            projectID=list[1].projectID;
            validateApiResponse("metrics/listCompanyProjectMetrics.json",response)
                });      
        });

    it("List the metrics for the projects - Record should Returns 200 Response", function () {
      claEndpointForNextKey= `${claEndpoint}project`;
      cy.request({
            method: 'GET',
            url: `${claEndpoint}project`,       
            auth: {
              'bearer': bearerToken,
            }
          }).then((response) => {
            validate_200_Status(response);
              let NextKey=response.body.nextKey;
            validateApiResponse("metrics/listProjectMetrics.json",response)
            console.log(NextKey);
            fetchNextRecords(claEndpointForNextKey,NextKey);
            
                });      
        });

    it("Get & Returns metrics of company - Record should Returns 200 Response", function () {
          cy.request({
            method: 'GET',
            url: `${claEndpoint}project?=${projectID}`,       
            auth: {
              'bearer': bearerToken,
            }
          }).then((response) => {
            validate_200_Status(response);
            validateApiResponse("metrics/getProjectMetric.json",response)
                });      
        });

    it("Get top company metrics - Record should Returns 200 Response", function () {
          cy.request({
            method: 'GET',
            url: `${claEndpoint}top-companies`,       
            auth: {
              'bearer': bearerToken,
            }
          }).then((response) => {
            validate_200_Status(response);
            validateApiResponse("metrics/getTopCompanies.json",response)
                });      
        });

    it("Get project metrics of the top projects", function () {
          cy.request({
            method: 'GET',
            url: `${claEndpoint}top-projects`,       
            auth: {
              'bearer': bearerToken,
            }
          }).then((response) => {
            validate_200_Status(response);
            validateApiResponse("metrics/getTopProjects.json",response)
                });      
        });
        
    it("Get total count metrics", function () {
          cy.request({
            method: 'GET',
            url: `${claEndpoint}total-count`,       
            auth: {
              'bearer': bearerToken,
            }
          }).then((response) => {
            validate_200_Status(response);
            validateApiResponse("metrics/getTotalCount.json",response)
                });      
        });
//List the metrics for the projects
        function fetchNextRecords(URL,NextKey){
          if(NextKey!==undefined){
          cy.request({
            method: 'GET',
            url: `${URL}?nextKey=${NextKey}&pageSize=100`,
            auth: {
              'bearer': bearerToken,
            }
          }).then((response) => {
            validate_200_Status(response);
            
              // Validate specific data in the response
              let updatedNextKey=response.body.nextKey;
             if(updatedNextKey!==undefined){
              fetchNextRecords(URL,updatedNextKey);
             }
            });
          }
        }
});
