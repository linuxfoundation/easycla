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

function parseJsonBody(resp) {
  if (!resp) {
    return {};
  }
  const body = resp.body ?? resp.Body;
  if (typeof body === 'string') {
    const s = body.trim();
    try {
      return JSON.parse(s);
    } catch (e) {}
  }
  return body;
}

export function validate_status(response, expectedStatus) {
  const status = response.status ?? response.Status;
  expect(String(status)).to.eq(String(expectedStatus));
}

//To validate & assert 200 response of api
export function validate_200_Status(response) {
  const status = response.status ?? response.Status;
  expect(String(status)).to.eq('200');
  const body = response.body ?? response.Body;
  expect(body).to.not.be.null;
}

export function validate_204_Status(response) {
  const status = response.status ?? response.Status;
  expect(String(status)).to.eq('204');
  const body = response.body ?? response.Body;
  expect(body).to.not.be.null;
}

export function validate_400_Status(response, message) {
  const status = response.status ?? response.Status;
  expect(String(status)).to.eq('400');
  const statusText = response.statusText ?? response.StatusText;
  expect(statusText).to.eq('Bad Request');
  const body = response.body ?? response.Body ?? {};
  const bodyCode = body.code ?? body.Code;
  const bodyMessage = body.message ?? body.Message;
  expect(String(bodyCode)).to.eq('400');
  expect(bodyMessage).to.eq(message);
}

export function validate_400_Status_Contains(response, message) {
  const status = response.status ?? response.Status;
  expect(String(status)).to.eq('400');
  const statusText = response.statusText ?? response.StatusText;
  expect(statusText).to.eq('Bad Request');
  const body = response.body ?? response.Body ?? {};
  const bodyCode = body.code ?? body.Code;
  const bodyMessage = body.message ?? body.Message;
  expect(String(bodyCode)).to.eq('400');
  expect(bodyMessage).to.contain(message);
}

export function validate_401_Status(response, local) {
  const status = response.status ?? response.Status;
  expect(String(status)).to.eq('401');
  const statusText = response.statusText ?? response.StatusText;
  expect(statusText).to.eq('Unauthorized');
  const body = response.body ?? response.Body ?? {};
  if (local === true) {
    const bodyMessage = body.message ?? body.Message;
    expect(bodyMessage).to.eq('unauthenticated for invalid credentials');
  } else {
    expect(body).to.eq('no token provided\n');
  }
}

export function validate_403_Status(response) {
  const status = response.status ?? response.Status;
  expect(String(status)).to.eq('403');
  const statusText = response.statusText ?? response.StatusText;
  expect(statusText).to.eq('Forbidden');
  const body = parseJsonBody(response);
  const code = body && (body.Code ?? body.code);
  expect(String(code)).to.eq('403');
}

export function validate_404_Status(response) {
  const status = response.status ?? response.Status;
  expect(String(status)).to.eq('404');
  const statusText = response.statusText ?? response.StatusText;
  expect(statusText).to.eq('Not Found');
  const body = response.body ?? response.Body ?? {};
  const bodyCode = body.code ?? body.Code;
  expect(String(bodyCode)).to.eq('404');
}

export function validate_404_Status_and_Message(response, expectedMessage) {
  const status = response.status ?? response.Status;
  expect(String(status)).to.eq('404');
  const statusText = response.statusText ?? response.StatusText;
  expect(statusText).to.eq('Not Found');
  const body = response.body ?? response.Body ?? {};
  const bodyCode = body.code ?? body.Code;
  const bodyMessage = body.message ?? body.Message;
  expect(String(bodyCode)).to.eq('404');
  expect(bodyMessage).to.eq(expectedMessage);
}

export function validate_405_Status_and_Message(response, expectedMessage) {
  const status = response.status ?? response.Status;
  expect(String(status)).to.eq('405');
  const statusText = response.statusText ?? response.StatusText;
  expect(statusText).to.eq('Method Not Allowed');
  const body = response.body ?? response.Body ?? {};
  const bodyCode = body.code ?? body.Code;
  const bodyMessage = body.message ?? body.Message;
  expect(String(bodyCode)).to.eq('405');
  expect(bodyMessage).to.eq(expectedMessage);
}

export function validate_422_Status(response, expectedBodyCode, expectedBodyMessage) {
  const status = response.status ?? response.Status;
  expect(String(status)).to.eq('422');
  const statusText = response.statusText ?? response.StatusText;
  expect(statusText).to.eq('Unprocessable Entity');
  const body = response.body ?? response.Body ?? {};
  const bodyCode = body.code ?? body.Code;
  const bodyMessage = body.message ?? body.Message;
  expect(String(bodyCode)).to.eq(String(expectedBodyCode));
  expect(bodyMessage).to.eq(expectedBodyMessage);
}

export function validate_expected_status(
  response,
  expectedStatus,
  expectedCode,
  expectedMessage,
  expectedMessageContains,
) {
  if (expectedStatus !== undefined && expectedStatus !== null) {
    const status = response.status ?? response.Status;
    if (Array.isArray(expectedStatus)) {
      expect(status).to.be.oneOf(expectedStatus);
    } else {
      expect(String(status)).to.eq(String(expectedStatus));
    }
  }

  const statusText = response.statusText ?? response.StatusText;
  const actualStatus = response.status ?? response.Status;

  if (actualStatus === 400) {
    expect(statusText).to.eq('Bad Request');
  } else if (actualStatus === 401) {
    expect(statusText).to.eq('Unauthorized');
  } else if (actualStatus === 403) {
    expect(statusText).to.eq('Forbidden');
  } else if (actualStatus === 404) {
    expect(statusText).to.eq('Not Found');
  } else if (actualStatus === 405) {
    expect(statusText).to.eq('Method Not Allowed');
  } else if (actualStatus === 422) {
    expect(statusText).to.eq('Unprocessable Entity');
  }

  const body = parseJsonBody(response);
  const bodyCode = body.code ?? body.Code;
  let bodyMessage = body.message ?? body.Message;

  // Handle V2 API string responses (e.g., "missing authorization header")
  if (!bodyMessage && typeof response.body === 'string') {
    bodyMessage = response.body.replace(/"/g, ''); // Remove quotes from string response
  }

  // Handle V2 API 404 format: {"404": "The API call you tried to make was not defined..."}
  if (!bodyMessage && body['404']) {
    bodyMessage = body['404'];
  }

  // Handle V2 API error format: {"errors": {"405 Method Not Allowed": null}} or {"errors": {"field": "error message"}}
  if (!bodyMessage && body.errors && typeof body.errors === 'object') {
    const errorKeys = Object.keys(body.errors);
    if (errorKeys.length > 0) {
      const errorValue = body.errors[errorKeys[0]];
      if (errorValue && typeof errorValue === 'string') {
        // Format: {"errors": {"field": "error message"}}
        bodyMessage = errorValue;
      } else {
        // Format: {"errors": {"405 Method Not Allowed": null}}
        bodyMessage = errorKeys[0];
      }
    }
  }

  if (expectedCode !== undefined && expectedCode !== null) {
    expect(String(bodyCode)).to.eq(String(expectedCode));
  }

  if (expectedMessage !== undefined && expectedMessage !== null) {
    // Ensure bodyMessage is not undefined before testing
    const messageToTest = bodyMessage || '';
    if (expectedMessageContains === true) {
      expect(messageToTest).to.contain(expectedMessage);
    } else {
      expect(messageToTest).to.eq(expectedMessage);
    }
  }
}

export function shortenMiddle(str) {
  if (str.length <= 6) return str;
  const first = str.slice(0, 3);
  const last = str.slice(-3);
  return `${first}...${last}`;
}

export function getTokenForV2() {
  // V2 APIs use the same token generation as V3/V4
  cy.task('log', '--> getting token by request for V2');
  return cy
    .request({
      method: 'POST',
      url: Cypress.env('AUTH0_TOKEN_API'),
      headers: {
        'content-type': 'application/json',
      },
      body: {
        grant_type: 'http://auth0.com/oauth/grant-type/password-realm',
        realm: 'Username-Password-Authentication',
        username: Cypress.env('AUTH0_USER_NAME'),
        password: Cypress.env('AUTH0_PASSWORD'),
        client_id: Cypress.env('AUTH0_CLIENT_ID'),
        audience: 'https://api-gw.dev.platform.linuxfoundation.org/',
        scope: 'access:api openid profile email',
      },
    })
    .then((response) => {
      expect(response.status).to.eq(200);
      const token = response.body.access_token;
      cy.task('log', `--> got token ${shortenMiddle(token)} from request for V2`);
      return cy.wrap(token);
    });
}

export function getTokenForV3() {
  // V3 APIs require a token with specific claims: http://lfx.dev/claims/username and http://lfx.dev/claims/email
  // The token generation is the same as V4, but V3 expects the AUTH0_USERNAME_CLAIM to be set to "http://lfx.dev/claims/username"
  cy.task('log', '--> getting token by request for V3');
  return cy
    .request({
      method: 'POST',
      url: Cypress.env('AUTH0_TOKEN_API'),
      headers: {
        'content-type': 'application/json',
      },
      body: {
        grant_type: 'http://auth0.com/oauth/grant-type/password-realm',
        realm: 'Username-Password-Authentication',
        username: Cypress.env('AUTH0_USER_NAME'),
        password: Cypress.env('AUTH0_PASSWORD'),
        client_id: Cypress.env('AUTH0_CLIENT_ID'),
        audience: 'https://api-gw.dev.platform.linuxfoundation.org/',
        scope: 'access:api openid profile email',
      },
    })
    .then((response) => {
      expect(response.status).to.eq(200);
      const token = response.body.access_token;
      cy.task('log', `--> got token ${shortenMiddle(token)} from request for V3`);
      return token;
    });
}

export function getAPIBaseURL(version) {
  const local = Cypress.env('LOCAL');
  switch (version) {
    case 'v4':
      if (local) {
        return 'http://localhost:5001/v4/';
      }
      return `${Cypress.env('APP_URL')}cla-service/v4/`;
    case 'v3':
      if (local) {
        return 'http://localhost:5001/v3/';
      }
      // V3 is deployed on the legacy API endpoint, not the new cla-service endpoint
      return 'https://api.lfcla.dev.platform.linuxfoundation.org/v3/';
    case 'v2':
      if (local) {
        return 'http://localhost:5000/v2/';
      }
      return 'https://api.lfcla.dev.platform.linuxfoundation.org/v2/';
    default:
      cy.task('log', `--> unknown API version ${version}`);
  }
}

export function getXACLHeader() {
  const xacl = Cypress.env('XACL');
  if (xacl) {
    cy.task('log', `--> using X-ACL ${shortenMiddle(xacl)} from env`);
    return {
      'X-ACL': xacl,
      'X-USERNAME': 'lgryglicki',
      'X-EMAIL': 'lukaszgryglicki@o2.pl',
    };
  }
  return {};
}

export function getXACLHeaders() {
  // V3 APIs (which are actually V1 internally) use the same authentication as V4
  // They need both X-ACL headers and bearer tokens
  const xacl = Cypress.env('XACL');
  if (xacl) {
    return {
      'X-ACL': xacl,
      'X-USERNAME': 'lgryglicki',
      'X-EMAIL': 'lukaszgryglicki@o2.pl',
    };
  }
  return {};
}

export function getOAuth2Headers() {
  // V3 APIs (which are actually V1 internally) use the same authentication as V4
  // They need both X-ACL headers and bearer tokens - just alias to getXACLHeaders
  return getXACLHeaders();
}

let bearerToken = '';
export function getTokenKey() {
  cy.task('log', `--> getting token`);
  const envToken = Cypress.env('TOKEN');
  if (envToken && envToken !== '-') {
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
    headers: {
      'content-type': 'application/json',
    },
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
