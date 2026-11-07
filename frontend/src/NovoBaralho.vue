<script setup>
import { inject, ref } from 'vue'
import AbasAdicionar from './AbasAdicionar.vue'
import { api } from './api.js'

// Um baralho vazio, para escrever, importar ou mandar cartões depois. Um pronto, a IA monta (Com IA).
const recado = inject('recado')
const nome = ref('')
const criando = ref(false)

async function criar() {
  if (!nome.value.trim()) return
  criando.value = true
  try {
    const b = await api('/api/baralhos', { metodo: 'POST', corpo: { nome: nome.value } })
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
    <p class="suave explicacao">Um baralho por assunto, como “Inglês” ou “História”. Ele começa vazio; os cartões entram escrevendo, importando, de um link ou pela IA.</p>
    <form class="novo" @submit.prevent="criar">
      <input v-model="nome" class="campo" placeholder="Nome do baralho" aria-label="Nome do novo baralho" />
      <button class="botao principal" :disabled="criando || !nome.trim()">Criar baralho</button>
    </form>
    <p class="gerar suave">Quer um baralho já com cartões? <a href="#/gerar">Peça à IA</a>, para revisar antes de estudar.</p>
  </div>
</template>

<style scoped>
.explicacao {
  margin: 0 0 20px;
  font-size: 0.9rem;
  line-height: 1.55;
}

.novo {
  display: flex;
  gap: 8px;
}

.novo .botao {
  flex-shrink: 0;
  min-height: 38px;
}

.gerar {
  margin: 12px 0 0;
  font-size: 0.85rem;
}
</style>
