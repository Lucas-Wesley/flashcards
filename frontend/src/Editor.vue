<script setup>
import { computed, inject, nextTick, onMounted, ref } from 'vue'
import { APRENDENDO, NOVO, REAPRENDENDO, api, lembrar } from './api.js'
import { vencimento } from './tempo.js'

// Adicionar (sem id) ou editar (com id) uma nota. Ao adicionar, o baralho, o tipo e as tags ficam
// para a próxima, e o foco volta para a frente: dá para cadastrar uma lista em sequência.
const props = defineProps({
  id: { type: String, default: '' },
  baralho: { type: String, default: '' },
})
const recado = inject('recado')

const baralhos = ref([])
const nota = ref(null)
const dados = ref({ baralho_id: '', tipo: 'basico', frente: '', verso: '', tags: '' })
const erro = ref('')
const salvando = ref(false)
const campoFrente = ref(null)

const TIPOS = [
  { valor: 'basico', nome: 'Frente e verso' },
  { valor: 'invertido', nome: 'Nos dois sentidos' },
]

const editando = computed(() => !!props.id)
const nomeDoBaralho = (id) => baralhos.value.find((b) => b.id === id)?.nome || ''

function situacao(c) {
  if (c.suspenso) return 'Suspenso'
  if (c.estado === NOVO) return 'Novo'
  if (c.estado === APRENDENDO || c.estado === REAPRENDENDO) return 'Aprendendo'
  return `Revisão ${vencimento(c.vence_em)}`
}

async function carregar() {
  try {
    baralhos.value = await api('/api/baralhos')
    if (editando.value) {
      nota.value = await api(`/api/notas/${props.id}`)
      const { baralho_id, tipo, frente, verso, tags } = nota.value
      dados.value = { baralho_id, tipo, frente, verso, tags }
    } else {
      const ultimo = props.baralho || lembrar('baralho')
      dados.value.baralho_id = baralhos.value.some((b) => b.id === ultimo) ? ultimo : baralhos.value[0]?.id || ''
      dados.value.tipo = lembrar('tipo') || 'basico'
      await nextTick()
      campoFrente.value?.focus()
    }
  } catch (e) {
    erro.value = e.status === 0 ? 'Sem conexão: os cartões só abrem na rede de casa.' : e.message
  }
}

async function salvar() {
  if (salvando.value) return
  salvando.value = true
  try {
    if (editando.value) {
      nota.value = await api(`/api/notas/${props.id}`, { metodo: 'PUT', corpo: dados.value })
      recado('Cartão salvo.')
    } else {
      const n = await api('/api/notas', { metodo: 'POST', corpo: dados.value })
      lembrar('baralho', dados.value.baralho_id)
      lembrar('tipo', dados.value.tipo)
      recado(n.cartoes.length === 2 ? 'Adicionado: dois cartões, um em cada sentido.' : 'Cartão adicionado.')
      dados.value.frente = ''
      dados.value.verso = ''
      campoFrente.value?.focus()
    }
  } catch (e) {
    recado(e.message, true)
  } finally {
    salvando.value = false
  }
}

async function suspender(c) {
  try {
    await api(`/api/cartoes/${c.id}`, { metodo: 'PATCH', corpo: { suspenso: !c.suspenso } })
    c.suspenso = !c.suspenso
    recado(c.suspenso ? 'Cartão suspenso: sai da fila até ser reativado.' : 'Cartão reativado.')
  } catch (e) {
    recado(e.message, true)
  }
}

async function apagar() {
  if (!confirm('Apagar esta nota, os cartões dela e o histórico de revisões? Não dá para desfazer.')) return
  try {
    await api(`/api/notas/${props.id}`, { metodo: 'DELETE' })
    recado('Nota apagada.')
    history.back()
  } catch (e) {
    recado(e.message, true)
  }
}

function aoTeclar(ev) {
  if ((ev.ctrlKey || ev.metaKey) && ev.key === 'Enter') salvar()
}

onMounted(carregar)
</script>

<template>
  <div class="pagina">
    <h1>{{ editando ? 'Editar cartão' : 'Adicionar' }}</h1>
    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <p v-else-if="!editando && !baralhos.length" class="vazio">
      Crie um baralho antes de adicionar cartões.<br />
      <a href="#/baralhos">Ir para Baralhos</a>
    </p>

    <form v-else class="formulario" @submit.prevent="salvar" @keydown="aoTeclar">
      <div class="dupla">
        <label>
          <span class="rotulo-campo">Baralho</span>
          <select v-model="dados.baralho_id" class="campo">
            <option v-for="b in baralhos" :key="b.id" :value="b.id">{{ b.nome }}</option>
          </select>
        </label>
        <div class="tipo">
          <span class="rotulo-campo">Tipo</span>
          <div class="pilulas" role="radiogroup" aria-label="Tipo de cartão">
            <button
              v-for="t in TIPOS"
              :key="t.valor"
              type="button"
              role="radio"
              class="pilula"
              :class="{ ativa: dados.tipo === t.valor }"
              :aria-checked="dados.tipo === t.valor"
              @click="dados.tipo = t.valor"
            >
              {{ t.nome }}
            </button>
          </div>
        </div>
      </div>
      <p class="ajuda">
        {{
          dados.tipo === 'invertido'
            ? 'Cria dois cartões: frente → verso e verso → frente. Bom para vocabulário.'
            : 'Um cartão: a frente é a pergunta, o verso é a resposta.'
        }}
      </p>

      <label>
        <span class="rotulo-campo">Frente</span>
        <textarea ref="campoFrente" v-model="dados.frente" class="campo texto-cartao" rows="3" aria-label="Frente" />
      </label>
      <label>
        <span class="rotulo-campo">Verso</span>
        <textarea v-model="dados.verso" class="campo texto-cartao" rows="3" aria-label="Verso" />
      </label>
      <label>
        <span class="rotulo-campo">Tags</span>
        <input v-model="dados.tags" class="campo" placeholder="separadas por espaço" aria-label="Tags" />
      </label>

      <div class="linha-acoes">
        <button class="botao principal" :disabled="salvando">{{ editando ? 'Salvar' : 'Adicionar' }}</button>
        <span class="atalho suave">Ctrl + Enter</span>
        <button v-if="editando" type="button" class="botao perigo apagar" @click="apagar">Apagar nota</button>
      </div>
    </form>

    <template v-if="nota">
      <p class="rotulo">{{ nota.cartoes.length === 1 ? 'Cartão' : 'Cartões' }} desta nota</p>
      <ul class="lista">
        <li v-for="c in nota.cartoes" :key="c.id" class="cartao-da-nota">
          <div>
            <p class="sentido">{{ c.sentido === 0 ? 'Frente → verso' : 'Verso → frente' }}</p>
            <p class="meta">
              {{ situacao(c) }}<template v-if="c.lapsos"> · {{ c.lapsos }} {{ c.lapsos === 1 ? 'erro' : 'erros' }}</template> · em {{ nomeDoBaralho(nota.baralho_id) }}
            </p>
          </div>
          <button class="botao" @click="suspender(c)">{{ c.suspenso ? 'Reativar' : 'Suspender' }}</button>
        </li>
      </ul>
    </template>
  </div>
</template>

<style scoped>
.formulario {
  display: grid;
  gap: 16px;
}

.formulario label,
.tipo {
  display: grid;
  gap: 6px;
  align-content: start;
}

.dupla {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.rotulo-campo {
  color: var(--texto-suave);
  font-size: 0.82rem;
  font-weight: 500;
}

.texto-cartao {
  font-family: var(--leitura);
  font-size: 1.05rem;
  line-height: 1.5;
  resize: vertical;
}

select.campo {
  appearance: auto;
}

.pilulas {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
  min-height: 38px;
}

.ajuda {
  margin: -8px 0 0;
  color: var(--texto-fraco);
  font-size: 0.8rem;
}

.atalho {
  font-size: 0.75rem;
}

.apagar {
  margin-left: auto;
}

.cartao-da-nota {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 0;
}

.cartao-da-nota p {
  margin: 0;
}

.sentido {
  font-weight: 500;
}

.meta {
  margin-top: 3px !important;
  color: var(--texto-fraco);
  font-size: 0.8rem;
}

.aviso {
  margin-bottom: 16px;
}

@media (max-width: 820px) {
  .dupla {
    grid-template-columns: 1fr;
  }

  .atalho {
    display: none;
  }
}
</style>
