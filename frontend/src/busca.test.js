import { describe, expect, it } from 'vitest'
import { contextoDoCartao, termoDaBusca } from './busca.js'

describe('termoDaBusca', () => {
  it('tira as palavras de pergunta e fica com o assunto', () => {
    expect(termoDaBusca('Por que a bandeira do Brasil mudou em 1822?')).toBe('bandeira Brasil mudou 1822')
  })
  it('usa só a primeira linha e no máximo seis palavras', () => {
    expect(termoDaBusca('Segundo a interpretação mais comum: verde da Casa de Bragança (Pedro I)\noutra linha')).toBe(
      'Segundo interpretação comum verde Casa Bragança',
    )
  })
  it('o cartão inteiro vira o contexto', () => {
    expect(contextoDoCartao({ frente: 'f', verso: 'v', tags: '' })).toBe('f\nv')
  })
})
