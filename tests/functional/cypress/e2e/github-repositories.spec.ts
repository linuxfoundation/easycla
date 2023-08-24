describe("To Validate github-organizations API call", function () {
    //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/github-repositories
    
    const Ajv = require('ajv');
    //Variable for GitHub    
    const projectSfidOrg='a09P000000DsNH2IAN'; //project name: easyAutom-child2  
  
  const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/project/${projectSfidOrg}/github/repositories`;
  let bearerToken: string = "";
  let claGroupId: string ="1baf67ab-d894-4edf-b6fc-c5f939db59f7";
  let repository_id: string="";
  let repository_external_id: string="";
  let repository_external_id2: string="";
  let gitHubOrgName: string="";
  let branch_name: string="";
  
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

  it("Get the GitHub repositories of the project which are CLA Enforced- Record should Returns 200 Response", function () {
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
        repository_id=list[0].repository_id;
        claGroupId= list[0].repository_cla_group_id;
        gitHubOrgName=list[0].repository_organization_name;
        repository_external_id=list[0].repository_external_id;
        repository_external_id2=list[1].repository_external_id;
           expect(list[0].repository_name).to.eql('ApiAutomStandaloneOrg/repo01')     
               //To validate schema of response
               schemaValidate("github-repositories/getRepositories.json",response.body);
    });
  });

  it("Remove 'disable CLA Enforced' the GitHub repository from the project - Record should Returns 204 Response", function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}/${repository_id}`,
      headers: optionalHeaders,
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
      headers: optionalHeaders,
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
      expect(response.status).to.eq(200);
      expect(response.body).to.not.be.null;
        // Validate specific data in the response
        expect(response.body).to.have.property('list');
        let list = response.body.list;
        repository_id=list[0].repository_id;
        claGroupId= list[0].repository_cla_group_id;
        gitHubOrgName=list[0].repository_organization_name;
        expect(list[0].repository_name).to.eql('ApiAutomStandaloneOrg/repo01')    
 
               //To validate schema of response
               schemaValidate("github-repositories/getRepositories.json",response.body);
});
  });

  it("Get GitHub branch protection for given repository - Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/${repository_id}/branch-protection`,
      headers: optionalHeaders,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      expect(response.status).to.eq(200);
      let list = response.body
      branch_name=list.branch_name;
      if(list.protection_enabled){
     
          //To validate schema of response
          schemaValidate("github-repositories/getBranchProtection.json",response.body);
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
      headers: optionalHeaders,
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
      expect(response.status).to.eq(200);
          //To validate schema of response
          schemaValidate("github-repositories/getBranchProtection.json",response.body);
        
 });
  });

  function schemaValidate(schemaPath,body){
    //To validate schema of response
    const ajv = new Ajv();
    // Load the JSON schema
      cy.fixture(schemaPath).then(
       (schema) => {
           const validate = ajv.compile(schema);
           const isValid = validate(body);
            // Assert that the response matches the schema
            expect(isValid, 'API response schema is valid').to.be.true;
          });
}

})    