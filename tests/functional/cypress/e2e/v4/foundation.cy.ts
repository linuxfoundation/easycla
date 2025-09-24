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

describe('To Validate & get list of Foundation ClaGroups via API call', function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../../appConfig/config.production.ts').appConfig;
  }

  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/foundation
  const claEndpoint = getAPIBaseURL('v4') + 'foundation-mapping';
  const foundationSFID = appConfig.foundationSFID; //project name: easyAutom foundation
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

  it('Get CLA Groups under a foundation - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}?foundationSFID=${foundationSFID}`,
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
      expect(list[0].foundation_sfid).to.eql(foundationSFID);
      // Assert that the response contains an array
      expect(list[0].cla_groups).to.be.an('array');
      // Assert that the array has at least one item
      expect(list[0].cla_groups.length).to.be.greaterThan(0);
      //To validate schema of response
      validateApiResponse('foundation/listFoundationClaGroups.json', response);
    });
  });

  it('Get all CLA Groups - Record should return 200 Response', function () {
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
      // Assert that the response contains an array
      expect(list[0].cla_groups).to.be.an('array');
      // Assert that the array has at least one item
      expect(list[0].cla_groups.length).to.be.greaterThan(0);
      //To validate schema of response
      validateApiResponse('foundation/listFoundationClaGroups.json', response);
    });
  });

  // ========================= Expected failures (foundation) =========================
  describe('Expected failures', () => {
    it('Returns 401 for all Foundation APIs when called without token', () => {
      const exampleSFID = '001000000000000AAA';

      const requests = [
        // GET /foundation-mapping?foundationSFID={id}
        {
          method: 'GET',
          url: `${claEndpoint}?foundationSFID=${encodeURIComponent(exampleSFID)}`,
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
            return cy.logJson('401 response (foundation)', response).then(() => {
              validate_401_Status(response, local);
            });
          });
      });
    });

    it('Returns errors due to missing or malformed parameters for Foundation APIs', function () {
      // Helpers: realistic-looking placeholders & malformed inputs
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
        // --- GET /foundation-mapping (missing/empty/malformed query param) ---
        {
          title: 'GET /foundation-mapping with malformed foundationSFID (too short)',
          method: 'GET',
          url: `${claEndpoint}?foundationSFID=${encodeURIComponent(badSFID)}`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMessage: 'foundationSFID in query should be at least 15 chars long',
          expectedMessageContains: false,
        },
        {
          title: 'GET /foundation-mapping with malformed foundationSFID (bad format)',
          method: 'GET',
          url: `${claEndpoint}?foundationSFID=${encodeURIComponent(badSFID2)}`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMessage: "foundationSFID in query should match '^([0-9A-Za-z]{15}|[0-9A-Za-z]{18})$'",
          expectedMessageContains: false,
        },

        // (Sanity) valid-looking id should succeed
        {
          title: 'GET /foundation-mapping with valid foundationSFID',
          method: 'GET',
          url: `${claEndpoint}?foundationSFID=${encodeURIComponent(exampleSFID)}`,
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
