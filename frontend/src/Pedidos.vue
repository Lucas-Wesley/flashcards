<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from './api.js'

// Os pedidos à IA e os links (e o que veio do Acervo): os que pedem atenção primeiro (na fila,
// montando, para revisar, com erro) e, embaixo, os últimos já decididos.
const geracoes = ref(null)
const erro = ref('')
let relogio

const ESTADOS = {
  na_fila: 'Na fila',
  rodando: 'Montando…',
  em_revisao: 'Para revisar',
  falhou: 'Falhou',
  concluida: 'Aprovado',
  rejeitada: 'Descartado',
  cancelada: 'Cancelado',
}
const ORIGENS = { ia: 'Com IA', link: 'Do link', acervo: 'Do Acervo', biblioteca: 'Da Biblioteca' }
const ABERTOS = ['na_fila', 'rodando', 'em_revisao', 'falhou']

const abertos = computed(() => (geracoes.value || []).filter((g) => ABERTOS.includes(g.estado)))
const anteriores = computed(() => (geracoes.value || []).filter((g) => !ABERTOS.includes(g.estado)))

async function carregar() {
  try {
    geracoes.value = await api('/api/geracoes')
    erro.value = ''
  } catch (e) {
    erro.value = e.status === 0 ? 'Sem conexão: os pedidos só abrem na rede de casa.' : e.message
  }
  clearTimeout(relogio)
  if (abertos.value.some((g) => g.estado === 'na_fila' || g.estado === 'rodando')) relogio = setTimeout(carregar, 4000)
}

const quando = (iso) => (iso ? new Date(iso).toLocaleString('pt-BR', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' }) : '')

onMounted(carregar)
onBeforeUnmount(() => clearTimeout(relogio))
</script>

<template>
  <div class="pagina">
    <h1>Pedidos</h1>
    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <template v-if="geracoes">
      <p v-if="!geracoes.length" class="vazio">
        Nenhum pedido ainda.<br />
        Os cartões pedidos <a href="#/gerar">à IA</a>, tirados <a href="#/link">de um link</a> ou mandados pelo Acervo e pela Biblioteca aparecem aqui para revisar.
      </p>

      <template v-if="abertos.length">
        <p class="rotulo primeiro">Pedem atenção</p>
        <ul class="lista">
          <li v-for="g in abertos" :key="g.id">
            <a class="pedido" :href="`#/geracao/${g.id}`">
              <span class="info">
                <span class="texto-pedido">{{ g.pedido }}</span>
                <span class="meta">{{ ORIGENS[g.origem] || g.origem }} · {{ quando(g.criado_em) }}</span>
              </span>
              <span class="chip" :class="{ andando: g.estado !== 'falhou', erro: g.estado === 'falhou' }">{{ ESTADOS[g.estado] }}</span>
            </a>
          </li>
        </ul>
      </template>

      <template v-if="anteriores.length">
        <p class="rotulo" :class="{ primeiro: !abertos.length }">Anteriores</p>
        <ul class="lista">
          <li v-for="g in anteriores" :key="g.id">
            <a class="pedido" :href="`#/geracao/${g.id}`">
              <span class="info">
                <span class="texto-pedido">{{ g.pedido }}</span>
                <span class="meta">{{ ORIGENS[g.origem] || g.origem }} · {{ quando(g.criado_em) }}</span>
              </span>
              <span class="chip">{{ ESTADOS[g.estado] || g.estado }}</span>
            </a>
          </li>
        </ul>
      </template>
    </template>
  </div>
</template>

<style scoped>
.rotulo.primeiro {
  margin-top: 0;
}

.pedido {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  padding: 12px 0;
  color: inherit;
  text-decoration: none;
}

.info {
  display: grid;
  flex: 1;
  gap: 3px;
  min-width: 0;
}

.texto-pedido {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pedido:hover .texto-pedido {
  color: var(--azul);
}

.meta {
  color: var(--texto-fraco);
  font-size: 0.8rem;
}

.chip {
  flex-shrink: 0;
}
</style>
