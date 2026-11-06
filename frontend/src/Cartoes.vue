<script setup>
import { onMounted, ref, watch } from 'vue'
import { APRENDENDO, NOVO, REAPRENDENDO, api } from './api.js'
import { vencimento } from './tempo.js'

// Todos os cartões, com busca e filtro por baralho. Cada linha é uma nota; tocar abre o editor.
const props = defineProps({ baralho: { type: String, default: '' } })

const baralhos = ref([])
const notas = ref(null)
const filtro = ref(props.baralho)
const busca = ref('')
const erro = ref('')

async function carregar() {
  const q = new URLSearchParams({ q: busca.value, baralho: filtro.value })
  try {
    notas.value = await api(`/api/notas?${q}`)
    erro.value = ''
  } catch (e) {
    erro.value = e.status === 0 ? 'Sem conexão: os cartões só abrem na rede de casa.' : e.message
  }
}

let atraso
watch(busca, () => {
  clearTimeout(atraso)
  atraso = setTimeout(carregar, 250)
})
watch(filtro, carregar)

function chip(c) {
  if (c.suspenso) return { texto: 'Suspenso', classe: 'erro' }
  if (c.estado === NOVO) return { texto: 'Novo', classe: 'ok' }
  if (c.estado === APRENDENDO || c.estado === REAPRENDENDO) return { texto: 'Aprendendo', classe: '' }
  return { texto: vencimento(c.vence_em), classe: '' }
}

const nomeDoBaralho = (id) => baralhos.value.find((b) => b.id === id)?.nome || ''

onMounted(async () => {
  carregar()
  try {
    baralhos.value = await api('/api/baralhos')
  } catch {
    // Sem os nomes, a lista continua funcionando.
  }
})
</script>

<template>
  <div class="pagina">
    <h1>Cartões</h1>

    <input v-model="busca" class="campo busca" type="search" placeholder="Buscar na frente, no verso ou nas tags" aria-label="Buscar cartões" />
    <div v-if="baralhos.length > 1" class="filtros" role="radiogroup" aria-label="Baralho">
      <button class="pilula" :class="{ ativa: !filtro }" role="radio" :aria-checked="!filtro" @click="filtro = ''">Todos</button>
      <button
        v-for="b in baralhos"
        :key="b.id"
        class="pilula"
        :class="{ ativa: filtro === b.id }"
        role="radio"
        :aria-checked="filtro === b.id"
        @click="filtro = b.id"
      >
        {{ b.nome }}
      </button>
    </div>

    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <template v-if="notas">
      <p class="contagem">{{ notas.length === 1 ? '1 nota' : `${notas.length} notas` }}{{ notas.length >= 300 ? ' (as mais recentes)' : '' }}</p>
      <ul v-if="notas.length" class="lista">
        <li v-for="n in notas" :key="n.id">
          <a class="nota" :href="`#/nota/${n.id}`">
            <span class="frente">{{ n.frente }}</span>
            <span class="verso">{{ n.verso }}</span>
            <span class="meta">
              <span v-for="c in n.cartoes" :key="c.id" class="chip" :class="chip(c).classe">{{ n.cartoes.length > 1 ? (c.sentido ? '← ' : '→ ') : '' }}{{ chip(c).texto }}</span>
              <span v-if="!filtro">{{ nomeDoBaralho(n.baralho_id) }}</span>
              <span v-if="n.tags" class="tags">{{ n.tags }}</span>
            </span>
          </a>
        </li>
      </ul>
      <p v-else class="vazio">{{ busca ? 'Nada encontrado.' : 'Nenhum cartão ainda.' }}<br /><a href="#/adicionar">Adicionar cartões</a></p>
    </template>
  </div>
</template>

<style scoped>
.busca {
  margin-bottom: 12px;
}

.filtros {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 8px;
}

.contagem {
  margin: 16px 0 8px;
  color: var(--texto-fraco);
  font-size: 0.75rem;
}

.nota {
  display: grid;
  gap: 3px;
  padding: 14px 0;
  color: inherit;
  text-decoration: none;
}

.frente,
.verso {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.frente {
  font-weight: 600;
  transition: color 0.15s ease;
}

.nota:hover .frente {
  color: var(--azul);
}

.verso {
  color: var(--texto-suave);
  font-size: 0.9rem;
}

.meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  color: var(--texto-fraco);
  font-size: 0.78rem;
}

.aviso {
  margin: 12px 0;
}
</style>
