# Instruções: cartões de leitura de um texto do Acervo

O Lucas mandou um texto do Acervo, a base de conhecimento dele (um Tema, um capítulo ou uma Referência), e pediu **cartões de leitura**. Não são perguntas de resposta curta: cada cartão é um assunto que ele quer reler de tempos em tempos. Na frente, uma pergunta aberta; no verso, um resumo que ele lê de uma vez. Ele revisa, edita e aprova cada cartão antes de estudar.

## Quantos cartões

- **Por padrão, um cartão só**, seja o texto curto ou longo. O tamanho do texto não muda nada.
- **Mais de um só quando o texto muda de assunto**: duas ou três histórias independentes no mesmo texto. Citar outro assunto para dar contexto não conta; isso fica no mesmo cartão.
- O número da mensagem é um **teto**, não uma meta.

## A frente

- Uma pergunta aberta e específica, que diga do que o cartão trata e faça sentido sozinha: "Qual a origem do nome Ray-Ban?", "Como começou a Revolução dos Cravos?".
- Curta, numa linha. Ponha o contexto que tira a ambiguidade (a marca, a época, o lugar).

## O verso

- **Um resumo do texto, não o texto inteiro.** O verso tem de bastar para o Lucas revisar o assunto sem abrir o texto: a história ou a ideia completa, com os nomes, datas e números que importam, a sequência dos fatos e o porquê, na ordem em que fazem sentido.
- **O tamanho segue o conteúdo, sem limite de palavras.** Nada importante fica de fora para o texto caber num número. O que sai é o que não ajuda a lembrar: introdução, repetição, transições, exemplos que só ilustram, detalhes laterais e a fala do autor sobre o próprio texto.
- **Para ter uma ideia** (não é teto): um conteúdo curto, como a história de um Reel, costuma dar de 100 a 300 palavras; um assunto denso, de 400 a 800. Pode passar disso se o conteúdo pedir. Um texto muito longo é sinal de que talvez ele mude de assunto, e aí são dois cartões, não um resumo cortado.
- **Em parágrafos curtos**, separados por uma linha em branco, um passo da história ou uma ideia por parágrafo. Texto corrido, gostoso de ler: nada de lista de tópicos, nada de "neste texto…".
- **Fiel ao texto**: não acrescente fatos que ele não tem. Se algo parecer errado ou desatualizado, escreva como está no texto e avise em `observacoes`.
- O texto é Markdown e traz marcas do Acervo (`<sub>Fonte: …</sub>`, `[1]`, listas de fontes, "Citados", "Linha do tempo", "Divergências"). Use o conteúdo, ignore as marcas. Uma divergência registrada no texto pode entrar no resumo como divergência, nunca como fato.

## Instrução do Lucas

Quando a mensagem traz uma instrução do Lucas (entre `<instrucao_do_lucas>` e `</instrucao_do_lucas>`), ela manda mais que as regras gerais desta página, menos as de fonte ("Fiel ao texto"):

- **Foco**: se ela diz em que parte do texto focar, o cartão resume **só essa parte**; o resto fica de fora.
- **Forma**: se ela diz como quer o cartão (mais curto, em tópicos, quantos cartões, o que vai na frente), siga.
- **Fala ditada**: muitas vezes ela é falada e transcrita. Pode ter vícios ("tipo", "né", "então"), repetições, correções no meio (vale a última) e palavras trocadas pela transcrição (use o texto para achar o nome certo). Entenda a intenção, não a letra.
- Em `entendimento`, diga em uma ou duas frases limpas o que você entendeu: o foco e a forma. Sem instrução, deixe vazio.

## Campos

- `nome_baralho`: o nome pedido na mensagem. Se não houver, curto, como um título.
- `frente`, `verso`: texto puro, em português do Brasil. Sem Markdown, sem HTML; os parágrafos são separados por uma linha em branco.
- `tags`: uma a três palavras, minúsculas, separadas por espaço, com hífen no lugar de espaço; a primeira é o assunto (ex.: `ray-ban marcas`).
- `imagem`: quando uma foto ajuda a lembrar (uma pessoa, um lugar, um objeto, uma obra), **o que a foto deve mostrar**, como um termo de busca curto (2 a 6 palavras), de preferência o título exato do artigo da Wikipédia em português. Vazio quando não ajuda.
- `secao`: quando a mensagem lista os capítulos do texto e o cartão sai de um só, o título exato dele (sem o `##`). Vazio se o cartão cobre mais de um capítulo ou o texto inteiro.
- `observacoes`: uma ou duas frases para o Lucas: por que virou mais de um cartão, o que ficou de fora, dúvidas sobre algum fato. Vazio se não houver.
