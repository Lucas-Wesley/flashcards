import os from 'node:os'
import path from 'node:path'
import { defineConfig } from '@playwright/test'

const remoto = process.env.E2E_BASE_URL
// O teste local roda num servidor e num banco só dele.
const dados = path.join(os.tmpdir(), 'flashcards-e2e')

export default defineConfig({
  testDir: './e2e',
  workers: 1,
  timeout: 60000,
  use: {
    baseURL: remoto || 'http://localhost:5179',
    ignoreHTTPSErrors: true,
  },
  webServer: remoto
    ? undefined
    : [
        {
          command: `rm -rf "${dados}" && go run .`,
          cwd: '../backend',
          env: { DATA_DIR: dados, ADDR: ':8095' },
          url: 'http://localhost:8095/api/health',
        },
        {
          // A versão de produção, com service worker, como no servidor.
          command: 'npm run build && npx vite preview --port 5179 --strictPort',
          env: { API_URL: 'http://localhost:8095' },
          url: 'http://localhost:5179',
          timeout: 180000,
        },
      ],
})
