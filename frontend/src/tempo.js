const DIA = 86400

const decimal = (n) => (Math.round(n * 10) / 10).toLocaleString('pt-BR')

// intervalo: o que aparece sobre os botões de resposta ("10 min", "3 d", "1,5 mês").
export function intervalo(segundos) {
  if (segundos < 60) return '<1 min'
  if (segundos < 3600) return `${Math.round(segundos / 60)} min`
  if (segundos < DIA) return `${Math.round(segundos / 3600)} h`
  const dias = segundos / DIA
  if (dias < 30) return `${Math.round(dias)} d`
  if (dias < 365) {
    const meses = dias / 30
    return `${decimal(meses)} ${meses < 1.95 ? 'mês' : 'meses'}`
  }
  const anos = dias / 365
  return `${decimal(anos)} ${anos < 1.95 ? 'ano' : 'anos'}`
}

// vencimento: quando o cartão volta, para a lista de cartões.
export function vencimento(iso, agora = new Date()) {
  const s = (new Date(iso) - agora) / 1000
  if (s <= 0) return 'para hoje'
  return `em ${intervalo(s)}`
}

export const hora = (iso) => new Date(iso).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })
