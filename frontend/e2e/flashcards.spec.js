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

  await page.getByRole('link', { name: 'Adicionar cartões' }).click()
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
  await page.getByRole('link', { name: 'Estudar agora' }).click()

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
  await expect(page.getByText('3 revisões neste baralho hoje.')).toBeVisible()
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
  await expect(page.getByLabel('Frente do cartão 1')).toHaveValue('I ran into an old friend yesterday.')
  await expect(page.getByLabel('Áudio do cartão 1')).toBeAttached()
  // O baralho pode ser trocado na revisão; fica o escolhido no pedido.
  await expect(page.getByLabel('Baralho', { exact: true }).locator('option:checked')).toHaveText('Inglês')
  await expect(page.getByLabel('Nome do baralho novo')).toHaveCount(0)
  // Até 3 frases prontas não passam pela IA: não há o que refazer.
  await expect(page.getByRole('button', { name: 'Refazer' })).toHaveCount(0)
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
  await page.getByLabel('Quantidade').fill('10')
  await page.getByRole('button', { name: 'Pedir à IA' }).click()

  await expect(page.getByText('Proposta para revisar')).toBeVisible({ timeout: 15000 })
  // Em Pedidos, a proposta espera a revisão.
  await page.getByRole('link', { name: 'Pedidos' }).click()
  await expect(page.getByRole('heading', { name: 'Pedidos' })).toBeVisible()
  await page.getByRole('link', { name: /presidentes do Brasil no século 21.*Para revisar/ }).click()
  await expect(page.getByText('2 cartões propostos · 2 marcados')).toBeVisible()
  await expect(page.getByLabel('Nome do baralho novo')).toHaveValue('Presidentes do Brasil no século 21')
  await page.getByLabel('Incluir o cartão 2').uncheck()
  await page.getByLabel('Verso do cartão 1').fill('Lula (2003–2010)')
  await page.getByRole('button', { name: 'Criar baralho com 1 cartão' }).click()

  await expect(page.getByRole('heading', { name: 'Presidentes do Brasil no século 21' })).toBeVisible()
  await page.getByRole('link', { name: 'Estudar agora' }).click()
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
  await expect(page.getByText('Atualiza o cartão que já existe')).toHaveCount(1)
  await page.getByRole('button', { name: 'Adicionar 2 ao baralho' }).click()
  await expect(page.getByText('1 nota criada, 1 atualizada.')).toBeVisible()
})

test('estuda todos os baralhos misturados pelo botão do topo', async ({ page }) => {
  await page.goto('/')
  await page.locator('.estudar-tudo').click()
  const baralhos = new Set()
  for (let i = 0; i < 12 && !(await page.getByText('Pronto por hoje.').isVisible()); i++) {
    await expect(page.locator('.frente')).toBeVisible()
    await expect(page.locator('.modo')).toContainText('Todos')
    baralhos.add((await page.locator('.barra .baralho').textContent()).replace('Todos · ', ''))
    await page.keyboard.press('Space')
    await page.keyboard.press('4')
    await page.waitForTimeout(150)
  }
  await expect(page.getByText('Pronto por hoje.')).toBeVisible()
  await expect(page.getByText(/revisões hoje, somando os baralhos/)).toBeVisible()
  expect([...baralhos].sort()).toEqual(['Inglês', 'Presidentes do Brasil no século 21'])
  // Sem nada para hoje, o botão continua lá, só que discreto.
  await page.goto('/')
  await expect(page.locator('.estudar-tudo')).toHaveText(/Estudar\s*0/)
  await expect(page.locator('.estudar-tudo')).not.toHaveClass(/principal/)
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
