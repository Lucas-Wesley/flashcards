<script setup>
import { computed, inject, onBeforeUnmount, onMounted, ref } from 'vue'
import BuscaDeImagem from './BuscaDeImagem.vue'
import { api } from './api.js'
import { contextoDoCartao, termoDaBusca } from './busca.js'

// Um pedido à IA ou um link: enquanto a IA trabalha, o andamento; depois, a proposta para revisar.
// Cada cartão pode ser editado, tirado ou ter a foto trocada; só os marcados entram no baralho.
// O pedido à IA pode ser editado e refeito depois de terminar, mesmo aprovado: aí a IA trabalha no
// baralho criado, e os cartões que voltarem com a mesma frente atualizam os que já existem.
const props = defineProps({ id: { type: String, required: true } })
const recado = inject('recado')

const g = ref(null)
const erro = ref('')
const nome = ref('')
const cartoes = ref([]) // cópia editável da proposta, com "incluir"
const buscandoEm = ref(-1) // índice do cartão com a busca de imagem aberta
const nomeDoDestino = ref('')
const enviando = ref(false)
const editando = ref(false)
const novoPedido = ref('')
const novaQuantidade = ref(20)
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

const doLink = computed(() => g.value?.origem === 'link')
const doAcervo = computed(() => g.value?.origem === 'acervo')
const trabalhando = computed(() => ['na_fila', 'rodando'].includes(g.value?.estado))
const escolhidos = computed(() => cartoes.value.filter((c) => c.incluir))
const atualizacoes = computed(() => escolhidos.value.filter((c) => c.atualiza).length)
const podeRefazer = computed(() => g.value && !doLink.value && !doAcervo.value && !trabalhando.value)

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
  if (qual === 'refazer' && g.value.estado === 'em_revisao' && !confirm('Refazer troca a proposta atual pela nova. Continuar?')) return
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
    const faltas = [
      r.sem_imagem && (r.sem_imagem === 1 ? '1 imagem não veio' : `${r.sem_imagem} imagens não vieram`),
      r.sem_audio && (r.sem_audio === 1 ? '1 áudio não veio' : `${r.sem_audio} áudios não vieram`),
    ].filter(Boolean)
    const feitas = [
      (r.criadas || !r.atualizadas) && (r.criadas === 1 ? '1 nota criada' : `${r.criadas} notas criadas`),
      r.atualizadas && (r.atualizadas === 1 ? '1 atualizada' : `${r.atualizadas} atualizadas`),
    ].filter(Boolean)
    recado(`${feitas.join(', ')}${faltas.length ? `; ${faltas.join(', ')}` : ''}.`)
    location.hash = `#/baralho/${r.baralho_id}`
  } catch (e) {
    recado(e.message, true)
  } finally {
    enviando.value = false
  }
}

function editar() {
  novoPedido.value = g.value.pedido
  novaQuantidade.value = g.value.quantidade
  editando.value = true
}

async function refazer() {
  if (!novoPedido.value.trim()) return
  if (g.value.estado === 'em_revisao' && !confirm('Refazer troca a proposta atual pela nova. Continuar?')) return
  enviando.value = true
  try {
    g.value = await api(`/api/geracoes/${props.id}/refazer`, {
      metodo: 'POST',
      corpo: { pedido: novoPedido.value, quantidade: Number(novaQuantidade.value) },
    })
    editando.value = false
    cartoes.value = []
    carregar()
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
    <a class="voltar" :href="doLink ? '#/link' : '#/baralhos'">{{ doLink ? 'Adicionar do link' : 'Baralhos' }}</a>
    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <template v-if="g">
      <h1>{{ doLink ? 'Cartões do link' : doAcervo ? 'Cartões do Acervo' : 'Gerar com IA' }}</h1>
      <p v-if="doLink" class="pedido link"><a :href="g.pedido" target="_blank" rel="noopener">{{ g.pedido }}</a></p>
      <p v-else-if="doAcervo" class="pedido">
        {{ g.pedido }}
        <a v-if="g.link" class="botao editar" :href="g.link" target="_blank" rel="noopener">Abrir no Acervo</a>
        <button v-if="!trabalhando" type="button" class="botao editar" @click="acao('refazer')">Refazer</button>
      </p>
      <form v-else-if="editando" class="edicao" @submit.prevent="refazer">
        <textarea
          v-model="novoPedido"
          class="campo"
          rows="4"
          aria-label="Pedido"
          @keydown.ctrl.enter="refazer"
          @keydown.meta.enter="refazer"
        />
        <label class="quantos">
          <span class="rotulo-campo">Quantos cartões, mais ou menos</span>
          <input v-model="novaQuantidade" class="campo" type="number" min="1" max="100" aria-label="Quantidade" />
        </label>
        <p v-if="g.estado === 'concluida'" class="suave dica">
          A IA trabalha no baralho que já foi criado. Os cartões que ela devolver com a mesma frente atualizam os que existem, sem perder o
          histórico de estudo; você revisa antes, como da primeira vez.
        </p>
        <div class="linha-acoes">
          <button class="botao principal" :disabled="enviando || !novoPedido.trim()">{{ enviando ? 'Enviando…' : 'Refazer com este pedido' }}</button>
          <button type="button" class="botao" @click="editando = false">Cancelar</button>
        </div>
      </form>
      <p v-else class="pedido">
        “{{ g.pedido }}”
        <button v-if="podeRefazer" type="button" class="botao editar" @click="editar">Editar e refazer</button>
      </p>
      <p class="meta">
        <span class="chip" :class="{ andando: trabalhando || g.estado === 'em_revisao', ok: g.estado === 'concluida', erro: g.estado === 'falhou' }">{{
          NOMES[g.estado]
        }}</span>
        <template v-if="doAcervo">até {{ g.quantidade }} · </template><template v-else-if="!doLink">cerca de {{ g.quantidade }} · </template>{{ g.tipo === 'invertido' ? 'nos dois sentidos' : 'frente e verso' }} · {{ doLink ? 'lido' : doAcervo ? 'enviado' : 'pedido' }} em {{ hora(g.criado_em) }}
      </p>

      <template v-if="trabalhando">
        <p class="suave espera">
          {{
            g.estado === 'na_fila'
              ? 'Esperando a vez: a IA faz um pedido por vez.'
              : doLink
                ? 'Ela lê as frases da página e escolhe as 3 mais didáticas; costuma levar menos de um minuto.'
                : doAcervo
                  ? 'Ela lê o texto e escolhe o que vale virar cartão; costuma levar alguns minutos.'
                  : 'Ela pesquisa para conferir os fatos; costuma levar alguns minutos.'
          }}
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
        <div v-if="g.proposta.observacoes" class="aviso observacoes"><strong v-if="!doLink">Da IA: </strong>{{ g.proposta.observacoes }}</div>

        <label v-if="!g.baralho_id" class="nome-baralho">
          <span class="rotulo-campo">Nome do baralho novo</span>
          <input v-model="nome" class="campo" aria-label="Nome do baralho novo" />
        </label>
        <p v-else class="suave">
          Os cartões vão para <strong>{{ nomeDoDestino }}</strong>.<template v-if="atualizacoes">
            {{ atualizacoes === 1 ? '1 já existe e vai ser atualizado' : `${atualizacoes} já existem e vão ser atualizados` }}, sem perder o histórico.</template
          >
        </p>

        <p class="rotulo">{{ cartoes.length }} cartões propostos · {{ escolhidos.length }} marcados</p>
        <ul class="lista">
          <li v-for="(c, i) in cartoes" :key="i" class="proposto" :class="{ fora: !c.incluir }">
            <input v-model="c.incluir" type="checkbox" class="marcar" :aria-label="`Incluir o cartão ${i + 1}`" />
            <div class="campos">
              <span v-if="c.atualiza" class="chip atualiza">Atualiza o cartão que já existe</span>
              <textarea v-model="c.frente" class="campo texto-cartao" rows="2" :aria-label="`Frente do cartão ${i + 1}`" />
              <textarea v-model="c.verso" class="campo texto-cartao" rows="2" :aria-label="`Verso do cartão ${i + 1}`" />
              <input v-model="c.tags" class="campo tags" placeholder="tags" :aria-label="`Tags do cartão ${i + 1}`" />
              <div v-if="c.audio_url" class="audio">
                <audio controls preload="none" :src="c.audio_url" :aria-label="`Áudio do cartão ${i + 1}`" />
                <button type="button" class="botao" @click="c.audio_url = ''">Sem áudio</button>
              </div>
              <div class="imagem">
                <img v-if="c.imagem_url" :src="c.imagem_url" alt="" loading="lazy" />
                <div class="linha-acoes">
                  <button type="button" class="botao" @click="buscandoEm = buscandoEm === i ? -1 : i">
                    {{ c.imagem_url ? 'Trocar imagem' : 'Buscar imagem' }}
                  </button>
                  <button v-if="c.imagem_url" type="button" class="botao" @click="c.imagem_url = ''">Sem imagem</button>
                </div>
              </div>
              <BuscaDeImagem v-if="buscandoEm === i" :termo="c.imagem || termoDaBusca(c.frente)" :contexto="contextoDoCartao(c)" @escolher="trocarImagem(i, $event)" @fechar="buscandoEm = -1" />
            </div>
          </li>
        </ul>

        <div class="rodape vidro">
          <button class="botao principal" :disabled="enviando || !escolhidos.length || (!g.baralho_id && !nome.trim())" @click="aprovar">
            {{
              enviando
                ? 'Criando…'
                : g.baralho_id
                  ? atualizacoes === escolhidos.length
                    ? `Atualizar ${escolhidos.length} no baralho`
                    : `Adicionar ${escolhidos.length} ao baralho`
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

.editar {
  min-height: 30px;
  margin-left: 6px;
  padding: 2px 10px;
  font-family: var(--interface);
  font-size: 0.8rem;
  vertical-align: middle;
}

.edicao {
  display: grid;
  gap: 12px;
  margin: 0 0 16px;
}

.edicao textarea {
  font-family: var(--leitura);
  font-size: 1rem;
  line-height: 1.5;
  resize: vertical;
}

.quantos {
  display: grid;
  gap: 6px;
  max-width: 260px;
}

.dica {
  margin: 0;
  font-size: 0.85rem;
  line-height: 1.5;
}

.atualiza {
  justify-self: start;
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
  /* O campo cresce com o texto: a tradução inteira aparece sem rolar. */
  field-sizing: content;
  min-height: 3.6em;
}

.tags {
  min-height: 32px;
  font-size: 0.82rem;
}

.pedido.link {
  overflow-wrap: anywhere;
  font-family: var(--interface);
  font-size: 0.9rem;
}

.audio {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.audio audio {
  height: 36px;
  max-width: 100%;
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
