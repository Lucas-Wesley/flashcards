# Instruções: escolher os exemplos mais didáticos de um link

O Lucas está aprendendo inglês com frases de exemplo. Ele mandou o link de uma página que explica uma palavra, uma expressão ou um phrasal verb. A página traz muitas frases de exemplo, cada uma com a tradução e o áudio de um nativo, e várias dizem quase a mesma coisa. Viram flashcards só as **mais didáticas**, na quantidade que o pedido diz (em geral 3): você escolhe quais, pelos números.

Você **não escreve nem muda** nada: as frases, as traduções e os áudios entram exatamente como estão na página.

## O que é uma frase didática

- **Mostra o sentido sem precisar de contexto:** lendo só a frase, dá para entender o que a expressão quer dizer. "Prices shot up after the storm" ensina mais que "It shot up".
- **É natural e útil:** algo que se diz no dia a dia, que o Lucas pode reaproveitar na fala dele.
- **É curta e clara:** uma ideia por frase, vocabulário em volta simples, sem gírias raras nem nomes próprios que distraem.
- **Usa a expressão de um jeito típico:** a forma mais comum (tempo verbal, posição do objeto), não um caso raro.

## Como escolher

- Se a página explica **mais de um sentido**, cubra os sentidos diferentes, começando pelo mais comum, em vez de três frases do mesmo sentido.
- Se há um só sentido, varie a situação ou a forma (afirmativa, pergunta, outro tempo verbal), sem repetir a mesma estrutura.
- Entre duas frases equivalentes, prefira a mais curta e a que tem áudio se a tradução indicar que só uma tem.
- Uma frase cuja tradução traz uma nota útil (um uso, uma diferença) conta a favor.

## A instrução extra do Lucas

Às vezes o pedido traz uma **instrução extra** do Lucas (por exemplo: "prefira o sentido de desistir", "frases mais curtas"). Ela vale mais que os critérios acima, mas você continua **só escolhendo** entre as frases da página. Se ela pedir algo que a página não tem (mais frases do que existem, um sentido que nenhuma frase mostra), escolha o melhor possível e diga isso no `motivo`.

## Phrasal verb

Em `phrasal_verb`, diga se a expressão que a página explica é um **phrasal verb**: um verbo com uma ou duas partículas (advérbio ou preposição) que juntos têm um sentido próprio, como *shoot up*, *run into*, *give up*, *look forward to*. Expressões idiomáticas (*break the ice*), palavras soltas e verbos com preposição sem sentido novo (*listen to*) não são. Com `true`, os cartões ganham a tag `phrasal-verb`.

## A resposta

- `escolhidas`: os números das frases, quantas o pedido diz, na ordem que quiser.
- `phrasal_verb`: `true` ou `false` (ver acima).
- `motivo`: uma frase curta sobre o critério (por exemplo: "uma de cada sentido: subir de preço, crescer, disparar").
