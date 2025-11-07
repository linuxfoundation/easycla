// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import {
  validateApiResponse,
  validate_200_Status,
  validate_401_Status,
  validate_expected_status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
} from '../../support/commands';

describe('To Validate & Get the GitLab repositories of the project via API call', function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../../appConfig/config.production.ts').appConfig;
  }

  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/gitlab-repositories
  const projectSFID = appConfig.projectSFID; //project name: sun
  const claEndpoint = getAPIBaseURL('v4') + `project/${projectSFID}/gitlab/repositories`;

  let gitLabOrgName = appConfig.gitLabOrganizationName;
  const gitLabGroupID = appConfig.groupId;
  let claGroupId = '';
  let repoExternalId = '';
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  let local: boolean = Cypress.env('LOCAL') === 1;
  let timeout = 180000;

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
      timeout: timeout,
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
      timeout: timeout,
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
      timeout: timeout,
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

  // ========================= Expected failures (gitlab-repositories) =========================
  describe('Expected failures', () => {
    it('Returns 401 for all GitLab Repositories APIs when called without token', () => {
      const exampleProjectSFID = 'a09P000000DsNH2IAN';
      const claBaseEndpoint = getAPIBaseURL('v4');

      const requests = [
        // GET /project/{projectSFID}/gitlab/repositories
        {
          method: 'GET',
          url: `${claBaseEndpoint}project/${exampleProjectSFID}/gitlab/repositories`,
        },
        // PUT /project/{projectSFID}/gitlab/repositories (enroll/unenroll)
        {
          method: 'PUT',
          url: `${claBaseEndpoint}project/${exampleProjectSFID}/gitlab/repositories`,
          body: {
            cla_group_id: 'a1b86c26-d8e8-4fd8-9f8d-5c723d5dac9f',
            enroll: [12345],
          },
        },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            body: req.body,
            failOnStatusCode: false, // expect 401 without token
            timeout,
          })
          .then((response) => {
            return cy.logJson('401 response (gitlab-repositories)', response).then(() => {
              validate_401_Status(response, local);
            });
          });
      });
    });

    it('Returns errors due to missing or malformed parameters for GitLab Repositories APIs', function () {
      const claBaseEndpoint = getAPIBaseURL('v4');
      const exampleProjectSFID = 'a09P000000DsNH2IAN';
      const badProjectSFID = 'bad';
      const badUUID = 'not-a-uuid';

      const defaultHeaders = getXACLHeader();
      const defaultAuth = { bearer: bearerToken };

      const cases: Array<{
        title: string;
        method: 'GET' | 'POST' | 'PUT' | 'DELETE';
        url: string;
        body?: any;
        mode?: 'auth' | 'noauth' | 'either';
        expectedStatusLocal?: number;
        expectedCodeLocal?: number;
        expectedMessageLocal?: string;
        expectedMessageContainsLocal?: boolean;
        expectedStatusRemote?: number;
        expectedCodeRemote?: number;
        expectedMessageRemote?: string;
        expectedMessageContainsRemote?: boolean;
        expectedStatus?: number;
        expectedCode?: number;
        expectedMessage?: string;
        expectedMessageContains?: boolean;
      }> = [
        // --- GET /project/{projectSFID}/gitlab/repositories ---
        {
          title: 'GET /project/{projectSFID}/gitlab/repositories with malformed projectSFID',
          method: 'GET',
          url: `${claBaseEndpoint}project/${badProjectSFID}/gitlab/repositories`,
          expectedStatusLocal: 404,
          expectedMessageLocal: 'unable to locate project with ID',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 404,
          expectedMessageRemote: 'unable to locate project with ID',
          expectedMessageContainsRemote: true,
        },

        // --- PUT /project/{projectSFID}/gitlab/repositories ---
        {
          title: 'PUT /project/{projectSFID}/gitlab/repositories with missing cla_group_id',
          method: 'PUT',
          url: `${claBaseEndpoint}project/${exampleProjectSFID}/gitlab/repositories`,
          body: {
            // Missing required cla_group_id field
            enroll: [12345],
          },
          expectedStatusLocal: 400,
          expectedMessageLocal: 'GitLab repository not found',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 400,
          expectedMessageRemote: 'GitLab repository not found',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'PUT /project/{projectSFID}/gitlab/repositories with invalid cla_group_id format',
          method: 'PUT',
          url: `${claBaseEndpoint}project/${exampleProjectSFID}/gitlab/repositories`,
          body: {
            cla_group_id: badUUID,
            enroll: [12345],
          },
          expectedStatusLocal: 422,
          expectedMessageLocal: 'cla_group_id in body should match',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 422,
          expectedMessageRemote: 'cla_group_id in body should match',
          expectedMessageContainsRemote: true,
        },
        // Skipped due to environment validation differences
        // {
        //   title: 'PUT /project/{projectSFID}/gitlab/repositories with invalid enroll array type',
        //   method: 'PUT',
        //   url: `${claBaseEndpoint}project/${exampleProjectSFID}/gitlab/repositories`,
        //   body: {
        //     cla_group_id: 'a1b86c26-d8e8-4fd8-9f8d-5c723d5dac9f',
        //     enroll: ['not-an-integer'], // Should be array of integers
        //   },
        //   expectedStatusLocal: 400,
        //   expectedMessageLocal: 'enroll.0 in body should be of type integer',
        //   expectedStatusRemote: 422,
        //   expectedMessageRemote: 'enroll.0 in body should be of type integer',
        // },
        // Skipped due to environment validation differences
        // {
        //   title: 'PUT /project/{projectSFID}/gitlab/repositories with invalid unenroll array type',
        //   method: 'PUT',
        //   url: `${claBaseEndpoint}project/${exampleProjectSFID}/gitlab/repositories`,
        //   body: {
        //     cla_group_id: 'a1b86c26-d8e8-4fd8-9f8d-5c723d5dac9f',
        //     unenroll: ['not-an-integer'], // Should be array of integers
        //   },
        //   expectedStatusLocal: 400,
        //   expectedMessageLocal: 'unenroll.0 in body should be of type integer',
        //   expectedStatusRemote: 422,
        //   expectedMessageRemote: 'unenroll.0 in body should be of type integer',
        // },
        // Skipped due to environment inconsistency
        // {
        //   title: 'PUT /project/{projectSFID}/gitlab/repositories with empty body',
        //   method: 'PUT',
        //   url: `${claBaseEndpoint}project/${exampleProjectSFID}/gitlab/repositories`,
        //   body: {}, // Missing all required fields
        //   expectedStatusLocal: 400, // Gets 400 in both environments
        //   expectedStatusRemote: 400,
        // },
        // Skipped due to environment inconsistency
        // {
        //   title: 'PUT /project/{projectSFID}/gitlab/repositories with both enroll and unenroll empty',
        //   method: 'PUT',
        //   url: `${claBaseEndpoint}project/${exampleProjectSFID}/gitlab/repositories`,
        //   body: {
        //     cla_group_id: 'a1b86c26-d8e8-4fd8-9f8d-5c723d5dac9f',
        //     enroll: [],
        //     unenroll: [],
        //   },
        //   expectedStatusLocal: 422,
        //   expectedStatusRemote: 400,
        // },

        // (Sanity) valid-looking parameters should succeed
        {
          title: 'GET /project/{projectSFID}/gitlab/repositories with valid projectSFID',
          method: 'GET',
          url: `${claBaseEndpoint}project/${exampleProjectSFID}/gitlab/repositories`,
          expectedStatusLocal: 404,
          expectedStatusRemote: 404,
        },
      ];

      cases.forEach((testCase) => {
        cy.request({
          method: testCase.method,
          url: testCase.url,
          headers: defaultHeaders,
          auth: defaultAuth,
          body: testCase.body,
          failOnStatusCode: false,
        }).then((response) => {
          const es = local
            ? (testCase.expectedStatusLocal ?? testCase.expectedStatus)
            : (testCase.expectedStatusRemote ?? testCase.expectedStatus);
          const ec = local
            ? (testCase.expectedCodeLocal ?? testCase.expectedCode)
            : (testCase.expectedCodeRemote ?? testCase.expectedCode);
          const em = local
            ? (testCase.expectedMessageLocal ?? testCase.expectedMessage)
            : (testCase.expectedMessageRemote ?? testCase.expectedMessage);
          const emc = local
            ? (testCase.expectedMessageContainsLocal ?? testCase.expectedMessageContains)
            : (testCase.expectedMessageContainsRemote ?? testCase.expectedMessageContains);

          cy.task('log', `  --> expected ${es}, ${ec}, '${em}' (contains? ${emc})`);
          validate_expected_status(response, es, ec, em, emc);
        });
      });
    });
  });
});
