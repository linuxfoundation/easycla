describe("To Validate github-organizations API call", function () {
    //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/foundation
    const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/foundation-mapping`;
    let bearerToken: string = "";
    const foundationSFID='a09P000000DsNGsIAN'; //project name: easyAutom foundation
    const Ajv = require('ajv');
     //Headers
  let optionalHeaders: Headers = {
    "X-LFX-CACHE": false,
  }

before(() => {   
   
    cy.request({
      method: 'POST',
      url: Cypress.env("AUTH0_TOKEN_API"),
     
      body: {
        "grant_type": "http://auth0.com/oauth/grant-type/password-realm",
        "realm": "Username-Password-Authentication",
        "username":Cypress.env("AUTH0_USER_NAME"),
        "password":Cypress.env("AUTH0_PASSWORD"),
        "client_id":Cypress.env("AUTH0_CLIENT_ID"),
        "audience": "https://api-gw.dev.platform.linuxfoundation.org/",
        "scope": "access:api openid profile email"
      }

    }).then(response => {        
      expect(response.status).to.eq(200);       
      bearerToken = response.body.access_token; 
    });
});

it("Get CLA Groups under a foundation- Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}?foundationSFID=${foundationSFID}`,
      headers: optionalHeaders,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      expect(response.status).to.eq(200);
      expect(response.body).to.not.be.null;
        // Validate specific data in the response
       expect(response.body).to.have.property('list');           
        let list = response.body.list;
        expect(list[0].foundation_sfid).to.eql(foundationSFID)
         // Assert that the response contains an array
        expect(list[0].cla_groups).to.be.an('array');
          // Assert that the array has at least one item
        expect(list[0].cla_groups.length).to.be.greaterThan(0);
          //To validate schema of response
        const ajv = new Ajv();
          // Load the JSON schema
            cy.fixture("foundation/listFoundationClaGroups.json").then(
             (schema) => {
                 const validate = ajv.compile(schema);
                 const isValid = validate(response.body);
                  // Assert that the response matches the schema
                  expect(isValid, 'API response schema is valid').to.be.true;
                });
        });
  });

})