# Flashcards

App próprio de **flashcards com repetição espaçada** (estilo Anki), para idiomas e conhecimentos
gerais. Parte do ecossistema de apps do homelab; roda em `https://192.168.1.20:8445`.

| Camada | Escolha |
|---|---|
| Frontend | Vue 3 + Vite, PWA |
| Backend | Go (biblioteca padrão) |
| Banco | SQLite (`modernc.org/sqlite`, sem CGO) |
| Repetição espaçada | FSRS-6 (`go-fsrs/v4`) |
| IA | Claude Code headless (`claude -p`), para gerar baralhos; imagens pela API da Wikipédia |
| Produção | Um binário Go com o PWA embutido, em container, atrás do Caddy |

## Como rodar

```sh
make dev-api   # API em :8080, dados em ./data
make dev-web   # PWA com hot reload, proxy de /api para :8080
make test      # Go, Vitest e Playwright
./deploy.sh    # publica o commit de origin/main no homelab
```

Decisões, estado atual e próximos passos: [`docs/DECISOES.md`](docs/DECISOES.md).
Design system comum às apps: [`docs/DESIGN-SYSTEM.md`](docs/DESIGN-SYSTEM.md).
