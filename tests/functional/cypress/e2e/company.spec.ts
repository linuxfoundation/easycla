import {validateApiResponse,validate_200_Status,getTokenKey} from '../support/commands'
describe("To Validate & get Company Activity Callback via API call", function () {
    //Reference api doc:  https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/company
    const claBaseEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/`;
    const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/company/`;
    let companyName="Infosys Limited";
    let companyExternalID="";
    let companyID="fcf59557-9708-4b2f-8200-4e6448761c0a";//Microsoft Corporation

    let signingEntityName="";
    const projectSFID="a09P000000DsCE5IAN";
    let claGroupId="";    

    const user_id="8f3e52b8-0072-11ee-9def-0ef17207dfe8";//vthakur+lfstaff@contractor.linuxfoundation.org
    const userEmail= "vthakur+lfstaff@contractor.linuxfoundation.org";
    const user_id2="4a4c1dba-407f-11ed-8c58-a6b0f8fb81a9"//vthakur+lfitstaff@contractor.linuxfoundation.org

    let bearerToken: string = null;
    before(() => { 
 
      getTokenKey(bearerToken);
        cy.window().then((win) => {
            bearerToken = win.localStorage.getItem('bearerToken');
          });
        
    });

it("Gets the company by name", function () {
  getCompanyByName();
  });

it("Get Company By Internal ID", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyID}`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      let list=response.body;
      companyExternalID=list.companyExternalID;
      companyID=list.companyID;
      signingEntityName=list.signingEntityName;
      validateApiResponse("company/getCompanyByName.json",response)
        });
  });

it("Gets the company by signing entity name", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}entityname/${signingEntityName}`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      let list=response.body;
      companyExternalID=list.companyExternalID;
      companyID=list.companyID;
      signingEntityName=list.signingEntityName;
      companyExternalID=list.companyExternalID;
      validateApiResponse("company/getCompanyByName.json",response)
        });
  });

it("Search companies from organization service", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}lookup?companyName=${companyName}`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse("company/searchCompanyLookup.json",response)
        });
  });

it("Get active CLA list of company for particular project/foundation", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyID}/project/${projectSFID}/active-cla-list`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse("company/getCompanyProjectActiveCla.json",response)
        });
  });

it("Get Company by External SFID", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}external/${companyExternalID}`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      let list=response.body;
      companyExternalID=list.companyExternalID;
      companyID=list.companyID;
      signingEntityName=list.signingEntityName;
      validateApiResponse("company/getCompanyByName.json",response)
        });
  });

it("Returns the CLA Groups associated with the Project and Company", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyExternalID}/project/${projectSFID}/cla`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      let list=response.body.list;
      if (list[0].signed_cla_list.length > 0 &&'cla_group_id' in list[0].signed_cla_list[0]) {
         claGroupId=list[0].signed_cla_list[0].cla_group_id;
      }else{          
        claGroupId=list[0].unsigned_project_list[0].cla_group_id;   
    }
        });
  });

it("Get list of CLA managers based on the CLA Group and v1 Company ID", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyID}/cla-group/${claGroupId}/cla-managers`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
        });
  });

it("Get active CLA list of company for particular project/foundation", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyID}/project/${projectSFID}/active-cla-list`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
        });
  });

it("Get CLA manager of company for particular project/foundation", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyID}/project/${projectSFID}/cla-managers`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
        });
  });

it("Get corporate contributors for project", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyID}/project/${projectSFID}/contributors`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse("company/getCompanyProjectContributors.json",response)
        });
  });

it("Returns a list of Company Admins (salesforce)", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyExternalID}/admin`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse("company/getCompanyAdmins.json",response)
        });
  });

it("Associates a contributor with a company", function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}${companyExternalID}/contributorAssociation`,
      auth: {
        'bearer': bearerToken,
      },
      body:{

        "userEmail": "veerendrat@proximabiz.com"
      }
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse("company/getCompanyAdmins.json",response)
        });
  });

it("Creates a new salesforce company", function () {
    cy.request({
      method: 'POST',
      url: `${claBaseEndpoint}user/${user_id}/company`,
      auth: {
        'bearer': bearerToken,
      },
      body:{
        "companyName": "lfx dev Test",
        "companyWebsite": "https://lfxdevtest.org",
        "note": "Added via automation",
        "signingEntityName": "lfx dev Test",
        "userEmail": userEmail
      }
    }).then((response) => {
      validate_200_Status(response);
      companyName="lfx dev Test";
      companyID=response.body.companyID;
      getCompanyByName();
        });
  });

it("Deletes the company by the SFID", function () {   
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}sfid/${companyExternalID}`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      expect(response.status).to.eq(204); 
        });
  });


  it("Creates a new salesforce company", function () {
    cy.request({
      method: 'POST',
      url: `${claBaseEndpoint}user/${user_id}/company`,
      auth: {
        'bearer': bearerToken,
      },
      body:{
        "companyName": "lfx dev Test",
        "companyWebsite": "https://lfxdevtest.org",
        "note": "Added via automation",
        "signingEntityName": "lfx dev Test",
        "userEmail": userEmail
      }
    }).then((response) => {
      validate_200_Status(response);
      companyName="lfx dev Test";
      companyID=response.body.companyID;
      getCompanyByName();
        });
  });

it("Deletes the company by ID", function () {   
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}id/${companyID}`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      expect(response.status).to.eq(204); 
        });
  });

it("Request Company Admin based on user request to sign CLA", function () {
    cy.request({
      method: 'POST',
      url: `${claBaseEndpoint}user/${user_id2}/request-company-admin`,
      auth: {
        'bearer': bearerToken,
      },
      failOnStatusCode: false,
      body:{
        "claManagerEmail": "vthakur@contractor.linuxfoundation.org",
        "claManagerName": "veerendra thakur",
        "companyName": "lfx dev Test1",
        "contributorEmail": "vthakur+lfitstaff@contractor.linuxfoundation.org",
        "contributorName": "vthakur lfitstaff",
        "projectName": "Sun foundation cla group",
        "version": "v1"
      }
    }).then((response) => {
      validate_200_Status(response);
        });
  });

function getCompanyByName(){
    cy.request({
      method: 'GET',
      url: `${claEndpoint}name/${companyName}`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      let list=response.body;
      companyExternalID=list.companyExternalID;
      companyID=list.companyID;
      signingEntityName=list.signingEntityName;
      validateApiResponse("company/getCompanyByName.json",response)
        });
  }

});

     