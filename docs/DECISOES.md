# Flashcards — Decisões

> App de flashcards com repetição espaçada, no estilo do Anki, simplificado para o uso do Lucas
> (idiomas em primeiro lugar, conhecimentos gerais em segundo). Registro até **07/11/2026**.
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
- **Cartões de um link** de qualquer site (colado em Adicionar → Do link, mandado pelo Compartilhar do Android ou pela
  extensão do Chrome): até 3 frases de exemplo viram cartões, com a tradução e o **áudio** da pronúncia, se houver
  (F31). Na revisão, o baralho pode ser trocado; um link sem baralho começa no "Inglês".
- Ajustes com **Sugestões de melhoria** (API comum do ecossistema).
- **Cartões do Acervo** (F28, F30): um Tema, capítulo ou Referência mandado de lá, para o baralho escolhido lá, vira um
  pedido à IA, revisado aqui (ou, no caso de uma Referência, também na página dela no Acervo). Cada cartão guarda o
  link do capítulo de onde saiu (**Ver no Acervo**, no estudo).

**Próximos passos** (ordem sugerida):

1. Ver, com o uso, se a IA tira bem os exemplos das páginas que não seguem o jeito comum (F31).
2. Receber trechos da Biblioteca (o Acervo já manda textos, F28; ver §4).
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
| F21 | **Cartões de um link** com um **leitor por site**, sem IA (desde F31, um leitor só, para qualquer site). O primeiro pegava cada parágrafo com a frase em inglês em negrito, uma quebra de linha e a tradução, e o `<audio>` logo depois; o resto da página (resposta rápida, títulos, propaganda) fica de fora. Vira uma proposta, como a da IA (`geracoes.origem = 'link'`) | Pedido do Lucas em 06/11/2026. A página já traz as frases prontas: ler direto é instantâneo (1 s), exato e não gasta a assinatura; a IA só poderia alterar as frases. Testado na página real de "Shoot Up": 18 frases, todas com áudio. |
| F22 | **Áudio** na nota (MP3 em `/data/media`, conferido pelo conteúdo), só dos áudios achados na página do link (F31). É a pronúncia da **frente**: toca sozinho quando ela aparece (na pergunta, ou na resposta do cartão invertido), com botão e tecla R para repetir | Ouvir a frase do nativo enquanto lê é o jeito de estudar frases que o Lucas usa. |
| F23 | Link pelo **Compartilhar** do Android (`share_target` do PWA, como no Acervo) ou colado em **Adicionar → Do link**. O destino padrão é o último baralho usado com links; mandar o mesmo link de novo para o mesmo baralho não duplica | Adicionar ganhou abas (Escrever · Do link · Com IA) em vez de mais um item no menu, que no celular já tem quatro abas. |
| F24 | **Estudar** no topo de Baralhos mistura todos os baralhos numa fila só (`GET /api/estudo`). Cada baralho mantém a própria fila e os próprios limites; o próximo cartão é, entre os próximos de cada baralho, o do passo mais urgente (aprendendo vencido → revisões → novos → aprender adiante). No mesmo passo, revisões e novos saem por um sorteio fixo pelo id do cartão, para os baralhos se intercalarem. A resposta conta no baralho do cartão; desfazer volta a última resposta de qualquer baralho | Igual ao "estudar o baralho pai" do Anki, sem precisar de hierarquia de baralhos. |
| F25 | Pedido à IA **editável e refeito** a qualquer momento depois de terminar (falhou, cancelado, descartado, em revisão ou já aprovado), sem histórico do pedido. Aprovado num baralho novo, o refazer passa a mirar o baralho criado; a IA recebe os cartões existentes (frente → verso) e devolve com a mesma frente os que forem mudar. Aprovar **atualiza** a nota de mesma frente (verso, tags, foto, áudio), e os cartões seguem com o histórico | Completar um baralho já estudado ("acrescente quem desenhou cada bandeira") sem duplicar nem zerar o agendamento. |
| F26 | Proposta da IA guarda só o **link** da foto; o arquivo só é baixado ao aprovar. Mídia sem uso (trocada, de cartão apagado, enviada e abandonada, sobras `.tmp`) é **apagada** ao subir o app e a cada 24 h, depois de 24 h sem uso; conta como em uso o que uma nota ou uma proposta em revisão aponta. O Claude roda com `--no-session-persistence` | Refazer e descartar pedidos não deixa arquivos ocupando o disco do servidor. |
| F27 | Busca de imagem em **três fontes abertas, sem chave**: Wikipédia (foto do artigo), **Wikimedia Commons** (os arquivos) e **Openverse** (outros acervos, só na busca manual). O termo vai para as fontes; o **cartão inteiro** (frente, verso, tags) ordena o resultado: palavras do termo e do cartão somam, ano citado pesa muito, palavra estranha ao cartão desconta. O ano do cartão também vira uma busca extra no Commons. A IA indica um termo específico do cartão. Baixar só de `HOSTS_DE_IMAGEM` (Wikimedia e `api.openverse.org`), também depois de redirecionamento. Google Imagens fica de fora: não tem API gratuita | Num baralho de um assunto só (bandeiras), cada cartão precisa da imagem do que ele pergunta, não a do assunto geral. |
| F28 | **Cartões de um texto do Acervo.** `POST /api/textos` recebe `{titulo, baralho, contexto, texto, link}` (até 150 mil caracteres) e cria um pedido à IA de origem `acervo`, na mesma fila e com a mesma revisão. A IA segue `regras/gerar-do-texto.md` (editável em Ajustes): os cartões saem **só do texto**, sem internet; o texto vai pela entrada padrão, não como argumento. O teto de cartões vem do tamanho (1 a cada 80 palavras, de 5 a 100); a IA decide quantos o texto rende. O baralho tem o nome do Tema principal: se já existe, os cartões vão para ele, e os de mesma frente atualizam os que existem. Refazer usa o mesmo texto. A proposta mostra o caminho no Acervo e o link de volta | Pedido do Lucas em 06/11/2026: estudar o que já está no Acervo sem montar cartão à mão. Endpoint próprio, não o contrato comum (que segue sem definir); quando ele existir, `/api/textos` vira um caso dele. |
| F29 | **Links: só os 3 exemplos mais didáticos.** Uma página com mais de 3 frases (fora as que já estão no baralho) vira um pedido na fila da IA: ela recebe as frases numeradas, com a tradução, e devolve **só os números** das 3 mais didáticas, seguindo `regras/escolher-exemplos.md` (editável em Ajustes): cobrir os sentidos diferentes da expressão, frases naturais, curtas e que se entendem sem contexto. Sem internet, sem arquivos. As frases, as traduções e os áudios continuam exatamente os da página (F21). Com 3 ou menos, a proposta sai na hora, como antes. O pedido guarda as frases lidas, então **Refazer** pede outra escolha | Pedido do Lucas em 06/11/2026: as páginas de exemplos costumam ter muitos parecidos (18 no "Shoot Up"), e estudar todos repete a mesma coisa. A IA só escolhe, não escreve: o texto do nativo e o áudio continuam exatos. Também é o caminho da **extensão do Chrome**, que manda o link da página aberta no navegador. |
| F30 | **Baralho escolhido no Acervo e link de volta em cada cartão.** `POST /api/textos` passou a receber `baralho_id` (um baralho que já existe) ou `baralho` (o nome do novo); sem um dos dois, recusa. Não procura mais baralho pelo nome: um novo com o nome de um que existe é outro baralho. Recebe também `secoes` (`[{titulo, link}]`, os capítulos do texto com o link de cada um no Acervo); a IA indica em `secao` de que capítulo saiu cada cartão, e a nota guarda o link na coluna nova `fonte` (sem capítulo, o link do texto inteiro). O estudo mostra **Ver no Acervo** depois da resposta; o editor mostra o link, com opção de tirar; a revisão da proposta mostra o capítulo de cada cartão. Atualizar um cartão que já existe troca a fonte só quando vem uma nova | Respostas do Lucas em 06/11/2026: escolher o baralho (ou criar um) ao mandar, e poder abrir o trecho do Acervo durante o estudo. O baralho é escolhido antes da IA rodar, porque ela recebe os cartões que o baralho já tem para não repetir. |
| F31 | **Links de qualquer site, e o baralho escolhido na revisão.** Não há mais leitor por site: o leitor procura em qualquer página o jeito comum de mostrar exemplos (frase em negrito, quebra de linha, tradução, áudio logo depois) e, achando, segue F21/F29. Se a página não mostra assim, o texto dela (o `<article>`, o `<main>` ou o `<body>`, sem menus, scripts e rodapés; até 40 mil caracteres; negritos com `**` e cada áudio marcado como `[áudio N]`) vai pela entrada padrão para a IA, na fila, que tira até 3 exemplos seguindo `regras/extrair-exemplos.md` (editável em Ajustes): as frases da própria página, a tradução dela quando houver, o número do áudio de cada uma. Sem internet. Áudio de qualquer servidor, mas só os endereços que vieram na proposta (o leitor os achou na página). Na revisão de qualquer proposta, o baralho pode ser trocado (`POST /api/geracoes/{id}/aprovar` aceita `baralho_id`: um que existe, ou `""` para um novo com `nome_baralho`); um link sem baralho começa no "Inglês", e a tela Do link também | Pedido do Lucas em 06/11/2026: nada específico de um site no app, na extensão ou na documentação, porque o link pode ser de qualquer lugar; e quase sempre os cartões vão para um baralho que já existe, o de inglês. |
| F32 | **Tag `phrasal-verb` nos cartões de link.** Além da tag da expressão (`shoot-up`), os cartões ganham `phrasal-verb` quando a expressão da página é um phrasal verb (verbo + partícula com sentido próprio; expressões idiomáticas e verbos com preposição sem sentido novo não contam). Quando a IA roda (escolher ou tirar os exemplos), ela responde `phrasal_verb`, seguindo a seção nova das duas instruções; quando não roda (até 3 frases prontas), vale o título ou o `<h1>` da página dizer "phrasal verb" | Pedido do Lucas em 07/11/2026: poder filtrar e estudar os phrasal verbs juntos. A proposta com até 3 frases continua instantânea, sem chamar a IA só para isso. |

## 3. Como roda

- **Desenvolvimento:** `make dev-api` (Go na porta 8080, dados em `./data`) e `make dev-web` (Vite com proxy para `/api`).
- **Testes:** `make test` — Go (algoritmo, fila e API), Vitest e Playwright (fluxo completo no navegador).
- **Publicar:** `git push origin main` e `./deploy.sh`. Dados no servidor em `/srv/data/flashcards`.

## 4. Integrações

**Feita:** Acervo → Flashcards (F28), por `POST /api/textos`, chamado pela rede interna do servidor
(`http://flashcards:8080`). O Acervo também lê e decide as propostas que ele mandou, por `GET /api/geracoes/{id}` e
`POST /api/geracoes/{id}/{aprovar|descartar|refazer}`.

**Planejada:** Biblioteca (seleção de texto) → Flashcards. Pelo princípio do ecossistema, o endpoint só nasce
quando o outro lado for mandar de verdade, e é aí que o contrato comum (`CONTRATO.md` do Hub) vira concreto. A ideia
inicial é `POST /api/v1/cards` recebendo um Fragmento de Conteúdo, que vira uma nota com a fonte guardada.
