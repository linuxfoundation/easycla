import {
  validateApiResponse,
  validate_200_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
  validate_expected_status,
} from '../../support/commands';

// LG:
beforeEach(function () {
  cy.task('log', `>>> starting test: ${Cypress.currentTest?.title || '(unknown)'}`);
});

afterEach(function () {
  cy.task('log', `<<< finished test: ${Cypress.currentTest?.title || '(unknown)'}`);
});

describe('To Validate github-repositories API call', function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../../appConfig/config.production.ts').appConfig;
  }

  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/github-repositories

  //Variable for GitHub
  const projectSfidOrg = appConfig.childProjectSFID; //project name: easyAutom-child2

  const claEndpoint = getAPIBaseURL('v4') + `project/${projectSfidOrg}/github/repositories`;
  let claGroupId: string = appConfig.claGroupId;
  let repository_id: string = 'f577ae5d-5616-453f-a77e-9a76ff2910ec';
  let repository_external_id: string = '';
  let repository_external_id2: string = '';
  let gitHubOrgName: string = '';
  let branch_name: string = '';
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

  it('Get the GitHub repositories of the project which are CLA Enforced- Record should return 200 Response', function () {
    cy.task('log', `--> GET ${claEndpoint}`);

    cy.request({
      method: 'GET',
      url: `${claEndpoint}`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
    }).then((response) => {
      return cy
        .task('log', `--> status=${response.status}`)
        .then(() => cy.logJson('response.body', response.body))
        .then(() => validate_200_Status(response))
        .then(() => {
          // Validate specific data in the response
          expect(response.body).to.have.property('list');
          let list = response.body.list;
          repository_id = list[0].repository_id;
          claGroupId = list[0].repository_cla_group_id;
          gitHubOrgName = list[0].repository_organization_name;
          repository_external_id = list[0].repository_external_id;
          // LG: API returns 1 row
          // repository_external_id2 = list[1].repository_external_id;
          // expect(list[0].repository_name).to.eql('ApiAutomStandaloneOrg/repo01');
          repository_external_id2 = repository_external_id;
          expect(list[0].repository_name).to.eql('ApiAutomStandaloneOrg/MyProject2');
          //To validate schema of response
          validateApiResponse('github-repositories/getRepositories.json', response.body);
        });
    });
  });

  it("Remove 'disable CLA Enforced' the GitHub repository from the project - Record should return 204 Response", function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}/${repository_id}`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      expect(response.status).to.eq(204);
    });
  });

  it("User should able to Add 'CLA Enforced' a GitHub repository to the project - Record should return 200 Response", function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        cla_group_id: claGroupId,
        github_organization_name: gitHubOrgName,
        repository_github_id: repository_external_id.toString(),
        repository_github_ids: [repository_external_id.toString(), repository_external_id2.toString()],
      },
    }).then((response) => {
      validate_200_Status(response);

      // Validate specific data in the response
      expect(response.body).to.have.property('list');
      let list = response.body.list;
      repository_id = list[0].repository_id;
      claGroupId = list[0].repository_cla_group_id;
      gitHubOrgName = list[0].repository_organization_name;
      // LG: update to match existing data in 9/2025
      // expect(list[0].repository_name).to.eql('ApiAutomStandaloneOrg/repo01');
      expect(list[0].repository_name).to.eql('ApiAutomStandaloneOrg/MyProject2');

      //To validate schema of response
      validateApiResponse('github-repositories/getRepositories.json', response.body);
    });
  });

  it('Get GitHub branch protection for given repository - Record should return 200 Response', function () {
    // cy.logJson("appConfig", appConfig);
    const url = `${claEndpoint}/${repository_id}/branch-protection`;
    cy.task('log', `--> GET ${url}`);
    // cy.task("log", `--> token ${bearerToken}`);
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
      return cy
        .task('log', `--> status=${response.status}`)
        .then(() => cy.logJson('response.body', response.body))
        .then(() => validate_200_Status(response)) // assertion runs after logs are flushed
        .then(() => {
          const list = response.body;
          branch_name = list.branch_name;
          if (list.protection_enabled) {
            validateApiResponse('github-repositories/getBranchProtection.json', response.body);
          } else {
            // use cy.task so it shows in terminal logs as well
            return cy.task('log', 'branch protection is false');
          }
        });
    });
  });

  it('Update github branch protection for given repository - Record should return 200 Response', function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}/${repository_id}/branch-protection`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        branch_name: branch_name,
        enforce_admin: true,
        status_checks: [
          {
            enabled: true,
            name: 'EasyCLA',
          },
        ],
      },
    }).then((response) => {
      validate_200_Status(response);
      //To validate schema of response
      validateApiResponse('github-repositories/getBranchProtection.json', response.body);
    });
  });

  describe('Expected failures', function () {
    it('Returns 401 for all GitHub Repositories APIs when called without token', function () {
      // Use the same project SFID and repository ID as the working tests
      const projectSfidOrg = appConfig.childProjectSFID;
      const repositoryID = 'f577ae5d-5616-453f-a77e-9a76ff2910ec';

      const defaultHeaders = getXACLHeader();
      const claBaseEndpoint = getAPIBaseURL('v4');

      const requests = [
        {
          title: 'GET /project/.../github/repositories without token',
          method: 'GET',
          url: `${claBaseEndpoint}project/${projectSfidOrg}/github/repositories`,
        },
        {
          title: 'POST /project/.../github/repositories without token',
          method: 'POST',
          url: `${claBaseEndpoint}project/${projectSfidOrg}/github/repositories`,
          body: {
            cla_group_id: 'test-cla-group-id',
            github_organization_name: 'test-org',
            repository_github_id: '12345',
            repository_github_ids: ['12345'],
          },
        },
        {
          title: 'DELETE /project/.../github/repositories/... without token',
          method: 'DELETE',
          url: `${claBaseEndpoint}project/${projectSfidOrg}/github/repositories/${repositoryID}`,
        },
        {
          title: 'GET /project/.../github/repositories/.../branch-protection without token',
          method: 'GET',
          url: `${claBaseEndpoint}project/${projectSfidOrg}/github/repositories/${repositoryID}/branch-protection`,
        },
        {
          title: 'POST /project/.../github/repositories/.../branch-protection without token',
          method: 'POST',
          url: `${claBaseEndpoint}project/${projectSfidOrg}/github/repositories/${repositoryID}/branch-protection`,
          body: {
            branch_name: 'main',
            enforce_admin: true,
            status_checks: [{ enabled: true, name: 'EasyCLA' }],
          },
        },
      ];

      requests.forEach((request) => {
        cy.request({
          method: request.method,
          url: request.url,
          body: request.body,
          failOnStatusCode: false,
        }).then((response) => {
          validate_expected_status(response, 401, undefined, undefined, undefined);
        });
      });
    });

    it('Returns errors due to missing or malformed parameters for GitHub Repositories APIs', function () {
      // Use the same project SFID and repository ID as the working tests
      const projectSfidOrg = appConfig.childProjectSFID;
      const repositoryID = 'f577ae5d-5616-453f-a77e-9a76ff2910ec';

      const defaultHeaders = getXACLHeader();
      const defaultAuth = { bearer: bearerToken };
      const claBaseEndpoint = getAPIBaseURL('v4');

      const cases: Array<{
        title: string;
        method: 'GET' | 'POST' | 'PUT' | 'DELETE';
        url: string;
        body?: any;
        mode?: 'auth' | 'noauth' | 'either';
        // when running locally
        expectedStatusLocal?: number;
        expectedCodeLocal?: number;
        expectedMessageLocal?: string;
        expectedMessageContainsLocal?: boolean;
        // when running against dev via ACS & API-gw
        expectedStatusRemote?: number;
        expectedCodeRemote?: number;
        expectedMessageRemote?: string;
        expectedMessageContainsRemote?: boolean;
        // if the same
        expectedStatus?: number;
        expectedCode?: number;
        expectedMessage?: string;
        expectedMessageContains?: boolean;
      }> = [
        // (Sanity) valid-looking parameters should succeed (or at least get past validation)
        {
          title: 'GET /project/.../github/repositories with valid projectSFID',
          method: 'GET',
          url: `${claBaseEndpoint}project/${projectSfidOrg}/github/repositories`,
          expectedStatus: 200,
        },
        {
          title: 'GET /project/.../github/repositories/.../branch-protection with valid parameters',
          method: 'GET',
          url: `${claBaseEndpoint}project/${projectSfidOrg}/github/repositories/${repositoryID}/branch-protection`,
          expectedStatus: 200,
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
          cy.task(
            'log',
            `--> expected ${testCase.expectedStatus}, ${testCase.expectedCode}, '${testCase.expectedMessage}' (contains? ${testCase.expectedMessageContains})`,
          );
          validate_expected_status(
            response,
            testCase.expectedStatus,
            testCase.expectedCode,
            testCase.expectedMessage,
            testCase.expectedMessageContains,
          );
        });
      });
    });
  });
});
