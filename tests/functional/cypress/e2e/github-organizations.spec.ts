// import {schemaValidate} from '../support/commands.js';
describe("To Validate github-organizations API call", function () {
//Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/github-organizations

const Ajv = require('ajv');
  //Variable for GitHub
  const gitHubOrgName='ApiAutomStandaloneOrg';
  const projectSfidOrg='a09P000000DsNH2IAN'; //project name: easyAutom-child2
  const gitHubOrg='cypressioTest';


const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/project/${projectSfidOrg}/github/organizations`;
let bearerToken: string = "";
const claGroupId: string ="1baf67ab-d894-4edf-b6fc-c5f939db59f7";

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

it("Get list of Github organization associated with project - Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}`,
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
        expect(list[0].github_organization_name).to.eql('ApiAutomStandaloneOrg')    
        expect(list[0].connection_status).to.eql('connected')   
               //To validate schema of response
        const ajv = new Ajv();
        // Load the JSON schema
   cy.fixture("github-organizations/getProjectGithubOrganizations.json").then(
     (schema) => {
       console.log(schema)
 const validate = ajv.compile(schema);
 const isValid = validate(response.body);

 // Assert that the response matches the schema
 expect(isValid, 'API response schema is valid').to.be.true;
});
    });
  });

  it("Update GitHub Organization Configuration - Record should Returns 200 Response", function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}/${gitHubOrgName}/config`,
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

  it("Add new GitHub Oranization in the project - Record should Returns 200 Response", function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}`,
      headers: optionalHeaders,
      auth: {
        'bearer': bearerToken,
      },
      body: {
        "autoEnabled": false,
        "autoEnabledClaGroupID": claGroupId,
        "branchProtectionEnabled": false,
        "organizationName": gitHubOrg
      },
    }).then((response) => {
      expect(response.status).to.eq(200);
      expect(response.body).to.not.be.null;
        // Validate specific data in the response
        expect(response.body).to.have.property('list');
        let list = response.body.list;
        expect(list[1].github_organization_name).to.eql(gitHubOrg)    
        expect(list[1].connection_status).to.eql('connected')   
        //To validate schema of response
        const ajv = new Ajv();
        // Load the JSON schema
   cy.fixture("github-organizations/addProjectGithubOrganization.json").then(
     (schema) => {
       console.log(schema)
 const validate = ajv.compile(schema);
 const isValid = validate(response.body);

 // Assert that the response matches the schema
 expect(isValid, 'API response schema is valid').to.be.true;
});    
    });
  });

  it("Delete GitHub oranization in the project - Record should Returns 204 Response", function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}/${gitHubOrg}`,
      headers: optionalHeaders,
      auth: {
        'bearer': bearerToken,
      },
      
    }).then((response) => {
      expect(response.status).to.eq(204);       
    });
  });

})