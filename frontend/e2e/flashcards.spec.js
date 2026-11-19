import { expect, test } from '@playwright/test'

// Um fluxo só, na ordem em que o Lucas usa: baralho → cartões → estudo → editar → Ajustes.
test.describe.configure({ mode: 'serial' })

test('cria o baralho, adiciona cartões e importa uma lista', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Nenhum baralho ainda.')).toBeVisible()
  await page.getByRole('link', { name: 'Crie o primeiro' }).click()
  await expect(page.getByRole('link', { name: 'Baralho novo' })).toHaveAttribute('aria-current', 'page')
  await page.getByLabel('Nome do novo baralho').fill('Inglês')
  await page.getByRole('button', { name: 'Criar baralho' }).click()
  await expect(page.getByRole('heading', { name: 'Inglês' })).toBeVisible()

  await page.getByRole('link', { name: 'Adicionar', exact: true }).click()
  await page.getByRole('radio', { name: 'Nos dois sentidos' }).click()
  await page.getByLabel('Frente').fill('casa')
  await page.getByLabel('Verso').fill('house')
  await page.getByRole('main').getByRole('button', { name: 'Adicionar' }).click()
  await expect(page.getByText('Adicionado: dois cartões, um em cada sentido.')).toBeVisible()
  // O formulário fica pronto para o próximo, com o foco na frente.
  await expect(page.getByLabel('Frente')).toBeFocused()
  await expect(page.getByLabel('Frente')).toHaveValue('')

  await page.goto('/#/baralhos')
  await page.getByRole('link', { name: /Inglês/ }).first().click()
  await page.getByRole('link', { name: 'Importar' }).click()
  await page.getByLabel('Linhas para importar').fill('cachorro;dog\ngato;cat\ncasa;house')
  await page.getByRole('button', { name: 'Importar 3 linhas' }).click()
  await expect(page.getByRole('status').filter({ hasText: '2 notas importadas; 1 já estava no baralho' })).toBeVisible()
})

test('estuda pelo teclado e desfaz', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByLabel('4 novos, 0 aprendendo, 0 para revisar')).toBeVisible()
  await page.getByRole('link', { name: /Inglês/ }).first().click()
  await page.getByRole('link', { name: 'Estudar', exact: true }).click()

  await expect(page.locator('.frente')).toHaveText('casa')
  await expect(page.locator('.verso')).toHaveCount(0)
  await page.keyboard.press('Space')
  await expect(page.locator('.verso')).toHaveText('house')
  // Os botões mostram quando o cartão volta.
  await expect(page.getByRole('button', { name: /1 min\s*Errei/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /10 min\s*Bom/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /1 d\s*Amanhã/ })).toBeVisible()

  await page.keyboard.press('3') // Bom: volta em 10 min
  // O outro sentido de "casa" fica para amanhã; vem o próximo novo.
  await expect(page.locator('.frente')).toHaveText('cachorro')

  await page.getByRole('button', { name: 'Desfazer a última resposta' }).click()
  await expect(page.locator('.frente')).toHaveText('casa')

  await page.getByRole('button', { name: 'Mostrar resposta' }).click()
  await page.getByRole('button', { name: /Fácil/ }).click()
  await expect(page.locator('.frente')).toHaveText('cachorro')
  for (const frente of ['cachorro', 'gato']) {
    await expect(page.locator('.frente')).toHaveText(frente)
    await page.keyboard.press('Space')
    await page.keyboard.press('4')
  }
  await expect(page.getByText('Pronto por hoje.')).toBeVisible()
  await expect(page.getByText('3 revisões hoje.')).toBeVisible()
})

test('busca, edita e suspende um cartão', async ({ page }) => {
  await page.goto('/#/cartoes')
  await expect(page.getByText('3 notas')).toBeVisible()
  await page.getByLabel('Buscar cartões').fill('gat')
  await expect(page.getByText('1 nota')).toBeVisible()
  await page.getByRole('link', { name: /gato/ }).click()

  await expect(page.getByLabel('Verso')).toHaveValue('cat') // a nota carregou
  await page.getByLabel('Verso').fill('cat, kitty')
  await page.keyboard.press('Control+Enter')
  await expect(page.getByText('Cartão salvo.')).toBeVisible()
  await page.getByRole('button', { name: 'Suspender' }).click()
  await expect(page.getByText(/Suspenso/)).toBeVisible()
  await page.getByRole('button', { name: 'Reativar' }).click()
  await expect(page.getByText(/Revisão em/)).toBeVisible()
})

test('monta cartões a partir de um link compartilhado', async ({ page }) => {
  // O "Compartilhar" do Android abre o app com o link na URL.
  const link = 'http://localhost:8096/frases.html'
  await page.goto(`/?text=${encodeURIComponent('Olha isso ' + link)}`)
  await expect(page.getByRole('heading', { name: 'Adicionar' })).toBeVisible()
  await expect(page.getByLabel('Link')).toHaveValue(link)
  await page.getByLabel('Baralho de destino').selectOption({ label: 'Inglês' })
  await page.getByRole('button', { name: 'Montar cartões' }).click()

  await expect(page.getByRole('heading', { name: 'Cartões do link' })).toBeVisible()
  await expect(page.getByText('3 frases de exemplo de “Run Into”, 2 com áudio.')).toBeVisible()
  // Na revisão, cada cartão aparece como no estudo; os campos só abrem em "Editar".
  await expect(page.locator('.p-frente').first()).toHaveText('I ran into an old friend yesterday.')
  await expect(page.getByLabel('Áudio do cartão 1')).toBeAttached()
  // O baralho pode ser trocado na revisão; fica o escolhido no pedido.
  await expect(page.getByLabel('Baralho', { exact: true }).locator('option:checked')).toHaveText('Inglês')
  await expect(page.getByLabel('Nome do baralho novo')).toHaveCount(0)
  // Até 3 frases prontas não passam pela IA: não há o que refazer.
  await expect(page.getByRole('button', { name: 'Refazer' })).toHaveCount(0)
  // Mas dá para pedir algo a mais (F42): a instrução e quantas frases, com 3 de partida.
  await page.getByRole('button', { name: 'Pedir algo a mais' }).click()
  await expect(page.getByLabel('O que mudar ou acrescentar')).toBeVisible()
  await expect(page.getByLabel('Quantidade')).toHaveValue('3')
  await page.getByRole('button', { name: 'Cancelar' }).click()
  await expect(page.getByLabel('O que mudar ou acrescentar')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Excluir' })).toBeVisible()
  await page.getByRole('button', { name: 'Adicionar 3 ao baralho' }).click()
  // O áudio do terceiro aponta para um servidor que não existe: o cartão entra sem ele.
  await expect(page.getByText('3 notas criadas; 1 áudio não veio.')).toBeVisible()

  await page.goto('/#/cartoes')
  await page.getByLabel('Buscar cartões').fill('ran into an old')
  await page.getByRole('link', { name: /ran into an old friend/ }).click()
  await expect(page.locator('audio')).toHaveAttribute('src', /\/api\/audios\/[0-9a-f]{32}\.mp3$/)
})

test('gera um baralho com a IA, revisa e aprova', async ({ page }) => {
  // A página Baralhos tem só os baralhos; o pedido à IA fica em Adicionar.
  await page.goto('/')
  await expect(page.getByText('Novo baralho')).toHaveCount(0)
  await page.getByRole('button', { name: 'Adicionar' }).click()
  await page.getByRole('link', { name: 'Com IA' }).click()
  await page.getByLabel('Pedido').fill('presidentes do Brasil no século 21')
  // A quantidade é opcional e fica recolhida em Opções: em branco, a IA decide.
  await expect(page.getByLabel('Quantidade')).toBeHidden()
  await expect(page.getByText('a IA decide quantos')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Falar em vez de escrever' })).toBeVisible()
  await page.getByRole('button', { name: 'Pedir à IA' }).click()

  await expect(page.getByText('Proposta para revisar')).toBeVisible({ timeout: 15000 })
  // Em Pedidos, a proposta espera a revisão.
  await page.getByRole('link', { name: 'Pedidos' }).click()
  await expect(page.getByRole('heading', { name: 'Pedidos' })).toBeVisible()
  await page.getByRole('link', { name: /presidentes do Brasil no século 21.*Para revisar/ }).click()
  await expect(page.getByText('2 de 2 cartões marcados')).toBeVisible()
  await expect(page.getByText(/cerca de/)).toHaveCount(0)
  await expect(page.getByLabel('Nome do baralho novo')).toHaveValue('Presidentes do Brasil no século 21')
  await page.getByLabel('Incluir o cartão 2').uncheck()
  await page.getByRole('button', { name: 'Editar', exact: true }).first().click()
  await page.getByLabel('Verso do cartão 1').fill('Lula (2003–2010)')
  await page.getByRole('button', { name: 'Criar baralho com 1 cartão' }).click()

  await expect(page.getByRole('heading', { name: 'Presidentes do Brasil no século 21' })).toBeVisible()
  await page.getByRole('link', { name: 'Estudar', exact: true }).click()
  await expect(page.locator('.frente')).toHaveText('Quem foi presidente do Brasil de 2003 a 2010?')
  await page.keyboard.press('Space')
  await expect(page.locator('.verso')).toHaveText('Lula (2003–2010)')

  // Depois de aprovado, o pedido ainda pode ser editado e refeito: o cartão de mesma frente é
  // atualizado, e o que não existia entra.
  await page.keyboard.press('Escape')
  await page.getByRole('link', { name: '“presidentes do Brasil no século 21”' }).click()
  await page.getByRole('button', { name: 'Editar e refazer' }).click()
  await page.getByLabel('Pedido').fill('presidentes do Brasil no século 21, com o partido de cada um')
  await page.getByRole('button', { name: 'Refazer com este pedido' }).click()
  await expect(page.getByText('Proposta para revisar')).toBeVisible({ timeout: 15000 })
  await expect(page.locator('.pedido')).toContainText('com o partido de cada um')
  await expect(page.getByText('Atualiza o que já existe')).toHaveCount(1)
  await page.getByRole('button', { name: 'Adicionar 2 ao baralho' }).click()
  await expect(page.getByText('1 nota criada, 1 atualizada.')).toBeVisible()
})

test('estuda um tema, com os baralhos dele misturados, e depois no aleatório', async ({ page }) => {
  // Os dois baralhos entram no tema "Estudos".
  for (const nome of ['Inglês', 'Presidentes do Brasil no século 21']) {
    await page.goto('/')
    await page.getByRole('link', { name: new RegExp(nome) }).first().click()
    await page.getByPlaceholder('Sem tema').fill('Estudos')
    await page.getByRole('button', { name: 'Salvar' }).click()
    await expect(page.getByText('Opções salvas.')).toBeVisible()
  }
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Estudos' })).toBeVisible()
  await page.locator('.topo .estudar').click()
  await expect(page.getByRole('heading', { name: 'Estudar' })).toBeVisible()
  await expect(page.locator('.linha', { hasText: 'Estudos' })).toContainText('2 baralhos')

  await page.locator('.linha', { hasText: 'Estudos' }).click()
  const baralhos = new Set()
  for (let i = 0; i < 12 && !(await page.getByText('Pronto por hoje.').isVisible()); i++) {
    await expect(page.locator('.frente')).toBeVisible()
    await expect(page.locator('.modo')).toContainText('Estudos')
    baralhos.add((await page.locator('.barra .baralho').textContent()).replace('Estudos · ', ''))
    await page.keyboard.press('Space')
    await page.keyboard.press('4')
    await page.waitForTimeout(150)
  }
  await expect(page.getByText('Pronto por hoje.')).toBeVisible()
  expect([...baralhos].sort()).toEqual(['Inglês', 'Presidentes do Brasil no século 21'])

  // Acabada a fila, o aleatório segue sorteando, sem nota e sem mexer no agendamento.
  await page.getByRole('link', { name: 'Aleatório' }).click()
  await expect(page.locator('.modo')).toContainText('Aleatório')
  await expect(page.locator('.frente')).toBeVisible()
  await page.keyboard.press('Space')
  await expect(page.locator('.verso')).toBeVisible()
  await expect(page.getByRole('button', { name: /Bom/ })).toHaveCount(0)
  await page.getByRole('button', { name: 'Próximo' }).click()
  await expect(page.locator('.vistos')).toHaveText('1')

  await page.goto('/')
  await expect(page.locator('.topo .estudar')).not.toHaveClass(/principal/)
})

test('edita a instrução da IA e volta ao padrão', async ({ page }) => {
  await page.goto('/#/ajustes')
  await page.getByRole('link', { name: /Instruções da IA/ }).click()
  await page.getByRole('link', { name: /Gerar um baralho/ }).click()
  await expect(page.locator('article.instrucao')).toContainText('Um fato por cartão')
  await page.getByRole('button', { name: 'Editar' }).click()
  await page.getByLabel('Texto da instrução').fill('Só cartões curtos.')
  await page.getByRole('button', { name: 'Salvar' }).click()
  await expect(page.locator('article.instrucao')).toHaveText('Só cartões curtos.')
  page.once('dialog', (d) => d.accept())
  await page.getByRole('button', { name: 'Voltar ao padrão' }).click()
  await expect(page.locator('article.instrucao')).toContainText('Um fato por cartão')
})

test('Ajustes recebe sugestões de melhoria', async ({ page }) => {
  await page.goto('/#/cartoes')
  await page.getByRole('button', { name: 'Ajustes' }).click()
  await page.getByRole('link', { name: /Sugestões de melhoria/ }).click()
  await page.getByLabel('Sugestão de melhoria').fill('Áudio de pronúncia')
  await page.getByRole('button', { name: 'Enviar sugestão' }).click()
  await expect(page.locator('.sugestao', { hasText: 'Áudio de pronúncia' })).toBeVisible()
})

test('mostra o estudo na TV, respondendo pelo celular', async ({ page, context }) => {
  // Um baralho só para a TV, depois dos outros testes (não muda as contas deles).
  await page.goto('/#/novo-baralho')
  await page.getByLabel('Nome do novo baralho').fill('Capitais')
  await page.getByRole('button', { name: 'Criar baralho' }).click()
  await page.getByRole('link', { name: 'Adicionar', exact: true }).click()
  await page.getByLabel('Frente').fill('França')
  await page.getByLabel('Verso').fill('Paris')
  await page.getByRole('main').getByRole('button', { name: 'Adicionar' }).click()
  await expect(page.getByText('Cartão adicionado.')).toBeVisible()

  const tv = await context.newPage()
  await tv.goto('http://localhost:8097/')
  await expect(tv.getByText('Flashcards na TV')).toBeVisible()

  await page.goto('/#/baralhos')
  await page.getByRole('link', { name: /Capitais/ }).click()
  await page.getByRole('link', { name: 'Estudar', exact: true }).click()
  await expect(page.locator('.frente')).toHaveText('França')
  await page.getByRole('button', { name: 'Mostrar na TV' }).click()
  await expect(page.getByRole('button', { name: 'Parar de mostrar na TV' })).toHaveAttribute('aria-pressed', 'true')

  await expect(tv.locator('#frente')).toHaveText('França')
  await expect(tv.locator('#baralho')).toHaveText('Capitais')
  await expect(tv.locator('#verso')).toBeHidden()
  await page.getByRole('button', { name: 'Mostrar resposta' }).click()
  await expect(tv.locator('#verso')).toHaveText('Paris')
  await page.getByRole('button', { name: /Fácil/ }).click()
  await expect(tv.getByText('Pronto por hoje.')).toBeVisible()

  // A escolha fica guardada; ao sair do estudo, a TV volta a esperar.
  await page.goto('/#/baralhos')
  await expect(tv.getByText('No celular, abra um baralho')).toBeVisible()
})

test('um cartão de leitura mostra o verso como texto para ler', async ({ page }) => {
  await page.goto('/#/novo-baralho')
  await page.getByLabel('Nome do novo baralho').fill('Curiosidades')
  await page.getByRole('button', { name: 'Criar baralho' }).click()
  await page.getByRole('link', { name: 'Adicionar', exact: true }).click()
  await page.getByRole('radio', { name: 'Leitura' }).click()
  await page.getByLabel('Frente').fill('Qual a origem do nome Ray-Ban?')
  await page.getByLabel('Verso').fill('Em 1937, a Bausch & Lomb lançou óculos para pilotos.\n\nO nome vem de banir (ban) os raios (rays) do sol.')
  await page.getByRole('main').getByRole('button', { name: 'Adicionar' }).click()
  await expect(page.getByText('Cartão adicionado.')).toBeVisible()

  await page.goto('/#/baralhos')
  await page.getByRole('link', { name: /Curiosidades/ }).click()
  await page.getByRole('link', { name: 'Estudar', exact: true }).click()
  await expect(page.locator('.face.leitura .frente')).toHaveText('Qual a origem do nome Ray-Ban?')
  await page.keyboard.press('Space')
  await expect(page.locator('.face.leitura .verso')).toContainText('banir (ban)')
  await expect(page.locator('.face.leitura .verso')).toHaveCSS('text-align', 'left')
})

test('um baralho de reconhecer: a bandeira na frente, o nome no verso', async ({ page }) => {
  await page.goto('/#/gerar')
  await page.getByLabel('Pedido').fill('BANDEIRAS DO MUNDO: na frente a bandeira, no verso o nome do país e um mapa')
  await page.getByRole('button', { name: 'Pedir à IA' }).click()

  await expect(page.getByText('Proposta para revisar')).toBeVisible({ timeout: 15000 })
  // A IA diz como entendeu o pedido; a frente se repete, e a bandeira é o que diferencia.
  await expect(page.locator('.entendimento')).toContainText('a bandeira na frente')
  await expect(page.locator('.p-frente')).toHaveText(['De que país é esta bandeira?', 'De que país é esta bandeira?'])
  await expect(page.locator('.img-frente')).toHaveCount(2)
  // A conferência (F40) diz se a proposta atende; os avisos ficam no cartão.
  await expect(page.locator('.conferencia')).toBeVisible()
  const comAviso = await page.locator('.avisos-do-cartao').count()
  if (comAviso) {
    await page.getByRole('button', { name: /com aviso/ }).click()
    await expect(page.locator('.proposto:visible')).toHaveCount(comAviso)
    await page.getByRole('button', { name: 'Ver todos' }).click()
  }
  await page.getByLabel('Nome do baralho novo').fill('Bandeiras do mundo')
  await page.getByRole('button', { name: 'Criar baralho com 2 cartões' }).click()
  await expect(page.getByText('2 notas criadas.')).toBeVisible()

  await page.getByRole('link', { name: 'Estudar', exact: true }).click()
  await expect(page.locator('img.da-frente')).toHaveAttribute('src', /\/api\/imagens\/[0-9a-f]{32}\.png$/)
  await expect(page.locator('.frente')).toHaveText('De que país é esta bandeira?')
  await page.keyboard.press('Space')
  await expect(page.locator('.verso')).toHaveText(/Japão|Brasil/)
})

test('no estudo, o menu ⋮ edita e exclui o cartão', async ({ page }) => {
  await page.goto('/#/novo-baralho')
  await page.getByLabel('Nome do novo baralho').fill('Rios')
  await page.getByRole('button', { name: 'Criar baralho' }).click()
  await page.getByRole('link', { name: 'Adicionar', exact: true }).click()
  for (const [frente, verso] of [['Rio mais longo do Brasil', 'Amazonas'], ['Rio do Velho Chico', 'São Francisco']]) {
    await page.getByLabel('Frente').fill(frente)
    await page.getByLabel('Verso').fill(verso)
    await page.getByRole('main').getByRole('button', { name: 'Adicionar' }).click()
    await expect(page.getByText('Cartão adicionado.')).toBeVisible()
  }

  await page.goto('/#/baralhos')
  await page.getByRole('link', { name: /Rios/ }).click()
  await page.getByRole('link', { name: 'Estudar', exact: true }).click()
  await expect(page.locator('.frente')).toHaveText('Rio mais longo do Brasil')

  // Editar: abre o editor da nota e, ao salvar, volta para o estudo com o texto novo.
  await page.getByRole('button', { name: 'Opções do cartão' }).click()
  await page.getByRole('menuitem', { name: 'Editar' }).click()
  await page.getByLabel('Frente').fill('Rio mais longo da América do Sul')
  await page.getByRole('button', { name: 'Salvar' }).click()
  await expect(page.locator('.frente')).toHaveText('Rio mais longo da América do Sul')

  // Excluir: some do estudo e vem o próximo.
  page.once('dialog', (d) => d.accept())
  await page.getByRole('button', { name: 'Opções do cartão' }).click()
  await page.getByRole('menuitem', { name: 'Excluir' }).click()
  await expect(page.getByText('Cartão excluído.')).toBeVisible()
  await expect(page.locator('.frente')).toHaveText('Rio do Velho Chico')
})

// pdfDeTeste: um PDF de uma página com as linhas dadas (o mesmo do teste Go).
function pdfDeTeste(linhas) {
  const conteudo = `BT /F1 12 Tf 50 750 Td 16 TL\n${linhas.map((l) => `(${l}) Tj T*`).join('\n')}\nET`
  const objetos = [
    '<< /Type /Catalog /Pages 2 0 R >>',
    '<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
    '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>',
    `<< /Length ${conteudo.length} >>\nstream\n${conteudo}\nendstream`,
    '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',
  ]
  let pdf = '%PDF-1.4\n'
  const posicoes = objetos.map((o, i) => {
    const p = pdf.length
    pdf += `${i + 1} 0 obj\n${o}\nendobj\n`
    return p
  })
  const xref = pdf.length
  pdf += `xref\n0 ${objetos.length + 1}\n0000000000 65535 f \n${posicoes.map((p) => `${String(p).padStart(10, '0')} 00000 n \n`).join('')}`
  pdf += `trailer\n<< /Size ${objetos.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`
  return Buffer.from(pdf, 'latin1')
}

test('sobe um PDF com cartões e a IA propõe os cartões dele', async ({ page }) => {
  await page.goto('/#/pdf')
  await expect(page.getByRole('link', { name: 'Do PDF' })).toHaveAttribute('aria-current', 'page')
  await page.getByLabel('Arquivo PDF').setInputFiles({ name: 'Licao 01.pdf', mimeType: 'application/pdf', buffer: pdfDeTeste(['Hello there, friend.', 'Ola, amigo.']) })
  await page.getByLabel('Baralho de destino').selectOption({ label: 'Inglês' })
  await page.getByRole('button', { name: 'Tirar os cartões' }).click()
  await expect(page.getByRole('heading', { name: 'Cartões do PDF' })).toBeVisible()
  await expect(page.getByText('Proposta para revisar')).toBeVisible({ timeout: 15000 })
  await expect(page.getByText(/Recebeu o PDF pela entrada/)).toBeVisible()
})

test('frases de um filme chegam do Streaming e o áudio entra depois', async ({ page, request }) => {
  const baralhos = await (await request.get('http://localhost:8095/api/baralhos')).json()
  const ingles = baralhos.find((b) => b.nome === 'Inglês')
  const fonte = (t) => `https://streaming.internal/#/titulo/filme/9?fala=${t}`
  const r = await request.post('http://localhost:8095/api/propostas', {
    data: {
      origem: 'streaming',
      titulo: 'Frases de Filme Inventado',
      link: 'https://streaming.internal/#/titulo/filme/9',
      baralho_id: ingles.id,
      contexto: '2 frases de 180 candidatas',
      cartoes: [
        { frente: "I'll figure it out.", verso: 'Eu vou dar um jeito.\n\nfigure out: resolver', tags: 'streaming', fonte: fonte(10), audio_pendente: true },
        { frente: 'Keep me posted.', verso: 'Me mantenha informado.', tags: 'streaming', fonte: fonte(20), audio_pendente: true },
      ],
    },
  })
  const g = await r.json()
  await page.goto(`/#/geracao/${g.id}`)
  await expect(page.getByRole('heading', { name: 'Frases do Streaming' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Abrir no Streaming' })).toBeVisible()
  await expect(page.getByText('2 frases ainda sem áudio')).toBeVisible()
  await expect(page.getByText('Áudio chegando')).toHaveCount(2)
  await expect(page.getByRole('link', { name: 'Fala no Streaming' })).toHaveCount(2)

  // O Streaming manda o áudio da primeira e avisa que a segunda ficou sem.
  const mp3 = Buffer.concat([Buffer.from('ID3\x04\x00\x00\x00\x00\x00\x00', 'latin1'), Buffer.alloc(256, 0xff)])
  const { nome } = await (await request.post('http://localhost:8095/api/audios', { multipart: { arquivo: { name: 'fala.mp3', mimeType: 'audio/mpeg', buffer: mp3 } } })).json()
  await request.post(`http://localhost:8095/api/geracoes/${g.id}/audios`, { data: { audios: [{ fonte: fonte(10), audio: nome }, { fonte: fonte(20) }] } })
  await expect(page.getByText('Áudio chegando')).toHaveCount(0, { timeout: 20000 })
  await expect(page.getByLabel('Áudio do cartão 1')).toBeVisible()
  // Com o áudio, a frente fica borrada até um toque (F47); a que ficou sem áudio aparece de cara.
  await expect(page.getByRole('button', { name: 'Revelar a frente do cartão 2' })).toHaveCount(0)
  await page.getByRole('button', { name: 'Revelar a frente do cartão 1' }).click()
  await expect(page.locator('p.p-frente').first()).toHaveText("I'll figure it out.")
  await page.getByRole('button', { name: 'Adicionar 2 ao baralho' }).click()
  await expect(page.getByText(/2 notas criadas/)).toBeVisible()
})

test('com áudio na pergunta, o texto fica borrado até um toque', async ({ page, request }) => {
  const b = await (await request.post('http://localhost:8095/api/baralhos', { data: { nome: 'Ouvir primeiro' } })).json()
  const mp3 = Buffer.concat([Buffer.from('ID3\x04\x00\x00\x00\x00\x00\x00', 'latin1'), Buffer.alloc(256, 0xff)])
  const { nome } = await (await request.post('http://localhost:8095/api/audios', { multipart: { arquivo: { name: 'fala.mp3', mimeType: 'audio/mpeg', buffer: mp3 } } })).json()
  await request.post('http://localhost:8095/api/notas', { data: { baralho_id: b.id, tipo: 'basico', frente: 'Keep it down.', verso: 'Fale mais baixo.', audio: nome } })

  await page.goto(`/#/estudar/${b.id}`)
  const revelar = page.getByRole('button', { name: 'Revelar o texto' })
  await expect(revelar).toBeVisible()
  await expect(page.getByRole('button', { name: 'Ouvir de novo' })).toBeVisible()
  await expect(page.locator('.borrado span')).toHaveCSS('filter', /blur/)
  await revelar.click()
  await expect(revelar).toHaveCount(0)
  await expect(page.locator('.frente')).toHaveText('Keep it down.')
})
