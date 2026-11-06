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
- **Gerar baralho com IA**: um pedido ("presidentes do Brasil no século 21") vira uma proposta de cartões, que o Lucas
  edita, filtra e aprova. As instruções da IA ficam em Ajustes, editáveis.
- **Imagens** nos cartões: buscadas na Wikipédia (pela IA, ao gerar, ou à mão, no editor) ou enviadas do aparelho.
- **Cartões de um link** do site do Mairo Vergara (colado em Adicionar → Do link ou mandado pelo Compartilhar do
  Android): cada frase de exemplo vira um cartão, com a tradução e o **áudio** da pronúncia.
- Ajustes com **Sugestões de melhoria** (API comum do ecossistema).

**Próximos passos** (ordem sugerida):

1. Mais sites em Adicionar → Do link, cada um com o seu leitor (o Lucas avisa antes de mandar links de um site novo).
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
| F15 | **Gerar baralho com IA** como no Acervo: Claude Code headless (`claude -p`) com o token da assinatura em `/opt/apps/flashcards.env`; o pedido entra numa **fila** (uma geração por vez, sobrevive a reinício, cancelável) e vira uma **proposta** que só cria cartões depois de revisada e aprovada | Pedido do Lucas em 06/11/2026. Mesmo caminho já provado no Acervo (D13, D14, D16 de lá). A IA pode errar fatos: nada entra no estudo sem passar pelo Lucas. |
| F16 | A IA **só tem a internet** (`WebSearch`, `WebFetch`): arquivos e terminal ficam proibidos. A instrução manda pesquisar sempre e devolver dúvidas em `observacoes` | No primeiro teste real ela pulou a pesquisa ("fatos conhecidos"); o conhecimento do modelo pode estar desatualizado. Com a instrução firme, ela pesquisou e avisou do que não conseguiu confirmar. |
| F17 | **Instruções da IA editáveis em Ajustes**, mesma API e mesmo comportamento do Acervo (D25 de lá): o padrão fica em `regras/`; a versão editada, em `/data/regras`, vale no lugar dele; "Voltar ao padrão" apaga a edição | Pedido do Lucas: ajustar pelo celular como os cartões são montados. |
| F18 | **Imagens pela API da Wikipédia**, não pela IA navegando: a IA indica o título do artigo (ex.: "Luiz Inácio Lula da Silva") e o servidor busca a foto principal dele. No editor, a mesma busca é manual; também dá para enviar do aparelho | É mais rápido, não gasta a assinatura, acerta a foto do artigo certo e as fotos têm licença livre (Wikimedia Commons). |
| F19 | Imagens **baixadas para o servidor** (`/data/media`, nome = hash do conteúdo, só JPG, PNG, WebP e GIF conferidos pelo conteúdo; SVG recusado). Da internet, só de `upload.wikimedia.org` e `thumb.wikimedia.org`. Na proposta, a foto fica só como link e é baixada ao aprovar | O cartão não depende da Wikipédia para abrir. A lista fechada de servidores impede o app de virar um baixador de qualquer coisa. O teste real mostrou que a Wikipédia passou a servir miniaturas por `thumb.wikimedia.org`. |
| F20 | A imagem é **da nota** e aparece **junto da resposta** | Simples e serve aos dois usos principais (rosto de quem é a resposta; ilustração do vocabulário). Imagem na frente fica para quando fizer falta. |
| F21 | **Cartões de um link** com um **leitor por site**, sem IA. O do Mairo Vergara pega cada parágrafo com a frase em inglês em negrito, uma quebra de linha e a tradução, e o `<audio>` logo depois; o resto da página (resposta rápida, títulos, propaganda) fica de fora. Vira uma proposta, como a da IA (`geracoes.origem = 'link'`) | Pedido do Lucas em 06/11/2026. A página já traz as frases prontas: ler direto é instantâneo (1 s), exato e não gasta a assinatura; a IA só poderia alterar as frases. Testado na página real de "Shoot Up": 18 frases, todas com áudio. Site novo = leitor novo (`leitorDoLink`), só depois de o Lucas avisar. |
| F22 | **Áudio** na nota (MP3 em `/data/media`, conferido pelo conteúdo), só dos sites conhecidos. É a pronúncia da **frente**: toca sozinho quando ela aparece (na pergunta, ou na resposta do cartão invertido), com botão e tecla R para repetir | É o método de frases do próprio Mairo Vergara: ouvir a frase do nativo enquanto lê. |
| F23 | Link pelo **Compartilhar** do Android (`share_target` do PWA, como no Acervo) ou colado em **Adicionar → Do link**. O destino padrão é o último baralho usado com links; mandar o mesmo link de novo para o mesmo baralho não duplica | Adicionar ganhou abas (Escrever · Do link · Com IA) em vez de mais um item no menu, que no celular já tem quatro abas. |

## 3. Como roda

- **Desenvolvimento:** `make dev-api` (Go na porta 8080, dados em `./data`) e `make dev-web` (Vite com proxy para `/api`).
- **Testes:** `make test` — Go (algoritmo, fila e API), Vitest e Playwright (fluxo completo no navegador).
- **Publicar:** `git push origin main` e `./deploy.sh`. Dados no servidor em `/srv/data/flashcards`.

## 4. Integrações (planejadas, não feitas)

Biblioteca (seleção de texto) e Acervo (trecho de Tema) → Flashcards. Pelo princípio do ecossistema, o endpoint só nasce
quando o outro lado for mandar de verdade, e é aí que o contrato comum (`CONTRATO.md` do Hub) vira concreto. A ideia
inicial é `POST /api/v1/cards` recebendo um Fragmento de Conteúdo, que vira uma nota com a fonte guardada.
