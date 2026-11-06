import { describe, expect, it } from 'vitest'
import { intervalo, vencimento } from './tempo.js'

describe('intervalo', () => {
  it('mostra minutos, horas, dias, meses e anos', () => {
    expect(intervalo(30)).toBe('<1 min')
    expect(intervalo(60)).toBe('1 min')
    expect(intervalo(600)).toBe('10 min')
    expect(intervalo(3 * 3600)).toBe('3 h')
    expect(intervalo(2 * 86400)).toBe('2 d')
    expect(intervalo(45 * 86400)).toBe('1,5 mês')
    expect(intervalo(90 * 86400)).toBe('3 meses')
    expect(intervalo(400 * 86400)).toBe('1,1 ano')
    expect(intervalo(800 * 86400)).toBe('2,2 anos')
  })
})

describe('vencimento', () => {
  const agora = new Date('2026-11-06T13:00:00Z')
  it('diz quando o cartão volta', () => {
    expect(vencimento('2026-11-06T12:00:00Z', agora)).toBe('para hoje')
    expect(vencimento('2026-11-09T13:00:00Z', agora)).toBe('em 3 d')
  })
})
