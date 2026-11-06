# Flashcards — Decisões

> App de flashcards com repetição espaçada, no estilo do Anki, simplificado para o uso do Lucas
> (idiomas em primeiro lugar, conhecimentos gerais em segundo). Registro até **06/11/2026**.
> Parte do ecossistema: arquitetura e contrato comum ficam no repo **Hub**
> ([`hub-home-lab-web`](https://github.com/Lucas-Wesley/hub-home-lab-web), `docs/`).

---

## 1. Onde paramos

**No ar** em `https://192.168.1.20:8445` (primeira versão, publicada em 06/11/2026). Tem:

- Baralhos com limites diários (novos e revisões) e retenção desejada.
- Notas de dois tipos: **frente e verso** (um cartão) e **nos dois sentidos** (dois cartões, para vocabulário).
- Estudo com "Mostrar resposta", as notas Errei, Amanhã, Difícil, Bom e Fácil com o intervalo previsto em cada botão,
  atalhos de teclado e **desfazer**.
- Lista de cartões com busca e filtro por baralho; edição, suspensão e exclusão.
- Importação de texto ou CSV (`frente;verso;tags`, tabulação ou vírgula).
- Ajustes com **Sugestões de melhoria** (API comum do ecossistema).

**Próximos passos** (ordem sugerida):

1. Mídia: imagem e áudio (pronúncia) nos cartões, em arquivos no disco (`/data/media`); o banco guarda só o nome.
2. Endpoint de integração para receber cartões da Biblioteca e do Acervo (quando os dois lados existirem; ver §4).
3. Estudo fora de casa (offline): baixar a fila do dia e subir as respostas depois.
4. Cloze (lacuna), estatísticas e otimização dos pesos do FSRS com o histórico do Lucas.
5. Importar `.apkg` do Anki.

## 2. Decisões

| # | Decisão | Por quê |
|---|---------|---------|
| F1 | Mesma stack e infraestrutura da Biblioteca e do Acervo: Go + SQLite + Vue PWA, um container na VM `apps`, Caddy na porta **8445**, `./deploy.sh` com testes e volta automática | Uma infraestrutura só para manter. Substitui o plano antigo (`PLANO.md`), que previa desenvolvimento em Windows/WSL e deploy por `git pull`. |
| F2 | Agendamento com **FSRS-6** pela biblioteca `go-fsrs` v4.0.0 (MIT), pesos padrão, passos de aprendizado 1 e 10 min, reaprendizado 10 min | É o algoritmo do Anki atual. O código da biblioteca foi lido e confere com as fórmulas do FSRS-6 (curva de esquecimento, estabilidade de acerto e de erro, dificuldade com reversão à média, estabilidade de curto prazo). |
| F3 | **O dia de estudo vira às 4h** no fuso `America/Sao_Paulo` (variável `FUSO`) | Como no Anki: estudar depois da meia-noite ainda conta como "hoje", e as revisões do dia aparecem desde a manhã, não só na hora exata em que vencem. |
| F4 | **Os horários entram no FSRS deslocados** para a meia-noite UTC cair na virada do nosso dia | O `go-fsrs` conta os dias de intervalo pela meia-noite UTC (21h em Brasília). Sem o ajuste, um cartão revisto às 22h e de novo às 8h seria tratado como "mesmo dia" e não ganharia estabilidade. Coberto por teste (`agenda_test.go`): sem o ajuste, a estabilidade fica em 10 em vez de subir para 13,05. |
| F5 | **Fuzz ligado** | Vem desligado na biblioteca. Sem ele, cartões criados juntos vencem juntos para sempre, e as revisões chegam em ondas. |
| F6 | Fila na ordem do Anki: aprendendo vencidos → revisões do dia → novos → aprendendo dos próximos 20 min | Cartão em aprendizado espera minutos, então vem antes. O "aprender adiante" evita a tela parada quando só falta um cartão que volta em instantes. |
| F7 | **Irmãos ficam para o dia seguinte**: estudado um sentido da nota, o outro só aparece amanhã | Ver "casa → house" e logo depois "house → casa" não treina nada. |
| F8 | Nota e cartão separados, como no Anki: a nota guarda o conteúdo; cada cartão guarda o agendamento de um sentido | Editar o texto não perde o histórico; trocar de "frente e verso" para "dois sentidos" cria só o cartão que falta. |
| F9 | **Histórico de cada resposta** na tabela `revisoes`, com uma cópia do cartão antes da resposta | Serve ao desfazer agora e, depois, às estatísticas e à otimização dos pesos do FSRS. |
| F10 | Resposta leva o número de repetições que o aparelho viu; se o cartão já mudou, o servidor recusa | Um toque duplo não responde o mesmo cartão duas vezes. |
| F11 | Conteúdo em **texto puro** (quebras de linha preservadas), sem Markdown nem HTML | Suficiente para idiomas e datas, e sem risco de HTML injetado por importação. Mídia entra no próximo passo. |
| F12 | Botões de resposta grandes (52 px) e teclado completo (espaço, 1–4, Z, Esc) | Exceção consciente ao "botões pequenos" do design system: a tela de estudo serve também à TV, com controle remoto. |
| F13 | Autenticação e backup ficam para o fim, como nas outras apps | Mesmo motivo da Biblioteca (R15): só o Lucas usa a rede local. |
| F14 | **Botão "Amanhã"** (tecla 5), ao lado de "Errei": para o FSRS é um erro (estabilidade de erro e, numa revisão, um lapso), mas sem os passos de minutos; o cartão volta em 24h, no dia de estudo seguinte. No histórico fica com a nota 5 | Pedido do Lucas em 06/11/2026: errar e rever em 10 minutos nem sempre ajuda, às vezes é melhor deixar para amanhã. Registrar como erro mantém o histórico honesto para o algoritmo e para uma futura otimização dos pesos, que deve tratar a nota 5 como 1. |

## 3. Como roda

- **Desenvolvimento:** `make dev-api` (Go na porta 8080, dados em `./data`) e `make dev-web` (Vite com proxy para `/api`).
- **Testes:** `make test` — Go (algoritmo, fila e API), Vitest e Playwright (fluxo completo no navegador).
- **Publicar:** `git push origin main` e `./deploy.sh`. Dados no servidor em `/srv/data/flashcards`.

## 4. Integrações (planejadas, não feitas)

Biblioteca (seleção de texto) e Acervo (trecho de Tema) → Flashcards. Pelo princípio do ecossistema, o endpoint só nasce
quando o outro lado for mandar de verdade, e é aí que o contrato comum (`CONTRATO.md` do Hub) vira concreto. A ideia
inicial é `POST /api/v1/cards` recebendo um Fragmento de Conteúdo, que vira uma nota com a fonte guardada.
