// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import {
  validate_200_Status,
  validateApiResponse,
  getAPIBaseURL,
  validate_expected_status,
} from '../../support/commands';

describe('To Validate & test Version APIs via API call (V3)', function () {
  const claEndpoint = getAPIBaseURL('v3');
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL');

  it('Returns the application version - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}ops/version`,
      timeout: timeout,
      failOnStatusCode: allowFail,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        expect(response.body).to.be.an('object');
        expect(response.body).to.have.property('version');
        expect(response.body).to.have.property('commit');
        validateApiResponse('version/getVersion.json', response);
      });
    });
  });

  describe('Expected failures', () => {
    it('Returns errors due to malformed requests for Version APIs', function () {
      const cases: Array<{
        title: string;
        method: 'GET' | 'POST' | 'PUT' | 'DELETE';
        url: string;
        body?: any;
        expectedStatus?: number;
        expectedCode?: number;
        expectedMessage?: string;
        expectedMessageContains?: boolean;
      }> = [
        {
          title: 'POST /ops/version (method not allowed)',
          method: 'POST',
          url: `${claEndpoint}ops/version`,
          body: {},
          expectedStatus: 405,
          expectedCode: 405,
          expectedMessage: 'method POST is not allowed, but [GET] are',
          expectedMessageContains: true,
        },
      ];

      cy.wrap(cases).each((c: any) => {
        return cy
          .request({
            method: c.method,
            url: c.url,
            body: c.body,
            failOnStatusCode: false,
            timeout,
          })
          .then((response) => {
            return cy.logJson('response', response).then(() => {
              cy.task('log', `Testing: ${c.title}`);
              validate_expected_status(
                response,
                c.expectedStatus,
                c.expectedCode,
                c.expectedMessage,
                c.expectedMessageContains,
              );
            });
          });
      });
    });
  });
});
