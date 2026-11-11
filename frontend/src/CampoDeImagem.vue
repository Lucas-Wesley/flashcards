<script setup>
import { inject, ref } from 'vue'
import BuscaDeImagem from './BuscaDeImagem.vue'
import { api } from './api.js'

// A imagem de um lado do cartão no editor: buscar na Wikipédia ou enviar do aparelho. A escolhida vem
// para o servidor na hora; a nota guarda só o nome do arquivo.
const imagem = defineModel({ type: String, default: '' })
defineProps({
  rotulo: { type: String, required: true },
  dica: { type: String, default: '' },
  termo: { type: String, default: '' },
  contexto: { type: String, default: '' },
})
const recado = inject('recado')
const buscando = ref(false)
const trazendo = ref(false)

async function escolher(achada) {
  trazendo.value = true
  try {
    imagem.value = (await api('/api/imagens/baixar', { metodo: 'POST', corpo: { url: achada.url }, tempo: 30000 })).nome
    buscando.value = false
  } catch (e) {
    recado(e.message, true)
  } finally {
    trazendo.value = false
  }
}

async function enviar(ev) {
  const arquivo = ev.target.files[0]
  ev.target.value = ''
  if (!arquivo) return
  trazendo.value = true
  const corpo = new FormData()
  corpo.append('arquivo', arquivo)
  try {
    const resposta = await fetch('/api/imagens', { method: 'POST', body: corpo })
    const r = await resposta.json()
    if (!resposta.ok) throw new Error(r.erro || `Erro ${resposta.status}`)
    imagem.value = r.nome
  } catch (e) {
    recado(e.message === 'Failed to fetch' ? 'Sem conexão com o servidor.' : e.message, true)
  } finally {
    trazendo.value = false
  }
}

// Ao adicionar em sequência, o editor limpa a imagem e fecha a busca.
defineExpose({ fechar: () => (buscando.value = false) })
</script>

<template>
  <div class="imagem">
    <span class="rotulo-campo">{{ rotulo }} <span v-if="dica" class="suave">· {{ dica }}</span></span>
    <div v-if="imagem" class="com-imagem">
      <img :src="`/api/imagens/${imagem}`" :alt="rotulo" />
      <button type="button" class="botao" @click="imagem = ''">Remover</button>
    </div>
    <div v-else-if="!buscando" class="linha-acoes">
      <button type="button" class="botao" :disabled="trazendo" @click="buscando = true">Buscar na Wikipédia</button>
      <label class="botao">
        Enviar do aparelho
        <input type="file" accept="image/jpeg,image/png,image/webp,image/gif" hidden @change="enviar" />
      </label>
      <span v-if="trazendo" class="suave">Trazendo a imagem…</span>
    </div>
    <BuscaDeImagem v-if="buscando && !imagem" :termo="termo" :contexto="contexto" @escolher="escolher" @fechar="buscando = false" />
  </div>
</template>

<style scoped>
.imagem {
  display: grid;
  gap: 6px;
}

.rotulo-campo {
  color: var(--texto-suave);
  font-size: 0.82rem;
  font-weight: 500;
}

.com-imagem {
  display: flex;
  align-items: flex-end;
  gap: 12px;
}

.com-imagem img {
  max-width: 160px;
  max-height: 160px;
  border-radius: var(--raio-p);
  object-fit: cover;
}
</style>
