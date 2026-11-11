// Temas (F38): o assunto que junta baralhos. Um tema soma os números dos baralhos dele; um baralho
// sem tema fica sozinho. Tudo em ordem de nome.
export function agrupar(baralhos) {
  const temas = new Map()
  const soltos = []
  for (const b of baralhos) {
    if (!b.tema) {
      soltos.push({ nome: b.nome, escopo: b.id, contagem: { ...b.contagem }, baralhos: [b] })
      continue
    }
    if (!temas.has(b.tema)) temas.set(b.tema, { nome: b.tema, tema: true, escopo: `tema/${encodeURIComponent(b.tema)}`, contagem: { novos: 0, aprendendo: 0, revisar: 0 }, baralhos: [] })
    const t = temas.get(b.tema)
    t.baralhos.push(b)
    for (const k of ['novos', 'aprendendo', 'revisar']) t.contagem[k] += b.contagem[k]
  }
  return [...temas.values(), ...soltos].sort((a, b) => a.nome.localeCompare(b.nome, 'pt-BR'))
}

export const totalDe = (c) => c.novos + c.aprendendo + c.revisar
