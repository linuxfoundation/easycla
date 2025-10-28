import { isNull } from 'cypress/types/lodash';
import {
  validateApiResponse,
  validate_200_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
  validate_401_Status,
  validate_expected_status,
} from '../../support/commands';

describe('To Validate cla-manager API call', function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');
  const timeout = 180000;

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../../appConfig/config.production.ts').appConfig;
  }

  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/metrics
  const claEndpoint = getAPIBaseURL('v4') + 'metrics/';
  const companyID = appConfig.companyID; //infosys limited
  const companyName = appConfig.companyName; //Infosys Limited
  const projectSFID = appConfig.projectSFID; //SUN
  let projectID = appConfig.projectID;
  let claEndpointForNextKey = '';
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 60000;
  const local = Cypress.env('LOCAL');
  let bearerToken: string = null;

  before(() => {
    if (bearerToken == null) {
      getTokenKey(bearerToken);
      cy.window().then((win) => {
        bearerToken = win.localStorage.getItem('bearerToken');
      });
    }
  });

  it('Get CLA manager distribution for EasyCLA - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}cla-manager-distribution`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('metrics/getClaManagerDistribution.json', response);
    });
  });

  it('Get & Returns metrics of company - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/${companyID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      expect(response.body.companyName).to.eql(companyName);
      expect(response.body.id).to.eql(companyID);
      validateApiResponse('metrics/getCompanyMetric.json', response);
    });
  });

  it('Get & Returns metrics of company - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/${companyID}/project/${projectSFID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body.list;
      expect(list[1].companyName).to.eql(companyName);
      expect(list[1].companyID).to.eql(companyID);
      expect(list[1].projectSFID).to.eql(projectSFID);
      projectID = list[1].projectID;
      validateApiResponse('metrics/listCompanyProjectMetrics.json', response);
    });
  });

  it('List the metrics for the projects - Record should return 200 Response', function () {
    claEndpointForNextKey = `${claEndpoint}project`;
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      let NextKey = response.body.nextKey;
      validateApiResponse('metrics/listProjectMetrics.json', response);
      console.log(NextKey);
      fetchNextRecords(claEndpointForNextKey, NextKey);
    });
  });

  it('Get & Returns metrics of company - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project?=${projectID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('metrics/getProjectMetric.json', response);
    });
  });

  it('Get top company metrics - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}top-companies`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('metrics/getTopCompanies.json', response);
    });
  });

  it('Get project metrics of the top projects', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}top-projects`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('metrics/getTopProjects.json', response);
    });
  });

  it('Get total count metrics', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}total-count`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('metrics/getTotalCount.json', response);
    });
  });

  // ========================= Expected failures (metrics) =========================
  describe('Expected failures', () => {
    it('Returns 401 for all Metrics APIs when called without token', () => {
      const exampleCompanyID = 'a1b86c26-d8e8-4fd8-9f8d-5c723d5dac9f';
      const exampleProjectSFID = 'a09P000000DsNH2IAN';
      const exampleProjectID = 'sample-project-id';

      const requests = [
        // GET /metrics/cla-manager-distribution
        {
          method: 'GET',
          url: `${claEndpoint}cla-manager-distribution`,
        },
        // GET /metrics/total-count
        {
          method: 'GET',
          url: `${claEndpoint}total-count`,
        },
        // GET /metrics/company/{companyID}
        {
          method: 'GET',
          url: `${claEndpoint}company/${exampleCompanyID}`,
        },
        // GET /metrics/top-companies
        {
          method: 'GET',
          url: `${claEndpoint}top-companies`,
        },
        // GET /metrics/top-projects
        {
          method: 'GET',
          url: `${claEndpoint}top-projects`,
        },
        // GET /metrics/project/{projectID}
        {
          method: 'GET',
          url: `${claEndpoint}project/${exampleProjectID}?idType=salesforce`,
        },
        // GET /metrics/project
        {
          method: 'GET',
          url: `${claEndpoint}project`,
        },
        // GET /metrics/company/{companyID}/project/{projectSFID}
        {
          method: 'GET',
          url: `${claEndpoint}company/${exampleCompanyID}/project/${exampleProjectSFID}`,
        },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false, // expect 401 without token
            timeout,
          })
          .then((response) => {
            return cy.logJson('401 response (metrics)', response).then(() => {
              // Local environment returns JSON with code/message
              if (local) {
                expect(response.status).to.eq(401);
                expect(response.body).to.have.property('code', 401);
                expect(response.body).to.have.property('message');
                expect(response.body.message).to.contain('unauthenticated');
              } else {
                // Remote environment returns different format
                validate_401_Status(response, local);
              }
            });
          });
      });
    });

    it('Returns errors due to missing or malformed parameters for Metrics APIs', function () {
      // Helpers: realistic-looking placeholders & malformed inputs
      const exampleCompanyID = 'a1b86c26-d8e8-4fd8-9f8d-5c723d5dac9f';
      const badCompanyID = 'bad-company-id';
      const exampleProjectSFID = 'a09P000000DsNH2IAN';
      const badProjectSFID = 'bad-project-id';
      const exampleProjectID = 'sample-project-id';
      const badProjectID = 'bad-project-id';
      const badUUID = 'not-a-valid-uuid';

      const defaultHeaders = getXACLHeader();
      const defaultAuth = { bearer: bearerToken };

      const cases: Array<{
        title: string;
        method: 'GET' | 'POST' | 'PUT' | 'DELETE';
        url: string;
        body?: any;
        headers?: any;
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
      }> = [
        {
          title: 'POST /metrics/cla-manager-distribution (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}cla-manager-distribution`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method POST is not allowed, but [GET] are',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'PUT /metrics/total-count (method not allowed)',
          method: 'PUT',
          url: `${claEndpoint}total-count`,
          body: {},
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method PUT is not allowed, but [GET] are',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'DELETE /metrics/top-companies (method not allowed)',
          method: 'DELETE',
          url: `${claEndpoint}top-companies`,
          expectedStatusLocal: 405,
          expectedMessageLocal: 'method DELETE is not allowed, but [GET] are',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /metrics/invalid-endpoint (not found)',
          method: 'GET',
          url: `${claEndpoint}invalid-endpoint`,
          expectedStatusLocal: 404,
          expectedMessageLocal: 'path /v4/metrics/invalid-endpoint was not found',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path',
          expectedMessageContainsRemote: true,
        },
      ];

      cy.wrap(cases).each((c: any) => {
        return cy
          .request({
            method: c.method,
            url: c.url,
            body: c.body,
            headers: c.headers || defaultHeaders,
            auth: defaultAuth,
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            cy.task('log', `Testing: ${c.title}`);

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

  //List the metrics for the projects
  function fetchNextRecords(URL, NextKey) {
    if (NextKey !== undefined) {
      cy.request({
        method: 'GET',
        url: `${URL}?nextKey=${NextKey}&pageSize=100`,
        timeout: timeout,
        failOnStatusCode: allowFail,
        headers: getXACLHeader(),
        auth: {
          bearer: bearerToken,
        },
      }).then((response) => {
        validate_200_Status(response);

        // Validate specific data in the response
        let updatedNextKey = response.body.nextKey;
        if (updatedNextKey !== undefined) {
          fetchNextRecords(URL, updatedNextKey);
        }
      });
    }
  }
});
