import {validateApiResponse,validate_200_Status} from '../support/commands'
describe("To Validate cla-manager API call", function () {
    //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/cla-manager
/* 
https://api-gw.dev.platform.linuxfoundation.org/acs/v1/api-docs#tag/UserRole
https://api-gw.dev.platform.linuxfoundation.org/acs/v1/api-docs#tag/Role/operation/getRoles
*/
    //Variable for GitHub    
   const companyID="f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc";
   const projectSFID="a09P000000DsCE5IAN";//sun
   const projectSFID_Designee="a09P000000DsNH2IAN"
  const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/`;
  let bearerToken: string = "";
  const claGroupID="1baf67ab-d894-4edf-b6fc-c5f939db59f7";
  const sun_claGroupID="01af041c-fa69-4052-a23c-fb8c1d3bef24"
  const userEmail="veerendrat@proximabiz.com";
  let companyName="Infosys limited";
  let organization_id="";
  let organization_name="";
  let companySFID="";
  let userLFID="veerendrat";
  let userId="c5ac2857-c263-11ed-94d1-d2349de32229";//veerendrat
  
  before(() => {   
     
      cy.request({
        method: 'POST',
        url: Cypress.env("AUTH0_TOKEN_API"),
       
        body: {
          "grant_type": "http://auth0.com/oauth/grant-type/password-realm",
          "realm": "Username-Password-Authentication",
          "username":Cypress.env("AUTH0_USER_NAME"),
          "password":Cypress.env("AUTH0_PASSWORD"),
          "client_id":Cypress.env("AUTH0_CLIENT_ID"),
          "audience": "https://api-gw.dev.platform.linuxfoundation.org/",
          "scope": "access:api openid profile email"
        }
      }).then(response => {        
        expect(response.status).to.eq(200);       
        bearerToken = response.body.access_token;    
  
      });
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
        organization_id=list.organization_id;
        organization_name=list.organization_name;
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