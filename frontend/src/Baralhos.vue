<script setup>
import { computed, inject, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from './api.js'

const recado = inject('recado')
const baralhos = ref(null)
const erro = ref('')
const nome = ref('')
const criando = ref(false)
const geracoes = ref([])
let relogio

// Pedidos à IA que ainda pedem atenção: na fila, rodando, para revisar ou com erro.
const ESTADOS = { na_fila: 'Na fila', rodando: 'Montando…', em_revisao: 'Para revisar', falhou: 'Falhou' }
const pedidos = computed(() => geracoes.value.filter((g) => ESTADOS[g.estado]))

async function carregarGeracoes() {
  try {
    geracoes.value = await api('/api/geracoes')
  } catch {
    // Sem rede, a seção some; os baralhos já mostram o aviso.
  }
  clearTimeout(relogio)
  if (geracoes.value.some((g) => g.estado === 'na_fila' || g.estado === 'rodando')) {
    relogio = setTimeout(() => {
      carregarGeracoes()
      carregar()
    }, 4000)
  }
}

async function carregar() {
  try {
    baralhos.value = await api('/api/baralhos')
    erro.value = ''
  } catch (e) {
    erro.value = e.status === 0 ? 'Sem conexão: os baralhos só abrem na rede de casa.' : e.message
  }
}

async function criar() {
  if (!nome.value.trim()) return
  criando.value = true
  try {
    const b = await api('/api/baralhos', { metodo: 'POST', corpo: { nome: nome.value } })
    nome.value = ''
    recado(`Baralho “${b.nome}” criado.`)
    await carregar()
  } catch (e) {
    recado(e.message, true)
  } finally {
    criando.value = false
  }
}

const total = (b) => b.contagem.novos + b.contagem.aprendendo + b.contagem.revisar
// O "Estudar" do topo junta todos os baralhos numa fila só.
const totalGeral = computed(() => (baralhos.value || []).reduce((soma, b) => soma + total(b), 0))

onMounted(() => {
  carregar()
  carregarGeracoes()
})
onBeforeUnmount(() => clearTimeout(relogio))
</script>

<template>
  <div class="pagina">
    <div class="topo">
      <h1>Baralhos</h1>
      <a v-if="totalGeral" class="botao principal estudar-tudo" href="#/estudar" title="Todos os baralhos, misturados"
        >Estudar <span class="total">{{ totalGeral }}</span></a
      >
    </div>

    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <template v-if="pedidos.length">
      <p class="rotulo primeiro">Pedidos à IA</p>
      <ul class="lista pedidos">
        <li v-for="g in pedidos" :key="g.id">
          <a class="pedido" :href="`#/geracao/${g.id}`">
            <span class="texto-pedido">{{ g.pedido }}</span>
            <span class="chip" :class="{ andando: g.estado !== 'falhou', erro: g.estado === 'falhou' }">{{ ESTADOS[g.estado] }}</span>
          </a>
        </li>
      </ul>
      <p class="rotulo">Seus baralhos</p>
    </template>

    <template v-if="baralhos">
      <ul v-if="baralhos.length" class="lista">
        <li v-for="b in baralhos" :key="b.id">
          <a class="baralho" :href="`#/baralho/${b.id}`">
            <span class="info">
              <span class="nome">{{ b.nome }}</span>
              <span class="meta">
                {{ b.cartoes === 1 ? '1 cartão' : `${b.cartoes} cartões` }}<template v-if="b.feitos_hoje"> · {{ b.feitos_hoje }} {{ b.feitos_hoje === 1 ? 'revisão' : 'revisões' }} hoje</template>
              </span>
            </span>
            <span class="numeros" :aria-label="`${b.contagem.novos} novos, ${b.contagem.aprendendo} aprendendo, ${b.contagem.revisar} para revisar`">
              <span class="numero novos" :class="{ zero: !b.contagem.novos }" title="Novos">{{ b.contagem.novos }}</span>
              <span class="numero aprendendo" :class="{ zero: !b.contagem.aprendendo }" title="Aprendendo">{{ b.contagem.aprendendo }}</span>
              <span class="numero revisar" :class="{ zero: !b.contagem.revisar }" title="Para revisar">{{ b.contagem.revisar }}</span>
            </span>
          </a>
          <a v-if="total(b)" class="botao estudar" :href="`#/estudar/${b.id}`">Estudar</a>
        </li>
      </ul>
      <p v-else class="vazio">Nenhum baralho ainda.<br />Crie o primeiro abaixo: um por assunto, como “Inglês” ou “História”.</p>

      <p v-if="baralhos.length" class="legenda">
        <span class="numero novos">Novos</span> · <span class="numero aprendendo">Aprendendo</span> · <span class="numero revisar">Para revisar</span>
      </p>

      <p class="rotulo">Novo baralho</p>
      <form class="novo" @submit.prevent="criar">
        <input v-model="nome" class="campo" placeholder="Nome do baralho" aria-label="Nome do novo baralho" />
        <button class="botao" :disabled="criando || !nome.trim()">Criar vazio</button>
      </form>
      <p class="gerar">ou <a href="#/gerar">peça à IA um baralho pronto</a> sobre um assunto, para revisar antes de estudar.</p>
    </template>
  </div>
</template>

<style scoped>
.topo {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 20px;
}

.topo h1 {
  margin-bottom: 0;
}

.estudar-tudo .total {
  font-variant-numeric: tabular-nums;
  opacity: 0.75;
}

.lista li {
  display: flex;
  align-items: center;
  gap: 12px;
}

.baralho {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 16px;
  min-width: 0;
  padding: 16px 0;
  color: inherit;
  text-decoration: none;
}

.info {
  display: grid;
  flex: 1;
  gap: 3px;
  min-width: 0;
}

.nome {
  overflow: hidden;
  font-family: var(--leitura);
  font-size: 1.19rem;
  font-weight: 550;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color 0.15s ease;
}

.baralho:hover .nome {
  color: var(--azul);
}

.meta {
  color: var(--texto-fraco);
  font-size: 0.8rem;
}

.numeros {
  display: flex;
  gap: 14px;
  font-variant-numeric: tabular-nums;
}

.numero {
  min-width: 2ch;
  font-size: 0.92rem;
  font-weight: 600;
  text-align: right;
}

/* Novos em azul (informação); revisões vencidas em terracota (pedem atenção). */
.numero.novos {
  color: var(--azul);
}

.numero.aprendendo {
  color: var(--texto-suave);
}

.numero.revisar {
  color: var(--terracota);
}

.numero.zero {
  color: var(--texto-fraco);
  font-weight: 400;
  opacity: 0.6;
}

.legenda {
  margin: 10px 0 0;
  color: var(--texto-fraco);
  font-size: 0.75rem;
  text-align: right;
}

.legenda .numero {
  font-size: inherit;
  font-weight: 500;
}

.rotulo.primeiro {
  margin-top: 0;
}

.pedidos {
  margin-bottom: 8px;
}

.pedido {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 0;
  color: inherit;
  text-decoration: none;
}

.texto-pedido {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pedido:hover .texto-pedido {
  color: var(--azul);
}

.gerar {
  margin: 10px 0 0;
  color: var(--texto-suave);
  font-size: 0.85rem;
}

.novo {
  display: flex;
  gap: 8px;
}

.novo .botao {
  min-height: 38px;
}

.aviso {
  margin-bottom: 16px;
}

@media (max-width: 820px) {
  .estudar {
    display: none;
  }
}
</style>
