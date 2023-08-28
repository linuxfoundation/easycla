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
};

//To validate & assert 200 response of api
export function validate_200_Status(response){
  expect(response.status).to.eq(200);
  expect(response.body).to.not.be.null;
  const jsonResponse = JSON.stringify(response.body, null, 2);
  cy.log(jsonResponse);
};