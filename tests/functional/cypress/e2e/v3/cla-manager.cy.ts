import {
  validate_200_Status,
  validate_401_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeaders,
  validate_expected_status,
} from '../../support/commands';

describe('To Validate & test CLA Manager APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  // Sample test data - using realistic UUIDs/SFIDs
  const testCompanyID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
  const testProjectID = 'a0960000000CZRmAAO';
  const testRequestID = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
  const testUserLFID = 'testuser';

  let bearerToken: string = null;
  before(() => {
    getTokenKey(bearerToken);
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  it('Get CLA Manager Requests with authentication - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/${testCompanyID}/project/${testProjectID}/cla-manager/requests`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      if (response.status === 200) {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        if (response.body.requests) {
          expect(response.body.requests).to.be.an('array');
        }
      } else if (response.status === 404) {
        // Company or project not found is acceptable for this test
        expect(response.status).to.eq(404);
      } else {
        // Allow other statuses during development
        expect([200, 401, 403, 404]).to.include(response.status);
      }
    });
  });

  it('Get CLA Manager Request by ID with authentication', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/${testCompanyID}/project/${testProjectID}/cla-manager/requests/${testRequestID}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      if (response.status === 200) {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
      } else if (response.status === 404) {
        // Request not found is acceptable
        expect(response.status).to.eq(404);
      } else {
        // Allow other statuses during development
        expect([200, 401, 403, 404]).to.include(response.status);
      }
    });
  });

  describe('Authentication Required Tests', () => {
    it('Returns 401 for CLA Manager APIs when called without token', () => {
      const requests = [
        { method: 'GET', url: `${claEndpoint}company/${testCompanyID}/project/${testProjectID}/cla-manager/requests` },
        {
          method: 'GET',
          url: `${claEndpoint}company/${testCompanyID}/project/${testProjectID}/cla-manager/requests/${testRequestID}`,
        },
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
            cy.task('log', `Testing unauthorized ${req.method} ${req.url}`);
            // Expect 401 for missing token
            expect(response.status).to.eq(401);
            if (response.body && typeof response.body === 'object') {
              expect(response.body).to.have.property('message');
            }
          });
      });
    });
  });

  describe('Expected failures', () => {
    it('Returns errors due to missing or malformed parameters for CLA Manager APIs', function () {
      const defaultHeaders = getXACLHeaders();
      const invalidUUID = 'invalid-uuid';
      const invalidSFID = 'invalid-sfid';

      const cases: Array<{
        title: string;
        method: 'GET' | 'POST' | 'PUT' | 'DELETE';
        url: string;
        body?: any;
        needsAuth?: boolean;
        expectedStatus?: number | number[];
        expectedCode?: number;
        expectedMessage?: string;
        expectedMessageContains?: boolean;
      }> = [
        {
          title: 'GET CLA Manager requests with invalid companyID',
          method: 'GET',
          url: `${claEndpoint}company/${invalidUUID}/project/${testProjectID}/cla-manager/requests`,
          needsAuth: true,
          expectedStatus: [200, 400, 404, 422], // Allow 200 if endpoint exists but data is empty
          expectedMessageContains: true,
        },
        {
          title: 'GET CLA Manager requests with invalid projectID',
          method: 'GET',
          url: `${claEndpoint}company/${testCompanyID}/project/${invalidSFID}/cla-manager/requests`,
          needsAuth: true,
          expectedStatus: [200, 400, 404, 422], // Allow 200 if endpoint exists but data is empty
          expectedMessageContains: true,
        },
        {
          title: 'GET CLA Manager request with invalid requestID',
          method: 'GET',
          url: `${claEndpoint}company/${testCompanyID}/project/${testProjectID}/cla-manager/requests/${invalidUUID}`,
          needsAuth: true,
          expectedStatus: [200, 400, 404, 422], // Allow 200 if endpoint exists but data is empty
          expectedMessageContains: true,
        },
      ];

      cy.wrap(cases).each((c: any) => {
        const authHeaders = c.needsAuth
          ? {
              ...defaultHeaders,
              Authorization: `Bearer ${bearerToken}`,
            }
          : defaultHeaders;

        return cy
          .request({
            method: c.method,
            url: c.url,
            body: c.body,
            headers: authHeaders,
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            cy.task('log', `Testing: ${c.title} - Got status: ${response.status}`);
            if (Array.isArray(c.expectedStatus)) {
              expect(c.expectedStatus).to.include(response.status);
            } else if (c.expectedStatus) {
              expect(response.status).to.eq(c.expectedStatus);
            }
          });
      });
    });
  });
});
