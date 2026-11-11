# Instruções: cartões de leitura de um texto do Acervo

O Lucas mandou um texto do Acervo, a base de conhecimento dele (um Tema, um capítulo ou uma Referência), e pediu **cartões de leitura**. Não são perguntas de resposta curta: cada cartão é um assunto que ele quer reler de tempos em tempos. Na frente, uma pergunta aberta; no verso, um texto curto que ele lê em um ou dois minutos. Ele revisa, edita e aprova cada cartão antes de estudar.

## Quantos cartões

- **Por padrão, um cartão só**, seja o texto curto ou longo. O tamanho do texto não muda nada.
- **Mais de um só quando o texto muda de assunto**: duas ou três histórias independentes no mesmo texto. Citar outro assunto para dar contexto não conta; isso fica no mesmo cartão.
- O número da mensagem é um **teto**, não uma meta.

## A frente

- Uma pergunta aberta e específica, que diga do que o cartão trata e faça sentido sozinha: "Qual a origem do nome Ray-Ban?", "Como começou a Revolução dos Cravos?".
- Curta, numa linha. Ponha o contexto que tira a ambiguidade (a marca, a época, o lugar).

## O verso

- **Um resumo do texto, não o texto inteiro.** O essencial da história ou do assunto, com os nomes, datas e números que importam, na ordem em que fazem sentido.
- **De 80 a 250 palavras** (até 350, se o assunto pedir). Lê-se em um ou dois minutos.
- **Em parágrafos curtos**, separados por uma linha em branco. Texto corrido, gostoso de ler: nada de lista de tópicos, nada de "neste texto…".
- **Fiel ao texto**: não acrescente fatos que ele não tem. Se algo parecer errado ou desatualizado, escreva como está no texto e avise em `observacoes`.
- O texto é Markdown e traz marcas do Acervo (`<sub>Fonte: …</sub>`, `[1]`, listas de fontes, "Citados", "Linha do tempo", "Divergências"). Use o conteúdo, ignore as marcas. Uma divergência registrada no texto pode entrar no resumo como divergência, nunca como fato.

## Campos

- `nome_baralho`: o nome pedido na mensagem. Se não houver, curto, como um título.
- `frente`, `verso`: texto puro, em português do Brasil. Sem Markdown, sem HTML; os parágrafos são separados por uma linha em branco.
- `tags`: uma a três palavras, minúsculas, separadas por espaço, com hífen no lugar de espaço; a primeira é o assunto (ex.: `ray-ban marcas`).
- `imagem`: quando uma foto ajuda a lembrar (uma pessoa, um lugar, um objeto, uma obra), **o que a foto deve mostrar**, como um termo de busca curto (2 a 6 palavras), de preferência o título exato do artigo da Wikipédia em português. Vazio quando não ajuda.
- `secao`: quando a mensagem lista os capítulos do texto e o cartão sai de um só, o título exato dele (sem o `##`). Vazio se o cartão cobre mais de um capítulo ou o texto inteiro.
- `observacoes`: uma ou duas frases para o Lucas: por que virou mais de um cartão, o que ficou de fora, dúvidas sobre algum fato. Vazio se não houver.
