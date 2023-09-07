import { defineConfig } from 'cypress'

export default defineConfig({
    defaultCommandTimeout: 20000,
    requestTimeout: 90000,
    "reporter": "cypress-mochawesome-reporter",
  e2e: {
    // baseUrl: 'http://localhost:1234',
    specPattern: 'cypress/e2e/**/**/*.{js,jsx,ts,tsx}',
  }  ,
  "env": {
    "file": "cypress.env.json"
  }
})