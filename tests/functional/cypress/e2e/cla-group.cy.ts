import {validateApiResponse,validate_200_Status,getTokenKey} from '../support/commands'

describe("To Validate 'GET, CREATE, UPDATE and DELETE' CLA groups API call on child project", function () {
 
   // Define a variable for the environment
   const environment = Cypress.env("CYPRESS_ENV");

   // Import the appropriate configuration based on the environment
   let appConfig;
   if (environment === 'dev') {
     appConfig = require('../appConfig/config.dev.ts').appConfig;
   } else if (environment === 'production') {
     appConfig = require('../appConfig/config.production.ts').appConfig;
   }

  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/cla-group

  const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4`;
  let claGroupId: string ="";
    
  //Variable for create cla group
  const foundation_sfid=appConfig.foundationSFID; //project name: easyAutom foundation
  const projectSfid=appConfig.createNewClaGroupSFID;  //project name: easyAutom-child1
  const cla_group_name=appConfig.claGroupName;
  const cla_group_description='Added via cypress script';

  //variable for update cla group
  const updated_cla_group_name='Cypress_Updated_ClaGroup';
  const update_cla_group_description='CLA group created and updated for easy cla automation child project 1'
 
  
  //Variable for GitHub
  const gitHubOrgName=appConfig.gitHubOrgPartialStatus;
  const projectSfidOrg=appConfig.projectSFID; //project name: sun
  

  //Enroll /unEnroll projects 
  const enrollProjectsSFID=appConfig.enrollProjectsSFID //project name: easyAutomChild1-GrandChild1
  const child_Project_name=appConfig.child_Project_name
 
  
  let bearerToken: string = null;
  before(() => { 
      if(bearerToken==null){
      getTokenKey(bearerToken);
      cy.window().then((win) => {
          bearerToken = win.localStorage.getItem('bearerToken');
        });
      }
  });

  it("Creates a new CLA Group at child level - Record should Returns 200 Response", function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}/cla-group`,
     
      auth: {
        'bearer': bearerToken,
      },
      failOnStatusCode: false,
      body: {
        "icla_enabled": true,
    "ccla_enabled": true,
    "ccla_requires_icla": true,
    "cla_group_description": cla_group_description,
    "cla_group_name": cla_group_name,
    "foundation_sfid": foundation_sfid,
    "project_sfid_list": [
      projectSfid
    ],
    
    "template_fields": {
        "TemplateID": "fb4cc144-a76c-4c17-8a52-c648f158fded",
        "MetaFields": [
            {
            "description": "Project's Full Name.",
            "name": "Project Name",
            "templateVariable": "PROJECT_NAME",
            "value" : "Test"
            },
            {
                "description": "The Full Entity Name of the Project.",
                "name": "Project Entity Name",
                "templateVariable": "PROJECT_ENTITY_NAME",
                "value" : "Test"
            },
            {
                "description": "The E-Mail Address of the Person managing the CLA. ",
                "name": "Contact Email Address",
                "templateVariable": "CONTACT_EMAIL",
                "value" : "veerendrat@proximabiz.com"
            }                       
         ]
    }
      },
    }).then((response) => {
      const jsonResponse = JSON.stringify(response.body, null, 2);
      cy.log(jsonResponse);
      // expect(response.duration).to.be.lessThan(20000);
      validate_200_Status(response);
      
        // Validate specific data in the response
        expect(response.body).to.have.property('cla_group_name', cla_group_name);
        claGroupId = response.body.cla_group_id;
       
     //To validate schema of response
     validateApiResponse("claGroup/create_claGroup2.json",response.body);
    });
  });

  it("Get list of cla group associated with project - Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/foundation/${projectSfid}/cla-groups`,
     
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      validate_200_Status(response);
      
     
        // Validate specific data in the response
        expect(response.body).to.have.property('list');
        let list = response.body.list;
        claGroupId= list[0].cla_group_id;
            expect(list[0].cla_group_name).to.eql(cla_group_name)     
            
           //To validate schema of response
    validateApiResponse("claGroup/list_claGroup.json",response.body);
    });
  });

  it("Updates a CLA Group details - Record should Returns 200 Response", function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}/cla-group/${claGroupId}`,
     
      auth: {
        'bearer': bearerToken,
      },
      body: {
          "cla_group_description": update_cla_group_description,
          "cla_group_name": updated_cla_group_name
      },
    }).then((response) => {
     // expect(response.duration).to.be.lessThan(20000);
     validate_200_Status(response);
     
       // Validate specific data in the response
       expect(response.body).to.have.property('cla_group_name', updated_cla_group_name);
       expect(response.body).to.have.property('cla_group_description', update_cla_group_description);       
           
          //To validate schema of response
             validateApiResponse("claGroup/update_claGroup2.json",response.body);
   });
  });

  it("Enroll projects in a CLA Group - Record should Returns 200 Response", function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}/cla-group/${claGroupId}/enroll-projects`,
     
      auth: {
        'bearer': bearerToken,
      },
      body: [enrollProjectsSFID],
    }).then((response) => {
     // expect(response.duration).to.be.lessThan(20000);
     validate_200_Status(response);
     // Check if the first API response status is 200
     if (response.status === 200) {
      // Run the second API request
      cy.request({
        method: 'GET',
        url: `${claEndpoint}/foundation/${projectSfid}/cla-groups`,
       
        auth: {
          'bearer': bearerToken,
        },
      }).then((secondResponse) => {       
        // Validate specific data in the response
        expect(secondResponse.body).to.have.property('list');
        let list = secondResponse.body.list;     
            expect(list[0].project_list[1].project_name).to.eql(child_Project_name)   
            expect(list[0].project_list[1].project_sfid).to.eql(enrollProjectsSFID)
            expect(list[0].project_list[0].project_sfid).to.eql(projectSfid)
      });
    } else {
      console.log('First API request did not return a 200 status.');
    }
  });
  });

  it("unenroll projects in a CLA Group - Record should Returns 200 Response", function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}/cla-group/${claGroupId}/unenroll-projects`,
     
      auth: {
        'bearer': bearerToken,
      },
      body: [enrollProjectsSFID],
    }).then((response) => {
     // expect(response.duration).to.be.lessThan(20000);
     validate_200_Status(response);
   });
  });
  
  it("Get list of Github organization associated with project - Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/project/${projectSfidOrg}/github/organizations`,
     
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      
        // Validate specific data in the response
        expect(response.body).to.have.property('list');
        let list = response.body.list;
        expect(list[2].github_organization_name).to.eql('Sun-lfxfoundationOrgTest')    
        expect(list[2].connection_status).to.eql('partial_connection')   
    });
  });

  it("Update GitHub Organization Configuration - Record should Returns 200 Response", function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}/project/${projectSfidOrg}/github/organizations/${gitHubOrgName}/config`,
     
      auth: {
        'bearer': bearerToken,
      },
      body: {
        "autoEnabled": true,
        "autoEnabledClaGroupID": claGroupId,
        "branchProtectionEnabled": true
      },
    }).then((response) => {
      validate_200_Status(response);
    }); 
  });
  
  it("Deletes the CLA Group - Record should Returns 204 Response", function () {
    if(claGroupId!=null){
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}/cla-group/${claGroupId}`,
     
      auth: {
        'bearer': bearerToken,
      },
    }).then((response) => {
      expect(response.status).to.eq(204);
      // Check if the first API response status is 200
      if (response.status === 204) {
        // Run the second API request
        cy.request({
          method: 'GET',
          url: `${claEndpoint}/foundation/${projectSfid}/cla-groups`,
         
          auth: {
            'bearer': bearerToken,
          },
        }).then((secondResponse) => {       
          // Validate specific data in the response
          cy.wrap(secondResponse.body.list)
          .should('be.an', 'array') // Check if the response is an array
          .and('have.length', 0);
        });
      } else {
        console.log('First API request did not return a 204 status.');
      }    
    }); 
  }else{
      console.log('claGroupId is null'+ claGroupId)
  }
  });

})