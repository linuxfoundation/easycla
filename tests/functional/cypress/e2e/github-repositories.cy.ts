import {validateApiResponse,validate_200_Status,getTokenKey} from '../support/commands'
describe("To Validate github-organizations API call", function () {
  
 // Define a variable for the environment
 const environment = Cypress.env("CYPRESS_ENV");

 // Import the appropriate configuration based on the environment
 let appConfig;
 if (environment === 'dev') {
   appConfig = require('../appConfig/config.dev.ts').appConfig;
 } else if (environment === 'production') {
   appConfig = require('../appConfig/config.production.ts').appConfig;
 }
  
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/github-repositories
    
    //Variable for GitHub    
    const projectSfidOrg=appConfig.childProjectSFID; //project name: easyAutom-child2  
  
  const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/project/${projectSfidOrg}/github/repositories`;
  let claGroupId: string =appConfig.claGroupId;
  let repository_id: string="";
  let repository_external_id: string="";
  let repository_external_id2: string="";
  let gitHubOrgName: string="";
  let branch_name: string="";
  
  
  let bearerToken: string = null;
    before(() => { 
        if(bearerToken==null){
        getTokenKey(bearerToken);
        cy.window().then((win) => {
            bearerToken = win.localStorage.getItem('bearerToken');
          });
        }
    });

  it("Get the GitHub repositories of the project which are CLA Enforced- Record should Returns 200 Response", function () {
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
        repository_id=list[0].repository_id;
        claGroupId= list[0].repository_cla_group_id;
        gitHubOrgName=list[0].repository_organization_name;
        repository_external_id=list[0].repository_external_id;
        repository_external_id2=list[1].repository_external_id;
           expect(list[0].repository_name).to.eql('ApiAutomStandaloneOrg/repo01')     
               //To validate schema of response
               validateApiResponse("github-repositories/getRepositories.json",response.body);
    });
  });

  it("Remove 'disable CLA Enforced' the GitHub repository from the project - Record should Returns 204 Response", function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}/${repository_id}`,
      
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      expect(response.status).to.eq(204);
      
    });
  });

  it("User should able to Add 'CLA Enforced' a GitHub repository to the project - Record should Returns 200 Response", function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}`,
      
      auth: {
        'bearer': bearerToken,
      },
      body:{
        
            "cla_group_id": claGroupId,
            "github_organization_name": gitHubOrgName,
            "repository_github_id": repository_external_id.toString(),
            "repository_github_ids": [
                repository_external_id.toString(),repository_external_id2.toString()
            ]
          
      }
    }).then((response) => {
      validate_200_Status(response);
      
        // Validate specific data in the response
        expect(response.body).to.have.property('list');
        let list = response.body.list;
        repository_id=list[0].repository_id;
        claGroupId= list[0].repository_cla_group_id;
        gitHubOrgName=list[0].repository_organization_name;
        expect(list[0].repository_name).to.eql('ApiAutomStandaloneOrg/repo01')    
 
               //To validate schema of response
               validateApiResponse("github-repositories/getRepositories.json",response.body);
});
  });

  it("Get GitHub branch protection for given repository - Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/${repository_id}/branch-protection`,
      
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body
      branch_name=list.branch_name;
      if(list.protection_enabled){
     
          //To validate schema of response
          validateApiResponse("github-repositories/getBranchProtection.json",response.body);
}
else{
    console.log('branch protection is false')
}
 });
  });

it("Update github branch protection for given repository - Record should Returns 200 Response", function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}/${repository_id}/branch-protection`,
      
      auth: {
        'bearer': bearerToken,
      },
      body:{
        "branch_name": branch_name,
        "enforce_admin": true,
        "status_checks": [
          {
            "enabled": true,
            "name": "EasyCLA"
          }
        ]
      }
    }).then((response) => {
      validate_200_Status(response);
          //To validate schema of response
          validateApiResponse("github-repositories/getBranchProtection.json",response.body);        
 });
  });

})    