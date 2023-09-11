import {validateApiResponse,validate_200_Status,getTokenKey} from '../support/commands';

describe("To Validate & get list of gitlab-organizations via API call", function () {
 
// Define a variable for the environment
const environment = Cypress.env("CYPRESS_ENV");

// Import the appropriate configuration based on the environment
let appConfig;
if (environment === 'dev') {
  appConfig = require('../appConfig/config.dev.ts').appConfig;
} else if (environment === 'production') {
  appConfig = require('../appConfig/config.production.ts').appConfig;
}

    //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/gitlab-organizations
    const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4`;
    const projectSFID=appConfig.projectSFID; //project name: sun
    let gitLabOrgName=appConfig.gitLabOrganizationName;
    const gitLabGroupID=appConfig.groupId;
    let gitLabOrganizationFullPath=appConfig.gitLabOrganizationFullPath;// it will update on POST request
    let claGroupId="";
    let organizationExternalId="";

    let bearerToken: string = null;
before(() => { 
     if(bearerToken==null){
      getTokenKey(bearerToken);
      cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
      });
     }
   });

it("Get the Gitlab organizations of the project", function () {
  getGitLabGroupMembers();
  });

it("List members of a given GitLab group", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/gitlab/group/${organizationExternalId}/members`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
        //To validate schema of response
        validateApiResponse("gitlab-organizations/getGitLabGroupMembers.json",response.body);
      });
  });

it("Update Gitlab Group/Organization Configuration", function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}/project/${projectSFID}/gitlab/group/${gitLabGroupID}/config`,
      auth: {
        'bearer': bearerToken,
      },
      body:{
        "auto_enabled": true,
        "auto_enabled_cla_group_id": claGroupId,
        "branch_protection_enabled": true
      }
    }).then((response) => {
      validate_200_Status(response);
        //To validate schema of response
        validateApiResponse("gitlab-organizations/updateProjectGitlabGroupConfig.json",response.body);
      });
  });

it("Add new Gitlab Organization in the project", function () {
  console.log("gitLabOrganizationFullPath: "+gitLabOrganizationFullPath)
    cy.request({
      method: 'POST',
      url: `${claEndpoint}/project/${projectSFID}/gitlab/organizations`,
      auth: {
        'bearer': bearerToken,
      },body:{
        "auto_enabled": false,
        "auto_enabled_cla_group_id": claGroupId,
        "branch_protection_enabled": false,
        "group_id": parseInt(gitLabGroupID, 10),
        "organization_full_path": gitLabOrganizationFullPath
      },failOnStatusCode: false
    }).then((response) => {
      validate_200_Status(response);
      //To validate schema of response
      validateApiResponse("gitlab-organizations/addProjectGitlabOrganization.json",response.body);
      });
  });

it("Delete Gitlab Group/Organization Configuration", function () {
   // Define the URL
   const url = gitLabOrganizationFullPath;
   // Use JavaScript string methods to extract the desired substring
   gitLabOrganizationFullPath = url.split('/').pop();
   // Log or use the extracted substring as needed
   cy.log(gitLabOrganizationFullPath);
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}/project/${projectSFID}/gitlab/organization?organization_full_path=${gitLabOrganizationFullPath}`,
      auth: {
        'bearer': bearerToken,
      },failOnStatusCode: false
    }).then((response) => {
      expect(response.status).to.eq(204);
      });
  });

function getGitLabGroupMembers(){
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/project/${projectSFID}/gitlab/organizations`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      let list=response.body.list;
      for(let i=0;i<=list.length-1;i++){
        if(list[i].organization_name===gitLabOrgName){
          organizationExternalId=list[i].organization_external_id;
          // gitLabOrganizationFullPath=list[i].organization_full_path;
          claGroupId=list[i].repositories[0].cla_group_id
          break;
        }
      }
      //To validate schema of response
           validateApiResponse("gitlab-organizations/getProjectGitlabOrganizations.json",response.body);
      });
  }

})