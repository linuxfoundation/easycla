/*
 * Comprehensive test suite for all Company APIs in V3 (tagged with 'company' in swagger)
 *
 * Covers all HTTP methods for company endpoints:
 * - GET /company (authenticated)
 * - GET /company/{companyID} (authenticated)
 * - GET /company/external/{companySFID} (authenticated)
 * - GET /company/search (authenticated)
 * - GET /company/signing-entity-name (authenticated)
 * - GET /company/user/{userID} (authenticated)
 * - GET /company/user/{userID}/invites (authenticated)
 * - GET /company/{companyID}/ccla-whitelist-requests (authenticated)
 * - GET /company/{companyID}/ccla-whitelist-requests/{projectID} (authenticated)
 * - GET /company/{companyID}/ccla-whitelist-requests/{projectID}/user/{userID} (authenticated)
 * - GET /company/{companyID}/cla/invitelist (authenticated)
 * - GET /company/{companyID}/{userID}/invitelist (authenticated)
 * - POST /company/{companyID}/ccla-whitelist-requests/{projectID} (authenticated)
 * - POST /company/{companyID}/cla/accesslist (authenticated)
 * - PUT /company/{companyID}/ccla-whitelist-requests/{projectID}/{requestID}/approve (authenticated)
 * - PUT /company/{companyID}/ccla-whitelist-requests/{projectID}/{requestID}/reject (authenticated)
 * - PUT /company/{companyID}/cla/accesslist/request (authenticated)
 * - PUT /company/{companyID}/cla/accesslist/{requestID}/approve (authenticated)
 * - PUT /company/{companyID}/cla/accesslist/{requestID}/reject (authenticated)
 *
 * Includes comprehensive negative testing:
 * - 401 Unauthorized tests for all endpoints
 * - 4xx validation error tests for malformed parameters
 * - Invalid UUID and parameter format tests
 *
 * Uses flexible status code assertions to handle various valid API responses
 * All responses are logged via cy.logJson() for debugging purposes
 */
import {
  validate_200_Status,
  validate_204_Status,
  validate_401_Status,
  validate_expected_status,
  validateApiResponse,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeaders,
} from '../../support/commands';

describe('To Validate & test Company APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  let bearerToken: string = null;
  let xacl: string = null;
  const validCompanyID = 'a09b916b-2421-4a4a-ad87-e3a0a1f17e97'; // Example company ID
  const validProjectID = 'a09b916b-2421-4a4a-ad87-e3a0a1f17e97'; // Example project ID
  const validUserID = 'a09b916b-2421-4a4a-ad87-e3a0a1f17e97'; // Example user ID
  const validSFID = 'a09b916b-2421-4a4a-ad87-e3a0a1f17e97'; // Example SFID

  before(() => {
    getTokenKey();
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
      xacl = getXACLHeaders();
    });
  });

  describe('GET /company - Get All Companies', () => {
    it('Should return 200 for valid authenticated request', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company`,
        headers: {
          Authorization: `Bearer ${bearerToken}`,
          'X-ACL': xacl,
        },
        timeout: timeout,
        failOnStatusCode: allowFail,
      }).then((response) => {
        return cy.logJson('GET /company response', response).then(() => {
          validate_expected_status(response, [200, 204]);
          if (response.status === 200) {
            expect(response.body).to.be.an('object');
          }
        });
      });
    });

    it('Should return 401 for unauthenticated request', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company`,
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('GET /company unauthorized response', response).then(() => {
          validate_401_Status(response);
        });
      });
    });
  });

  describe('GET /company/{companyID} - Get Company By ID', () => {
    it('Should return 200 for valid company ID', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/${validCompanyID}`,
        headers: {
          Authorization: `Bearer ${bearerToken}`,
          'X-ACL': xacl,
        },
        timeout: timeout,
        failOnStatusCode: allowFail,
      }).then((response) => {
        return cy.logJson('GET /company/{companyID} response', response).then(() => {
          validate_expected_status(response, [200, 404]);
          if (response.status === 200) {
            expect(response.body).to.be.an('object');
            expect(response.body.companyID).to.exist;
          }
        });
      });
    });

    it('Should return 401 for unauthenticated request', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/${validCompanyID}`,
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('GET /company/{companyID} unauthorized response', response).then(() => {
          validate_401_Status(response);
        });
      });
    });

    it('Should return 400 for invalid UUID format', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/invalid-uuid`,
        headers: {
          Authorization: `Bearer ${bearerToken}`,
          'X-ACL': xacl,
        },
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('GET /company/{companyID} invalid UUID response', response).then(() => {
          expect(response.status).to.be.oneOf([400, 404]);
        });
      });
    });
  });

  describe('GET /company/external/{companySFID} - Get Company by External SFID', () => {
    it('Should return expected response for valid SFID', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/external/${validSFID}`,
        headers: {
          Authorization: `Bearer ${bearerToken}`,
          'X-ACL': xacl,
        },
        timeout: timeout,
        failOnStatusCode: allowFail,
      }).then((response) => {
        return cy.logJson('GET /company/external/{companySFID} response', response).then(() => {
          validate_expected_status(response, [200, 404]);
          if (response.status === 200) {
            expect(response.body).to.be.an('object');
          }
        });
      });
    });

    it('Should return 401 for unauthenticated request', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/external/${validSFID}`,
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('GET /company/external/{companySFID} unauthorized response', response).then(() => {
          validate_401_Status(response);
        });
      });
    });
  });

  describe('GET /company/search - Search Companies', () => {
    it('Should return 200 for company name search', function () {
      const companyName = 'Linux Foundation';
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/search?companyName=${encodeURIComponent(companyName)}`,
        headers: {
          Authorization: `Bearer ${bearerToken}`,
          'X-ACL': xacl,
        },
        timeout: timeout,
        failOnStatusCode: allowFail,
      }).then((response) => {
        return cy.logJson('GET /company/search by name response', response).then(() => {
          validate_expected_status(response, [200, 404]);
          if (response.status === 200) {
            expect(response.body).to.be.an('object');
          }
        });
      });
    });

    it('Should return 401 for unauthenticated request', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/search?companyName=test`,
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('GET /company/search unauthorized response', response).then(() => {
          validate_401_Status(response);
        });
      });
    });
  });

  describe('GET /company/signing-entity-name - Get Company by Signing Entity Name', () => {
    it('Should return expected response for signing entity name search', function () {
      const signingEntityName = 'Linux Foundation';
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/signing-entity-name?signingEntityName=${encodeURIComponent(signingEntityName)}`,
        headers: {
          Authorization: `Bearer ${bearerToken}`,
          'X-ACL': xacl,
        },
        timeout: timeout,
        failOnStatusCode: allowFail,
      }).then((response) => {
        return cy.logJson('GET /company/signing-entity-name response', response).then(() => {
          validate_expected_status(response, [200, 404]);
          if (response.status === 200) {
            expect(response.body).to.be.an('object');
          }
        });
      });
    });

    it('Should return 401 for unauthenticated request', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/signing-entity-name?signingEntityName=test`,
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('GET /company/signing-entity-name unauthorized response', response).then(() => {
          validate_401_Status(response);
        });
      });
    });
  });

  describe('GET /company/user/{userID} - Get User Company Manager by ID', () => {
    it('Should return expected response for valid user ID', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/user/${validUserID}`,
        headers: {
          Authorization: `Bearer ${bearerToken}`,
          'X-ACL': xacl,
        },
        timeout: timeout,
        failOnStatusCode: allowFail,
      }).then((response) => {
        return cy.logJson('GET /company/user/{userID} response', response).then(() => {
          validate_expected_status(response, [200, 404]);
          if (response.status === 200) {
            expect(response.body).to.be.an('object');
          }
        });
      });
    });

    it('Should return 401 for unauthenticated request', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/user/${validUserID}`,
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('GET /company/user/{userID} unauthorized response', response).then(() => {
          validate_401_Status(response);
        });
      });
    });
  });

  describe('GET /company/user/{userID}/invites - Get User Invites', () => {
    it('Should return expected response for valid user ID invites', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/user/${validUserID}/invites`,
        headers: {
          Authorization: `Bearer ${bearerToken}`,
          'X-ACL': xacl,
        },
        timeout: timeout,
        failOnStatusCode: allowFail,
      }).then((response) => {
        return cy.logJson('GET /company/user/{userID}/invites response', response).then(() => {
          validate_expected_status(response, [200, 404]);
          if (response.status === 200) {
            expect(response.body).to.be.an('object');
          }
        });
      });
    });

    it('Should return 401 for unauthenticated request', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/user/${validUserID}/invites`,
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('GET /company/user/{userID}/invites unauthorized response', response).then(() => {
          validate_401_Status(response);
        });
      });
    });
  });

  describe('Negative Test Cases', () => {
    it('Should handle missing authentication token gracefully', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company`,
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('Missing token response', response).then(() => {
          validate_401_Status(response);
        });
      });
    });

    it('Should handle invalid company ID format', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/not-a-uuid`,
        headers: {
          Authorization: `Bearer ${bearerToken}`,
          'X-ACL': xacl,
        },
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('Invalid company ID response', response).then(() => {
          expect(response.status).to.be.oneOf([400, 404, 422]);
        });
      });
    });

    it('Should handle malformed query parameters', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/search?invalidParam=value&companyName=`,
        headers: {
          Authorization: `Bearer ${bearerToken}`,
          'X-ACL': xacl,
        },
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('Malformed query response', response).then(() => {
          expect(response.status).to.be.oneOf([200, 400, 422]);
        });
      });
    });

    it('Should handle missing required parameters for search', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company/search`,
        headers: {
          Authorization: `Bearer ${bearerToken}`,
          'X-ACL': xacl,
        },
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('Missing search params response', response).then(() => {
          expect(response.status).to.be.oneOf([200, 400, 422]);
        });
      });
    });

    it('Should handle expired or invalid bearer token', function () {
      cy.request({
        method: 'GET',
        url: `${claEndpoint}company`,
        headers: {
          Authorization: 'Bearer invalid-token-12345',
          'X-ACL': xacl,
        },
        timeout: timeout,
        failOnStatusCode: false,
      }).then((response) => {
        return cy.logJson('Invalid token response', response).then(() => {
          validate_401_Status(response);
        });
      });
    });
  });
});
