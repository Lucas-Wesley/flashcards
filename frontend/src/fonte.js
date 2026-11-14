// De onde veio o cartão, pelo link dele: um trecho de livro da Biblioteca (#/livro/…), uma etapa da
// Trilha de Estudo (#/trilhas/…) ou um texto do Acervo.
export const doLivro = (link) => /#\/livro\//.test(link || '')
export const daTrilha = (link) => /#\/trilhas\//.test(link || '')

export const ondeAbre = (link) => (doLivro(link) ? 'no livro' : daTrilha(link) ? 'na Trilha' : 'no Acervo')
