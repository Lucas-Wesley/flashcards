# Flashcards

App do **ecossistema de apps pessoais do Lucas**: flashcards com repetição espaçada, no estilo do
Anki, simplificado. É o **3º app** (depois da Biblioteca Digital e do Acervo; ver ROADMAP do Hub).

**Comece lendo:** [`docs/DECISOES.md`](docs/DECISOES.md) — estado atual, decisões (F1–F13) e próximos passos.

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
- Dados (banco e, no futuro, mídia) **nunca** vão para o Git.
- Integração com Biblioteca/Acervo só quando os dois lados existirem; **não definir o contrato comum antes**.
- **Commits:** autoria do Lucas; **sem linha `Co-Authored-By`**.
