<script setup>
import { computed, onMounted, ref } from 'vue'
import Icone from './Icone.vue'
import Info from './Info.vue'
import Numeros from './Numeros.vue'
import { api } from './api.js'
import { agrupar } from './temas.js'

// Estudar (F38): escolher o tema (todos os baralhos dele, misturados) ou o baralho sem tema. Tocar na
// linha segue a fila do dia; o botão ao lado abre o aleatório, que não mexe no agendamento.
const baralhos = ref(null)
const erro = ref('')

const grupos = computed(() => agrupar(baralhos.value || []))
const tudo = computed(() => {
  const c = { novos: 0, aprendendo: 0, revisar: 0 }
  for (const g of grupos.value) for (const k in c) c[k] += g.contagem[k]
  return { nome: 'Tudo', escopo: 'tudo', contagem: c }
})

onMounted(async () => {
  try {
    baralhos.value = await api('/api/baralhos')
  } catch (e) {
    erro.value = e.status === 0 ? 'Sem conexão: o estudo só funciona na rede de casa.' : e.message
  }
})
</script>

<template>
  <div class="pagina">
    <a class="voltar" href="#/baralhos">Baralhos</a>
    <h1>
      Estudar
      <Info>Toque num tema para seguir a repetição espaçada de hoje, com os baralhos dele misturados. O <Icone nome="aleatorio" class="mini" /> sorteia cartões a qualquer hora, sem mudar o agendamento.</Info>
    </h1>
    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <ul v-if="baralhos?.length" class="lista">
      <li v-for="g in grupos.length > 1 ? [...grupos, tudo] : grupos" :key="g.escopo" :class="{ tudo: g.escopo === 'tudo' }">
        <a class="linha" :href="`#/estudar/${g.escopo}`">
          <span class="nome">{{ g.nome }}</span>
          <span v-if="g.tema" class="qtos">{{ g.baralhos.length }} {{ g.baralhos.length === 1 ? 'baralho' : 'baralhos' }}</span>
          <Numeros :contagem="g.contagem" />
        </a>
        <a class="botao icone" :href="`#/aleatorio/${g.escopo}`" :aria-label="`${g.nome}: aleatório`" title="Aleatório"><Icone nome="aleatorio" /></a>
      </li>
    </ul>
    <p v-else-if="baralhos" class="vazio">Nenhum baralho ainda. <a href="#/novo-baralho">Crie o primeiro</a>.</p>
  </div>
</template>

<style scoped>
.voltar {
  display: inline-block;
  margin-bottom: 10px;
  color: var(--texto-suave);
  font-size: 0.85rem;
  text-decoration: none;
}

.voltar::before {
  content: '‹ ';
}

h1 {
  display: flex;
  align-items: center;
  gap: 6px;
}

.mini {
  width: 15px;
  height: 15px;
  vertical-align: -3px;
}

.lista li {
  display: flex;
  align-items: center;
  gap: 4px;
}

.linha {
  display: flex;
  flex: 1;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
  padding: 16px 0;
  color: inherit;
  text-decoration: none;
}

.nome {
  overflow: hidden;
  font-size: 1.06rem;
  font-weight: 650;
  letter-spacing: -0.012em;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.linha:hover .nome {
  color: var(--marca);
}

.qtos {
  flex: none;
  color: var(--texto-fraco);
  font-size: 0.78rem;
}

.linha :deep(.numeros) {
  margin-left: auto;
}

.tudo .nome {
  color: var(--texto-suave);
  font-weight: 550;
}
</style>
