<script setup>
import { computed, inject, onBeforeUnmount, onMounted, ref } from 'vue'
import BuscaDeImagem from './BuscaDeImagem.vue'
import { api } from './api.js'

// Um pedido à IA: enquanto ela trabalha, o andamento; quando volta, a proposta para revisar.
// Cada cartão pode ser editado, tirado ou ter a foto trocada; só os marcados entram no baralho.
const props = defineProps({ id: { type: String, required: true } })
const recado = inject('recado')

const g = ref(null)
const erro = ref('')
const nome = ref('')
const cartoes = ref([]) // cópia editável da proposta, com "incluir"
const buscandoEm = ref(-1) // índice do cartão com a busca de imagem aberta
const nomeDoDestino = ref('')
const enviando = ref(false)
let relogio

const NOMES = {
  na_fila: 'Na fila',
  rodando: 'A IA está montando os cartões…',
  em_revisao: 'Proposta para revisar',
  concluida: 'Aprovada',
  falhou: 'Falhou',
  cancelada: 'Cancelada',
  rejeitada: 'Descartada',
}

const trabalhando = computed(() => ['na_fila', 'rodando'].includes(g.value?.estado))
const escolhidos = computed(() => cartoes.value.filter((c) => c.incluir))

async function carregar() {
  try {
    const anterior = g.value?.estado
    g.value = await api(`/api/geracoes/${props.id}`)
    erro.value = ''
    if (g.value.estado === 'em_revisao' && anterior !== 'em_revisao') {
      nome.value = g.value.proposta.nome_baralho
      cartoes.value = g.value.proposta.cartoes.map((c) => ({ ...c, incluir: true }))
    }
    if (g.value.baralho_id && !nomeDoDestino.value) nomeDoDestino.value = (await api(`/api/baralhos/${g.value.baralho_id}`)).nome
  } catch (e) {
    erro.value = e.status === 0 ? 'Sem conexão: os pedidos à IA só abrem na rede de casa.' : e.message
  }
  clearTimeout(relogio)
  if (trabalhando.value) relogio = setTimeout(carregar, 3000)
}

async function acao(qual) {
  if (qual === 'descartar' && !confirm('Descartar esta proposta? Nenhum cartão será criado.')) return
  try {
    g.value = await api(`/api/geracoes/${props.id}/${qual}`, { metodo: 'POST' })
    if (qual === 'refazer') carregar()
  } catch (e) {
    recado(e.message, true)
  }
}

async function aprovar() {
  enviando.value = true
  try {
    const r = await api(`/api/geracoes/${props.id}/aprovar`, {
      metodo: 'POST',
      corpo: { nome_baralho: nome.value, cartoes: escolhidos.value },
      tempo: 120000,
    })
    recado(
      `${r.criadas === 1 ? '1 nota criada' : `${r.criadas} notas criadas`}` +
        (r.sem_imagem ? `; ${r.sem_imagem === 1 ? '1 imagem não veio' : `${r.sem_imagem} imagens não vieram`}.` : '.'),
    )
    location.hash = `#/baralho/${r.baralho_id}`
  } catch (e) {
    recado(e.message, true)
  } finally {
    enviando.value = false
  }
}

function trocarImagem(i, achada) {
  cartoes.value[i].imagem_url = achada.url
  buscandoEm.value = -1
}

const hora = (iso) => (iso ? new Date(iso).toLocaleString('pt-BR', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' }) : '')

onMounted(carregar)
onBeforeUnmount(() => clearTimeout(relogio))
</script>

<template>
  <div class="pagina">
    <a class="voltar" href="#/baralhos">Baralhos</a>
    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <template v-if="g">
      <h1>Gerar com IA</h1>
      <p class="pedido">“{{ g.pedido }}”</p>
      <p class="meta">
        <span class="chip" :class="{ andando: trabalhando || g.estado === 'em_revisao', ok: g.estado === 'concluida', erro: g.estado === 'falhou' }">{{
          NOMES[g.estado]
        }}</span>
        cerca de {{ g.quantidade }} · {{ g.tipo === 'invertido' ? 'nos dois sentidos' : 'frente e verso' }} · pedido em {{ hora(g.criado_em) }}
      </p>

      <template v-if="trabalhando">
        <p class="suave espera">
          {{ g.estado === 'na_fila' ? 'Esperando a vez: a IA faz um pedido por vez.' : 'Ela pesquisa para conferir os fatos; costuma levar alguns minutos.' }}
          Pode sair desta tela: o pedido continua e aparece em Baralhos quando a proposta estiver pronta.
        </p>
        <button class="botao perigo" @click="acao('cancelar')">Cancelar pedido</button>
      </template>

      <template v-else-if="g.estado === 'falhou' || g.estado === 'cancelada' || g.estado === 'rejeitada'">
        <div v-if="g.erro" class="aviso erro">{{ g.erro }}</div>
        <div class="linha-acoes acoes">
          <button class="botao principal" @click="acao('refazer')">Pedir de novo</button>
        </div>
      </template>

      <p v-else-if="g.estado === 'concluida'" class="acoes">
        <a class="botao principal" :href="`#/baralho/${g.resultado}`">Abrir o baralho</a>
      </p>

      <template v-else-if="g.estado === 'em_revisao'">
        <div v-if="g.proposta.observacoes" class="aviso observacoes"><strong>Da IA:</strong> {{ g.proposta.observacoes }}</div>

        <label v-if="!g.baralho_id" class="nome-baralho">
          <span class="rotulo-campo">Nome do baralho novo</span>
          <input v-model="nome" class="campo" aria-label="Nome do baralho novo" />
        </label>
        <p v-else class="suave">Os cartões vão para <strong>{{ nomeDoDestino }}</strong>.</p>

        <p class="rotulo">{{ cartoes.length }} cartões propostos · {{ escolhidos.length }} marcados</p>
        <ul class="lista">
          <li v-for="(c, i) in cartoes" :key="i" class="proposto" :class="{ fora: !c.incluir }">
            <input v-model="c.incluir" type="checkbox" class="marcar" :aria-label="`Incluir o cartão ${i + 1}`" />
            <div class="campos">
              <textarea v-model="c.frente" class="campo texto-cartao" rows="2" :aria-label="`Frente do cartão ${i + 1}`" />
              <textarea v-model="c.verso" class="campo texto-cartao" rows="2" :aria-label="`Verso do cartão ${i + 1}`" />
              <input v-model="c.tags" class="campo tags" placeholder="tags" :aria-label="`Tags do cartão ${i + 1}`" />
              <div class="imagem">
                <img v-if="c.imagem_url" :src="c.imagem_url" alt="" loading="lazy" />
                <div class="linha-acoes">
                  <button type="button" class="botao" @click="buscandoEm = buscandoEm === i ? -1 : i">
                    {{ c.imagem_url ? 'Trocar imagem' : 'Buscar imagem' }}
                  </button>
                  <button v-if="c.imagem_url" type="button" class="botao" @click="c.imagem_url = ''">Sem imagem</button>
                </div>
              </div>
              <BuscaDeImagem v-if="buscandoEm === i" :termo="c.imagem || c.verso.split('\n')[0]" @escolher="trocarImagem(i, $event)" @fechar="buscandoEm = -1" />
            </div>
          </li>
        </ul>

        <div class="rodape vidro">
          <button class="botao principal" :disabled="enviando || !escolhidos.length || (!g.baralho_id && !nome.trim())" @click="aprovar">
            {{
              enviando
                ? 'Criando…'
                : g.baralho_id
                  ? `Adicionar ${escolhidos.length} ao baralho`
                  : `Criar baralho com ${escolhidos.length} ${escolhidos.length === 1 ? 'cartão' : 'cartões'}`
            }}
          </button>
          <button class="botao perigo" :disabled="enviando" @click="acao('descartar')">Descartar</button>
        </div>
      </template>
    </template>
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

.pedido {
  margin: -8px 0 8px;
  font-family: var(--leitura);
  font-size: 1.1rem;
  line-height: 1.5;
}

.meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin: 0 0 20px;
  color: var(--texto-fraco);
  font-size: 0.8rem;
}

.espera {
  margin: 0 0 16px;
  font-size: 0.9rem;
  line-height: 1.55;
}

.acoes {
  margin-top: 16px;
}

.observacoes {
  margin-bottom: 18px;
}

.nome-baralho {
  display: grid;
  gap: 6px;
}

.rotulo-campo {
  color: var(--texto-suave);
  font-size: 0.82rem;
  font-weight: 500;
}

.proposto {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 12px;
  padding: 16px 0;
}

.marcar {
  width: 18px;
  height: 18px;
  margin-top: 10px;
  accent-color: var(--azul);
}

.campos {
  display: grid;
  gap: 6px;
}

.proposto.fora .campos {
  opacity: 0.4;
}

.texto-cartao {
  font-family: var(--leitura);
  font-size: 1rem;
  line-height: 1.45;
  resize: vertical;
}

.tags {
  min-height: 32px;
  font-size: 0.82rem;
}

.imagem {
  display: flex;
  align-items: flex-end;
  gap: 10px;
}

.imagem img {
  width: 72px;
  height: 72px;
  border-radius: var(--raio-p);
  background: var(--superficie);
  object-fit: cover;
}

/* A barra de aprovar fica à mão no fim da tela, mesmo numa proposta longa. */
.rodape {
  position: sticky;
  bottom: 0;
  display: flex;
  gap: 8px;
  margin: 16px calc(-1 * var(--margem)) 0;
  padding: 12px var(--margem) max(12px, env(safe-area-inset-bottom));
  border-top: 1px solid var(--linha);
}

@media (max-width: 820px) {
  .rodape {
    bottom: calc(61px + env(safe-area-inset-bottom));
  }
}
</style>
