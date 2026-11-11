import os from 'node:os'
import path from 'node:path'
import { defineConfig } from '@playwright/test'

const remoto = process.env.E2E_BASE_URL
// O teste local roda num servidor e num banco só dele.
const dados = path.join(os.tmpdir(), 'flashcards-e2e')
const testdata = path.resolve('../backend/testdata')

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
          // Claude falso; a Wikipédia aponta para uma porta fechada, e o Commons responde sempre a mesma
          // bandeira, do site falso (nada sai da máquina).
          env: {
            DATA_DIR: dados,
            ADDR: ':8095',
            ADDR_TV: ':8097',
            URL_DA_TV: 'http://localhost:8097',
            CLAUDE_BIN: path.join(testdata, 'claude-falso'),
            REGRAS_DIR: path.resolve('../regras'),
            WIKI_API: 'http://127.0.0.1:9/w/api.php',
            COMMONS_API: 'http://localhost:8096/commons.json',
            OPENVERSE_API: '',
            HOSTS_DE_IMAGEM: 'localhost:8096',
          },
          url: 'http://localhost:8095/api/health',
        },
        {
          // Um site falso para os links: a página de teste e um áudio.
          command: `python3 -m http.server 8096 --bind 127.0.0.1 --directory "${testdata}"`,
          url: 'http://localhost:8096/frases.html',
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
