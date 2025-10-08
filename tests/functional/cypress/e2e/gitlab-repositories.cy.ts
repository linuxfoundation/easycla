import {
  validateApiResponse,
  validate_200_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
} from '../support/commands';

describe('To Validate & Get the GitLab repositories of the project via API call', function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../appConfig/config.production.ts').appConfig;
  }

  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/gitlab-repositories
  const projectSFID = appConfig.projectSFID; //project name: sun
  const claEndpoint = getAPIBaseURL('v4') + `project/${projectSFID}/gitlab/repositories`;

  let gitLabOrgName = appConfig.gitLabOrganizationName;
  const gitLabGroupID = appConfig.groupId;
  let claGroupId = '';
  let repoExternalId = '';
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);

  let bearerToken: string = null;
  before(() => {
    if (bearerToken == null) {
      getTokenKey(bearerToken);
      cy.window().then((win) => {
        bearerToken = win.localStorage.getItem('bearerToken');
      });
    }
  });

  it('Get the GitLab repositories of the project', function () {
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
      let list = response.body.list;
      for (let i = 0; i <= list.length - 1; i++) {
        if (list[i].repository_organization_name === gitLabOrgName) {
          repoExternalId = list[i].repository_external_id;
          claGroupId = list[i].repository_cla_group_id;
          break;
        }
      }
      //To validate schema of response
      validateApiResponse('gitlab-repositories/getProjectGitLabRepositories.json', response.body);
    });
  });

  it("Un-Enrolls 'Enforce CLA' GitLab repositories for the CLA Group", function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        cla_group_id: claGroupId,
        unenroll: [parseInt(repoExternalId, 10)],
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body.list;
      for (let i = 0; i <= list.length - 1; i++) {
        if (list[i].repository_organization_name === gitLabOrgName) {
          expect(list[i].enabled).to.eql(false);
          break;
        }
      }
      //To validate schema of response
      validateApiResponse('gitlab-repositories/enrollGitLabRepository.json', response.body);
    });
  });

  it("Enrolls 'Enforce CLA'  GitLab repositories for the CLA Group", function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        cla_group_id: claGroupId,
        enroll: [parseInt(repoExternalId, 10)],
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body.list;
      for (let i = 0; i <= list.length - 1; i++) {
        if (list[i].repository_organization_name === gitLabOrgName) {
          expect(list[i].enabled).to.eql(true);
          break;
        }
      }
      //To validate schema of response
      validateApiResponse('gitlab-repositories/enrollGitLabRepository.json', response.body);
    });
  });
});
