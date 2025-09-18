import {
  validateApiResponse,
  validate_200_Status,
  validate_204_Status,
  validate_400_Status,
  validate_400_Status_Contains,
  validate_401_Status,
  validate_403_Status,
  validate_404_Status,
  validate_404_Status_and_Message,
  validate_405_Status_and_Message,
  validate_422_Status,
  getAPIBaseURL,
  getTokenKey,
  getXACLHeader,
} from '../support/commands';
describe('To Validate cla-manager API call', function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../appConfig/config.production.ts').appConfig;
  }

  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/cla-manager
  /* 
https://api-gw.dev.platform.linuxfoundation.org/acs/v1/api-docs#tag/UserRole
https://api-gw.dev.platform.linuxfoundation.org/acs/v1/api-docs#tag/Role/operation/getRoles
*/
  //Variable for GitHub
  const companyID = appConfig.companyID; //infosys limited
  const projectSFID = appConfig.projectSFID; //sun
  const projectSFID_Designee = appConfig.childProjectSFID; //EASYAUTOM-CHILD2
  const claEndpoint = getAPIBaseURL('v4');
  const timeout = 180000;
  const local = Cypress.env('LOCAL') ? true : false;
  let bearerToken: string = null;
  const claGroupID = appConfig.claGroupId;
  const sun_claGroupID = appConfig.claGroupId_projectSFID; //sun
  const userEmail = 'veerendrat@proximabiz.com';
  let companyName = appConfig.companyName; //"Infosys limited";
  let companySFID = '';
  let userLFID = 'veerendrat';
  let userId = appConfig.userIdclaManager; //"c5ac2857-c263-11ed-94d1-d2349de32229";//veerendrat
  let userId2 = appConfig.userIdclaManager2; //"9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5";//lgryglicki
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);

  before(() => {
    if (bearerToken == null) {
      getTokenKey(bearerToken);
      cy.window().then((win) => {
        bearerToken = win.localStorage.getItem('bearerToken');
      });
    }
  });

  describe('Expected failures', () => {
    const companySFID = appConfig.companyExternalID;
    const companyID = appConfig.companyID;
    const userLFID = appConfig.lfid2;

    // UUID helpers
    const badUUID = '00000000-0000-0000-0000-000000000000'; // fails UUIDv4 validation
    const badSFID = projectSFID + 'abcd';
    const exampleV4 = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // valid UUIDv4 shape

    it('Returns 401 for all CLA-Manager APIs when called without token', function () {
      // Minimal-but-valid bodies where the spec requires a body (so auth is checked first)
      const managerBody = {
        firstName: 'Alice',
        lastName: 'Tester',
        userEmail: 'alice@example.org',
      };
      const managerReqBody = { fullName: 'Alice Tester' };
      const notifyListBody = { list: [{ email: 'alice@example.org', name: 'Alice' }], claGroupID: exampleV4 };

      const requests = [
        // Add CLA Manager (POST /company/{companyID}/project/{projectSFID}/cla-manager)
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager`,
          body: managerBody,
        },

        // Add CLA Manager Request (POST /company/{companyID}/project/{projectSFID}/cla-manager/requests)
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager/requests`,
          body: managerReqBody,
        },

        // Delete CLA Manager (DELETE /company/{companyID}/project/{projectSFID}/cla-manager/{userLFID})
        { method: 'DELETE', url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager/${userLFID}` },

        // GET CLA Manager Designee Status (GET /company/{companySFID}/user/{userLFID}/claGroupID/{claGroupID}/is-cla-manager-designee)
        // LG: this endpoint does not require token
        //{
        //  method: 'GET',
        //  url: `${claEndpoint}company/${companySFID}/user/${userLFID}/claGroupID/${claGroupID}/is-cla-manager-designee`,
        //},

        // Assign CLA Manager Designee (POST /company/{companySFID}/claGroup/{claGroupID}/cla-manager-designee)
        {
          method: 'POST',
          url: `${claEndpoint}company/${companySFID}/claGroup/${exampleV4}/cla-manager-designee`,
        },

        // Assign CLA Manager Designee (POST /company/{companyID}/project/{projectSFID}/cla-manager-designee)
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager-designee`,
        },

        // List CLA Managers (GET /company/{companyID}/project/{projectSFID}/cla-managers)
        { method: 'GET', url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-managers` },

        // List CLA Managers by CLA Group (GET /company/{companyID}/cla-group/{claGroupID}/cla-managers)
        // LG: In swagger those APIs have `security: []` so they don't need token.
        // { method: 'GET', url: `${claEndpoint}company/${companyID}/cla-group/${exampleV4}/cla-managers` },
        // { method: 'POST', url: `${claEndpoint}notify-cla-managers` },
        // { method: 'POST', url: `${claEndpoint}user/${userId2}/invite-company-admin` },
      ];

      cy.wrap(requests).each((req: any) => {
        cy.task('log', `--> ${req.method} ${req.url}`);
        cy.request({
          method: req.method,
          url: req.url,
          failOnStatusCode: false,
          timeout,
          ...(req.body ? { body: req.body } : {}),
        }).then((response) => {
          return cy.logJson('401 response', response).then(() => {
            // In local env the message body differs
            validate_401_Status(response, local);
          });
        });
      });
    });

    it('Returns errors due to missing parameters', function () {
      // Build a suite mirroring cla-group "missing params" checks, mapped to cla-manager endpoints
      const effectiveMode = local ? 'local' : 'remote';
      const both = 'both';
      const claManagerDesigneeBody = { userEmail: 'luke@o2.pl' };
      const inviteCompanyAdminBody = {
        claGroupID: sun_claGroupID,
        companyID: companyID,
        contactAdmin: true,
        name: 'veerendra thakur',
        userEmail: userEmail,
      };

      const requests = [
        // Notify CLA Managers
        {
          method: 'POST',
          url: `${claEndpoint}notify-cla-managers`,
          expectedStatus: 422,
          expectedCode: 602,
          expectedMsg: 'body in body is required',
          mode: both,
        },

        // Invite Company Admin
        {
          method: 'POST',
          url: `${claEndpoint}user/${userId2}/invite-company-admin`,
          expectedStatus: 422,
          expectedCode: 602,
          expectedMsg: 'body in body is required',
          mode: both,
        },
        {
          method: 'POST',
          url: `${claEndpoint}user//invite-company-admin`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'path /v4/user//invite-company-admin was not found',
          mode: both,
        },
        {
          method: 'POST',
          url: `${claEndpoint}user/aa/invite-company-admin`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMsg: 'userID in path should be at least 5 chars long',
          body: inviteCompanyAdminBody,
          mode: both,
        },

        // 1) Add CLA Manager: missing body
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager`,
          expectedStatus: 422,
          expectedCode: 602,
          expectedMsg: 'body in body is required',
          mode: both,
        },
        {
          method: 'POST',
          url: `${claEndpoint}company//project//cla-manager`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'path /v4/company//project//cla-manager was not found',
          mode: 'local',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company//project//cla-manager`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project//cla-manager`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `path /v4/company/${companyID}/project//cla-manager was not found`,
          mode: 'local',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project//cla-manager`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company//project/${projectSFID}/cla-manager`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `path /v4/company//project/${projectSFID}/cla-manager was not found`,
          mode: 'local',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company//project/${projectSFID}/cla-manager`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },

        // 2) Add CLA Manager: missing projectSFID (route with //)
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project//cla-manager`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'path /v4/company/' + companyID + '/project//cla-manager was not found',
          mode: 'local',
        },

        // Remote gateway often 403 on malformed paths
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project//cla-manager`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },

        // 3) Add CLA Manager Request: missing body (required fullName)
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager/requests`,
          expectedStatus: 422,
          expectedCode: 602,
          expectedMsg: 'body in body is required',
          // LG: comment out until new api is deployed to dev for remote testing
          // mode: 'local',
          mode: both,
        },

        // 4) Delete CLA Manager: missing userLFID
        {
          method: 'DELETE',
          url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager/`,
          expectedStatus: 405,
          expectedCode: 405,
          expectedMsg: 'method DELETE is not allowed, but [POST] are',
          mode: 'local',
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager/`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}company//project//cla-manager/`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'path /v4/company//project//cla-manager/ was not found',
          mode: 'local',
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}company//project//cla-manager/`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}company/${companyID}/project//cla-manager/`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `path /v4/company/${companyID}/project//cla-manager/ was not found`,
          mode: 'local',
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}company/${companyID}/project//cla-manager/`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}company//project/${projectSFID}/cla-manager/`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `path /v4/company//project/${projectSFID}/cla-manager/ was not found`,
          mode: 'local',
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}company//project/${projectSFID}/cla-manager/`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}company//project//cla-manager/${userLFID}`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `path /v4/company//project//cla-manager/${userLFID} was not found`,
          mode: 'local',
        },
        {
          method: 'DELETE',
          url: `${claEndpoint}company//project//cla-manager/${userLFID}`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },

        // Is CLA manager designee with any of parameters missing
        {
          method: 'GET',
          url: `${claEndpoint}company//user//claGroupID//is-cla-manager-designee`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'path /v4/company//user//claGroupID//is-cla-manager-designee was not found',
          mode: both,
        },
        {
          method: 'GET',
          url: `${claEndpoint}company//user/${userLFID}/claGroupID/${claGroupID}/is-cla-manager-designee`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `path /v4/company//user/${userLFID}/claGroupID/${claGroupID}/is-cla-manager-designee was not found`,
          mode: both,
        },
        {
          method: 'GET',
          url: `${claEndpoint}company/${companySFID}/user//claGroupID/${claGroupID}/is-cla-manager-designee`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `path /v4/company/${companySFID}/user//claGroupID/${claGroupID}/is-cla-manager-designee was not found`,
          mode: both,
        },
        {
          method: 'GET',
          url: `${claEndpoint}company/${companySFID}/user/${userLFID}/claGroupID//is-cla-manager-designee`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `path /v4/company/${companySFID}/user/${userLFID}/claGroupID//is-cla-manager-designee was not found`,
          mode: both,
        },

        // 5a) Assign CLA Manager Designee by claGroup/companySFID: missing/wrong parameters, missy body parameter
        {
          method: 'POST',
          url: `${claEndpoint}company//claGroup//cla-manager-designee`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'path /v4/company//claGroup//cla-manager-designee was not found',
          mode: 'local',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company//claGroup//cla-manager-designee`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/claGroup//cla-manager-designee`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `path /v4/company/${companyID}/claGroup//cla-manager-designee was not found`,
          mode: 'local',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/claGroup//cla-manager-designee`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company//claGroup/${claGroupID}/cla-manager-designee`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `path /v4/company//claGroup/${claGroupID}/cla-manager-designee was not found`,
          mode: 'local',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company//claGroup/${claGroupID}/cla-manager-designee`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company/${badUUID}/claGroup/${claGroupID}/cla-manager-designee`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMsg:
            "companyID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          mode: both,
          body: claManagerDesigneeBody,
        },
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/claGroup/${badUUID}/cla-manager-designee`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMsg:
            "claGroupID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          mode: both,
          body: claManagerDesigneeBody,
        },
        {
          // missing body parameter
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/claGroup/${claGroupID}/cla-manager-designee`,
          expectedStatus: 422,
          expectedCode: 602,
          expectedMsg: 'body in body is required',
          mode: both,
          extra: 'includes',
        },

        // 5b) Assign CLA Manager Designee by claGroup/companySFID: wrong claGroupID (bad UUIDv4)
        {
          method: 'POST',
          url: `${claEndpoint}company//project//cla-manager-designee`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'path /v4/company//project//cla-manager-designee was not found',
          mode: 'local',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company//project//cla-manager-designee`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project//cla-manager-designee`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `path /v4/company/${companyID}/project//cla-manager-designee was not found`,
          mode: 'local',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project//cla-manager-designee`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company//project/${projectSFID}/cla-manager-designee`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `path /v4/company//project/${projectSFID}/cla-manager-designee was not found`,
          mode: 'local',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company//project/${projectSFID}/cla-manager-designee`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        {
          method: 'POST',
          url: `${claEndpoint}company/${badUUID}/project/${projectSFID}/cla-manager-designee`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMsg:
            "companyID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          mode: both,
          body: claManagerDesigneeBody,
        },
        {
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project/${badSFID}/cla-manager-designee`,
          expectedStatus: 422,
          expectedCode: 603,
          expectedMsg: 'projectSFID in path should be at most 18 chars long',
          mode: both,
          body: claManagerDesigneeBody,
        },
        {
          // missing body parameter
          method: 'POST',
          url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager-designee`,
          expectedStatus: 422,
          expectedCode: 602,
          expectedMsg: 'body in body is required',
          extra: 'includes',
        },

        // 6) List CLA managers: missing projectSFID
        {
          method: 'GET',
          url: `${claEndpoint}company/${companyID}/project//cla-managers`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'path /v4/company/' + companyID + '/project//cla-managers was not found',
          mode: 'local',
        },
        {
          method: 'GET',
          url: `${claEndpoint}company/${companyID}/project//cla-managers`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },

        // 7) List CLA managers by CLA Group: invalid/missing claGroupID (double slash)
        {
          method: 'GET',
          url: `${claEndpoint}company/${companyID}/cla-group//cla-managers`,
          expectedStatus: 404,
          expectedCode: 1,
          expectedMsg: 'path /v4/company/' + companyID + '/cla-group//cla-managers was not found',
          // mode: 'local',
          mode: both, // LG: It returns 404 in both local and remote
        },
        /*
        // LG: this should be commented out
        {
          method: 'GET',
          url: `${claEndpoint}company/${companyID}/cla-group//cla-managers`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },
        */

        // 8) List CLA managers by CLA Group: wrong claGroupID format (not UUID v4)
        {
          method: 'GET',
          url: `${claEndpoint}company/${companyID}/cla-group/${badUUID}/cla-managers`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMsg:
            "claGroupID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          mode: both,
        },
      ];

      const filtered = requests.filter((r) => r.mode === both || r.mode === effectiveMode);
      cy.wrap(filtered).each((req: any) => {
        const { method, url, expectedStatus, expectedCode, expectedMsg } = req;
        const extra = req.extra || '';
        const body = req.body || null;
        cy.task('log', `--> ${method} ${url}`);
        cy.request({
          method,
          url,
          failOnStatusCode: false,
          headers: getXACLHeader(),
          auth: { bearer: bearerToken },
          timeout,
          ...(body ? { body: body } : {}),
        }).then((response) => {
          return cy.logJson('response', response).then(() => {
            switch (expectedStatus) {
              case 400:
                if (extra === 'includes') {
                  return validate_400_Status_Contains(response, expectedMsg);
                } else {
                  return validate_400_Status(response, expectedMsg);
                }
              case 403:
                return validate_403_Status(response);
              case 404:
                return validate_404_Status_and_Message(response, expectedMsg);
              case 405:
                return validate_405_Status_and_Message(response, expectedMsg);
              case 422:
                return validate_422_Status(response, expectedCode, expectedMsg);
              default:
                throw new Error(`Unexpected expectedStatus: ${expectedStatus}`);
            }
          });
        });
      });
    });
  });

  it('Assigns CLA Manager designee to a given user 1', function () {
    let url = `${claEndpoint}company/${companyID}/claGroup/${claGroupID}/cla-manager-designee`;
    cy.task('log', `--> POST ${url}`);
    cy.request({
      method: 'POST',
      url: url,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
      body: { userEmail: userEmail },
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        companySFID = response.body.list[0].company_sfid;
        userLFID = response.body.list[0].lf_username;
        cy.log('company_sfid : ' + companySFID);
        cy.log('lf_username : ' + userLFID);
        //To validate Post response
        if (response.status === 200) {
          getClaManager();
        }
        validateApiResponse('cla-manager/assignCLAManager.json', response);
      });
    });
  });

  it('Attempts to assign CLA Manager designee to a given user in project', function () {
    let url = `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager-designee`;
    cy.task('log', `--> POST ${url}`);
    cy.request({
      method: 'POST',
      url: url,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
      body: { userEmail: userEmail },
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      return cy.logJson('response', response).then(() => {
        validate_400_Status_Contains(response, 'EasyCLA - 400 Bad Request');
        validate_400_Status_Contains(response, 'error: project already signed');
      });
    });
  });

  it('Allows an existing CLA Manager to add another CLA Manager to the specified Company and Project.', function () {
    let url = `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager`;
    cy.task('log', `--> POST ${url}`);
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
        firstName: 'veerendrat',
        lastName: 'thakur',
        userEmail: userEmail,
      },
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      return cy.logJson('response', response).then(() => {
        if (response.status === 200) {
          validate_200_Status(response);
          // Validate specific data in the response
          let list = response.body;
          expect(list.project_sfid).to.eql(projectSFID);
          //To validate schema of response
        } else {
          expect(response.body.Message).to.include('error: manager already in signature ACL');
        }
        validateApiResponse('cla-manager/createCLAManager.json', response);
      });
    });
  });

  it('Deletes the CLA Manager from CLA Manager list for specified Company and Project', function () {
    cy.request({
      method: 'DELETE',
      url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager/${userLFID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      expect(response.status).to.eq(204);
    });
  });

  it('Assigns CLA Manager designee to a given user 2', function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}company/${companyID}/project/${projectSFID_Designee}/cla-manager-designee`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        userEmail: userEmail,
      },
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      validate_200_Status(response);
      // Validate specific data in the response
      expect(response.body.project_sfid).to.eql(projectSFID_Designee);
      if (response.status === 200) {
        getClaManager();
      }
      validateApiResponse('cla-manager/createCLAManagerDesignee.json', response);
    });
  });

  it('Adds a CLA Manager Designee to the specified Company and Project', function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}company/${companyID}/project/${projectSFID_Designee}/cla-manager/requests`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        contactAdmin: false,
        fullName: 'Lukasz Gryglicki',
        userEmail: 'lukaszgryglicki@o2.pl',
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        // expect(response.duration).to.be.lessThan(20000);
        validate_200_Status(response);
        // Validate specific data in the response
        expect(response.body.project_sfid).to.eql(projectSFID_Designee);
        //To validate schema of response
        validateApiResponse('cla-manager/createCLAManagerDesignee.json', response);
      });
    });
  });

  it('Send Notification to CLA Managaers', function () {
    cy.task('log', `--> POST ${claEndpoint}notify-cla-managers`);
    let body = {
      claGroupID: claGroupID,
      companyName: companyName,
      list: [
        {
          email: 'lukaszgryglicki@o2.pl',
          name: 'lgryglicki',
        },
      ],
      signingEntityName: 'Linux Foundation',
      userID: userId2,
    };
    cy.logJson('body', body);
    cy.request({
      method: 'POST',
      url: `${claEndpoint}notify-cla-managers`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: body,
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      return cy.logJson('response', response).then(() => validate_204_Status(response));
      // expect(response.status).to.eq(204);
    });
  });

  it('Send Notification to non-existing CLA Managaer', function () {
    cy.task('log', `--> POST ${claEndpoint}notify-cla-managers`);
    let body = {
      claGroupID: claGroupID,
      companyName: companyName,
      list: [
        {
          email: 'vthakur@contractor.linuxfoundation.org',
          name: 'vthakur',
        },
      ],
      signingEntityName: 'Linux Foundation',
      userID: userId,
    };
    cy.logJson('body', body);
    cy.request({
      method: 'POST',
      url: `${claEndpoint}notify-cla-managers`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: body,
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      return cy.logJson('response', response).then(() => validate_404_Status(response));
      // expect(response.status).to.eq(204);
    });
  });

  it('Invite Company Admin based on user request to sign CLA', function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}user/${userId2}/invite-company-admin`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        claGroupID: sun_claGroupID,
        companyID: companyID,
        contactAdmin: true,
        name: 'veerendra thakur',
        userEmail: userEmail,
      },
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      return cy.logJson('response', response).then(() => validate_200_Status(response));
      // validate_200_Status(response);
      // validateApiResponse("cla-manager/assignCLAManager.json",response)
    });
  });

  it('Lists CLA managers by Project/Company - returns 200', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-managers`,
      timeout,
      failOnStatusCode: true,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        // Minimal shape assertions - adapt if you have a schema file
        const body = response.body ?? response.Body ?? {};
        // expect array-ish payload under a common key if present
        if (Array.isArray(body)) {
          expect(body).to.be.an('array');
        } else if (body.list) {
          expect(body.list).to.be.an('array');
        }
      });
    });
  });

  it('Lists CLA managers by CLA Group/Company - returns 200', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/${companyID}/cla-group/${claGroupID}/cla-managers`,
      timeout,
      failOnStatusCode: true,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        const body = response.body ?? response.Body ?? {};
        if (Array.isArray(body)) {
          expect(body).to.be.an('array');
        } else if (body.list) {
          expect(body.list).to.be.an('array');
        }
      });
    });
  });

  function getClaManager() {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/${companySFID}/user/${userLFID}/claGroupID/${claGroupID}/is-cla-manager-designee`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        expect(response.body.hasRole).to.eq(true);
        expect(response.body.lfUsername).to.eq(userLFID);
        // validateApiResponse("cla-manager/isCLAManagerDesignee.json",response)
      });
    });
  }
});
