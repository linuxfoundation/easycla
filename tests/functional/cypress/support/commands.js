import Ajv from 'ajv';

const ajv = new Ajv();
//To validate API response using schema
export function validateApiResponse(schemaPath, response) {
  cy.fixture(schemaPath).then((schema) => {
    const validate = ajv.compile(schema);
    const isValid = validate(response.body);

    // Assert that the response matches the schema

    if (isValid) {
      cy.log('API response schema is valid');
      expect(isValid, 'API response schema is valid').to.be.true;
    } else {
      Cypress.on('test:after:run', (test, runnable) => {
        const testName = `${runnable.parent.title} - ${test.title}`;
        cy.log(`API response schema is not valid for Test Case : ${testName}`);
        console.log(`API response schema is not valid for Test Case : ${testName}`);
        cy.log('Schema Error : ', validate.errors);
        console.error('Schema Error : ', validate.errors);
      });
    }
  });
}

//To validate & assert 200 response of api
export function validate_200_Status(response) {
  expect(response.status).to.eq(200);
  expect(response.body).to.not.be.null;
  const jsonResponse = JSON.stringify(response.body, null, 2);
  cy.log(jsonResponse);
}

export function validate_204_Status(response) {
  expect(response.status).to.eq(204);
  expect(response.body).to.not.be.null;
  const jsonResponse = JSON.stringify(response.body, null, 2);
  cy.log(jsonResponse);
}

export function validate_400_Status(response, message) {
  expect(response.status).to.eq(400);
  expect(response.statusText).to.eq('Bad Request');
  expect(response.body.Code).to.eq('400');
  expect(response.body.Message).to.eq(message);
}

export function validate_401_Status(response, local) {
  expect(response.status).to.eq(401);
  expect(response.statusText).to.eq('Unauthorized');
  if (local === true) {
    expect(response.body.message).to.eq('unauthenticated for invalid credentials');
  } else {
    expect(response.body).to.eq('no token provided\n');
  }
}

function parseJsonBody(resp) {
  if (resp && typeof resp.body === 'string') {
    const s = resp.body.trim();
    try {
      return JSON.parse(s);
    } catch (e) {}
  }
  return resp.body;
}

export function validate_403_Status(response) {
  const body = parseJsonBody(response);
  expect(response.status).to.eq(403);
  expect(response.statusText).to.eq('Forbidden');
  const code = body && (body.Code ?? body.code);
  expect(code).to.eq(403);
}

export function validate_404_Status(response) {
  expect(response.status).to.eq(404);
  expect(response.statusText).to.eq('Not Found');
  expect(response.body.Code).to.eq('404');
}

export function validate_404_Status_and_Message(response, message) {
  expect(response.status).to.eq(404);
  expect(response.statusText).to.eq('Not Found');
  expect(response.body.code).to.eq(404);
  expect(response.body.message).to.eq(message);
}

export function validate_404_Status_and_Message2(response, message) {
  expect(response.status).to.eq(404);
  expect(response.statusText).to.eq('Not Found');
  expect(response.body.Code).to.eq('404');
  expect(response.body.Message).to.eq(message);
}

export function validate_405_Status_and_Message(response, message) {
  expect(response.status).to.eq(405);
  expect(response.statusText).to.eq('Method Not Allowed');
  expect(response.body.code).to.eq(405);
  expect(response.body.message).to.eq(message);
}

export function validate_422_Status(response, bodyCode, bodyMessage) {
  expect(response.status).to.eq(422);
  expect(response.statusText).to.eq('Unprocessable Entity');
  expect(response.body.code).to.eq(bodyCode);
  expect(response.body.message).to.eq(bodyMessage);
}

export function shortenMiddle(str) {
  if (str.length <= 6) return str;
  const first = str.slice(0, 3);
  const last = str.slice(-3);
  return `${first}...${last}`;
}

export function getAPIBaseURL(version) {
  const local = Cypress.env('LOCAL');
  switch (version) {
    case 'v4':
      if (local) {
        return 'http://localhost:5001/v4/';
      }
      return `${Cypress.env('APP_URL')}cla-service/v4/`;
    default:
      cy.task('log', `--> unknown API version ${version}`);
  }
}

export function getXACLHeader() {
  const xacl = Cypress.env('XACL');
  if (xacl) {
    // cy.task('log', `--> using X-ACL ${shortenMiddle(xacl)} from env`);
    return {
      'X-ACL': xacl,
      'X-USERNAME': 'lgryglicki',
      'X-EMAIL': 'lukaszgryglicki@o2.pl',
    };
  }
  return {};
}

let bearerToken = '';
export function getTokenKey() {
  const envToken = Cypress.env('TOKEN');
  if (envToken) {
    cy.task('log', `--> getting token from env`);
    bearerToken = envToken;
    cy.window().then((win) => {
      win.localStorage.setItem('bearerToken', envToken);
      cy.task('log', `--> got token ${shortenMiddle(envToken)} from env`);
    });
    return;
  }
  cy.task('log', `--> getting token by request`);
  cy.request({
    method: 'POST',
    url: Cypress.env('AUTH0_TOKEN_API'),

    body: {
      grant_type: 'http://auth0.com/oauth/grant-type/password-realm',
      realm: 'Username-Password-Authentication',
      username: Cypress.env('AUTH0_USER_NAME'),
      password: Cypress.env('AUTH0_PASSWORD'),
      client_id: Cypress.env('AUTH0_CLIENT_ID'),
      audience: 'https://api-gw.dev.platform.linuxfoundation.org/',
      scope: 'access:api openid profile email',
    },
  }).then((response) => {
    expect(response.status).to.eq(200);
    bearerToken = response.body.access_token;
    cy.window().then((win) => {
      win.localStorage.setItem('bearerToken', response.body.access_token);
      cy.task('log', `--> got token ${shortenMiddle(response.body.access_token)} from request`);
    });
  });
}

Cypress.Commands.add('logJson', (label, value) => {
  const dbg = Cypress.env('DEBUG');
  if (!dbg) return;
  const seen = new WeakSet();
  const safe = JSON.stringify(
    value,
    (k, v) => {
      if (typeof v === 'object' && v !== null) {
        if (seen.has(v)) return '[Circular]';
        seen.add(v);
      }
      if (typeof v === 'string' && v.length > 800) return v.slice(0, 800) + '…';
      return v;
    },
    2,
  );
  cy.task('log', `${label}: ${safe}`);
});
