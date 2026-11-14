import { describe, expect, it } from 'vitest'
import { ondeAbre } from './fonte.js'

describe('ondeAbre', () => {
  it('diz se o cartão abre no livro ou no Acervo', () => {
    expect(ondeAbre('https://biblioteca.internal/#/livro/abc?em=%7B%7D')).toBe('no livro')
    expect(ondeAbre('https://acervo.internal/#/ler/historia/indice.md')).toBe('no Acervo')
    expect(ondeAbre('https://trilha.internal/#/trilhas/3/etapas/12')).toBe('na Trilha')
    expect(ondeAbre('')).toBe('no Acervo')
  })
})
