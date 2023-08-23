// import {getBearerToken,bearerToken } from '../../support/commands.js';

describe("To Validate 'GET, CREATE, UPDATE and DELETE' CLA groups API call on child project", function () {
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/cla-group

  const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4`;
  const Ajv = require('ajv');
  let bearerToken: string = "";
  let claGroupId: string ="";
    
  //Variable for create cla group
  const foundation_sfid='a09P000000DsNGsIAN'; //project name: easyAutom foundation
  const projectSfid='a09P000000DsNGxIAN'; //project name: easyAutom-child1
  const cla_group_name='CypressClaGroup';
  const cla_group_description='Added via cypress script';

  //variable for update cla group
  const updated_cla_group_name='Cypress_Updated_ClaGroup1';
  const update_cla_group_description='CLA group created and updated for easy cla automation child project 1'
 
  
  //Variable for GitHub
  const gitHubOrgName='Sun-lfxfoundationOrgTest';
  const projectSfidOrg='a09P000000DsCE5IAN'; //project name: sun
  

  //Enroll /unEnroll projects 
  const EnrollProjectsSFID='a09P000000DsNHCIA3' //project name: easyAutomChild1-GrandChild1
  const child_Project_name='easyAutomChild1-GrandChild1'
 
  //Headers
  let optionalHeaders: Headers = {
    "X-LFX-CACHE": false,
  }
  
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

  it("Creates a new CLA Group at child level - Record should Returns 200 Response", function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}/cla-group`,
      headers: optionalHeaders,
      auth: {
        'bearer': bearerToken,
      },
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
      // expect(response.duration).to.be.lessThan(20000);
      expect(response.status).to.eq(200);
      expect(response.body).to.not.be.null;
        // Validate specific data in the response
        expect(response.body).to.have.property('cla_group_name', cla_group_name);
        claGroupId = response.body.cla_group_id;
       
     //To validate schema of response
        const ajv = new Ajv();
        // Load the JSON schema
        cy.fixture("claGroup/create_claGroup2.json").then(
          (schema) => {
          const validate = ajv.compile(schema);
      const isValid = validate(response.body);

      // Assert that the response matches the schema
      expect(isValid, 'API response schema is valid').to.be.true;
    });
    });
  });

  it("Get list of cla group associated with project - Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/foundation/${projectSfid}/cla-groups`,
      headers: optionalHeaders,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      expect(response.status).to.eq(200);
      expect(response.body).to.not.be.null;
     
        // Validate specific data in the response
        expect(response.body).to.have.property('list');
        let list = response.body.list;
        claGroupId= list[0].cla_group_id;
            expect(list[0].cla_group_name).to.eql(cla_group_name)     
            
           //To validate schema of response
        const ajv = new Ajv();
             // Load the JSON schema
        cy.fixture("claGroup/list_claGroup.json").then(
          (schema) => {
        const validate = ajv.compile(schema);
      const isValid = validate(response.body);

      // Assert that the response matches the schema
      expect(isValid, 'API response schema is valid').to.be.true;
    });
    });
  });

  it("Updates a CLA Group details - Record should Returns 200 Response", function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}/cla-group/${claGroupId}`,
      headers: optionalHeaders,
      auth: {
        'bearer': bearerToken,
      },
      body: {
          "cla_group_description": update_cla_group_description,
          "cla_group_name": updated_cla_group_name
      },
    }).then((response) => {
     // expect(response.duration).to.be.lessThan(20000);
     expect(response.status).to.eq(200);
     expect(response.body).to.not.be.null;
       // Validate specific data in the response
       expect(response.body).to.have.property('cla_group_name', updated_cla_group_name);
       expect(response.body).to.have.property('cla_group_description', update_cla_group_description);       
           
          //To validate schema of response
       const ajv = new Ajv();
            // Load the JSON schema
       cy.fixture("claGroup/update_claGroup2.json").then(
         (schema) => {
     const validate = ajv.compile(schema);
     const isValid = validate(response.body);

     // Assert that the response matches the schema
     expect(isValid, 'API response schema is valid').to.be.true;
   });
   });
  });

  it("Enroll projects in a CLA Group - Record should Returns 200 Response", function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}/cla-group/${claGroupId}/enroll-projects`,
      headers: optionalHeaders,
      auth: {
        'bearer': bearerToken,
      },
      body: [EnrollProjectsSFID],
    }).then((response) => {
     // expect(response.duration).to.be.lessThan(20000);
     expect(response.status).to.eq(200);
     // Check if the first API response status is 200
     if (response.status === 200) {
      // Run the second API request
      cy.request({
        method: 'GET',
        url: `${claEndpoint}/foundation/${projectSfid}/cla-groups`,
        headers: optionalHeaders,
        auth: {
          'bearer': bearerToken,
        },
      }).then((secondResponse) => {       
        // Validate specific data in the response
        expect(secondResponse.body).to.have.property('list');
        let list = secondResponse.body.list;     
            expect(list[0].project_list[1].project_name).to.eql(child_Project_name)   
            expect(list[0].project_list[1].project_sfid).to.eql(EnrollProjectsSFID)
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
      headers: optionalHeaders,
      auth: {
        'bearer': bearerToken,
      },
      body: [EnrollProjectsSFID],
    }).then((response) => {
     // expect(response.duration).to.be.lessThan(20000);
     expect(response.status).to.eq(200);
   });
  });
  
  it("Get list of Github organization associated with project - Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/project/${projectSfidOrg}/github/organizations`,
      headers: optionalHeaders,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      expect(response.status).to.eq(200);
      expect(response.body).to.not.be.null;
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
      headers: optionalHeaders,
      auth: {
        'bearer': bearerToken,
      },
      body: {
        "autoEnabled": true,
        "autoEnabledClaGroupID": claGroupId,
        "branchProtectionEnabled": true
      },
    }).then((response) => {
      expect(response.status).to.eq(200);
    }); 
  });
  
  it("Deletes the CLA Group - Record should Returns 204 Response", function () {
    if(claGroupId!=null){
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}/cla-group/${claGroupId}`,
      headers: optionalHeaders,
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
          headers: optionalHeaders,
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