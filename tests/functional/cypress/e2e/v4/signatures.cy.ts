import {
  validateApiResponse,
  validate_200_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
} from '../../support/commands';
//import {appConfig} from  '../../support/config.${Cypress.env("CYPRESS_ENV")}'
describe('To Validate & get list of signatures of ClaGroups via API call', function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../../appConfig/config.production.ts').appConfig;
  }

  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/signatures
  const claEndpoint = getAPIBaseURL('v4');
  const claGroupID = appConfig.claGroupId_projectSFID; //Sun
  const lfid = appConfig.lfid;
  const lfid2 = appConfig.lfid2;
  const lfid3 = appConfig.lfid3;
  const companyID = appConfig.companyID; //Infosys Limited
  const companyName = appConfig.companyName; //Infosys Limited
  const projectSFID = appConfig.projectSFID; //sun
  const userID = appConfig.userIdclaManager; //veerendrat
  const userID2 = appConfig.userIdclaManager2;

  //Aprroval list veriable
  const emailApprovalList = appConfig.emailApprovalList;
  const gitOrgApprovalList = `test-${Date.now()}-${Math.random().toString(36).substr(2, 9)}.com`;
  const gitUsernameApprovalList = appConfig.gitUsernameApprovalList;
  const gitLabOrgApprovalList = appConfig.gitLabOrgApprovalList;
  const domainApprovalList = `test-${Date.now()}-${Math.random().toString(36).substr(2, 9)}.com`;

  let signatureIclaID = '';
  let signatureCclaID = '';
  let signatureID = '';
  let signatureApproved = true;
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  let bearerToken: string = null;

  before(() => {
    if (bearerToken == null) {
      getTokenKey(bearerToken);
      cy.window().then((win) => {
        bearerToken = win.localStorage.getItem('bearerToken');
      });
    }
  });

  it('Returns a list of corporate contributor for the CLA Group', function () {
    let url = `${claEndpoint}cla-group/${claGroupID}/corporate-contributors?companyID=${companyID}&pageSize=100`;
    cy.task('log', 'Returns a list of corporate contributor for the CLA Group URL: ' + url);
    cy.request({
      method: 'GET',
      url: url,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      timeout: 180000,
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        let list = response.body.list;
        for (let i = 0; i <= list.length - 1; i++) {
          if (list[i].linux_foundation_id === lfid3) {
            if (list[i].signatureApproved === true) {
              expect(list[i].signatureApproved).to.be.true;
              signatureApproved = true;
            } else if (list[i].signatureApproved === false) {
              expect(list[i].signatureApproved).to.be.false;
              signatureApproved = false;
            }
            signatureCclaID = list[i].signatureID;
            break;
          }
        }
        cy.task('log', 'Signature ID: ' + signatureCclaID + ' and Approved: ' + signatureApproved);
        validateApiResponse('signatures/listClaGroupCorporateContributors.json', response);
      });
    });
  });

  it('Returns the signature when provided the signature ID, ecla records', function () {
    let url = `${claEndpoint}signatures/id/${signatureCclaID}`;
    cy.task('log', 'Returns the signature when provided the signature ID, ecla records URL: ' + url);
    cy.request({
      method: 'GET',
      url: url,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        let list = response.body;
        expect(list.signatureApproved).to.eql(signatureApproved);
        expect(list.signatureType).to.eql('ecla');
      });
    });
  });

  it('Returns a list of individual signatures for a CLA Group', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}cla-group/${claGroupID}/icla/signatures`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      timeout: 180000,
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('signatures/listClaGroupIclaSignature.json', response);
    });
  });

  it('Returns a list of individual signatures for a CLA Group with searchTerm', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}cla-group/${claGroupID}/icla/signatures?approved=true&signed=true&sortOrder=asc`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      timeout: 180000,
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body.list;
      for (let i = 0; i <= list.length - 1; i++) {
        expect(list[i].signatureApproved).to.eql(true);
        expect(list[i].signatureSigned).to.eql(true);
        signatureIclaID = list[i].signature_id;
      }
    });
  });

  it('Returns the signature when provided the signature ID, icla records', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/id/${signatureIclaID}`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body;
      expect(list.signatureApproved).to.eql(true);
      expect(list.claType).to.eql('icla');
    });
  });

  it('Returns a list of company signatures when provided the company ID', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/company/${companyID}?signatureType=ccla`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      timeout: 180000,
    }).then((response) => {
      validate_200_Status(response);
      let signatures = response.body.signatures;
      for (let i = 0; i <= signatures.length - 1; i++) {
        expect(signatures[i].companyName).to.eql(companyName);
      }
      validateApiResponse('signatures/getCompanySignatures.json', response);
    });
  });

  it('Returns a list of project signature models when provided the project ID', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${claGroupID}?pageSize=10`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      timeout: 180000,
    }).then((response) => {
      validate_200_Status(response);
      validateApiResponse('signatures/getProjectSignatures.json', response);
    });
  });

  it('Downloads the corporate CLA information as a CSV document for this project', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${claGroupID}/ccla/csv`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      timeout: 180000,
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it('Downloads all the corporate CLAs for this project, as pdf', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${claGroupID}/ccla/pdfs`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      timeout: 180000,
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it.skip('Downloads the corporate CLA for this project, as pdf', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${claGroupID}/ccla/${signatureCclaID}/pdf`,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      timeout: 180000,
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it.skip('Downloads all employee CLA information as a CSV document for this project', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${claGroupID}/company/${companyID}/employee/csv`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it('Downloads all ICLA information as a CSV document for this project', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${claGroupID}/icla/csv`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it.skip('Downloads all ICLAs for this project', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${claGroupID}/icla/pdfs`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it.skip("Downloads the user's ICLA for this project", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${claGroupID}/icla/${signatureIclaID}/pdf`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it('Returns a list of ccla signature models when provided the project ID and company ID', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      let signatures = response.body.signatures;
      for (let i = 0; i <= signatures.length - 1; i++) {
        expect(signatures[i].companyName).to.eql(companyName);
        expect(signatures[i].claType).to.eql('ccla');
      }
      validateApiResponse('signatures/getProjectCompanySignatures.json', response);
    });
  });

  it('Get project company signatures for the employees', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/employee`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
      let signatures = response.body.signatures;
      for (let i = 0; i <= signatures.length - 1; i++) {
        expect(signatures[i].companyName).to.eql(companyName);
      }
      validateApiResponse('signatures/getProjectCompanySignatures.json', response);
    });
  });

  it('Returns a list of user signatures when provided the user ID', function () {
    let url = `${claEndpoint}signatures/user/${userID2}`;
    cy.task('log', 'Returns a list of user signatures when provided the user ID URL: ' + url);
    cy.request({
      method: 'GET',
      url: url,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        let signatures = response.body.signatures;
        for (let i = 0; i <= signatures.length - 1; i++) {
          // LG: API /signatures/user/{userID} internally skips ECLA records, and for ICLA we never have company
          expect(signatures[i].companyName).to.be.undefined;
          expect(signatures[i].signatureReferenceType).to.eql('user');
          signatureID = signatures[i].signatureID;
        }
        validateApiResponse('signatures/getProjectCompanySignatures.json', response);
      });
    });
  });

  it('GET: Updates the specified signature GitHub Organization approval list', function () {
    cy.request({
      method: 'GET',
      // we can't use inclusive name yet as it is inside API URL.
      url: `${claEndpoint}signatures/${signatureID}/gh-org-whitelist`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it.skip('POST: Updates the specified signature GitHub organization approval list', function () {
    cy.request({
      method: 'POST',
      // we can't use inclusive name yet as it is inside API URL.
      url: `${claEndpoint}signatures/${signatureID}/gh-org-whitelist`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        organization_id: '35275118',
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it.skip('Returns the signature signed document when provided the signature ID', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}signatures/${signatureID}/signed-document`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  /* Below test case for Updates the Project / Organization/Company Approval list */

  it('Add Email as Approval List to the Project/Company', function () {
    let url = `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/clagroup/${claGroupID}/approval-list`;
    cy.task('log', 'Add Email as Approval List to the Project/Company URL: ' + url);
    cy.request({
      method: 'PUT',
      url: url,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        AddEmailApprovalList: [emailApprovalList],
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        let list = response.body.emailApprovalList;
        expect(list).to.include(emailApprovalList);
      });
    });
  });

  it('Remove Email form Approval List to the Project/Company', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/clagroup/${claGroupID}/approval-list`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        RemoveEmailApprovalList: [emailApprovalList],
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body.emailApprovalList;
      if (list != null) {
        for (let i = 0; i < list.length; i++) {
          expect(list[i]).to.not.equal(emailApprovalList);
        }
      }
    });
  });

  it('Add GithubOrg as Approval List to the Project/Company', function () {
    let url = `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/clagroup/${claGroupID}/approval-list`;
    cy.task('log', 'Add GithubOrg as Approval List to the Project/Company URL: ' + url);
    cy.request({
      method: 'PUT',
      url: url,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        AddGithubOrgApprovalList: [gitOrgApprovalList],
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        cy.task('log', 'domain ' + gitOrgApprovalList + ' should be added to approval list');
        let list = response.body.githubOrgApprovalList;
        expect(list).to.include(gitOrgApprovalList);
      });
    });
  });

  it('Remove GithubOrg form Approval List to the Project/Company', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/clagroup/${claGroupID}/approval-list`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        RemoveGithubOrgApprovalList: [gitOrgApprovalList],
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        cy.task('log', 'domain ' + gitOrgApprovalList + ' should be removed from approval list');
        let list = response.body.githubOrgApprovalList;
        if (list != null) {
          for (let i = 0; i < list.length; i++) {
            expect(list[i]).to.not.equal(gitOrgApprovalList);
          }
        }
      });
    });
  });

  it('Add Github Username as Approval List to the Project/Company', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/clagroup/${claGroupID}/approval-list`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        AddGithubUsernameApprovalList: [gitUsernameApprovalList],
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body.githubUsernameApprovalList;
      let found = false;
      for (let i = 0; i < list.length; i++) {
        if (list[i] === gitUsernameApprovalList) {
          expect(list[i]).to.eql(gitUsernameApprovalList);
          found = true;
          break;
        }
      }
      if (!found) {
        expect.fail('GitHub Username not found in approval list');
      }
    });
  });

  it('Remove Github Username form Approval List to the Project/Company', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/clagroup/${claGroupID}/approval-list`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        RemoveGithubusernameApprovalList: [gitUsernameApprovalList],
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body.githubUsernameApprovalList;
      if (list != null) {
        for (let i = 0; i < list.length; i++) {
          expect(list[i]).to.not.equal(gitUsernameApprovalList);
        }
      }
    });
  });

  it('Add GitLab Username as Approval List to the Project/Company', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/clagroup/${claGroupID}/approval-list`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        AddGitlabUsernameApprovalList: [gitUsernameApprovalList],
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body.gitlabUsernameApprovalList;
      let found = false;
      for (let i = 0; i < list.length; i++) {
        if (list[i] === gitUsernameApprovalList) {
          expect(list[i]).to.eql(gitUsernameApprovalList);
          found = true;
          break;
        }
      }
      if (!found) {
        expect.fail('GitLab Username not found in approval list');
      }
    });
  });

  it('Remove GitLab Username form Approval List to the Project/Company', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/clagroup/${claGroupID}/approval-list`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        RemoveGitlabUsernameApprovalList: [gitUsernameApprovalList],
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body.gitlabUsernameApprovalList;
      if (list != null) {
        for (let i = 0; i < list.length; i++) {
          expect(list[i]).to.not.equal(gitUsernameApprovalList);
        }
      }
    });
  });

  it('Add GitLab Org as Approval List to the Project/Company', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/clagroup/${claGroupID}/approval-list`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        AddGitlabOrgApprovalList: [gitLabOrgApprovalList],
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body.gitlabOrgApprovalList;
      let found = false;
      for (let i = 0; i < list.length; i++) {
        if (list[i] === gitLabOrgApprovalList) {
          expect(list[i]).to.eql(gitLabOrgApprovalList);
          found = true;
          break;
        }
      }
      if (!found) {
        expect.fail('GitLab Org not found in approval list');
      }
    });
  });

  it('Remove GitLab Org form Approval List to the Project/Company', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/clagroup/${claGroupID}/approval-list`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        RemoveGitlabOrgApprovalList: [gitLabOrgApprovalList],
      },
    }).then((response) => {
      validate_200_Status(response);
      let list = response.body.gitlabOrgApprovalList;
      if (list != null) {
        for (let i = 0; i < list.length; i++) {
          expect(list[i]).to.not.equal(gitLabOrgApprovalList);
        }
      }
    });
  });

  it('Add domain as Approval List to the Project/Company', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/clagroup/${claGroupID}/approval-list`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        AddDomainApprovalList: [domainApprovalList],
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        cy.task('log', 'domain ' + domainApprovalList + ' should be added to approval list');
        let list = response.body.domainApprovalList;
        let found = false;
        for (let i = 0; i < list.length; i++) {
          if (list[i] === domainApprovalList) {
            expect(list[i]).to.eql(domainApprovalList);
            found = true;
            break;
          }
        }
        if (!found) {
          expect.fail('Domain not found in approval list');
        }
      });
    });
  });

  it('Remove domain form Approval List to the Project/Company', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}/clagroup/${claGroupID}/approval-list`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        RemoveDomainApprovalList: [domainApprovalList],
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        validate_200_Status(response);
        cy.task('log', 'domain ' + domainApprovalList + ' should be removed from approval list');
        let list = response.body.domainApprovalList;
        if (list != null) {
          for (let i = 0; i < list.length; i++) {
            expect(list[i]).to.not.equal(domainApprovalList);
          }
        }
      });
    });
  });

  //Updates CCLA signature record for the auto_create_ecla flag.

  it('Updates CCLA signature record for the auto_create_ecla flag to false', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/company/${companyID}/clagroup/${claGroupID}/ecla-auto-create`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        auto_create_ecla: false,
      },
    }).then((response) => {
      validate_200_Status(response);
      if (response.status === 200) {
        cy.request({
          method: 'GET',
          url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}`,
          timeout: 180000,
          failOnStatusCode: allowFail,
          headers: getXACLHeader(),
          auth: {
            bearer: bearerToken,
          },
        }).then((response) => {
          validate_200_Status(response);
          let list = response.body.signatures;
          expect(list[0].autoCreateECLA).to.eql(false);
        });
      }
    });
  });

  it('Updates CCLA signature record for the auto_create_ecla flag to true', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}signatures/company/${companyID}/clagroup/${claGroupID}/ecla-auto-create`,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        auto_create_ecla: true,
      },
    }).then((response) => {
      validate_200_Status(response);
      if (response.status === 200) {
        cy.request({
          method: 'GET',
          url: `${claEndpoint}signatures/project/${projectSFID}/company/${companyID}`,
          timeout: 180000,
          failOnStatusCode: allowFail,
          headers: getXACLHeader(),
          auth: {
            bearer: bearerToken,
          },
        }).then((response) => {
          validate_200_Status(response);
          let list = response.body.signatures;
          expect(list[0].autoCreateECLA).to.eql(true);
        });
      }
    });
  });

  //Invalidates a given ICLA record for a user
  //worked only ine time, So skiping this test case, Refer screenshot: https://prnt.sc/ti6ERw8XZur0

  // LG:skip
  it.skip('Invalidates a given ICLA record for a user', function () {
    let user_id = '23121f2a-d48b-11ed-b70f-d2f23b35d89e';
    let url = `${claEndpoint}cla-group/${claGroupID}/user/${user_id}/icla`;
    cy.task('log', 'Invalidates a given ICLA record for a user URL: ' + url);
    cy.request({
      method: 'PUT',
      url: url,
      timeout: 180000,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      return cy.logJson('response', response).then(() => {
        if (response.status === 500) {
          Cypress.on('test:after:run', (test, runnable) => {
            const testName = `${test.title}`;
            const jsonResponse = JSON.stringify(response.body, null, 2);
            cy.log(jsonResponse);
            console.log(testName);
            console.error('User_id not available for invalidate : ', jsonResponse);
          });
        } else {
          validate_200_Status(response);
        }
      });
    });
  });
});
