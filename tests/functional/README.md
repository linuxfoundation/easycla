# Cypress Framework

This repository contains a Cypress framework setup for automated testing. The framework is structured as follows:

## Project Folder Structure

Project Folder<br>
├── node_modules <br>
└── cypress<br>
&nbsp; &nbsp; &nbsp; &nbsp;├── appConfig<br>
&nbsp; &nbsp; &nbsp; &nbsp;├── downloads<br>
&nbsp; &nbsp; &nbsp; &nbsp;├── e2e<br>
&nbsp; &nbsp; &nbsp; &nbsp;├── fixtures<br>
&nbsp; &nbsp; &nbsp; &nbsp;├── reports<br>
&nbsp; &nbsp; &nbsp; &nbsp;├── screenshot<br>
&nbsp; &nbsp; &nbsp; &nbsp;├── support<br>
&nbsp; &nbsp; &nbsp; &nbsp;├── video<br>

├─ cypress.config.ts<br>
│ .eslintrc.json<br>
│ readme.md<br>
│ .gitignore<br>
│ package-lock.json<br>
│ package.json<br>
│ tsconfig.json<br>
├─ .github<br>
│ &nbsp; &nbsp; &nbsp; &nbsp;└── workflows<br>
│ &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; &nbsp; └── main.yml<br>

## Description

- `.gitignore`: Specifies intentionally untracked files to ignore in Git.
- `package-lock.json` and `package.json`: Node.js package files specifying project dependencies.
- `cypress.config.ts`: Configuration file for Playwright settings.
- `tsconfig.json`: TypeScript compiler options file.

### `.GitHub`

- `.GitHub/workflows/main.yml`: GitLub Actions workflow file for continuous integration.

### `node_modules`

- Directory containing Node.js modules installed by npm.

### `Cypress-report`

- Directory for storing Cypress test reports.

### `src`

- Source code directory containing project files.

#### `api`

- Directory for API-related scripts.

#### `config`

- Directory containing environment configuration files and authentication data.

#### `fixtures`

- Directory for test fixtures, such as reusable functions for mock.

### `test-results`

- Directory for storing test execution results, including screenshots, trace files, and videos.

## Usage

Make sure that you have `node` and `npm` installed.

Clone the repository and install dependencies using `npm install`.

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

- Run `npx cypress install`
- Run tests using cmd `npx cypress run`.
- Run tests using UI `npx cypress open`. Choose **E2E testing**, select **Chrome** browser.
- View test reports in the `cypress-report` directory.
- Explore source code files for detailed implementation.

## Contributing

Contributions are welcome! Please follow the established coding style and guidelines. If you find any issues or have suggestions for improvements, feel free to open an issue or submit a pull request.

## License

This project is licensed under the [](LICENSE).
