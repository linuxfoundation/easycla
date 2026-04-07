// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import { getAPIBaseURL, getTokenKey, getXACLHeader, getXACLHeaders, validate_200_Status } from '../../support/commands';

describe('Current user name refresh regression', function () {
  const v2Endpoint = getAPIBaseURL('v2');
  const v3Endpoint = getAPIBaseURL('v3');
  const v4Endpoint = getAPIBaseURL('v4');
  const timeout = 180000;

  let bearerToken: string = null;
  let restorePending = false;
  let originalUserID: string = null;
  let originalLFUsername: string = null;
  let originalUserName: string = null;

  before(() => {
    getTokenKey();
    cy.window().then((win) => {
      bearerToken = win.localStorage.getItem('bearerToken');
    });
  });

  afterEach(() => {
    if (!restorePending || !originalLFUsername || !originalUserName) {
      return;
    }

    cy.request({
      method: 'PUT',
      url: `${v3Endpoint}users`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeaders(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        userID: originalUserID,
        lfUsername: originalLFUsername,
        username: originalUserName,
      },
    }).then((response) => {
      expect(response.status).to.eq(200);
      restorePending = false;
      originalUserID = null;
      originalLFUsername = null;
      originalUserName = null;
    });
  });

  function assertUserNameRefresh(currentUserURL: string): void {
    const staleUserName = `stale-e2e-${Date.now()}-${Math.floor(Math.random() * 10000)}`;

    cy.request({
      method: 'GET',
      url: currentUserURL,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((currentUserResponse) => {
      validate_200_Status(currentUserResponse);

      originalUserID = currentUserResponse.body.userID || currentUserResponse.body.user_id;
      originalLFUsername = currentUserResponse.body.lfUsername || currentUserResponse.body.lf_username;
      originalUserName = currentUserResponse.body.username || currentUserResponse.body.user_name;

      expect(originalUserID).to.be.a('string').and.not.be.empty;
      expect(originalLFUsername).to.be.a('string').and.not.be.empty;
      expect(originalUserName).to.be.a('string').and.not.be.empty;

      cy.request({
        method: 'PUT',
        url: `${v3Endpoint}users`,
        timeout: timeout,
        failOnStatusCode: false,
        headers: getXACLHeaders(),
        auth: {
          bearer: bearerToken,
        },
        body: {
          userID: originalUserID,
          lfUsername: originalLFUsername,
          username: staleUserName,
        },
      }).then((updateResponse) => {
        expect(updateResponse.status).to.eq(200);
        restorePending = true;

        cy.request({
          method: 'GET',
          url: currentUserURL,
          timeout: timeout,
          failOnStatusCode: false,
          headers: getXACLHeader(),
          auth: {
            bearer: bearerToken,
          },
        }).then((refreshedResponse) => {
          validate_200_Status(refreshedResponse);

          const refreshedUserName = refreshedResponse.body.username || refreshedResponse.body.user_name;

          expect(refreshedUserName).to.eq(originalUserName);
          expect(refreshedUserName).to.not.eq(staleUserName);
        });
      });
    });
  }

  it('refreshes a stale stored user_name through the legacy /v2/user-from-token endpoint', function () {
    assertUserNameRefresh(`${v2Endpoint}user-from-token`);
  });

  it('refreshes a stale stored user_name through the current /v4/user-from-token endpoint', function () {
    assertUserNameRefresh(`${v4Endpoint}user-from-token`);
  });
});
