import {
  validateApiResponse,
  validate_200_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
} from '../support/commands';
describe('To Validate & get Company Activity Callback via API call', function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../appConfig/config.production.ts').appConfig;
  }

  //Reference api doc:  https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/company
  const claBaseEndpoint = getAPIBaseURL('v4');
  const claEndpoint = getAPIBaseURL('v4') + 'company/';

  let companyExternalID = '';
  let companyID = '';
  let signingEntityName = '';
  let claGroupId = '';

  let companyName = appConfig.companyName;
  const projectSFID = appConfig.projectSFID; //project name: sun
  const user_id = appConfig.user_id; //vthakur+lfstaff@contractor.linuxfoundation.org
  const userEmail = appConfig.userEmail;
  const user_id2 = appConfig.user_id2; //vthakur+lfitstaff@contractor.linuxfoundation.org
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);

  let bearerToken: string = null;
  before(() => {
    getTokenKey(bearerToken);
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  it('Gets the company by name', function () {
    getCompanyByName();
  });

  it('Get Company By Internal ID', function () {
    let url = `${claEndpoint}${companyID}`;
    cy.task('log', 'Getting company with URL: ' + url);
    cy.request({
      method: 'GET',
      url: url,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        let list = response.body;
        companyExternalID = list.companyExternalID;
        companyID = list.companyID;
        signingEntityName = list.signingEntityName;
        validateApiResponse('company/getCompanyByName.json', response);
      });
    });
  });

  it('Gets the company by signing entity name', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}entityname/${signingEntityName}`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body;
      companyExternalID = list.companyExternalID;
      companyID = list.companyID;
      signingEntityName = list.signingEntityName;
      companyExternalID = list.companyExternalID;
      validateApiResponse('company/getCompanyByName.json', response);
    });
  });

  it('Search companies from organization service', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}lookup?companyName=${companyName}`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('company/searchCompanyLookup.json', response);
    });
  });

  it('Get active CLA list of company for particular project/foundation', function () {
    cy.request({
      method: 'GET',
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      url: `${claEndpoint}${companyID}/project/${projectSFID}/active-cla-list`,
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('company/getCompanyProjectActiveCla.json', response);
    });
  });

  it('Get Company by External SFID', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}external/${companyExternalID}`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body;
      companyExternalID = list.companyExternalID;
      companyID = list.companyID;
      signingEntityName = list.signingEntityName;
      validateApiResponse('company/getCompanyByName.json', response);
    });
  });

  it('Returns the CLA Groups associated with the Project and Company', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyExternalID}/project/${projectSFID}/cla`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body.list;
      if (list[0].signed_cla_list.length > 0 && 'cla_group_id' in list[0].signed_cla_list[0]) {
        claGroupId = list[0].signed_cla_list[0].cla_group_id;
      } else {
        claGroupId = list[0].unsigned_project_list[0].cla_group_id;
      }
    });
  });

  it('Get list of CLA managers based on the CLA Group and v1 Company ID', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyID}/cla-group/${claGroupId}/cla-managers`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it('Get active CLA list of company for particular project/foundation', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyID}/project/${projectSFID}/active-cla-list`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it('Get CLA manager of company for particular project/foundation', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyID}/project/${projectSFID}/cla-managers`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it('Get corporate contributors for project', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyID}/project/${projectSFID}/contributors`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('company/getCompanyProjectContributors.json', response);
    });
  });

  it('Returns a list of Company Admins (salesforce)', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}${companyExternalID}/admin`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('company/getCompanyAdmins.json', response);
    });
  });

  it.skip('Associates a contributor with a company', function () {
    let url = `${claEndpoint}${companyExternalID}/contributorAssociation`;
    cy.task('log', 'Associating contributor with URL: ' + url);
    cy.request({
      method: 'POST',
      url: url,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        userEmail: 'veerendrat@proximabiz.com',
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        validateApiResponse('company/getCompanyAdmins.json', response);
      });
    });
  });

  it('Creates a new salesforce company', function () {
    cy.request({
      method: 'POST',
      url: `${claBaseEndpoint}user/${user_id}/company`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        companyName: 'lfx dev Test',
        companyWebsite: 'https://lfxdevtest.org',
        note: 'Added via automation',
        signingEntityName: 'lfx dev Test',
        userEmail: userEmail,
      },
    }).then((response) => {
      validate_200_Status(response);
      companyName = 'lfx dev Test';
      companyID = response.body.companyID;
      getCompanyByName();
    });
  });

  it.skip('Deletes the company by the SFID', function () {
    let url = `${claEndpoint}sfid/${companyExternalID}`;
    cy.task('log', 'Deleting company with URL: ' + url);
    cy.request({
      method: 'DELETE',
      url: url,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        expect(response.status).to.eq(204);
      });
    });
  });

  it('Creates a new salesforce company', function () {
    cy.request({
      method: 'POST',
      url: `${claBaseEndpoint}user/${user_id}/company`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        companyName: 'lfx dev Test',
        companyWebsite: 'https://lfxdevtest.org',
        note: 'Added via automation',
        signingEntityName: 'lfx dev Test',
        userEmail: userEmail,
      },
    }).then((response) => {
      validate_200_Status(response);
      companyName = 'lfx dev Test';
      companyID = response.body.companyID;
      getCompanyByName();
    });
  });

  it('Deletes the company by ID', function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}id/${companyID}`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      expect(response.status).to.eq(204);
    });
  });

  it('Request Company Admin based on user request to sign CLA', function () {
    cy.request({
      method: 'POST',
      url: `${claBaseEndpoint}user/${user_id2}/request-company-admin`,
      auth: {
        bearer: bearerToken,
      },
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      body: {
        claManagerEmail: 'vthakur@contractor.linuxfoundation.org',
        claManagerName: 'veerendra thakur',
        companyName: 'lfx dev Test1',
        contributorEmail: 'vthakur+lfitstaff@contractor.linuxfoundation.org',
        contributorName: 'vthakur lfitstaff',
        projectName: 'Sun foundation cla group',
        version: 'v1',
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  function getCompanyByName() {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}name/${companyName}`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body;
      companyExternalID = list.companyExternalID;
      companyID = list.companyID;
      signingEntityName = list.signingEntityName;
      validateApiResponse('company/getCompanyByName.json', response);
    });
  }
});
