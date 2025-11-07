// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

import {
  validateApiResponse,
  validate_200_Status,
  validate_400_Status,
  validate_401_Status,
  validate_403_Status,
  validate_404_Status,
  validate_404_Status_and_Message,
  validate_405_Status_and_Message,
  validate_422_Status,
  getTokenKey,
  getAPIBaseURL,
  getXACLHeader,
} from '../../support/commands';

describe("To Validate 'GET, CREATE, UPDATE and DELETE' CLA groups API call on child project", function () {
  // Define a variable for the environment
  const environment = Cypress.env('CYPRESS_ENV');

  // Import the appropriate configuration based on the environment
  let appConfig;
  if (environment === 'dev') {
    appConfig = require('../../appConfig/config.dev.ts').appConfig;
  } else if (environment === 'production') {
    appConfig = require('../../appConfig/config.production.ts').appConfig;
  }

  //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/cla-group

  const claEndpoint = getAPIBaseURL('v4');
  let claGroupId: string = '';
  let claGroupId2: string = appConfig.claGroupId; //sun project claGroupID

  //Variable for create cla group
  const foundation_sfid = appConfig.foundationSFID; //project name: easyAutom foundation
  const projectSfid = appConfig.createNewClaGroupSFID; //project name: easyAutom-child1
  const cla_group_name = appConfig.claGroupName;
  const cla_group_description = 'Added via cypress script';

  //variable for update cla group
  const updated_cla_group_name = 'Cypress_Updated_ClaGroup';
  const update_cla_group_description = 'CLA group created and updated for easy cla automation child project 1';

  //Variable for GitHub
  const gitHubOrgName = appConfig.gitHubOrgPartialStatus;
  const projectSfidOrg = appConfig.projectSFID; //project name: sun

  //Enroll /unEnroll projects
  const enrollProjectsSFID = appConfig.enrollProjectsSFID; //project name: easyAutomChild1-GrandChild1
  const child_Project_name = appConfig.child_Project_name;
  let allowFail: boolean = !(Cypress.env('ALLOW_FAIL') === 1);
  const timeout = 180000;
  const local = Cypress.env('LOCAL') ? true : false;

  let bearerToken: string = null;
  before(() => {
    if (bearerToken == null) {
      getTokenKey(bearerToken);
      cy.window().then((win) => {
        bearerToken = win.localStorage.getItem('bearerToken');
      });
    }
  });

  describe('Expected failures', () => {
    it('Returns 401 for all CLA APIs when called without token', function () {
      const dummyClaGroupId = '00000000-0000-0000-0000-000000000000';

      // Minimal-but-valid bodies (in case schema is checked before auth; server should still 401)
      const createBody = {
        icla_enabled: true,
        ccla_enabled: true,
        ccla_requires_icla: true,
        cla_group_description: cla_group_description,
        cla_group_name: cla_group_name,
        foundation_sfid: foundation_sfid,
        project_sfid_list: [projectSfid],
        template_fields: {
          TemplateID: 'fb4cc144-a76c-4c17-8a52-c648f158fded',
          MetaFields: [
            {
              description: "Project's Full Name.",
              name: 'Project Name',
              templateVariable: 'PROJECT_NAME',
              value: 'Test',
            },
            {
              description: 'The Full Entity Name of the Project.',
              name: 'Project Entity Name',
              templateVariable: 'PROJECT_ENTITY_NAME',
              value: 'Test',
            },
            {
              description: 'The E-Mail Address of the Person managing the CLA. ',
              name: 'Contact Email Address',
              templateVariable: 'CONTACT_EMAIL',
              value: 'veerendrat@proximabiz.com',
            },
          ],
        },
      };

      const updateBody = {
        cla_group_description: update_cla_group_description,
        cla_group_name: updated_cla_group_name,
      };

      const enrollBody = [enrollProjectsSFID];
      const unenrollBody = [enrollProjectsSFID];

      const ghOrgConfigBody = {
        autoEnabled: true,
        autoEnabledClaGroupID: dummyClaGroupId,
        branchProtectionEnabled: true,
      };

      const requests = [
        // Create CLA group
        { method: 'POST', url: `${claEndpoint}cla-group`, body: createBody },

        // List CLA groups for a project (valid SFID)
        { method: 'GET', url: `${claEndpoint}foundation/${projectSfid}/cla-groups` },

        // List CLA groups for a project (wrong SFID)
        { method: 'GET', url: `${claEndpoint}foundation/${projectSfid}-xyz/cla-groups` },

        // Update CLA group (using dummy ID)
        { method: 'PUT', url: `${claEndpoint}cla-group/${dummyClaGroupId}`, body: updateBody },

        // Enroll projects (using dummy ID)
        { method: 'PUT', url: `${claEndpoint}cla-group/${dummyClaGroupId}/enroll-projects`, body: enrollBody },

        // Unenroll projects (using dummy ID)
        { method: 'PUT', url: `${claEndpoint}cla-group/${dummyClaGroupId}/unenroll-projects`, body: unenrollBody },

        // Get GitHub orgs for project
        { method: 'GET', url: `${claEndpoint}project/${projectSfidOrg}/github/organizations` },

        // Update GitHub org config (using dummy CLA group ID)
        {
          method: 'PUT',
          url: `${claEndpoint}project/${projectSfidOrg}/github/organizations/${gitHubOrgName}/config`,
          body: ghOrgConfigBody,
        },

        // Delete CLA group (using dummy ID)
        { method: 'DELETE', url: `${claEndpoint}cla-group/${dummyClaGroupId}` },
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
          // Helpful debug when something fails
          return cy.logJson('401 response', response).then(() => {
            validate_401_Status(response, local);
          });
        });
      });
    });

    it('Returns errors due to missing parameters', function () {
      const dummyClaGroupId = '00000000-0000-0000-0000-000000000000';
      const exampleClaGroupId = 'd9428888-122b-4b20-8c4a-0c9a1a6f9b8e';
      const requests = [
        // Create CLA group without body
        {
          method: 'POST',
          url: `${claEndpoint}cla-group`,
          expectedStatus: 422,
          expectedCode: 602,
          expectedMsg: 'claGroupInput in body is required',
          mode: 'both',
        },

        // List CLA groups for a project - wrong SFID (missing)
        {
          method: 'GET',
          url: `${claEndpoint}foundation//cla-groups`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'path /v4/foundation//cla-groups was not found',
          mode: 'local',
        },

        // List CLA groups for a project - wrong SFID (missing)
        {
          method: 'GET',
          url: `${claEndpoint}foundation//cla-groups`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },

        // List CLA groups for a project - wrong SFID (too short)
        {
          method: 'GET',
          url: `${claEndpoint}foundation/xyz/cla-groups`,
          expectedStatus: 422,
          expectedCode: 604,
          expectedMsg: 'projectSFID in path should be at least 15 chars long',
          mode: 'both',
        },

        // List CLA groups for a project - wrong SFID (not matching regexp)
        {
          method: 'GET',
          url: `${claEndpoint}foundation/xyz-abc-123-abcd/cla-groups`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMsg: "projectSFID in path should match '^([0-9A-Za-z]{15}|[0-9A-Za-z]{18})$'",
          mode: 'both',
        },

        // List CLA groups for a project (wrong SFID - too long)
        {
          method: 'GET',
          url: `${claEndpoint}foundation/${projectSfid}aaaa/cla-groups`,
          expectedStatus: 422,
          expectedCode: 603,
          expectedMsg: 'projectSFID in path should be at most 18 chars long',
          mode: 'both',
        },

        // Update CLA group (using dummy ID which isn't correct UUID v4)
        {
          method: 'PUT',
          url: `${claEndpoint}cla-group/${dummyClaGroupId}`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMsg:
            "claGroupID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          mode: 'both',
        },

        // Update CLA group (using dummy ID which is well-formed but no body parameters)
        {
          method: 'PUT',
          url: `${claEndpoint}cla-group/${exampleClaGroupId}`,
          expectedStatus: 400,
          expectedCode: 602,
          expectedMsg: 'EasyCLA - 400 Bad Request - missing update parameters - body missing required values',
          mode: 'both',
        },

        // Enroll projects (using dummy ID - incorrect UUIDv4)
        {
          method: 'PUT',
          url: `${claEndpoint}cla-group/${dummyClaGroupId}/enroll-projects`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMsg:
            "claGroupID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          mode: 'both',
        },

        // Enroll projects (using correct but non existing ID)
        {
          method: 'PUT',
          url: `${claEndpoint}cla-group/${exampleClaGroupId}/enroll-projects`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `EasyCLA - 404 Not Found - problem loading CLA Group by ID: ${exampleClaGroupId} - error: cla group ${exampleClaGroupId} not found`,
          mode: 'both',
        },

        // Enroll projects (using correct ID - but without PUT data)
        {
          method: 'PUT',
          url: `${claEndpoint}cla-group/${claGroupId2}/enroll-projects`,
          expectedStatus: 400,
          expectedCode: 400,
          expectedMsg:
            'EasyCLA - 400 Bad Request - unable to enroll projects in CLA Group - error: enroll validation error: invalid project ID value due to error: validation failure - there should be at least one project provided for the enroll request',
          mode: 'both',
        },

        // Unenroll projects (using dummy ID - incorrect UUIDv4)
        {
          method: 'PUT',
          url: `${claEndpoint}cla-group/${dummyClaGroupId}/unenroll-projects`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMsg:
            "claGroupID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          mode: 'both',
        },

        // Unenroll projects (using correct but non-existing ID)
        {
          method: 'PUT',
          url: `${claEndpoint}cla-group/${exampleClaGroupId}/unenroll-projects`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `EasyCLA - 404 Not Found - unable to locate CLA Group by ID: ${exampleClaGroupId} - error: cla group ${exampleClaGroupId} not found`,
          mode: 'both',
        },

        // Unenroll projects (using correct ID - but without PUT data)
        {
          method: 'PUT',
          url: `${claEndpoint}cla-group/${claGroupId2}/unenroll-projects`,
          expectedStatus: 400,
          expectedCode: 400,
          expectedMsg:
            'EasyCLA - 400 Bad Request - unable to enroll projects in CLA Group - error: unenroll validation error: invalid project ID value due to error: validation failure - there should be at least one project provided for the unenroll request',
          mode: 'both',
        },

        // Get GitHub orgs for project - no project provided
        {
          method: 'GET',
          url: `${claEndpoint}project//github/organizations`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'path /v4/project//github/organizations was not found',
          mode: 'local',
        },

        // Get GitHub orgs for project - no project provided
        {
          method: 'GET',
          url: `${claEndpoint}project//github/organizations`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },

        // LG: Get GitHub orgs for project - wrong SFID provided, but API is not verifying this in swagger
        {
          method: 'GET',
          url: `${claEndpoint}project/aaaa/github/organizations`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'EasyCLA - 404 Not Found - github organization with project SFID not found: aaaa',
          mode: 'both',
        },

        // Update GitHub org config missing project and github org name and update configuration data
        {
          method: 'PUT',
          url: `${claEndpoint}project//github/organizations//config`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'path /v4/project//github/organizations//config was not found',
          mode: 'local',
        },

        // Update GitHub org config missing project and github org name and update configuration data
        {
          method: 'PUT',
          url: `${claEndpoint}project//github/organizations//config`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },

        // Update GitHub org config missing github org name and update configuration data
        {
          method: 'PUT',
          url: `${claEndpoint}project//github/organizations/org/config`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: 'path /v4/project//github/organizations/org/config was not found',
          mode: 'local',
        },

        // Update GitHub org config missing github org name and update configuration data
        {
          method: 'PUT',
          url: `${claEndpoint}project//github/organizations/org/config`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },

        // Update GitHub org config missing project and update configuration data
        {
          method: 'PUT',
          url: `${claEndpoint}project/aaa/github/organizations//config`,
          expectedStatus: 405,
          expectedCode: 405,
          expectedMsg: 'method PUT is not allowed, but [DELETE] are',
          mode: 'local',
        },

        // Update GitHub org config missing project and update configuration data
        {
          method: 'PUT',
          url: `${claEndpoint}project/aaa/github/organizations//config`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },

        // Update GitHub org config missing update configuration data
        {
          method: 'PUT',
          url: `${claEndpoint}project/aaa/github/organizations/aaa/config`,
          expectedStatus: 422,
          expectedCode: 602,
          expectedMsg: 'body in body is required',
          mode: 'both',
        },

        // Delete CLA group - but not specifying claGroupID at all
        {
          method: 'DELETE',
          url: `${claEndpoint}cla-group/`,
          expectedStatus: 405,
          expectedCode: 405,
          expectedMsg: 'method DELETE is not allowed, but [POST] are',
          mode: 'local',
        },

        // Delete CLA group - but not specifying claGroupID at all
        {
          method: 'DELETE',
          url: `${claEndpoint}cla-group/`,
          expectedStatus: 403,
          expectedCode: 403,
          expectedMsg: '',
          mode: 'remote',
        },

        // Delete CLA group specifying dummy ID which is not a correct UUIDv4
        {
          method: 'DELETE',
          url: `${claEndpoint}cla-group/${dummyClaGroupId}`,
          expectedStatus: 422,
          expectedCode: 605,
          expectedMsg:
            "claGroupID in path should match '^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?4[a-fA-F0-9]{3}-?[89ab][a-fA-F0-9]{3}-?[a-fA-F0-9]{12}$'",
          mode: 'both',
        },

        // Delete CLA group specifying an ID that does not exist
        {
          method: 'DELETE',
          url: `${claEndpoint}cla-group/${exampleClaGroupId}`,
          expectedStatus: 404,
          expectedCode: 404,
          expectedMsg: `EasyCLA - 404 Not Found - cla group ${exampleClaGroupId} not found`,
          mode: 'both',
        },
      ];

      const effectiveMode = local ? 'local' : 'remote';
      const filtered = requests.filter((r) => r.mode === 'both' || r.mode === effectiveMode);
      cy.wrap(filtered).each((req: any) => {
        const { method, url, expectedStatus, expectedCode, expectedMsg, mode } = req;
        cy.task('log', `--> ${method} ${url}`);
        cy.request({
          method: method,
          url: url,
          failOnStatusCode: false,
          headers: getXACLHeader(),
          auth: {
            bearer: bearerToken,
          },
          timeout,
        }).then((response) => {
          // Helpful debug when something fails
          return cy.logJson('response', response).then(() => {
            switch (expectedStatus) {
              case 400:
                return validate_400_Status(response, expectedMsg);
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

  it('Creates a new CLA Group at child level - Record should return 200 Response', function () {
    cy.request({
      method: 'POST',
      url: `${claEndpoint}cla-group`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        icla_enabled: true,
        ccla_enabled: true,
        ccla_requires_icla: true,
        cla_group_description: cla_group_description,
        cla_group_name: cla_group_name,
        foundation_sfid: foundation_sfid,
        project_sfid_list: [projectSfid],

        template_fields: {
          TemplateID: 'fb4cc144-a76c-4c17-8a52-c648f158fded',
          MetaFields: [
            {
              description: "Project's Full Name.",
              name: 'Project Name',
              templateVariable: 'PROJECT_NAME',
              value: 'Test',
            },
            {
              description: 'The Full Entity Name of the Project.',
              name: 'Project Entity Name',
              templateVariable: 'PROJECT_ENTITY_NAME',
              value: 'Test',
            },
            {
              description: 'The E-Mail Address of the Person managing the CLA. ',
              name: 'Contact Email Address',
              templateVariable: 'CONTACT_EMAIL',
              value: 'veerendrat@proximabiz.com',
            },
          ],
        },
      },
    }).then((response) => {
      const jsonResponse = JSON.stringify(response.body, null, 2);
      cy.log(jsonResponse);
      // expect(response.duration).to.be.lessThan(20000);
      validate_200_Status(response);

      // Validate specific data in the response
      expect(response.body).to.have.property('cla_group_name', cla_group_name);
      claGroupId = response.body.cla_group_id;

      //To validate schema of response
      validateApiResponse('claGroup/create_claGroup2.json', response.body);
    });
  });

  it('Get list of cla group associated with project - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}foundation/${projectSfid}/cla-groups`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      validate_200_Status(response);

      // Validate specific data in the response
      expect(response.body).to.have.property('list');
      let list = response.body.list;
      claGroupId = list[0].cla_group_id;
      expect(list[0].cla_group_name).to.eql(cla_group_name);

      //To validate schema of response
      validateApiResponse('claGroup/list_claGroup.json', response.body);
    });
  });

  it('Attempt to get list of cla group associated with project given by wrong SFID - Record should Return 404 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}foundation/${projectSfid}-xyz/cla-groups`,
      timeout: timeout,
      failOnStatusCode: false,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
    }).then((response) => {
      // LG:chain log before async status check
      return cy
        .logJson('404 response', response)
        .then(() => validate_422_Status(response, 603, 'projectSFID in path should be at most 18 chars long'));
      //validate_404_Status(response);
    });
  });

  it('Updates a CLA Group details - Record should return 200 Response', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}cla-group/${claGroupId}`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        cla_group_description: update_cla_group_description,
        cla_group_name: updated_cla_group_name,
      },
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      validate_200_Status(response);

      // Validate specific data in the response
      expect(response.body).to.have.property('cla_group_name', updated_cla_group_name);
      expect(response.body).to.have.property('cla_group_description', update_cla_group_description);

      //To validate schema of response
      validateApiResponse('claGroup/update_claGroup2.json', response.body);
    });
  });

  it('Enroll projects in a CLA Group - Record should return 200 Response', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}cla-group/${claGroupId}/enroll-projects`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: [enrollProjectsSFID],
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      validate_200_Status(response);
      // Check if the first API response status is 200
      if (response.status === 200) {
        // Run the second API request
        cy.request({
          method: 'GET',
          url: `${claEndpoint}foundation/${projectSfid}/cla-groups`,
          timeout: timeout,
          failOnStatusCode: allowFail,
          headers: getXACLHeader(),
          auth: {
            bearer: bearerToken,
          },
        }).then((secondResponse) => {
          // Validate specific data in the response
          expect(secondResponse.body).to.have.property('list');
          let list = secondResponse.body.list;
          expect(list[0].project_list[1].project_name).to.eql(child_Project_name);
          expect(list[0].project_list[1].project_sfid).to.eql(enrollProjectsSFID);
          expect(list[0].project_list[0].project_sfid).to.eql(projectSfid);
        });
      } else {
        console.log('First API request did not return a 200 status.');
      }
    });
  });

  it('Unenroll projects in a CLA Group - Record should return 200 Response', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}cla-group/${claGroupId}/unenroll-projects`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: [enrollProjectsSFID],
    }).then((response) => {
      // expect(response.duration).to.be.lessThan(20000);
      validate_200_Status(response);
    });
  });

  it('Get list of Github organization associated with project - Record should return 200 Response', function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}project/${projectSfidOrg}/github/organizations`,
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
      // LG:
      // expect(list[2].github_organization_name).to.eql('Sun-lfxfoundationOrgTest');
      // expect(list[2].connection_status).to.eql('partial_connection');
      expect(list[2].github_organization_name).to.eql('lukaszgryglicki-org');
      expect(list[2].connection_status).to.eql('connected');
    });
  });

  it('Update GitHub Organization Configuration - Record should return 200 Response', function () {
    cy.request({
      method: 'PUT',
      url: `${claEndpoint}project/${projectSfidOrg}/github/organizations/${gitHubOrgName}/config`,
      timeout: timeout,
      failOnStatusCode: allowFail,
      headers: getXACLHeader(),
      auth: {
        bearer: bearerToken,
      },
      body: {
        autoEnabled: true,
        autoEnabledClaGroupID: claGroupId,
        branchProtectionEnabled: true,
      },
    }).then((response) => {
      validate_200_Status(response);
    });
  });

  it('Deletes the CLA Group - Record should return 204 Response', function () {
    if (claGroupId != null) {
      cy.request({
        method: 'DELETE',
        url: `${claEndpoint}cla-group/${claGroupId}`,
        timeout: timeout,
        failOnStatusCode: allowFail,
        headers: getXACLHeader(),
        auth: {
          bearer: bearerToken,
        },
      }).then((response) => {
        expect(response.status).to.eq(204);
        // Check if the first API response status is 200
        if (response.status === 204) {
          // Run the second API request
          cy.request({
            method: 'GET',
            url: `${claEndpoint}foundation/${projectSfid}/cla-groups`,
            timeout: timeout,
            failOnStatusCode: allowFail,
            headers: getXACLHeader(),
            auth: {
              bearer: bearerToken,
            },
          }).then((secondResponse) => {
            // Validate specific data in the response
            cy.wrap(secondResponse.body.list)
              .should('be.an', 'array') // Check if the response is an array
              .and('have.length', 0);
          });
        } else {
          console.log('First API request did not return a 204 status.');
        }
      });
    } else {
      console.log('claGroupId is null' + claGroupId);
    }
  });
});
