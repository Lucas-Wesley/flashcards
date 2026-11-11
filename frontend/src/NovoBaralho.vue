<script setup>
import { inject, onMounted, ref } from 'vue'
import AbasAdicionar from './AbasAdicionar.vue'
import { api } from './api.js'

// Um baralho vazio, para escrever, importar ou mandar cartões depois. Um pronto, a IA monta (Com IA).
const recado = inject('recado')
const nome = ref('')
const tema = ref('')
const temas = ref([])
const criando = ref(false)

onMounted(() =>
  api('/api/baralhos')
    .then((lista) => (temas.value = [...new Set(lista.map((b) => b.tema).filter(Boolean))].sort()))
    .catch(() => {}),
)

async function criar() {
  if (!nome.value.trim()) return
  criando.value = true
  try {
    const b = await api('/api/baralhos', { metodo: 'POST', corpo: { nome: nome.value, tema: tema.value } })
    recado(`Baralho “${b.nome}” criado.`)
    location.hash = `#/baralho/${b.id}`
  } catch (e) {
    recado(e.message, true)
  } finally {
    criando.value = false
  }
}
</script>

<template>
  <div class="pagina">
    <h1>Adicionar</h1>
    <AbasAdicionar atual="novo-baralho" />
    <form class="novo" @submit.prevent="criar">
      <input v-model="nome" class="campo" placeholder="Nome do baralho" aria-label="Nome do novo baralho" />
      <input v-model="tema" class="campo" list="temas-do-novo" placeholder="Tema (opcional)" aria-label="Tema do novo baralho" />
      <datalist id="temas-do-novo">
        <option v-for="t in temas" :key="t" :value="t" />
      </datalist>
      <button class="botao principal" :disabled="criando || !nome.trim()">Criar baralho</button>
    </form>
  </div>
</template>

<style scoped>
.novo {
  display: grid;
  gap: 8px;
  max-width: 420px;
}

.novo .botao {
  justify-self: start;
  min-height: 38px;
}
</style>
