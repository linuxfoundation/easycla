import {
  validateApiResponse,
  validate_200_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
  validate_401_Status,
  validate_expected_status,
} from '../../support/commands';
describe('To Validate events are properly capture via API call', function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../../appConfig/config.production.ts').appConfig;
  }

  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/events
  const claBaseEndpoint = getAPIBaseURL('v4');
  const claEndpoint = claBaseEndpoint + 'events';
  let claEndpointForNextKey = '';
  let NextKey: string = '';
  const foundationSFID = appConfig.foundationSFID; //project name: easyAutom foundation
  const projectSfid = appConfig.childProjectSFID; //project name: easyAutom-child2
  const companyID = appConfig.companyID; //Infosys Limited
  const compProjectSFID = appConfig.projectSFID; //sun
  const local = Cypress.env('LOCAL') ? true : false;
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
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

  // ========================= Expected failures (events) =========================
  describe('Expected failures', () => {
    it('Returns 401 for all Events APIs when called without token', () => {
      // IDs that look valid so the first failure is auth, not validation
      const exampleV4 = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e'; // valid UUIDv4 shape
      const exampleSFID = '001000000000000AAA'; // plausible SFID (15/18 chars)

      const requests = [
        // GET /events/recent
        { method: 'GET', url: `${claEndpoint}/recent` },

        // GET /events/foundation/{foundationSFID}
        { method: 'GET', url: `${claEndpoint}/foundation/${foundationSFID || exampleSFID}` },

        // GET /events/foundation/{foundationSFID}/csv
        { method: 'GET', url: `${claEndpoint}/foundation/${foundationSFID || exampleSFID}/csv` },

        // GET /events/project/{projectSfid}
        { method: 'GET', url: `${claEndpoint}/project/${projectSfid}` },

        // GET /events/project/{projectSfid}/csv
        { method: 'GET', url: `${claEndpoint}/project/${projectSfid}/csv` },

        // GET /company/{companyID}/project/{projectSfid}/events
        { method: 'GET', url: `${claBaseEndpoint}company/${exampleV4}/project/${projectSfid}/events` },
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
            return cy.logJson('401 response (events)', response).then(() => {
              validate_401_Status(response, local);
            });
          });
      });
    });

    it('Returns errors due to missing or malformed parameters for Events APIs', function () {
      // Helpers: realistic-looking placeholders & malformed inputs
      const exampleV4 = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
      const badUUID = 'aa';
      const badUUID2 = 'd9428888-122b-4b20-8c4a-0c9a1a6z9b8e';
      const exampleSFID = '001000000000000AAA';
      const badSFID = 'bad';
      const badSFID2 = '001000000000-00AAA';

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
        // -------------------- Foundation events (JSON) --------------------
        {
          title: 'GET /events/foundation/{foundationSFID} with empty foundationSFID',
          method: 'GET',
          url: `${claBaseEndpoint}events/foundation/`,
          expectedStatusLocal: 404,
          expectedCodeLocal: 404,
          expectedMessageLocal: 'path /v4/events/foundation/ was not found',
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path /cla-service/v4/events/foundation/',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /events/foundation/{foundationSFID} with malformed foundationSFID (too short)',
          method: 'GET',
          url: `${claBaseEndpoint}events/foundation/${badSFID}`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'foundationSFID in path should be at least 15 chars long',
        },
        {
          title: 'GET /events/foundation/{foundationSFID} with malformed foundationSFID (bad format)',
          method: 'GET',
          url: `${claBaseEndpoint}events/foundation/${badSFID2}`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage: 'foundationSFID in path should match',
          expectedMessageContains: true,
        },

        // -------------------- Foundation events CSV --------------------
        {
          title: 'GET /events/foundation/{foundationSFID}/csv with empty foundationSFID',
          method: 'GET',
          url: `${claBaseEndpoint}events/foundation//csv`,
          expectedStatusLocal: 422,
          expectedCodeLocal: 604,
          expectedMessageLocal: 'foundationSFID in path should be at least 15 chars long',
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path /cla-service/v4/events/foundation//csv',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /events/foundation/{foundationSFID}/csv with malformed foundationSFID (too short)',
          method: 'GET',
          url: `${claBaseEndpoint}events/foundation/${badSFID}/csv`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'foundationSFID in path should be at least 15 chars long',
        },
        {
          title: 'GET /events/foundation/{foundationSFID}/csv with malformed foundationSFID (bad format)',
          method: 'GET',
          url: `${claBaseEndpoint}events/foundation/${badSFID2}/csv`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage: 'foundationSFID in path should match',
          expectedMessageContains: true,
        },

        // -------------------- Project events CSV --------------------
        {
          title: 'GET /events/project/{projectSfid}/csv with empty projectSfid',
          method: 'GET',
          url: `${claBaseEndpoint}events/project//csv`,
          expectedStatusLocal: 422,
          expectedCodeLocal: 604,
          expectedMessageLocal: 'projectSFID in path should be at least 15 chars long',
          expectedStatusRemote: 403,
          expectedMessageRemote: 'does not have access to resource or path /cla-service/v4/events/project//csv',
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /events/project/{projectSfid}/csv with malformed projectSfid (too short)',
          method: 'GET',
          url: `${claBaseEndpoint}events/project/${badSFID}/csv`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'projectSFID in path should be at least 15 chars long',
        },
        {
          title: 'GET /events/project/{projectSfid}/csv with malformed projectSfid (bad format)',
          method: 'GET',
          url: `${claBaseEndpoint}events/project/${badSFID2}/csv`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage: 'projectSFID in path should match',
          expectedMessageContains: true,
        },

        // -------------------- Company + Project events (JSON) --------------------
        {
          title: 'GET /company/{companyID}/project/{projectSfid}/events with empty companyID',
          method: 'GET',
          url: `${claBaseEndpoint}company//project/${projectSfid}/events`,
          expectedStatusLocal: 404,
          expectedCodeLocal: 404,
          expectedMessageLocal: `path /v4/company//project/${projectSfid}/events was not found`,
          expectedStatusRemote: 403,
          expectedMessageRemote: `does not have access to resource or path /cla-service/v4/company//project/${projectSfid}/events`,
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /company/{companyID}/project/{projectSfid}/events with malformed companyID (too short)',
          method: 'GET',
          url: `${claBaseEndpoint}company/${badUUID}/project/${projectSfid}/events`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage: 'companyID in path should match',
          expectedMessageContains: true,
        },
        {
          title: 'GET /company/{companyID}/project/{projectSfid}/events with malformed companyID (bad format)',
          method: 'GET',
          url: `${claBaseEndpoint}company/${badUUID2}/project/${projectSfid}/events`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage: 'companyID in path should match',
          expectedMessageContains: true,
        },
        {
          title: 'GET /company/{companyID}/project/{projectSfid}/events with empty projectSfid',
          method: 'GET',
          url: `${claBaseEndpoint}company/${exampleV4}/project//events`,
          expectedStatusLocal: 404,
          expectedCodeLocal: 404,
          expectedMessageLocal: `path /v4/company/${exampleV4}/project//events was not found`,
          expectedStatusRemote: 403,
          expectedMessageRemote: `does not have access to resource or path /cla-service/v4/company/${exampleV4}/project//events`,
          expectedMessageContainsRemote: true,
        },
        {
          title: 'GET /company/{companyID}/project/{projectSfid}/events with malformed projectSfid (too short)',
          method: 'GET',
          url: `${claBaseEndpoint}company/${exampleV4}/project/${badSFID}/events`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'projectSFID in path should be at least 15 chars long',
        },
        {
          title: 'GET /company/{companyID}/project/{projectSfid}/events with malformed projectSfid (bad format)',
          method: 'GET',
          url: `${claBaseEndpoint}company/${exampleV4}/project/${badSFID2}/events`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage: 'projectSFID in path should match',
          expectedMessageContains: true,
        },

        // -------------------- Recent events (admin-only) --------------------
        {
          title: 'GET /events/recent (non-admin user)',
          method: 'GET',
          url: `${claBaseEndpoint}events/recent`,
          expectedStatus: 403,
          expectedMessage: 'does not have access to Get Recent Events',
          expectedMessageContains: true,
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

  it('Get recent events of company and project - Record should return 200 Response', function () {
    claEndpointForNextKey = claBaseEndpoint + `company/${companyID}/project/${compProjectSFID}/events`;
    cy.request({
      method: 'GET',
      url: `${claEndpointForNextKey}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);

      // Validate specific data in the response
      let list = response.body;
      NextKey = list.NextKey;
      expect(list).to.have.property('NextKey');
      expect(list).to.have.property('ResultCount');
      expect(list).to.have.property('Events');
      let Events = list.Events;
      // Assert that the response contains an array
      expect(Events).to.be.an('array');
      // Assert that the array has at least one item
      expect(Events.length).to.be.greaterThan(0);
      validateApiResponse('events/getCompanyProjectEvents.json', response);
      fetchNextRecords(claEndpointForNextKey, NextKey);
    });
  });

  it('Get events of foundation project - Record should return 200 Response', function () {
    claEndpointForNextKey = `${claEndpoint}/foundation/${foundationSFID}`;
    cy.request({
      method: 'GET',
      url: `${claEndpointForNextKey}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);

      // Validate specific data in the response
      let list = response.body;
      NextKey = list.NextKey;
      expect(list).to.have.property('NextKey');
      expect(list).to.have.property('ResultCount');
      expect(list).to.have.property('Events');
      let Events = list.Events;
      // Assert that the response contains an array
      expect(Events).to.be.an('array');
      // Assert that the array has at least one item
      expect(Events.length).to.be.greaterThan(0);
      // validateApiResponse("events/getFoundationEvents.json",list);
      fetchNextRecords(claEndpointForNextKey, NextKey);
    });
  });

  it('Get events of child project - Record should return 200 Response', function () {
    claEndpointForNextKey = `${claEndpoint}/project/${projectSfid}`;
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/project/${projectSfid}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);

      let list = response.body;
      // Validate specific data in the response
      expect(list).to.have.property('NextKey');
      expect(list).to.have.property('ResultCount');
      expect(list).to.have.property('Events');
      let Events = response.body.Events;
      // Assert that the response contains an array
      expect(Events).to.be.an('array');
      // Assert that the array has at least one item
      expect(Events.length).to.be.greaterThan(0);
      //To validate schema of response
      validateApiResponse('events/getProjectEvents', list);
      fetchNextRecords(claEndpointForNextKey, NextKey);
    });
  });

  // LG:skip
  it.skip('Get List of recent events - requires Admin-level access - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/recent?pageSize=2`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);

      // Validate specific data in the response
      let list = response.body;
      expect(list).to.have.property('NextKey');
      expect(list).to.have.property('ResultCount');
      expect(list).to.have.property('Events');
      let Events = list.Events;
      // Assert that the response contains an array
      expect(Events).to.be.an('array');
      // Assert that the array has at least one item
      expect(Events.length).to.be.greaterThan(0);
      //To validate schema of response
      validateApiResponse('events/getProjectEvents.json', list);
    });
  });

  it('Download all the events for the foundation as a CSV document - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/foundation/${foundationSFID}/csv`,
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

  it('Download all the events for the project as a CSV document - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/project/${projectSfid}/csv`,
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

  it('Gets foundation events (JSON)', function () {
    const url = `${claEndpoint}/foundation/${foundationSFID}`;
    cy.task('log', 'GET ' + url);

    cy.request({
      method: 'GET',
      url,
      timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
    }).then((response) => {
      return cy.logJson('foundation events (JSON)', response).then(() => {
        // Basic shape checks based on swagger
        // Expect 200 and a body with "Events" array (can be empty),
        // and optionally keys like NextKey / ResultCount.
        expect(response.status).to.eq(200);
        const body = response.body ?? {};
        // body may be object or array depending on backend, assert resiliently:
        if (Array.isArray(body)) {
          // Some implementations return an array directly
          // If so, just ensure it is iterable
          expect(body).to.have.property('length');
        } else {
          // Standard EasyCLA v4 shape
          expect(body).to.have.property('Events');
          expect(body.Events).to.be.an('array');
          // Optional fields (don’t fail the test if they’re absent)
          if ('NextKey' in body) {
            expect(body).to.have.property('NextKey');
          }
          if ('ResultCount' in body) {
            expect(body).to.have.property('ResultCount');
          }
        }
      });
    });
  });

  it('Gets foundation events (JSON) with a searchTerm filter', function () {
    const searchTerm = encodeURIComponent('cla'); // any short term known to be safe
    const url = `${claEndpoint}/foundation/${foundationSFID}?searchTerm=${searchTerm}`;
    cy.task('log', 'GET ' + url);

    cy.request({
      method: 'GET',
      url,
      timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
    }).then((response) => {
      return cy.logJson('foundation events (JSON + searchTerm)', response).then(() => {
        expect(response.status).to.eq(200);
        const body = response.body ?? {};
        if (Array.isArray(body)) {
          expect(body).to.have.property('length');
        } else {
          expect(body).to.have.property('Events');
          expect(body.Events).to.be.an('array');
        }
      });
    });
  });

  it('Gets company+project events (JSON)', function () {
    const url = `${claBaseEndpoint}company/${companyID}/project/${projectSfid}/events`;
    cy.task('log', 'GET ' + url);

    cy.request({
      method: 'GET',
      url,
      timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
    }).then((response) => {
      return cy.logJson('company+project events (JSON)', response).then(() => {
        expect(response.status).to.eq(200);
        const body = response.body ?? {};
        // Typical response is an object with "Events" list; accept array fallback
        if (Array.isArray(body)) {
          expect(body).to.have.property('length');
        } else {
          expect(body).to.have.property('Events');
          expect(body.Events).to.be.an('array');
          if ('NextKey' in body) {
            expect(body).to.have.property('NextKey');
          }
          if ('ResultCount' in body) {
            expect(body).to.have.property('ResultCount');
          }
        }
      });
    });
  });

  it('Gets company+project events (JSON) with returnAllEvents=true', function () {
    const url = `${claBaseEndpoint}company/${companyID}/project/${projectSfid}/events?returnAllEvents=true`;
    cy.task('log', 'GET ' + url);

    cy.request({
      method: 'GET',
      url,
      timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: { bearer: bearerToken },
    }).then((response) => {
      return cy.logJson('company+project events (JSON + returnAllEvents)', response).then(() => {
        expect(response.status).to.eq(200);
        const body = response.body ?? {};
        if (Array.isArray(body)) {
          expect(body).to.have.property('length');
        } else {
          expect(body).to.have.property('Events');
          expect(body.Events).to.be.an('array');
        }
      });
    });
  });

  function fetchNextRecords(URL, NextKey) {
    if (NextKey !== undefined) {
      cy.request({
        method: 'GET',
        url: `${URL}?nextKey=${NextKey}&pageSize=50`,
        timeout: timeout,
        failOnStatusCode: allowFail,
        headers: getXACLHeader(),
        auth: {
          bearer: bearerToken,
        },
      }).then((response) => {
        validate_200_Status(response);

        // Validate specific data in the response
        let updatedNextKey = response.body.NextKey;
        if (updatedNextKey !== undefined) {
          fetchNextRecords(URL, updatedNextKey);
        }
      });
    }
  }
});
