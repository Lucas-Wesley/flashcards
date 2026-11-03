# Flashcards: app próprio de repetição espaçada

> **Status (29/10/2026):** só planejamento. Nenhum código foi criado ainda; a pasta `flashcards/` está vazia, exceto por este arquivo.
>
> **Próximo passo:** criar a estrutura inicial (`git init`, `.gitignore`, `README`, backend Go com endpoint de saúde, frontend Vue, compose de desenvolvimento).

---

## 1. Objetivo

Um app parecido com o **Anki** (flashcards com repetição espaçada), construído do zero para **aprender no processo** e **usar de verdade**.

- **Uso principal:** estudo de **idiomas**.
- **Uso secundário:** conhecimentos gerais para revisar (datas históricas etc.). Não é para concurso nem prova, então cartões normais (frente/verso) resolvem.
- **Onde usar:** celular, PC e TV (no navegador primeiro; app de TV fica para depois).
- **Tudo local:** textos no SQLite; imagens e áudios em arquivos no disco do servidor. Nada vai para a nuvem.
- **Cadastro de conteúdo:** pela web, no PC (editor com upload de imagem e áudio).

## 2. Decisões

| Tema | Decisão |
|---|---|
| Frontend | **Vue 3** + Vite + TypeScript, Pinia, Vue Router, `vite-plugin-pwa` |
| Backend | **Go**, biblioteca padrão (`net/http`, roteamento do Go 1.22+) ou `chi` |
| Banco | **SQLite** via `modernc.org/sqlite` (Go puro, sem CGO); só texto e metadados |
| Mídia | Pasta `/media` no disco; o banco guarda só o nome do arquivo (ex.: `a3f9.mp3`) |
| Algoritmo | **FSRS v6** com `github.com/open-spaced-repetition/go-fsrs/v4` |
| Migrações | `goose` ou `golang-migrate` |
| Acesso a dados | SQL escrito à mão (`database/sql` ou `sqlc`), sem ORM, para aprender |
| Desenvolvimento | **100% em containers** (sem instalar Go/Node no Windows) |
| Produção | Um binário Go que embute o build do Vue (`go:embed`) e serve front e API na mesma porta |
| Repositório | GitHub, monorepo, nome previsto: `flashcards` |

### Sobre o go-fsrs (verificado em 27/10/2026)
- Repositório oficial: `open-spaced-repetition/go-fsrs`, licença MIT.
- O README indica **FSRS v6**; módulo `.../go-fsrs/v4`.
- Uso: `fsrs.NewFSRS(fsrs.DefaultParam())`, `fsrs.NewCard(now)`, `s.Repeat(card, now)` (prévia das 4 notas) e `s.Next(card, now, fsrs.Good)` (aplica a nota).
- Cobre **só o agendamento**; baralhos, banco e histórico de revisões são nossos.
- Não consegui confirmar data do último commit nem tag de release. Fixar a versão no `go.mod`. Se der problema, a lógica é pequena e dá para portar.

## 3. Arquitetura

```
Celular / TV / PC (navegador ou PWA)
        │  HTTP (rede local ou VPN)
        ▼
  Container (M920q ou Mac mini)
  ├─ API em Go (serve também o front)
  ├─ SQLite  → baralhos, cartões, histórico de revisões
  └─ /media  → imagens e áudios
```

Um volume Docker guarda o banco e `/media`. Backup = copiar essa pasta.

### Estrutura do repositório
```
flashcards/
├─ backend/     (Go: cmd/, internal/, migrations/)
├─ frontend/    (Vue)
├─ Dockerfile   (multi-stage: build Vue → build Go → imagem final)
├─ compose.yaml       (produção)
├─ compose.dev.yaml   (desenvolvimento)
├─ PLANO.md
└─ README.md
```

### Desenvolvimento em containers
- `compose.dev.yaml` com dois serviços: `backend` (imagem `golang` + `air` para hot reload) e `frontend` (imagem `node` + servidor do Vite).
- Código montado como volume: edita no Windows, o container recarrega.
- O Vite faz proxy de `/api` para o backend.
- Atenção: volumes do Windows para o WSL2 podem deixar o hot reload lento. Se incomodar, clonar o repositório dentro do WSL.
- Ambiente já verificado: Docker 29.8, Compose 5.5 (Docker Desktop/WSL2) e Git 2.44.

### GitHub
- `.gitignore` cobrindo `node_modules`, binários, arquivos `.db` e a pasta `/media`. **Dados e mídia nunca vão para o repositório.**
- Commits pequenos, mensagens claras, `README` com como rodar.
- Opcional: GitHub Actions para testes e build da imagem.

## 4. Funcionalidades

**MVP**
- Baralhos e cartões (frente/verso), com tags.
- Tela de revisão com 4 botões (errei, difícil, bom, fácil) e contagem de cartões do dia.
- Editor de cartões no PC com upload de imagem e áudio.
- Importação de CSV.

**Pensar desde o início (idiomas)**
- Campo de **áudio** (pronúncia).
- Cartões nos **dois sentidos** (palavra → tradução e tradução → palavra).

**Depois**
- Cartões **cloze** (lacuna), úteis para datas históricas.
- Importar `.apkg` do Anki.
- Estatísticas.
- Modo offline.

## 5. PWA e TV

- Para instalar como PWA, o navegador exige **HTTPS**. Na rede local: proxy reverso (Caddy ou Nginx Proxy Manager) com certificado, ou Tailscale (já entrega HTTPS). Sem HTTPS o site funciona, mas não vira app instalável nem tem offline.
- **TV:** começar pelo navegador da TV. App de verdade depende do sistema (Android TV, Tizen, webOS) e fica para depois.
- Desenhar a tela de revisão já pensando no controle remoto: botões grandes, navegação por foco, sem digitação.

## 6. Onde vai rodar

- **Desenvolvimento:** tudo neste PC, em containers (`compose.dev.yaml`, hot reload com air + vite). O PC é a oficina.
- **Destino (decidido em 03/11/2026): M920q.** O app roda numa **VM Debian com Docker** no Proxmox (dedicada a apps próprios; mesmo padrão da VM do Home Assistant). Docker em VM é mais robusto que em LXC. Deploy = `git pull` + `docker compose up -d` na VM.
- **Git é a ponte:** programa no PC, `git push`; no M920q, `git pull` + sobe a imagem de produção (binário Go único com front embutido).
- Opção futura: editar direto na VM via VS Code Remote-SSH, se o hot reload sobre volume Windows/WSL2 incomodar. Começar 100% local.
- Acesso só pela rede local; fora de casa, apenas por VPN. Nenhuma porta aberta para a internet.
- O Mac mini M4 fica descartado como destino deste app (segue como servidor secundário para outras coisas, se for o caso).

## 7. Ordem de trabalho

1. Estrutura do repositório + compose de desenvolvimento + endpoint de saúde.
2. API + SQLite + FSRS rodando.
3. Tela de revisão no navegador.
4. Editor de cartões com upload de imagem e áudio.
5. Dockerfile de produção e subida no servidor.
6. HTTPS, PWA e ajustes para a TV.
7. Cloze, importação, estatísticas.

## 8. Pendências

- [ ] Criar a estrutura inicial do projeto
- [ ] `git init` e repositório no GitHub (confirmar o nome `flashcards`)
- [x] Decidir o servidor final → **M920q** (VM Debian + Docker no Proxmox) — decidido 03/11/2026
- [ ] Confirmar o suporte do `go-fsrs` na prática (rodar um exemplo)
