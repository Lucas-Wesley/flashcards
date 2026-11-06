import { expect, test } from '@playwright/test'

// Um fluxo só, na ordem em que o Lucas usa: baralho → cartões → estudo → editar → Ajustes.
test.describe.configure({ mode: 'serial' })

test('cria o baralho, adiciona cartões e importa uma lista', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Nenhum baralho ainda.')).toBeVisible()
  await page.getByLabel('Nome do novo baralho').fill('Inglês')
  await page.getByRole('button', { name: 'Criar' }).click()
  await page.getByRole('link', { name: /Inglês/ }).first().click()
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

  await page.getByLabel('Verso').fill('cat, kitty')
  await page.keyboard.press('Control+Enter')
  await expect(page.getByText('Cartão salvo.')).toBeVisible()
  await page.getByRole('button', { name: 'Suspender' }).click()
  await expect(page.getByText(/Suspenso/)).toBeVisible()
  await page.getByRole('button', { name: 'Reativar' }).click()
  await expect(page.getByText(/Revisão em/)).toBeVisible()
})

test('Ajustes recebe sugestões de melhoria', async ({ page }) => {
  await page.goto('/#/cartoes')
  await page.getByRole('button', { name: 'Ajustes' }).click()
  await page.getByLabel('Sugestão de melhoria').fill('Áudio de pronúncia')
  await page.getByRole('button', { name: 'Enviar sugestão' }).click()
  await expect(page.locator('.sugestao', { hasText: 'Áudio de pronúncia' })).toBeVisible()
})
