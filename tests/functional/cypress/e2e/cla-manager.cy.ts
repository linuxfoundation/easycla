import {
  validateApiResponse,
  validate_200_Status,
  validate_204_Status,
  validate_404_Status,
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

  it('Assigns CLA Manager designee to a given user.', function () {
    let url = `${claEndpoint}company/${companyID}/claGroup/${claGroupID}/cla-manager-designee`;
    cy.task('log', `--> POST ${url}`);
    cy.request({
      method: 'POST',
      url: url,
      timeout: 180000,
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

  it('Allows an existing CLA Manager to add another CLA Manager to the specified Company and Project.', function () {
    let url = `${claEndpoint}company/${companyID}/project/${projectSFID}/cla-manager`;
    cy.task('log', `--> POST ${url}`);
    cy.request({
      method: 'POST',
      url: url,
      timeout: 180000,
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
      timeout: 180000,
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

  it('Assigns CLA Manager designee to a given user', function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}company/${companyID}/project/${projectSFID_Designee}/cla-manager-designee`,
      timeout: 180000,
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
      timeout: 180000,
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
      timeout: 180000,
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
      timeout: 180000,
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
      timeout: 180000,
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

  function getClaManager() {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}company/${companySFID}/user/${userLFID}/claGroupID/${claGroupID}/is-cla-manager-designee`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      expect(response.body.hasRole).to.eq(true);
      expect(response.body.lfUsername).to.eq(userLFID);
      // validateApiResponse("cla-manager/isCLAManagerDesignee.json",response)
    });
  }
});
