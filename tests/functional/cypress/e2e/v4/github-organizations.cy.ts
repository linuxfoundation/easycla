import {
  validateApiResponse,
  validate_200_Status,
  validate_401_Status,
  validate_expected_status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
} from '../../support/commands';
describe('To Validate github-organizations API call', function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../../appConfig/config.production.ts').appConfig;
  }
  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/github-organizations

  //Variable for GitHub
  const gitHubOrgName = appConfig.gitHubOrgUpdate;
  const projectSfidOrg = appConfig.childProjectSFID; //project name: easyAutom-child2
  const gitHubOrg = appConfig.gitHubNewOrg;

  const claEndpoint = getAPIBaseURL('v4') + `project/${projectSfidOrg}/github/organizations`;
  const claGroupId: string = appConfig.claGroupId;
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const local = Cypress.env('LOCAL') ? true : false;
  const timeout = 180000;

  let bearerToken: string = null;
  before(() => {
    if (bearerToken == null) {
      getTokenKey(bearerToken);
      cy.window().then((win) => {
        bearerToken = win.localStorage.getItem('bearerToken');
      });
    }
  });

  it('Get list of Github organization associated with project - Record should return 200 Response', function () {
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

      // Validate specific data in the response
      expect(response.body).to.have.property('list');
      let list = response.body.list;
      expect(list[0].github_organization_name).to.eql('ApiAutomStandaloneOrg');
      expect(list[0].connection_status).to.eql('connected');
      //To validate schema of response
      validateApiResponse('github-organizations/getProjectGithubOrganizations.json', response.body);
    });
  });

  it('Update GitHub Organization Configuration - Record should return 200 Response', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}/${gitHubOrgName}/config`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        autoEnabled: true,
        autoEnabledClaGroupID: claGroupId,
        branchProtectionEnabled: true,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it('Add new GitHub Oranization in the project - Record should return 200 Response', function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        autoEnabled: false,
        autoEnabledClaGroupID: claGroupId,
        branchProtectionEnabled: false,
        organizationName: gitHubOrg,
      },
    }).then((response) => {
      cy.logJson('response', response).then(() => {
        validate_200_Status(response);

        // Validate specific data in the response
        expect(response.body.organizationName).to.eql(gitHubOrg);
        //To validate schema of response
        validateApiResponse('github-organizations/addProjectGithubOrganization.json', response.body);
      });
    });
  });

  it('Delete GitHub oranization in the project - Record should return 204 Response', function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}/${gitHubOrg}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      expect(response.status).to.eq(204);
    });
  });

  // ========================= Expected failures (github-organizations) =========================
  describe('Expected failures', () => {
    it('Returns 401 for all GitHub Organizations APIs when called without token', () => {
      // Use the same base as the rest of this spec:
      const claBaseEndpoint = getAPIBaseURL('v4');

      // Dummy-but-plausible ids/names so server can fail on auth first
      const exampleProjectSFID = '001000000000000AAA'; // plausible SFID shape
      const exampleOrgName = 'test-org-123'; // valid org name pattern

      // NOTE: Endpoints below are ONLY those that require auth in Swagger.
      // Endpoints with security: [] are intentionally excluded.

      const requests = [
        // GET /project/{projectSFID}/github/organizations
        { method: 'GET', url: `${claBaseEndpoint}project/${exampleProjectSFID}/github/organizations` },

        // POST /project/{projectSFID}/github/organizations
        {
          method: 'POST',
          url: `${claBaseEndpoint}project/${exampleProjectSFID}/github/organizations`,
          body: {
            organizationName: exampleOrgName,
            autoEnabled: false,
            branchProtectionEnabled: false,
          },
        },

        // PUT /project/{projectSFID}/github/organizations/{orgName}/config
        {
          method: 'PUT',
          url: `${claBaseEndpoint}project/${exampleProjectSFID}/github/organizations/${exampleOrgName}/config`,
          body: {
            autoEnabled: true,
            branchProtectionEnabled: true,
          },
        },

        // DELETE /project/{projectSFID}/github/organizations/{orgName}
        {
          method: 'DELETE',
          url: `${claBaseEndpoint}project/${exampleProjectSFID}/github/organizations/${exampleOrgName}`,
        },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method as any,
            url: req.url,
            body: req.body,
            failOnStatusCode: false, // expect 401
            timeout,
          })
          .then((response) => {
            return cy.logJson('401 response (github-organizations)', response).then(() => {
              validate_401_Status(response, local);
            });
          });
      });
    });

    it('Returns errors due to missing or malformed parameters for GitHub Organizations APIs', function () {
      // Use the same project SFID as the working tests
      const projectSfidOrg = appConfig.childProjectSFID;
      const exampleOrgName = 'test-org-123';

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
        // --- POST /project/{projectSFID}/github/organizations (missing required fields) ---
        {
          title: 'POST /project/.../github/organizations with missing organizationName',
          method: 'POST',
          url: `${claBaseEndpoint}project/${projectSfidOrg}/github/organizations`,
          body: {
            autoEnabled: false,
            branchProtectionEnabled: false,
          },
          expectedStatus: 422,
          expectedCode: 602,
          expectedMessage: 'organizationName in body is required',
          expectedMessageContains: false,
        },

        // --- PUT /project/{projectSFID}/github/organizations/{orgName}/config (missing required fields) ---
        {
          title: 'PUT /project/.../github/organizations/.../config with missing autoEnabled',
          method: 'PUT',
          url: `${claBaseEndpoint}project/${projectSfidOrg}/github/organizations/${exampleOrgName}/config`,
          body: {
            branchProtectionEnabled: true,
          },
          expectedStatus: 422,
          expectedCode: 602,
          expectedMessage: 'autoEnabled in body is required',
          expectedMessageContains: false,
        },

        // (Sanity) valid-looking parameters should succeed (or at least get past validation)
        {
          title: 'GET /project/.../github/organizations with valid projectSFID',
          method: 'GET',
          url: `${claBaseEndpoint}project/${projectSfidOrg}/github/organizations`,
          expectedStatus: 200,
        },
      ];

      cy.wrap(cases).each((c: any) => {
        cy.task('log', `--> ${c.title} | ${c.method} ${c.url}`);
        const opts: any = {
          method: c.method,
          url: c.url,
          headers: defaultHeaders,
          auth: defaultAuth,
          failOnStatusCode: false,
          timeout,
        };
        if (c.body) opts.body = c.body;

        cy.request(opts).then((response) => {
          return cy.logJson('response', response).then(() => {
            const es = local
              ? (c.expectedStatusLocal ?? c.expectedStatus)
              : (c.expectedStatusRemote ?? c.expectedStatus);
            const ec = local ? (c.expectedCodeLocal ?? c.expectedCode) : (c.expectedCodeRemote ?? c.expectedCode);
            const em = local
              ? (c.expectedMessageLocal ?? c.expectedMessage)
              : (c.expectedMessageRemote ?? c.expectedMessage);
            const emc = local
              ? (c.expectedMessageContainsLocal ?? c.expectedMessageContains)
              : (c.expectedMessageContainsRemote ?? c.expectedMessageContains);

            cy.task('log', `  --> expected ${es}, ${ec}, '${em}' (contains? ${emc})`);
            validate_expected_status(response, es, ec, em, emc);
          });
        });
      });
    });
  });
});
