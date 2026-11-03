# flashcards

App próprio de **flashcards com repetição espaçada** (tipo Anki), construído do zero para
aprender no processo e usar de verdade — foco em **idiomas** e conhecimentos gerais.

> **Status:** planejamento. Ainda não há código; o desenho completo está em
> [`PLANO.md`](./PLANO.md).

## Visão geral

| Camada | Escolha |
|---|---|
| Frontend | Vue 3 + Vite + TypeScript (PWA) |
| Backend | Go (stdlib / `chi`) |
| Banco | SQLite (`modernc.org/sqlite`, sem CGO) |
| Repetição espaçada | FSRS v6 (`go-fsrs/v4`) |
| Mídia | arquivos em `/media`; o banco guarda só o nome |
| Desenvolvimento | 100% em containers (Docker) |
| Produção | binário Go único com o front embutido (`go:embed`) |

## Como vai rodar

- **Desenvolvimento:** containers com hot reload (`air` + Vite).
- **Produção (home lab):** M920q, em VM Debian com Docker no Proxmox.
  Deploy = `git pull` + `docker compose up -d`.
- Acesso só pela rede local; fora de casa, apenas por VPN. Nada exposto à internet.

Detalhes de arquitetura, funcionalidades e ordem de trabalho: veja [`PLANO.md`](./PLANO.md).
