# Flashcards

App do **ecossistema de apps pessoais do Lucas**: flashcards com repetição espaçada, no estilo do
Anki, simplificado. É o **3º app** (depois da Biblioteca Digital e do Acervo; ver ROADMAP do Hub).

**Comece lendo:** [`docs/DECISOES.md`](docs/DECISOES.md) — estado atual, decisões (F1–F34) e próximos passos.

## Em uma frase
Baralhos → notas (frente e verso, ou nos dois sentidos) → cartões agendados pelo **FSRS-6**
(`go-fsrs`), com o dia de estudo virando às 4h em Brasília.

## Contexto do ecossistema
App **independente** (banco + API próprios) que conversa com os outros **por API**. Visão geral,
roadmap e contrato comum no repo **Hub**: `github.com/Lucas-Wesley/hub-home-lab-web` (pasta `docs/`).
Mesma infraestrutura e mesmo design system (`docs/DESIGN-SYSTEM.md`) da Biblioteca e do Acervo.

## Regras e convenções (IMPORTANTE)
- **Mexeu no agendamento? Rode `go test ./...`.** `agenda_test.go` prova o comportamento do FSRS e
  a virada do dia (F3, F4); `estudo_test.go`, a ordem da fila, os limites e os irmãos.
- Horários no banco em RFC 3339, UTC, sem fração de segundo (a ordem do texto é a ordem do tempo).
- Dados (banco e imagens em `/data/media`) **nunca** vão para o Git.
- **A instrução da IA em vigor pode estar editada no app** (F17). Antes de mudar `regras/`, ler a do
  servidor (`GET https://192.168.1.20:8445/api/instrucoes/gerar-baralho.md`, com
  `--cacert ../certs/homelab-ca.crt`). Se estiver editada, levar a edição para `regras/` e só então
  voltar a do servidor ao padrão (`DELETE`), para nada se perder.
- Imagens só da Wikimedia e do Openverse (F19, F27); para aceitar outro servidor, mudar `HOSTS_DE_IMAGEM` com cuidado.
- **Links de qualquer site** (F31): um leitor só em `links.go`, sem nada específico de um site (nem no código, nem nos
  testes, nem na documentação); o que ele não lê, a IA tira do texto (`regras/extrair-exemplos.md`).
- Integração com Biblioteca/Acervo só quando os dois lados existirem; **não definir o contrato comum antes**. O Acervo já
  manda textos (`POST /api/textos`, F28, F30): mudar esse endpoint é mudar os dois apps.
- A **extensão do Chrome** (repo `home-lab-plugin-web`) usa `POST /api/links` e `GET /api/baralhos`:
  mudar um deles é mudar a extensão também.
- **Commits:** autoria do Lucas; **sem linha `Co-Authored-By`**.
