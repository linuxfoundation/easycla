import {
  validate_200_Status,
  validate_401_Status,
  validate_expected_status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeaders,
} from '../../support/commands';

describe('To Validate & test User APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  let bearerToken: string = null;
  before(() => {
    getTokenKey();
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  // Test public endpoints (no auth required)
  it('GET /user-compat/{userID} - Public endpoint for existing user', function () {
    const testUserID = '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}user-compat/${testUserID}`,
      timeout: timeout,
      failOnStatusCode: false,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body.user_id).to.be.eql(testUserID);
      });
    });
  });

  it('GET /user-compat/{userID} - Public endpoint for non-existing user', function () {
    const testUserID = '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b4';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}user-compat/${testUserID}`,
      timeout: timeout,
      failOnStatusCode: false,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        expect(response.body.Message).to.eq(`user not found for user_id: ${testUserID}`);
      });
    });
  });

  // Test authenticated endpoints - positive cases
  it('Search Users with authentication - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}users/search?searchTerm=test&searchField=username&pageSize=10`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      expect(response.body).to.be.an('object');
      expect(response.body).to.have.property('resultCount');
      expect(response.body).to.have.property('totalCount');
      if (response.body.users) {
        expect(response.body.users).to.be.an('array');
      }
    });
  });

  it('GET /users/{userID} with authentication - existing user', function () {
    const testUserID = '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}users/${testUserID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        expect([200]).to.include(response.status);
        expect(response.body).to.be.an('object');
        expect(response.body).to.have.property('userID');
        expect(response.body.userID).to.eq(testUserID);
      });
    });
  });

  it('GET /users/{userID} with authentication - non-existing user', function () {
    const testUserID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}users/${testUserID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        expect([200]).to.include(response.status);
        expect(response.body).not.to.have.property('userID');
      });
    });
  });

  it('GET /users/username/{userName} with authentication -existing user', function () {
    const testUserName = 'lukaszgryglicki';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}users/username/${testUserName}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      // For positive tests with valid authentication, expect proper responses
      return cy.logJson('response', response).then(() => {
        expect([200]).to.include(response.status);
        expect(response.body).to.be.an('object');
        expect(response.body).to.have.property('userID');
      });
    });
  });

  it('GET /users/username/{userName} with authentication - non-existing user', function () {
    const testUserName = 'non-existing-user-xyz';
    cy.request({
      method: 'GET',
      url: `${claEndpoint}users/username/${testUserName}`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      // For positive tests with valid authentication, expect proper responses
      return cy.logJson('response', response).then(() => {
        expect(response.status).to.eq(404);
        expect(response.statusText).to.eq('Not Found');
      });
    });
  });

  describe('Expected failures', () => {
    it('Returns 401 for User APIs when called without token', () => {
      const exampleUserID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
      const exampleUserName = 'testuser';

      const requests = [
        { method: 'GET', url: `${claEndpoint}users/search?searchTerm=test&searchField=name` },
        { method: 'GET', url: `${claEndpoint}users/${exampleUserID}` },
        { method: 'GET', url: `${claEndpoint}users/username/${exampleUserName}` },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);
              // For negative tests, expect 401 Unauthorized
              expect(response.status).to.eq(401);
            });
          });
      });
    });

    it('Returns 4xx for malformed User search parameters', () => {
      const requests = [
        {
          method: 'GET',
          url: `${claEndpoint}users/search?searchTerm=&searchField=invalid`,
          expectedStatus: '400',
          expectedCode: '400',
        },
        {
          method: 'GET',
          url: `${claEndpoint}users/search?pageSize=invalid`,
          expectedStatus: '422',
          expectedCode: '601',
        },
      ];

      cy.wrap(requests).each((req: any) => {
        return cy
          .request({
            method: req.method,
            url: req.url,
            failOnStatusCode: false,
            timeout,
            headers: getXACLHeaders(),
            auth: {
              bearer: bearerToken,
            },
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing malformed params ${req.method} ${req.url}`);
              expect(req.expectedStatus).to.eq(String(response.status));
              expect(req.expectedCode).to.eq(String(response.body.code ?? response.body.Code));
            });
          });
      });
    });
  });
});
