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

  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/cla-manager
/* 
https://api-gw.dev.platform.linuxfoundation.org/acs/v1/api-docs#tag/UserRole
https://api-gw.dev.platform.linuxfoundation.org/acs/v1/api-docs#tag/Role/operation/getRoles
*/
    //Variable for GitHub    
   const companyID=appConfig.companyID;//infosys limited
   const projectSFID=appConfig.projectSFID;//sun
   const projectSFID_Designee=appConfig.childProjectSFID//EASYAUTOM-CHILD2
  const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/`;
  let bearerToken: string = null;
  const claGroupID=appConfig.claGroupId;
  const sun_claGroupID=appConfig.claGroupId_projectSFID //sun
  const userEmail="veerendrat@proximabiz.com";
  let companyName=appConfig.companyName//"Infosys limited";
  let companySFID="";
  let userLFID="veerendrat";
  let userId=appConfig.userIdclaManager//"c5ac2857-c263-11ed-94d1-d2349de32229";//veerendrat
  
  before(() => {   
     
    if(bearerToken==null){
      getTokenKey(bearerToken);
      cy.window().then((win) => {
          bearerToken = win.localStorage.getItem('bearerToken');
        });
      }
  });

  it("Assigns CLA Manager designee to a given user.", function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}company/${companyID}/claGroup/${claGroupID}/cla-manager-designee`,
     
      auth: {
        'bearer': bearerToken,
      },
      body:{
        "userEmail": userEmail        
      }
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      validate_200_Status(response);
        companySFID=response.body.list[0].company_sfid;
        userLFID=response.body.list[0].lf_username;
        cy.log('company_sfid : '+ companySFID);
        cy.log('lf_username : '+ userLFID);
           //To validate Post response
           if (response.status === 200) {
            getClaManager();
           }
           validateApiResponse("cla-manager/assignCLAManager.json",response)
    });
  });

  it("Allows an existing CLA Manager to add another CLA Manager to the specified Company and Project.", function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager`,
     
      auth: {
        'bearer': bearerToken,
      },
      failOnStatusCode: false,
      body:{

        "firstName": "veerendrat",
        "lastName": "thakur",
        "userEmail": userEmail
        
      }
    }).then((response) => {
   
      // expect(response.duration).to.be.lessThan(20000);
    
      if(response.status === 200) {
        validate_200_Status(response);      
        // Validate specific data in the response       
        let list = response.body;
        expect(list.project_sfid).to.eql(projectSFID)    
           //To validate schema of response
      }else{
             expect(response.body.Message).to.include('error: manager already in signature ACL');
      }
      validateApiResponse("cla-manager/createCLAManager.json",response)
    });
  });

  it("Deletes the CLA Manager from CLA Manager list for specified Company and Project", function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager/${userLFID}`,
     
      auth: {
        'bearer': bearerToken,
      },
      
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      expect(response.status).to.eq(204);
    });
  });

  it("Assigns CLA Manager designee to a given user", function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}company/${companyID}/project/${projectSFID_Designee}/cla-manager-designee`,
     
      auth: {
        'bearer': bearerToken,
      },
      body:{
        "userEmail": userEmail        
      }
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      validate_200_Status(response);      
        // Validate specific data in the response
        expect(response.body.project_sfid).to.eql(projectSFID_Designee) 
        if (response.status === 200) {
          getClaManager();
         }    
         validateApiResponse("cla-manager/createCLAManagerDesignee.json",response)
    });
  });

  it("Adds a CLA Manager Designee to the specified Company and Project", function () {
    cy.request({
        method: 'POST',
        url: `${claEndpoint}company/${companyID}/project/${projectSFID_Designee}/cla-manager/requests`,
       
        auth: {
          'bearer': bearerToken,
        },
        body:{
            "contactAdmin": false,
            "fullName": "veerendrat cla",
            "userEmail": "veerendrat+cla@proximabiz.com"
          }
      }).then((response) => {
        // expect(response.duration).to.be.lessThan(20000);
        validate_200_Status(response);        
          // Validate specific data in the response         
          expect(response.body.project_sfid).to.eql(projectSFID_Designee)    
             //To validate schema of response
             validateApiResponse("cla-manager/createCLAManagerDesignee.json",response)
      });
    });

    it("Send Notification to CLA Managaers", function () {
      cy.request({
          method: 'POST',
          url: `${claEndpoint}notify-cla-managers`,
         
          auth: {
            'bearer': bearerToken,
          },
          body:{
            "claGroupID": claGroupID,
            "companyName": companyName,
            "list": [
              {
                "email": "vthakur@contractor.linuxfoundation.org",
                "name": "vthakur"
              }
            ],
            "signingEntityName": "Linux Foundation",
            "userID": userId
          }
        }).then((response) => {
          // expect(response.duration).to.be.lessThan(20000);
          expect(response.status).to.eq(204); 
        });
      });

      it("Invite Company Admin based on user request to sign CLA", function () {
        cy.request({
            method: 'POST',
            url: `${claEndpoint}user/${userId}/invite-company-admin`,
           
            auth: {
              'bearer': bearerToken,
            },
            body:{
              "claGroupID": sun_claGroupID,
              "companyID": companyID,
              "contactAdmin": true,
              "name": "veerendra thakur",
              "userEmail": userEmail
            }
          }).then((response) => {
            // expect(response.duration).to.be.lessThan(20000);
            validate_200_Status(response);
            // validateApiResponse("cla-manager/assignCLAManager.json",response)
          });
        });
    

    function getClaManager(){
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/${companySFID}/user/${userLFID}/claGroupID/${claGroupID}/is-cla-manager-designee`,
        auth: {
            'bearer': bearerToken,
          },  
        }).then((response) => {
          validate_200_Status(response);
            expect(response.body.hasRole).to.eq(true);
            expect(response.body.lfUsername).to.eq(userLFID);
            // validateApiResponse("cla-manager/isCLAManagerDesignee.json",response)
          })
    }
});