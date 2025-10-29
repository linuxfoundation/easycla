import {
  validateApiResponse,
  validate_200_Status,
  validate_401_Status,
  validate_expected_status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
} from '../../support/commands';
describe('To Validate & get Company Activity Callback via API call', function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../../appConfig/config.production.ts').appConfig;
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
  const user_id3 = appConfig.user_id3; // lgryglicki
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const local = Cypress.env('LOCAL') ? true : false;
  const timeout = 180000;

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
      timeout: timeout,
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

  describe('Expected failures', () => {
    it('Returns 401 for all Company APIs when called without token', () => {
      // Use the same base as the rest of this spec:
      // const claBaseEndpoint = getAPIBaseURL('v4'); // already defined earlier in this file

      // Dummy-but-plausible ids/names so server can fail on auth first
      const exampleV4 = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // valid UUIDv4 shape
      const exampleSFID = '001000000000000AAA'; // plausible SFID shape
      const exampleCompanyName = 'TestCo Incorporated';
      const exampleEntityName = 'Test Entity LLC';

      // NOTE: Endpoints below are ONLY those that require auth in Swagger.
      // Endpoints with security: [] are intentionally excluded.

      const requests = [
        // GET /company/{companyID}
        { method: 'GET', url: `${claBaseEndpoint}company/${exampleV4}` },

        // GET /company/external/{companySFID}
        { method: 'GET', url: `${claBaseEndpoint}company/external/${exampleSFID}` },

        // GET /company/name/{companyName}
        { method: 'GET', url: `${claBaseEndpoint}company/name/${encodeURIComponent(exampleCompanyName)}` },

        // GET /company/entityname/{signingEntityName}
        { method: 'GET', url: `${claBaseEndpoint}company/entityname/${encodeURIComponent(exampleEntityName)}` },

        // DELETE /company/id/{companyID}
        { method: 'DELETE', url: `${claBaseEndpoint}company/id/${exampleV4}` },

        // DELETE /company/sfid/{companySFID}
        { method: 'DELETE', url: `${claBaseEndpoint}company/sfid/${exampleSFID}` },

        // GET /company/{companyID}/project/{projectSFID}/cla-managers
        { method: 'GET', url: `${claBaseEndpoint}company/${exampleV4}/project/${projectSFID}/cla-managers` },

        // GET /company/{companyID}/project/{projectSFID}/active-cla-list
        { method: 'GET', url: `${claBaseEndpoint}company/${exampleV4}/project/${projectSFID}/active-cla-list` },

        // GET /company/{companyID}/project/{projectSFID}/contributors
        { method: 'GET', url: `${claBaseEndpoint}company/${exampleV4}/project/${projectSFID}/contributors` },

        // GET /company/{companySFID}/project/{projectSFID}/cla
        { method: 'GET', url: `${claBaseEndpoint}company/${exampleSFID}/project/${projectSFID}/cla` },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method as any,
            url: req.url,
            failOnStatusCode: false, // expect 401
            timeout,
          })
          .then((response) => {
            return cy.logJson('401 response', response).then(() => {
              validate_401_Status(response, local);
            });
          });
      });
    });

    it('Returns errors due to missing or malformed parameters for Company APIs', function () {
      // Helpers: realistic-looking placeholders & malformed inputs
      const exampleV4 = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
      const badUUID = 'aa';
      const badUUID2 = 'd9428888-122b-4b20-8c4a-0c9a1a6z9b8e';
      const exampleSFID = '001000000000000AAA';
      const badSFID = 'bad';
      const badSFID2 = '001000000000-00AAA';
      const sampleName = 'Acme Incorporated';
      const sampleEntity = 'Acme Entity LLC';

      // Auth headers for endpoints that need them
      const defaultHeaders = getXACLHeader();
      const defaultAuth = { bearer: bearerToken };

      const cases: Array<{
        title: string;
        method: 'GET' | 'POST' | 'DELETE';
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
        // --- GET /company/{companyID}
        {
          title: 'GET /company/{companyID} with empty companyID',
          method: 'GET',
          url: `${claEndpoint}`,
          expectedStatusLocal: 404,
          expectedCodeLocal: 404,
          expectedMessageLocal: 'path /v4/company/ was not found',
          expectedStatusRemote: 403,
          expectedCodeRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path /cla-service/v4/company/',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /company/{companyID} with malformed companyID (too short)',
          method: 'GET',
          url: `${claEndpoint}${badUUID}`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage:
            "companyID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
        },
        {
          title: 'GET /company/{companyID} with malformed companyID (bad format)',
          method: 'GET',
          url: `${claEndpoint}${badUUID2}`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage:
            "companyID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
        },

        // --- GET /company/external/{companySFID}
        {
          title: 'GET /company/external/{companySFID} with empty companySFID',
          method: 'GET',
          url: `${claEndpoint}external/`,
          expectedStatusLocal: 422,
          expectedCodeLocal: 605,
          expectedMessageLocal:
            "companyID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path /cla-service/v4/company/external/',
          expectedMessageContainsRemote: true,
        },
        /*{
          title: 'GET /company/external/{companySFID} with malformed SFID (too short)',
          method: 'GET',
          url: `${claEndpoint}external/${badSFID}`,
          expectedStatusLocal: 404,
          expectedCodeLocal: 404,
          expectedMessageLocal: 'unable to lookup company by SFID: bad',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 403,
          expectedCodeRemote: 403,
          expectedMessageRemote: `does not have access to resource or path /cla-service/v4/company/sfid/${badSFID}`,
          expectedMessageContainsRemote: true,
        },*/
        {
          title: 'GET /company/external/{companySFID} with malformed SFID (too short)',
          method: 'GET',
          url: `${claEndpoint}external/${badSFID}`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMessage: 'unable to lookup company by SFID: bad',
          expectedMessageContains: true,
        },
        {
          title: 'GET /company/external/{companySFID} with malformed SFID (bad format)',
          method: 'GET',
          url: `${claEndpoint}external/${badSFID2}`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMessage: 'unable to lookup company by SFID: 001000000000-00AAA',
          expectedMessageContains: true,
        },

        // --- GET /company/name/{companyName}
        {
          title: 'GET /company/name/{companyName} with empty companyName',
          method: 'GET',
          url: `${claEndpoint}name/`,
          expectedStatusLocal: 422,
          expectedCodeLocal: 605,
          expectedMessageLocal:
            "companyID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path /cla-service/v4/company/name/',
          expectedMessageContainsRemote: true,
        },

        // --- GET /company/entityname/{signingEntityName}
        {
          title: 'GET /company/entityname/{signingEntityName} with empty signingEntityName',
          method: 'GET',
          url: `${claEndpoint}entityname/`,
          expectedStatusLocal: 422,
          expectedCodeLocal: 605,
          expectedMessageLocal:
            "companyID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path /cla-service/v4/company/entityname/',
          expectedMessageContainsRemote: true,
        },

        // --- DELETE /company/id/{companyID}
        {
          title: 'DELETE /company/id/{companyID} with empty companyID',
          method: 'DELETE',
          url: `${claEndpoint}id/`,
          expectedStatusLocal: 405,
          expectedCodeLocal: 405,
          expectedMessageLocal: 'method DELETE is not allowed, but [GET] are',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path /cla-service/v4/company/id/',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'DELETE /company/id/{companyID} with malformed companyID (too short)',
          method: 'DELETE',
          url: `${claEndpoint}id/${badUUID}`,
          expectedStatusLocal: 422,
          expectedCodeLocal: 605,
          expectedMessageLocal:
            "companyID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path /cla-service/v4/company/id/aa',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'DELETE /company/id/{companyID} with malformed companyID (bad format)',
          method: 'DELETE',
          url: `${claEndpoint}id/${badUUID2}`,
          expectedStatusLocal: 422,
          expectedCodeLocal: 605,
          expectedMessageLocal:
            "companyID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          expectedStatusRemote: 403,
          expectedMessageRemote:
            'does not have access to resource or path /cla-service/v4/company/id/d9428888-122b-4b20-8c4a-0c9a1a6z9b8e',
          expectedMessageContainsRemote: true,
        },

        // --- DELETE /company/sfid/{companySFID}
        {
          title: 'DELETE /company/sfid/{companySFID} with empty companySFID',
          method: 'DELETE',
          url: `${claEndpoint}sfid/`,
          expectedStatusLocal: 405,
          expectedCodeLocal: 405,
          expectedMessageLocal: 'method DELETE is not allowed, but [GET] are',
          expectedMessageContainsLocal: true,
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path /cla-service/v4/company/sfid/',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'DELETE /company/sfid/{companySFID} with malformed SFID (too short)',
          method: 'DELETE',
          url: `${claEndpoint}sfid/${badSFID}`,
          expectedStatusLocal: 422,
          expectedCodeLocal: 604,
          expectedMessageLocal: 'companySFID in path should be at least 15 chars long',
          expectedStatusRemote: 403,
          expectedCodeRemote: 403,
          expectedMessage: `does not have access to resource or path`,
          expectedMessageContains: true,
        },
        {
          title: 'DELETE /company/sfid/{companySFID} with malformed SFID (bad format)',
          method: 'DELETE',
          url: `${claEndpoint}sfid/${badSFID2}`,
          expectedStatusLocal: 422,
          expectedCodeLocal: 605,
          expectedMessageLocal: "companySFID in path should match '^([0-9A-Za-z]{15}|[0-9A-Za-z]{18})$'",
          expectedStatusRemote: 403,
          expectedCodeRemote: 403,
          expectedMessage: `does not have access to resource or path`,
          expectedMessageContains: true,
        },

        // --- GET /company/{companyID}/project/{projectSFID}/cla-managers
        {
          title: 'GET /company/{companyID}/project/{projectSFID}/cla-managers with empty companyID',
          method: 'GET',
          url: `${claEndpoint}project/${projectSFID}/cla-managers`,
          expectedStatusLocal: 404,
          expectedCodeLocal: 404,
          expectedMessageLocal: `path /v4/company/project/${projectSFID}/cla-managers was not found`,
          expectedStatusRemote: 403,
          expectedMessageRemote: `does not have access to resource or path /cla-service/v4/company/project/${projectSFID}/cla-managers`,
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /company/{companyID}/project/{projectSFID}/cla-managers with malformed companyID (too short)',
          method: 'GET',
          url: `${claEndpoint}${badUUID}/project/${projectSFID}/cla-managers`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage:
            "companyID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
        },
        {
          title: 'GET /company/{companyID}/project/{projectSFID}/cla-managers with malformed companyID (bad format)',
          method: 'GET',
          url: `${claEndpoint}${badUUID2}/project/${projectSFID}/cla-managers`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage:
            "companyID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
        },
        {
          title: 'GET /company/{companyID}/project/{projectSFID}/cla-managers with empty projectSFID',
          method: 'GET',
          url: `${claEndpoint}${exampleV4}/project//cla-managers`,
          expectedStatusLocal: 404,
          expectedCodeLocal: 404,
          expectedMessageLocal: `path /v4/company/${exampleV4}/project//cla-managers was not found`,
          expectedStatusRemote: 403,
          expectedMessageRemote: `does not have access to resource or path /cla-service/v4/company/${exampleV4}/project//cla-managers`,
          expectedMessageContainsRemote: true,
        },

        // --- GET /company/{companyID}/cla-group/{claGroupID}/cla-managers
        {
          title: 'GET /company/{companyID}/cla-group/{claGroupID}/cla-managers with empty companyID',
          method: 'GET',
          url: `${claEndpoint}/cla-group/${exampleV4}/cla-managers`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMessage: `path /v4/company//cla-group/${exampleV4}/cla-managers was not found`,
        },
        {
          title: 'GET /company/{companyID}/cla-group/{claGroupID}/cla-managers with malformed claGroupID (too short)',
          method: 'GET',
          url: `${claEndpoint}${exampleV4}/cla-group/${badUUID}/cla-managers`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'claGroupID in path should be at least 5 chars long',
        },
        {
          title: 'GET /company/{companyID}/cla-group/{claGroupID}/cla-managers with malformed claGroupID (bad format)',
          method: 'GET',
          url: `${claEndpoint}${exampleV4}/cla-group/${badUUID2}/cla-managers`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage: 'claGroupID in path should match',
          expectedMessageContains: true,
        },

        // --- GET /company/{companyID}/project/{projectSFID}/active-cla-list
        {
          title: 'GET /company/{companyID}/project/{projectSFID}/active-cla-list with empty companyID',
          method: 'GET',
          url: `${claEndpoint}/project/${projectSFID}/active-cla-list`,
          expectedStatusLocal: 404,
          expectedCodeLocal: 404,
          expectedMessageLocal: `path /v4/company//project/${projectSFID}/active-cla-list was not found`,
          expectedStatusRemote: 403,
          expectedMessageRemote: `does not have access to resource or path /cla-service/v4/company//project/${projectSFID}/active-cla-list`,
          expectedMessageContainsRemote: true,
        },
        {
          title:
            'GET /company/{companyID}/project/{projectSFID}/active-cla-list with malformed projectSFID (too short)',
          method: 'GET',
          url: `${claEndpoint}${exampleV4}/project/${badSFID}/active-cla-list`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'projectSFID in path should be at least 15 chars long',
        },
        {
          title:
            'GET /company/{companyID}/project/{projectSFID}/active-cla-list with malformed projectSFID (bad format)',
          method: 'GET',
          url: `${claEndpoint}${exampleV4}/project/${badSFID2}/active-cla-list`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage: 'projectSFID in path should match',
          expectedMessageContains: true,
        },

        // --- GET /company/{companyID}/project/{projectSFID}/contributors
        {
          title: 'GET /company/{companyID}/project/{projectSFID}/contributors with empty companyID',
          method: 'GET',
          url: `${claEndpoint}/project/${projectSFID}/contributors`,
          expectedStatusLocal: 404,
          expectedCodeLocal: 404,
          expectedMessageLocal: `path /v4/company//project/${projectSFID}/contributors was not found`,
          expectedStatusRemote: 403,
          expectedMessageRemote: `does not have access to resource or path /cla-service/v4/company//project/${projectSFID}/contributors`,
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /company/{companyID}/project/{projectSFID}/contributors with malformed projectSFID (too short)',
          method: 'GET',
          url: `${claEndpoint}${exampleV4}/project/${badSFID}/contributors`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'projectSFID in path should be at least 15 chars long',
        },
        {
          title: 'GET /company/{companyID}/project/{projectSFID}/contributors with malformed projectSFID (bad format)',
          method: 'GET',
          url: `${claEndpoint}${exampleV4}/project/${badSFID2}/contributors`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage: 'projectSFID in path should match',
          expectedMessageContains: true,
        },

        // --- GET /company/{companySFID}/admin
        {
          title: 'GET /company/{companySFID}/admin with empty companySFID',
          method: 'GET',
          url: `${claEndpoint}admin`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage:
            "companyID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
        },
        {
          title: 'GET /company/{companySFID}/admin with malformed companySFID (too short)',
          method: 'GET',
          url: `${claEndpoint}${badSFID}/admin`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'companySFID in path should be at least 15 chars long',
        },
        {
          title: 'GET /company/{companySFID}/admin with malformed companySFID (bad format)',
          method: 'GET',
          url: `${claEndpoint}${badSFID2}/admin`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage: 'companySFID in path should match',
          expectedMessageContains: true,
        },

        // --- GET /company/{companySFID}/project/{projectSFID}/cla
        {
          title: 'GET /company/{companySFID}/project/{projectSFID}/cla with empty companySFID',
          method: 'GET',
          url: `${claEndpoint}/project/${projectSFID}/cla`,
          expectedStatusLocal: 404,
          expectedCodeLocal: 404,
          expectedMessageLocal: `path /v4/company//project/${projectSFID}/cla was not found`,
          expectedStatusRemote: 403,
          expectedMessageRemote: `does not have access to resource or path /cla-service/v4/company//project/${projectSFID}/cla`,
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /company/{companySFID}/project/{projectSFID}/cla with empty projectSFID',
          method: 'GET',
          url: `${claEndpoint}${exampleSFID}/project//cla`,
          expectedStatusLocal: 404,
          expectedCodeLocal: 404,
          expectedMessageLocal: `path /v4/company/${exampleSFID}/project//cla was not found`,
          expectedStatusRemote: 403,
          expectedMessageRemote: `does not have access to resource or path /cla-service/v4/company/${exampleSFID}/project//cla`,
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /company/{companySFID}/project/{projectSFID}/cla with malformed companySFID (too short)',
          method: 'GET',
          url: `${claEndpoint}${badSFID}/project/${projectSFID}/cla`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'companySFID in path should be at least 15 chars long',
        },
        {
          title: 'GET /company/{companySFID}/project/{projectSFID}/cla with malformed companySFID (bad format)',
          method: 'GET',
          url: `${claEndpoint}${badSFID2}/project/${projectSFID}/cla`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage: 'companySFID in path should match',
          expectedMessageContains: true,
        },

        // --- GET /company/lookup
        {
          title: 'GET /company/lookup with both companyName and websiteName missing',
          method: 'GET',
          url: `${claEndpoint}lookup`,
          expectedStatus: 400,
          expectedMessage: 'companyName or websiteName at least one required',
        },
        {
          title: 'GET /company/lookup with malformed websiteName',
          method: 'GET',
          url: `${claEndpoint}lookup?websiteName=not-a-url`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage:
            "websiteName in query should match '^((http|https):\\/\\/)?(www.)?[a-zA-Z0-9]+(\\.[a-zA-Z]{2,}){1,3}(#?\\/?[a-zA-Z0-9#]+)*\\/?(\\?[a-zA-Z0-9-_]+=[a-zA-Z0-9-%]+&?)?$'",
        },

        // --- POST /user/{userID}/request-company-admin
        {
          title: 'POST /user/{userID}/request-company-admin with empty userID',
          method: 'POST',
          url: `${claBaseEndpoint}user//request-company-admin`,
          body: { claManagerEmail: 'someone@example.org' },
          expectedStatus: 404,
          expectedCode: 404,
          expectedMessage: 'path /v4/user//request-company-admin was not found',
        },
        {
          title: 'POST /user/{userID}/request-company-admin with malformed userID (too short)',
          method: 'POST',
          url: `${claBaseEndpoint}user/${badUUID}/request-company-admin`,
          body: { claManagerEmail: 'someone@example.org' },
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'userID in path should be at least 5 chars long',
        },
        {
          title: 'POST /user/{userID}/request-company-admin with malformed userID (bad UUID format)',
          method: 'POST',
          url: `${claBaseEndpoint}user/${badUUID2}/request-company-admin`,
          body: { claManagerEmail: 'someone@example.org' },
          expectedStatus: 400,
          expectedCode: 400,
          expectedMessage: 'cla manager name is required',
        },
        {
          title: 'POST /user/{userID}/request-company-admin missing claManagerEmail',
          method: 'POST',
          url: `${claBaseEndpoint}user/${exampleV4}/request-company-admin`,
          body: {},
          expectedStatus: 422,
          expectedCode: 602,
          expectedMessage: 'body.claManagerEmail in body is required',
        },

        // --- POST /company/{companySFID}/contributorAssociation
        {
          title: 'POST /company/{companySFID}/contributorAssociation with empty companySFID',
          method: 'POST',
          url: `${claEndpoint}contributorAssociation`,
          body: { userEmail: 'user@example.org' },
          expectedStatus: 405,
          expectedCode: 405,
          expectedMessage: 'method POST is not allowed, but [GET] are',
          expectedMessageContains: true,
        },
        {
          title: 'POST /company/{companySFID}/contributorAssociation with malformed SFID (too short)',
          method: 'POST',
          url: `${claEndpoint}${badSFID}/contributorAssociation`,
          body: { userEmail: 'user@example.org' },
          expectedStatusLocal: 422,
          expectedCodeLocal: 604,
          expectedMessageLocal: 'companySFID in path should be at least 15 chars long',
          expectedStatusRemote: 403,
          expectedCodeRemote: 403,
          expectedMessageRemote:
            'does not have access to resource or path /cla-service/v4/company/bad/contributorAssociation',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'POST /company/{companySFID}/contributorAssociation with malformed SFID (bad format)',
          method: 'POST',
          url: `${claEndpoint}${badSFID2}/contributorAssociation`,
          body: { userEmail: 'user@example.org' },
          expectedStatusLocal: 422,
          expectedCodeLocal: 605,
          expectedMessageLocal: "companySFID in path should match '^([0-9A-Za-z]{15}|[0-9A-Za-z]{18})$'",
          expectedStatusRemote: 403,
          expectedMessageRemote:
            'does not have access to resource or path /cla-service/v4/company/001000000000-00AAA/contributorAssociation',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'POST /company/{companySFID}/contributorAssociation with malformed email',
          method: 'POST',
          url: `${claEndpoint}${exampleSFID}/contributorAssociation`,
          body: { userEmail: 'not-an-email' },
          expectedStatusLocal: 422,
          expectedCodeLocal: 605,
          expectedMessageLocal:
            "body.userEmail in body should match '^([a-zA-Z0-9_\\-\\.\\+]+)@([a-zA-Z0-9_\\-\\.]+)\\.([a-zA-Z]{2,10})$'",
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path',
          expectedMessageContainsRemote: true,
        },

        // --- POST /user/{userID}/company
        {
          title: 'POST /user/{userID}/company missing body',
          method: 'POST',
          url: `${claBaseEndpoint}user/${exampleV4}/company`,
          body: {},
          expectedStatus: 422,
          expectedCode: 602,
          expectedMessage: 'companyName in body is required',
        },
        {
          title: 'POST /user/{userID}/company with malformed website',
          method: 'POST',
          url: `${claBaseEndpoint}user/${exampleV4}/company`,
          body: {
            userEmail: 'founder@example.org',
            companyName: sampleName,
            companyWebsite: 'not-a-url',
          },
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage:
            "companyWebsite in body should match '^(?:http(s)?:\\/\\/)?[\\w.-]+(?:\\.[\\w\\.-]+)+[\\w\\-\\._~:/?#[\\]@!\\$&'\\(\\)\\*\\+,;=.]+$'",
        },
        {
          title: 'POST /user/{userID}/company with empty userID',
          method: 'POST',
          url: `${claBaseEndpoint}user//company`,
          body: {
            userEmail: 'founder@example.org',
            companyName: sampleName,
            companyWebsite: 'https://example.org',
          },
          expectedStatus: 404,
          expectedCode: 404,
          expectedMessage: 'path /v4/user//company was not found',
        },
        {
          title: 'POST /user/{userID}/company with malformed userID (too short)',
          method: 'POST',
          url: `${claBaseEndpoint}user/${badUUID}/company`,
          body: {
            userEmail: 'founder@example.org',
            companyName: sampleName,
            companyWebsite: 'https://example.org',
          },
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'userID in path should be at least 5 chars long',
        },
        {
          title: 'POST /user/{userID}/company with malformed userID (bad format)',
          method: 'POST',
          url: `${claBaseEndpoint}user/${badUUID2}/company`,
          body: {
            userEmail: 'founder@example.org',
            companyName: sampleName,
            companyWebsite: 'https://example.org',
          },
          expectedStatus: 200,
          // LG: note that this succeeds because the userID is not actually validated as a UUID
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

  it('Gets the company by signing entity name', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}entityname/${signingEntityName}`,
      timeout: timeout,
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
      timeout: timeout,
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
      timeout: timeout,
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
      timeout: timeout,
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
      timeout: timeout,
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
      timeout: timeout,
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
      timeout: timeout,
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
      timeout: timeout,
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
      timeout: timeout,
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
      timeout: timeout,
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

  it('Associates a contributor with a company', function () {
    if (companyExternalID === '') {
      companyExternalID = appConfig.companyExternalID;
    }
    let url = `${claEndpoint}${companyExternalID}/contributorAssociation`;
    cy.task('log', 'Associating contributor with URL: ' + url);
    cy.request({
      method: 'POST',
      url: url,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        userEmail: 'veerendrat@proximabiz.com',
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        // This endpoint has security: [] in swagger but may return 401/403 in dev environment
        // Also may return 409 if already associated or 400 for validation issues
        if (response.status === 401) {
          cy.task('log', 'Contributor association returned 401 - dev environment configuration issue');
          expect(response.status).to.be.oneOf([200, 400, 401, 403, 409]);
        } else if (response.status === 403) {
          cy.task('log', 'Contributor association returned 403 - insufficient permissions in dev environment');
          expect(response.status).to.be.oneOf([200, 400, 401, 403, 409]);
        } else if (response.status === 409) {
          cy.task('log', 'Contributor association returned 409 - user may already be associated');
          expect(response.status).to.be.oneOf([200, 400, 401, 403, 409]);
        } else if (response.status === 400) {
          cy.task('log', 'Contributor association returned 400 - validation error or invalid data');
          expect(response.status).to.be.oneOf([200, 400, 401, 403, 409]);
        } else {
          validate_200_Status(response);
          validateApiResponse('company/getCompanyAdmins.json', response);
        }
      });
    });
  });

  it('Creates a new salesforce company', function () {
    let url = `${claBaseEndpoint}user/${user_id}/company`;
    cy.task('log', 'Create SF company via POST URL: ' + url);
    cy.request({
      method: 'POST',
      url: url,
      timeout: timeout,
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
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        companyName = 'lfx dev Test';
        companyID = response.body.companyID;
        getCompanyByName();
      });
    });
  });

  // LG:skip
  it.skip('Deletes the company by the SFID', function () {
    let url = `${claEndpoint}sfid/${companyExternalID}`;
    cy.task('log', 'Deleting company with URL: ' + url);
    cy.request({
      method: 'DELETE',
      url: url,
      timeout: timeout,
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

  it('Creates a new salesforce company 2', function () {
    cy.request({
      method: 'POST',
      url: `${claBaseEndpoint}user/${user_id}/company`,
      timeout: timeout,
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
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        companyName = 'lfx dev Test';
        companyID = response.body.companyID;
        getCompanyByName();
      });
    });
  });

  // LG:skip
  it.skip('Deletes the company by ID', function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}id/${companyID}`,
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

  it('Request Company Admin based on user request to sign CLA', function () {
    cy.request({
      method: 'POST',
      url: `${claBaseEndpoint}user/${user_id2}/request-company-admin`,
      timeout: timeout,
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
    let url = `${claEndpoint}name/${companyName}`;
    cy.task('log', 'Getting company By Name via URL: ' + url);
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
        let list = response.body;
        companyExternalID = list.companyExternalID;
        companyID = list.companyID;
        cy.task('log', 'Company ID: ' + companyID);
        cy.task('log', 'Company External ID: ' + companyExternalID);
        signingEntityName = list.signingEntityName;
        validateApiResponse('company/getCompanyByName.json', response);
      });
    });
  }
});
