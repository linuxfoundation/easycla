# Installation

Make sure that you have `node` and `npm` installed.

Create `.env` file under `tests/functional` (it is git-ignored), with contents like this:

```
APP_URL=https://api-gw.dev.platform.linuxfoundation.org/
AUTH0_TOKEN_API=https://linuxfoundation-dev.auth0.com/oauth/token
AUTH0_USER_NAME=[your-username]
AUTH0_PASSWORD=[your-password]
LFX_API_TOKEN=[token]
AUTH0_CLIENT_SECRET=[client-secret]
AUTH0_CLIENT_ID=[client-id]
CYPRESS_ENV=dev
```

You can ask for example `.env` file over slack.

Then to run tests:

- Run `npm install`.
- Run all tests via: `npx cypress run`.
- Run specific tests from Chrome browser: `npx cypress open`. Choose **E2E testing**, select **Chrome** browser.

