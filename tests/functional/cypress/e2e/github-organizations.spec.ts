import {validateApiResponse,validate_200_Status,getTokenKey} from '../support/commands'
describe("To Validate github-organizations API call", function () {
//Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/github-organizations


  //Variable for GitHub
  const gitHubOrgName='ApiAutomStandaloneOrg';
  const projectSfidOrg='a09P000000DsNH2IAN'; //project name: easyAutom-child2
  const gitHubOrg='cypressioTest';


const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/project/${projectSfidOrg}/github/organizations`;
const claGroupId: string ="1baf67ab-d894-4edf-b6fc-c5f939db59f7";


let bearerToken: string = null;
before(() => { 
    if(bearerToken==null){
    getTokenKey(bearerToken);
    cy.window().then((win) => {
        bearerToken = win.localStorage.getItem('bearerToken');
      });
    }
});

it("Get list of Github organization associated with project - Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}`,
     
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      
        // Validate specific data in the response
        expect(response.body).to.have.property('list');
        let list = response.body.list;
        expect(list[0].github_organization_name).to.eql('ApiAutomStandaloneOrg')    
        expect(list[0].connection_status).to.eql('connected')   
               //To validate schema of response
               validateApiResponse("github-organizations/getProjectGithubOrganizations.json",response.body);
    });
  });

  it("Update GitHub Organization Configuration - Record should Returns 200 Response", function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}/${gitHubOrgName}/config`,
     
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

  it("Add new GitHub Oranization in the project - Record should Returns 200 Response", function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}`,
     
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
      validate_200_Status(response);
      
        // Validate specific data in the response
        expect(response.body).to.have.property('list');
        let list = response.body.list;
        expect(list[1].github_organization_name).to.eql(gitHubOrg)    
        expect(list[1].connection_status).to.eql('connected')   
        //To validate schema of response  
        validateApiResponse("github-organizations/addProjectGithubOrganization.json",response.body);
    });
  });

  it("Delete GitHub oranization in the project - Record should Returns 204 Response", function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}/${gitHubOrg}`,
     
      auth: {
        'bearer': bearerToken,
      },
      
    }).then((response) => {
      expect(response.status).to.eq(204);       
    });
  });

})