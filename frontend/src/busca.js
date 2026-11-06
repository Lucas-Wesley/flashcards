// O termo que já vem no campo da busca de imagem: as palavras que dizem o assunto, sem as de
// pergunta ("Por que a bandeira do Brasil mudou em 1822?" → "bandeira Brasil mudou 1822"). O
// cartão inteiro vai junto, à parte, para o servidor ordenar o que voltar.
const VAZIAS = new Set(
  `a o as os e é de do da dos das em no na nos nas um uma uns umas por que qual quais quem quando onde como porque para
  com sem pelo pela pelos pelas foi era são ser seu sua seus suas este esta esse essa isso isto mais menos muito entre
  sobre até desde após the and for with from of`.split(/\s+/),
)

export function termoDaBusca(texto, maximo = 6) {
  const linha = (texto || '').split('\n')[0]
  return linha
    .split(/[^\p{L}\p{N}-]+/u)
    .filter((w) => w && !VAZIAS.has(w.toLowerCase()))
    .slice(0, maximo)
    .join(' ')
}

export function contextoDoCartao(c) {
  return [c.frente, c.verso, c.tags].filter(Boolean).join('\n')
}
