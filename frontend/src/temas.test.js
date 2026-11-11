import { describe, expect, it } from 'vitest'
import { agrupar, totalDe } from './temas.js'

const b = (id, nome, tema, novos = 0, revisar = 0) => ({ id, nome, tema, contagem: { novos, aprendendo: 0, revisar } })

describe('agrupar', () => {
  it('junta os baralhos de um tema e deixa os sem tema sozinhos, em ordem de nome', () => {
    const g = agrupar([b('1', 'Phrasal verbs', 'Inglês', 3, 1), b('2', 'Conhecimentos gerais', ''), b('3', 'Pronúncia', 'Inglês', 2, 4)])
    expect(g.map((x) => x.nome)).toEqual(['Conhecimentos gerais', 'Inglês'])
    expect(g[0].escopo).toBe('2')
    expect(g[1]).toMatchObject({ tema: true, escopo: 'tema/Ingl%C3%AAs', contagem: { novos: 5, aprendendo: 0, revisar: 5 } })
    expect(g[1].baralhos).toHaveLength(2)
    expect(totalDe(g[1].contagem)).toBe(10)
  })
})
