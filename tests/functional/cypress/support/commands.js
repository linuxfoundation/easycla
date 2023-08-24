import Ajv from 'ajv';

const ajv = new Ajv();
 //To validate API response using schema
export function validateApiResponse (schemaPath,response) {
  cy.fixture(schemaPath).then(
    (schema) => {
  const validate = ajv.compile(schema);
  const isValid = validate(response.body);

  // Assert that the response matches the schema
  if (isValid) {
} else {
    console.log('Data is not valid.', validate.errors);
}
expect(isValid, 'API response schema is valid').to.be.true; 
});
}
// let bearerToken={};

// Cypress.Commands.add('setBearerToken', (value) => {
//     bearerToken = value;
// });

/*
Cypress.Commands.add('getBearerToken', () => {
    // console.log('Here is token key at getBearerToken: '+bearerToken)
  return bearerToken;
});

Cypress.Commands.add('login', () => {
    
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
        bearerToken =  response.body.access_token;    
        // console.log('Here is token key at cmd: '+bearerToken)
        // return bearerToken;
      });
})
*/