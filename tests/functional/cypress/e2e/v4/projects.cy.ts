import {
  validateApiResponse,
  validate_200_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
} from '../../support/commands';
describe('To Validate & get projects Activity Callback via API call', function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../../appConfig/config.production.ts').appConfig;
  }

  //Reference api doc:  https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/project
  const claEndpoint = getAPIBaseURL('v4') + 'project';

  let foundationSFID = appConfig.foundationSFID; //project name: easyAutom foundation
  let bearerToken: string = null;
  let projectSfid = appConfig.foundationSFID; //project name: easyAutom foundation
  let projectSfid2 = appConfig.projectSFID2;
  let projectSfid3 = appConfig.projectSFID3;
  let projectId2 = appConfig.projectID2;
  let externalID = appConfig.foundationSFID; //project name: easyAutom foundation
  let projectName = appConfig.projectName;
  let projectName2 = appConfig.projectName2;
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);

  before(() => {
    if (bearerToken == null) {
      getTokenKey(bearerToken);
      cy.window().then((win) => {
        bearerToken = win.localStorage.getItem('bearerToken');
      });
    }
  });

  it('Endpoint to fetch the project list', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('projects/getProjects.json', response);
    });
  });

  it('Get CLA enabled projects', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/enabled/${foundationSFID}`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        let list = response.body.list;
        let projectItem = list.find((item) => item.project_type === 'Project');
        if (!projectItem) {
          throw new Error("No project with type 'Project' found in response");
        }
        projectSfid = projectItem.project_sfid;
        externalID = projectSfid;
        projectName = projectItem.project_name;
        validateApiResponse('projects/getCLAProjectsByID.json', response);
      });
    });
  });

  it('Get CLA Groups By SFDC ID', function () {
    externalID = appConfig.foundationSFID;
    let url = `${claEndpoint}/external/${externalID}`;
    cy.task('log', 'Getting project by externalID with URL: ' + url);
    cy.request({
      method: 'GET',
      url: url,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('projects/getCLAProjectsByID.json', response);
    });
  });

  it('Get Project By Name', function () {
    let url = `${claEndpoint}/name/${projectName2}`;
    cy.task('log', 'Getting project by name with URL: ' + url);
    cy.request({
      method: 'GET',
      url: url,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
      });
    });
  });

  it('Get Project by ID', function () {
    let url = `${claEndpoint}/${projectId2}`;
    cy.task('log', 'Getting project by ID with URL: ' + url);
    cy.request({
      method: 'GET',
      url: url,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
      });
    });
  });

  // This endpoint is not used by consumers and is not considered in ACS.
  it.skip('Get SF Project Info by ID', function () {
    let url = `${claEndpoint}-info/${projectSfid3}`;
    cy.task('log', 'Getting project info by ID with URL: ' + url);
    cy.request({
      method: 'GET',
      url: url,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
      });
    });
  });

  it.skip('Delete Project by ID', function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}/${projectSfid}`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      // validate_200_Status(response);
      const jsonResponse = JSON.stringify(response.body, null, 2);
      cy.log(jsonResponse);
    });
  });
});
