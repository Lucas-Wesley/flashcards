<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import Icone from './Icone.vue'
import Info from './Info.vue'
import { api } from './api.js'

// A página de um baralho: o que há para hoje, o botão de estudar e as opções.
const props = defineProps({ id: { type: String, required: true } })
const recado = inject('recado')

const b = ref(null)
const erro = ref('')
const opcoes = ref({ nome: '', tema: '', novos: 20, revisoes: 200, retencao: 90 })
const temas = ref([]) // os temas que já existem, para sugerir
const salvando = ref(false)
// Os pedidos à IA que criaram ou alimentaram este baralho: dá para abrir, editar o pedido e refazer.
const pedidos = ref([])

const paraHoje = computed(() => (b.value ? b.value.contagem.novos + b.value.contagem.aprendendo + b.value.contagem.revisar : 0))

function preencher(dados) {
  b.value = dados
  opcoes.value = { nome: dados.nome, tema: dados.tema, novos: dados.novos_por_dia, revisoes: dados.revisoes_por_dia, retencao: Math.round(dados.retencao * 100) }
}

async function carregar() {
  try {
    preencher(await api(`/api/baralhos/${props.id}`))
  } catch (e) {
    erro.value = e.status === 0 ? 'Sem conexão: o baralho só abre na rede de casa.' : e.message
  }
}

async function salvar() {
  salvando.value = true
  try {
    preencher(
      await api(`/api/baralhos/${props.id}`, {
        metodo: 'PATCH',
        corpo: {
          nome: opcoes.value.nome,
          tema: opcoes.value.tema,
          novos_por_dia: Number(opcoes.value.novos),
          revisoes_por_dia: Number(opcoes.value.revisoes),
          retencao: Number(opcoes.value.retencao) / 100,
        },
      }),
    )
    recado('Opções salvas.')
  } catch (e) {
    recado(e.message, true)
  } finally {
    salvando.value = false
  }
}

async function apagar() {
  const n = b.value.cartoes
  if (!confirm(`Apagar “${b.value.nome}”${n ? ` e ${n === 1 ? 'o cartão' : `os ${n} cartões`} dele, com o histórico` : ''}? Não dá para desfazer.`)) return
  try {
    await api(`/api/baralhos/${props.id}`, { metodo: 'DELETE' })
    recado('Baralho apagado.')
    location.hash = '#/baralhos'
  } catch (e) {
    recado(e.message, true)
  }
}

onMounted(async () => {
  carregar()
  api('/api/baralhos')
    .then((lista) => (temas.value = [...new Set(lista.map((x) => x.tema).filter(Boolean))].sort()))
    .catch(() => {})
  try {
    pedidos.value = await api(`/api/geracoes?baralho=${props.id}`)
  } catch {
    // Sem a lista, a seção não aparece.
  }
})
</script>

<template>
  <div class="pagina">
    <a class="voltar" href="#/baralhos">Baralhos</a>
    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <template v-if="b">
      <p v-if="b.tema" class="tema">{{ b.tema }}</p>
      <h1 class="titulo">{{ b.nome }}</h1>

      <div class="hoje">
        <div class="cifra">
          <span class="valor novos">{{ b.contagem.novos }}</span>
          <span class="nome">Novos</span>
        </div>
        <div class="cifra">
          <span class="valor">{{ b.contagem.aprendendo }}</span>
          <span class="nome">Aprendendo</span>
        </div>
        <div class="cifra">
          <span class="valor revisar">{{ b.contagem.revisar }}</span>
          <span class="nome">Para revisar</span>
        </div>
      </div>

      <div class="linha-acoes principal-acoes">
        <a class="botao" :class="{ principal: paraHoje }" :href="`#/estudar/${b.id}`">Estudar</a>
        <a v-if="b.cartoes" class="botao" :href="`#/aleatorio/${b.id}`"><Icone nome="aleatorio" /> Aleatório</a>
      </div>
      <div class="linha-acoes">
        <a class="botao" :href="`#/adicionar/${b.id}`">Adicionar</a>
        <a class="botao" :href="`#/gerar/${b.id}`">Gerar com IA</a>
        <a class="botao" :href="`#/importar/${b.id}`">Importar</a>
        <a v-if="b.cartoes" class="botao" :href="`#/cartoes/${b.id}`">{{ b.cartoes }} {{ b.cartoes === 1 ? 'cartão' : 'cartões' }}</a>
      </div>

      <template v-if="pedidos.length">
        <p class="rotulo">Pedidos à IA</p>
        <ul class="lista pedidos">
          <li v-for="p in pedidos" :key="p.id">
            <a class="pedido" :href="`#/geracao/${p.id}`">“{{ p.pedido }}”</a>
          </li>
        </ul>
      </template>

      <p class="rotulo">Opções</p>
      <form class="opcoes" @submit.prevent="salvar">
        <div class="dupla">
          <label>
            <span class="rotulo-campo">Nome</span>
            <input v-model="opcoes.nome" class="campo" />
          </label>
          <label>
            <span class="rotulo-campo">Tema <Info>O assunto que junta baralhos, como “Inglês” para phrasal verbs, pronúncia e verbos irregulares. No Estudar, o tema mistura os cartões de todos os baralhos dele.</Info></span>
            <input v-model="opcoes.tema" class="campo" list="temas-existentes" placeholder="Sem tema" />
            <datalist id="temas-existentes">
              <option v-for="t in temas" :key="t" :value="t" />
            </datalist>
          </label>
        </div>
        <div class="dupla">
          <label>
            <span class="rotulo-campo">Novos por dia</span>
            <input v-model="opcoes.novos" class="campo" type="number" min="0" max="9999" />
          </label>
          <label>
            <span class="rotulo-campo">Revisões por dia</span>
            <input v-model="opcoes.revisoes" class="campo" type="number" min="0" max="9999" />
          </label>
        </div>
        <label>
          <span class="rotulo-campo"
            >Retenção desejada (%)
            <Info>A chance de lembrar o cartão no dia da revisão. Com 90%, o padrão, você acerta 9 de cada 10. Mais alta pede revisões mais frequentes; mais baixa, menos revisões e mais esquecimento.</Info></span
          >
          <input v-model="opcoes.retencao" class="campo curto" type="number" min="70" max="99" />
        </label>
        <div class="linha-acoes">
          <button class="botao" :disabled="salvando">{{ salvando ? 'Salvando…' : 'Salvar' }}</button>
          <button type="button" class="botao perigo" @click="apagar">Apagar baralho</button>
        </div>
      </form>
    </template>
  </div>
</template>

<style scoped>
.pedido {
  display: block;
  overflow: hidden;
  padding: 12px 0;
  color: inherit;
  font-family: var(--leitura);
  text-decoration: none;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pedido:hover {
  color: var(--azul);
}

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

.voltar:hover {
  color: var(--azul);
}

.tema {
  margin: 0 0 2px;
  color: var(--texto-suave);
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.titulo {
  font-family: var(--leitura);
  font-weight: 600 !important;
}

.hoje {
  display: flex;
  gap: 32px;
  margin: 4px 0 24px;
}

.cifra {
  display: grid;
  gap: 2px;
}

.valor {
  font-size: 1.75rem;
  font-weight: 650;
  letter-spacing: -0.02em;
  font-variant-numeric: tabular-nums;
}

.valor.novos {
  color: var(--azul);
}

.valor.revisar {
  color: var(--vermelho);
}

.cifra .nome {
  color: var(--texto-fraco);
  font-size: 0.75rem;
}

.principal-acoes {
  margin-bottom: 12px;
}

.principal-acoes .botao {
  min-height: 40px;
  padding: 0 22px;
}



.opcoes {
  display: grid;
  gap: 16px;
}

.opcoes label {
  display: grid;
  gap: 6px;
}

.dupla {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.rotulo-campo {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  gap: 2px;
  color: var(--texto-suave);
  font-size: 0.82rem;
  font-weight: 500;
}

.curto {
  max-width: 120px;
}


.aviso {
  margin-bottom: 16px;
}
</style>
