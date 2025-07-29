import { defineConfig } from 'cypress';
import * as dotenv from 'dotenv';
dotenv.config();

export default defineConfig({
  defaultCommandTimeout: 30000,
  requestTimeout: 300000,
  reporter: 'cypress-mochawesome-reporter',
  e2e: {
    // baseUrl: 'http://localhost:1234',
    specPattern: 'cypress/e2e/**/**/*.{js,jsx,ts,tsx}',
  },
  env: {
    APP_URL: process.env.APP_URL,
    AUTH0_TOKEN_API: process.env.AUTH0_TOKEN_API,
    AUTH0_USER_NAME: process.env.AUTH0_USER_NAME,
    AUTH0_PASSWORD: process.env.AUTH0_PASSWORD,
    LFX_API_TOKEN: process.env.LFX_API_TOKEN,
    AUTH0_CLIENT_SECRET: process.env.AUTH0_CLIENT_SECRET,
    AUTH0_CLIENT_ID: process.env.AUTH0_CLIENT_ID,
    CYPRESS_ENV: process.env.CYPRESS_ENV,
  }
});

