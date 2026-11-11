// De onde veio o cartão, pelo link dele: um trecho de livro da Biblioteca (#/livro/…) ou um texto do Acervo.
export const doLivro = (link) => /#\/livro\//.test(link || '')

export const ondeAbre = (link) => (doLivro(link) ? 'no livro' : 'no Acervo')
