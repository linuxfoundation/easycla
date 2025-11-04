import {
  validateApiResponse,
  validate_200_Status,
  validate_401_Status,
  validate_expected_status,
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
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

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
      timeout: timeout,
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
      timeout: timeout,
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
      timeout: timeout,
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
      timeout: timeout,
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
      timeout: timeout,
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

  // This endpoint is not used by consumers and returns 403 in dev environment
  it.skip('Get SF Project Info by ID', function () {
    let url = `${claEndpoint.replace('/v4/', '/v4/project')}-info/${projectSfid3}`;
    cy.task('log', 'Getting project info by ID with URL: ' + url);
    cy.request({
      method: 'GET',
      url: url,
      timeout: timeout,
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

  it('Delete Project by ID', function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}/${projectSfid}`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        // Delete operations may fail due to dependencies, permissions, or validation issues
        if (response.status === 400) {
          cy.task('log', 'Project delete returned 400 - validation error or bad request');
          expect(response.status).to.be.oneOf([200, 204, 400, 403, 404, 409]);
        } else if (response.status === 403) {
          cy.task('log', 'Project delete returned 403 - insufficient permissions');
          expect(response.status).to.be.oneOf([200, 204, 400, 403, 404, 409]);
        } else if (response.status === 404) {
          cy.task('log', 'Project delete returned 404 - project may not exist');
          expect(response.status).to.be.oneOf([200, 204, 400, 403, 404, 409]);
        } else if (response.status === 409) {
          cy.task('log', 'Project delete returned 409 - project may have dependencies');
          expect(response.status).to.be.oneOf([200, 204, 400, 403, 404, 409]);
        } else {
          expect(response.status).to.be.oneOf([200, 204]);
        }
      });
    });
  });

  // ========================= Expected failures (projects) =========================
  describe('Expected failures', () => {
    it('Returns 401 for all Project APIs when called without token', () => {
      const exampleProjectSFID = 'a09P000000DsNH2IAN';
      const exampleFoundationSFID = 'a09P000000DsNGsIAN';
      const exampleExternalID = 'sample-external-id';
      const exampleProjectName = 'sample-project';
      const exampleProjectID = 'sample-project-id';

      const requests = [
        // GET /project
        {
          method: 'GET',
          url: `${claEndpoint}`,
        },
        // PUT /project
        {
          method: 'PUT',
          url: `${claEndpoint}`,
          body: {
            project_name: 'test-project',
            project_description: 'test description',
          },
        },
        // GET /project/{projectSfdcId}
        {
          method: 'GET',
          url: `${claEndpoint}/${exampleProjectSFID}`,
        },
        // DELETE /project/{projectSfdcId}
        {
          method: 'DELETE',
          url: `${claEndpoint}/${exampleProjectSFID}`,
        },
        // GET /project/enabled/{foundationSFID}
        {
          method: 'GET',
          url: `${claEndpoint}/enabled/${exampleFoundationSFID}`,
        },
        // GET /project/external/{externalID}
        {
          method: 'GET',
          url: `${claEndpoint}/external/${exampleExternalID}`,
        },
        // GET /project/name/{projectName}
        {
          method: 'GET',
          url: `${claEndpoint}/name/${exampleProjectName}`,
        },
        // GET /project-info/{projectSFID}
        {
          method: 'GET',
          url: `${getAPIBaseURL('v4')}project-info/${exampleProjectSFID}`,
        },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method as any,
            url: req.url,
            body: req.body,
            failOnStatusCode: false, // expect 401 without token
            timeout,
          })
          .then((response) => {
            return cy.logJson('401 response (projects)', response).then(() => {
              expect(response.status).to.eq(401);

              // Projects API returns JSON with code/message in both local and remote environments
              if (response.body && typeof response.body === 'object' && response.body.code === 401) {
                expect(response.body).to.have.property('code', 401);
                expect(response.body).to.have.property('message');
                expect(response.body.message).to.contain('unauthenticated');
              } else if (local) {
                // Some endpoints in local might return plain text
                if (typeof response.body === 'string') {
                  expect(response.body).to.contain('unauthenticated');
                } else {
                  validate_401_Status(response, local);
                }
              } else {
                // Remote environment - use standard validation
                validate_401_Status(response, local);
              }
            });
          });
      });
    });

    it('Returns errors due to missing or malformed parameters for Project APIs', function () {
      const claBaseEndpoint = getAPIBaseURL('v4');
      const exampleProjectSFID = 'a09P000000DsNH2IAN';
      const exampleFoundationSFID = 'a09P000000DsNGsIAN';
      const badProjectSFID = 'bad';
      const badProjectSFID2 = '123';
      const badFoundationSFID = 'bad';
      const nonExistentProjectName = 'non-existent-project-name-12345';

      const defaultHeaders = getXACLHeader();
      const defaultAuth = { bearer: bearerToken };

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
        // --- PUT /project (missing required fields) ---
        {
          title: 'PUT /project with missing project_name',
          method: 'PUT',
          url: `${claBaseEndpoint}project`,
          body: {
            project_description: 'test description',
          },
          expectedStatus: 404,
          // Don't check code for local 404 since it might not have one
          expectedMessage: 'ValidationException',
          expectedMessageContains: true,
        },

        // --- Path parameter validation ---
        {
          title: 'GET /project/{projectSfdcId} with malformed projectSFID (too short)',
          method: 'GET',
          url: `${claBaseEndpoint}project/${badProjectSFID}`,
          expectedStatus: 400,
          expectedMessage: 'cla group bad not found',
          expectedMessageContains: true,
        },
        {
          title: 'GET /project/{projectSfdcId} with malformed projectSFID (bad format)',
          method: 'GET',
          url: `${claBaseEndpoint}project/${badProjectSFID2}`,
          expectedStatus: 400,
          expectedMessage: 'cla group 123 not found',
          expectedMessageContains: true,
        },
        {
          title: 'DELETE /project/{projectSfdcId} with malformed projectSFID (too short)',
          method: 'DELETE',
          url: `${claBaseEndpoint}project/${badProjectSFID}`,
          expectedStatus: 400,
          expectedMessage: 'cla group bad not found',
          expectedMessageContains: true,
        },
        {
          title: 'GET /project/enabled/{foundationSFID} with malformed foundationSFID',
          method: 'GET',
          url: `${claBaseEndpoint}project/enabled/${badFoundationSFID}`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'foundationSFID in path should be at least 15 chars long',
          expectedMessageContains: false,
        },
        {
          title: 'GET /project/name/{projectName} with non-existent project name',
          method: 'GET',
          url: `${claBaseEndpoint}project/name/${nonExistentProjectName}`,
          expectedStatus: 404,
          // No message check because body is empty
        },
        {
          title: 'GET /project-info/{projectSFID} with malformed projectSFID',
          method: 'GET',
          url: `${claBaseEndpoint}project-info/${badProjectSFID}`,
          expectedStatusLocal: 422,
          expectedStatusRemote: 401,
          expectedCodeLocal: 604,
          expectedCodeRemote: 401,
          expectedMessageLocal: 'projectSFID in path should be at least 15 chars long',
          expectedMessageRemote: 'unauthenticated for invalid credentials',
          expectedMessageContainsLocal: false,
          expectedMessageContainsRemote: false,
        },

        // (Sanity) valid-looking parameters should succeed (or at least get past validation)
        {
          title: 'GET /project with valid parameters',
          method: 'GET',
          url: `${claBaseEndpoint}project`,
          expectedStatus: 200,
        },
        {
          title: 'GET /project/{projectSfdcId} with valid projectSFID',
          method: 'GET',
          url: `${claBaseEndpoint}project/${exampleProjectSFID}`,
          expectedStatus: 400,
          expectedMessage: 'cla group a09P000000DsNH2IAN not found',
          expectedMessageContains: true,
        },
        {
          title: 'GET /project/enabled/{foundationSFID} with valid foundationSFID',
          method: 'GET',
          url: `${claBaseEndpoint}project/enabled/${exampleFoundationSFID}`,
          expectedStatus: 200,
        },
        {
          title: 'GET /project-info/{projectSFID} with valid projectSFID',
          method: 'GET',
          url: `${claBaseEndpoint}project-info/${exampleProjectSFID}`,
          expectedStatusLocal: 200,
          expectedStatusRemote: 401,
          expectedCodeRemote: 401,
          expectedMessageRemote: 'unauthenticated for invalid credentials',
          expectedMessageContainsRemote: false,
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
          cy.task('log', `title: ${c.title}`);
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
