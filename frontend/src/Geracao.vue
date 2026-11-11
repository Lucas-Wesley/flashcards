<script setup>
import { computed, inject, onBeforeUnmount, onMounted, ref } from 'vue'
import BuscaDeImagem from './BuscaDeImagem.vue'
import Microfone from './Microfone.vue'
import { api } from './api.js'
import { contextoDoCartao, termoDaBusca } from './busca.js'
import { doLivro } from './fonte.js'

// Um pedido à IA ou um link: enquanto a IA trabalha, o andamento; depois, a proposta para revisar.
// Cada cartão pode ser editado, tirado ou ter a foto trocada; só os marcados entram no baralho.
// O pedido à IA pode ser editado e refeito depois de terminar, mesmo aprovado: aí a IA trabalha no
// baralho criado, e os cartões que voltarem com a mesma frente atualizam os que já existem.
// O baralho de destino pode ser trocado na revisão; um link sem baralho começa no "Inglês".
// Na revisão, cada cartão aparece como no estudo (F39: a imagem da frente, a frente, o verso e a
// imagem do verso); os campos só abrem em "Editar". A conferência (F40) e as imagens que não vieram
// deixam avisos nos cartões, e dá para ver só os com aviso.
const props = defineProps({ id: { type: String, required: true } })
const recado = inject('recado')

const g = ref(null)
const erro = ref('')
const nome = ref('')
const cartoes = ref([]) // cópia editável da proposta, com "incluir"
const buscandoEm = ref(null) // { i, lado }: o cartão e o lado ('frente' ou 'verso') com a busca de imagem aberta
const editandoCartao = ref(-1)
const baralhos = ref([])
const destino = ref('') // id do baralho, ou '' para um novo
const enviando = ref(false)
const editando = ref(false)
const novoPedido = ref('')
const novaQuantidade = ref('')
const soComAviso = ref(false)
// "Pedir algo a mais" (F42): num link ou num texto do Acervo, que não têm um pedido escrito para editar.
const pedindoMais = ref(false)
const instrucao = ref('')
const quantidadeMais = ref('')
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
// Do Acervo ou de uma marcação da Biblioteca: o texto veio pronto, e a IA só tira os cartões dele.
const doAcervo = computed(() => ['acervo', 'biblioteca'].includes(g.value?.origem))
const daMarcacao = computed(() => g.value?.origem === 'biblioteca')
const trabalhando = computed(() => ['na_fila', 'rodando'].includes(g.value?.estado))
const escolhidos = computed(() => cartoes.value.filter((c) => c.incluir))
// "Atualiza" foi visto no baralho do pedido; em outro, quem decide é a aprovação.
const atualizacoes = computed(() => (destino.value && destino.value === g.value?.baralho_id ? escolhidos.value.filter((c) => c.atualiza).length : 0))
const podeRefazer = computed(() => g.value && !doLink.value && !doAcervo.value && !trabalhando.value)

async function carregar() {
  try {
    const anterior = g.value?.estado
    g.value = await api(`/api/geracoes/${props.id}`)
    erro.value = ''
    if (g.value.estado === 'em_revisao' && anterior !== 'em_revisao') {
      nome.value = g.value.proposta.nome_baralho
      cartoes.value = g.value.proposta.cartoes.map((c) => ({ ...c, incluir: true }))
      editandoCartao.value = -1
      await carregarBaralhos()
    }
  } catch (e) {
    erro.value = e.status === 0 ? 'Sem conexão: os pedidos à IA só abrem na rede de casa.' : e.message
  }
  clearTimeout(relogio)
  if (trabalhando.value) relogio = setTimeout(carregar, 3000)
}

const semAcento = (s) => s.normalize('NFD').replace(/[\u0300-\u036f]/g, '').trim().toLowerCase()

async function carregarBaralhos() {
  try {
    baralhos.value = await api('/api/baralhos')
  } catch {
    baralhos.value = [] // sem a lista, fica o baralho do pedido
  }
  destino.value = g.value.baralho_id || (doLink.value ? (baralhos.value.find((b) => semAcento(b.nome) === 'ingles')?.id ?? '') : '')
}

async function excluir() {
  const aviso = g.value.estado === 'concluida' ? 'Excluir este pedido? Os cartões já aprovados continuam no baralho.' : 'Excluir este pedido? Nenhum cartão será criado.'
  if (!confirm(aviso)) return
  try {
    await api(`/api/geracoes/${props.id}`, { metodo: 'DELETE' })
    recado('Pedido excluído.')
    location.hash = '#/pedidos'
  } catch (e) {
    recado(e.message, true)
  }
}

async function acao(qual) {
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
      corpo: { baralho_id: destino.value, nome_baralho: nome.value, cartoes: escolhidos.value },
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
  novaQuantidade.value = g.value.quantidade || ''
  editando.value = true
}

function abrirPedirMais() {
  instrucao.value = g.value.instrucao || ''
  quantidadeMais.value = doLink.value ? g.value.quantidade || 3 : g.value.quantidade || ''
  pedindoMais.value = true
}

function ouvidoMais(texto) {
  instrucao.value = instrucao.value.trim() ? `${instrucao.value.trim()} ${texto}` : texto
}

async function pedirMais() {
  if (g.value.estado === 'em_revisao' && !confirm('Refazer troca a proposta atual pela nova. Continuar?')) return
  enviando.value = true
  try {
    const corpo = { instrucao: instrucao.value }
    if (Number(quantidadeMais.value)) corpo.quantidade = Number(quantidadeMais.value)
    g.value = await api(`/api/geracoes/${props.id}/refazer`, { metodo: 'POST', corpo })
    pedindoMais.value = false
    cartoes.value = []
    carregar()
  } catch (e) {
    recado(e.message, true)
  } finally {
    enviando.value = false
  }
}

function ouvido(texto) {
  novoPedido.value = novoPedido.value.trim() ? `${novoPedido.value.trim()} ${texto}` : texto
}

const comAviso = computed(() => cartoes.value.filter((c) => c.avisos?.length).length)
const conferencia = computed(() => g.value?.proposta?.conferencia)

const todosMarcados = computed(() => cartoes.value.length > 0 && escolhidos.value.length === cartoes.value.length)
function marcarTodos() {
  const marcar = !todosMarcados.value
  cartoes.value.forEach((c) => (c.incluir = marcar))
}

// A quantidade do pedido: em branco (0), a IA decidiu.
const textoDaQuantidade = computed(() => {
  if (doLink.value) return g.value.quantidade && g.value.quantidade !== 3 ? `${g.value.quantidade} frases · ` : ''
  if (doAcervo.value) return `até ${g.value.quantidade} · `
  return g.value.quantidade ? `cerca de ${g.value.quantidade} · ` : ''
})

async function refazer() {
  if (!novoPedido.value.trim()) return
  if (g.value.estado === 'em_revisao' && !confirm('Refazer troca a proposta atual pela nova. Continuar?')) return
  enviando.value = true
  try {
    g.value = await api(`/api/geracoes/${props.id}/refazer`, {
      metodo: 'POST',
      corpo: { pedido: novoPedido.value, quantidade: Number(novaQuantidade.value) || 0 },
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

function trocarImagem(achada) {
  const { i, lado } = buscandoEm.value
  cartoes.value[i][lado === 'frente' ? 'imagem_frente_url' : 'imagem_url'] = achada.url
  buscandoEm.value = null
}

const buscandoNo = (i, lado) => buscandoEm.value?.i === i && buscandoEm.value?.lado === lado
function abrirBusca(i, lado) {
  buscandoEm.value = buscandoNo(i, lado) ? null : { i, lado }
}

const hora = (iso) => (iso ? new Date(iso).toLocaleString('pt-BR', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' }) : '')

onMounted(carregar)
onBeforeUnmount(() => clearTimeout(relogio))
</script>

<template>
  <div class="pagina">
    <a class="voltar" href="#/pedidos">Pedidos</a>
    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <template v-if="g">
      <h1>{{ doLink ? 'Cartões do link' : daMarcacao ? 'Cartão da marcação' : doAcervo ? 'Cartões do Acervo' : 'Gerar com IA' }}</h1>
      <p v-if="doLink" class="pedido link">
        <a :href="g.pedido" target="_blank" rel="noopener">{{ g.pedido }}</a>
        <button v-if="!trabalhando && !pedindoMais" type="button" class="botao editar" @click="abrirPedirMais">Pedir algo a mais</button>
      </p>
      <p v-else-if="doAcervo" class="pedido">
        {{ g.pedido }}
        <a v-if="g.link" class="botao editar" :href="g.link" target="_blank" rel="noopener">Abrir {{ daMarcacao ? 'no livro' : 'no Acervo' }}</a>
        <button v-if="!trabalhando && g.estado !== 'em_revisao'" type="button" class="botao editar" @click="acao('refazer')">Refazer</button>
        <button v-if="!trabalhando && !pedindoMais" type="button" class="botao editar" @click="abrirPedirMais">Pedir algo a mais</button>
      </p>
      <form v-else-if="editando" class="edicao" @submit.prevent="refazer">
        <div class="caixa">
          <textarea
            v-model="novoPedido"
            class="campo"
            rows="4"
            aria-label="Pedido"
            @keydown.ctrl.enter="refazer"
            @keydown.meta.enter="refazer"
          />
          <Microfone class="falar" @texto="ouvido" />
        </div>
        <label class="quantos">
          <span class="rotulo-campo">Quantos cartões (opcional)</span>
          <input v-model="novaQuantidade" class="campo" type="number" min="1" max="300" placeholder="A IA decide" aria-label="Quantidade" />
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
      <form v-if="pedindoMais" class="edicao" @submit.prevent="pedirMais">
        <span class="rotulo-campo">O que mudar ou acrescentar</span>
        <div class="caixa">
          <textarea
            v-model="instrucao"
            class="campo"
            rows="3"
            maxlength="1000"
            aria-label="O que mudar ou acrescentar"
            :placeholder="doLink ? 'Ex.: prefira o sentido de desistir; frases mais curtas' : 'Ex.: mais cartões sobre as datas; deixe os nomes de fora'"
            @keydown.ctrl.enter="pedirMais"
            @keydown.meta.enter="pedirMais"
          />
          <Microfone class="falar" @texto="ouvidoMais" />
        </div>
        <label class="quantos">
          <span class="rotulo-campo">{{ doLink ? 'Quantas frases' : 'No máximo quantos cartões' }}</span>
          <input v-model="quantidadeMais" class="campo" type="number" min="1" :max="doLink ? 10 : 300" aria-label="Quantidade" />
        </label>
        <p v-if="doLink" class="suave dica">As frases e os áudios continuam os da página: a IA só escolhe. Se a página tiver menos frases que o pedido, vêm todas.</p>
        <div class="linha-acoes">
          <button class="botao principal" :disabled="enviando">{{ enviando ? 'Enviando…' : 'Refazer com isto' }}</button>
          <button type="button" class="botao" @click="pedindoMais = false">Cancelar</button>
        </div>
      </form>
      <p v-else-if="(doLink || doAcervo) && g.instrucao" class="instrucao"><span class="rotulo-campo">Você pediu</span>{{ g.instrucao }}</p>
      <p class="meta">
        <span class="chip" :class="{ andando: trabalhando || g.estado === 'em_revisao', ok: g.estado === 'concluida', erro: g.estado === 'falhou' }">{{
          NOMES[g.estado]
        }}</span>
        {{ textoDaQuantidade }}{{ g.tipo === 'invertido' ? 'nos dois sentidos' : g.tipo === 'leitura' ? 'de leitura' : 'frente e verso' }} · {{ doLink ? 'lido' : doAcervo ? 'enviado' : 'pedido' }} em {{ hora(g.criado_em) }}
      </p>

      <template v-if="trabalhando">
        <p class="suave espera">
          {{
            g.estado === 'na_fila'
              ? 'Esperando a vez: a IA faz um pedido por vez.'
              : doLink
                ? `Ela lê as frases da página e escolhe as ${g.quantidade || 3} mais didáticas; costuma levar menos de um minuto.`
                : daMarcacao
                  ? 'Ela lê o trecho marcado e monta a frente e o verso; costuma levar um ou dois minutos.'
                  : doAcervo
                  ? 'Ela lê o texto e escolhe o que vale virar cartão; costuma levar alguns minutos.'
                  : 'Ela pesquisa para conferir os fatos; costuma levar alguns minutos.'
          }}
          Pode sair desta tela: o pedido continua e aparece em Pedidos quando a proposta estiver pronta.
        </p>
        <button class="botao perigo" @click="acao('cancelar')">Cancelar pedido</button>
      </template>

      <template v-else-if="g.estado === 'falhou' || g.estado === 'cancelada' || g.estado === 'rejeitada'">
        <div v-if="g.erro" class="aviso erro">{{ g.erro }}</div>
        <div class="linha-acoes acoes">
          <button v-if="g.pode_refazer" class="botao principal" @click="acao('refazer')">Pedir de novo</button>
          <button class="botao perigo" @click="excluir">Excluir</button>
        </div>
      </template>

      <p v-else-if="g.estado === 'concluida'" class="linha-acoes acoes">
        <a class="botao principal" :href="`#/baralho/${g.resultado}`">Abrir o baralho</a>
        <button class="botao perigo" @click="excluir">Excluir</button>
      </p>

      <template v-else-if="g.estado === 'em_revisao'">
        <p v-if="g.proposta.entendimento" class="entendimento"><span class="rotulo-campo">A IA entendeu</span>{{ g.proposta.entendimento }}</p>
        <div v-if="conferencia" class="conferencia" :class="{ falhou: conferencia.falhou }">
          <span class="rotulo-campo">A conferência</span>
          <template v-if="conferencia.falhou">Não rodou desta vez ({{ conferencia.falhou }}). Confira os cartões você mesmo.</template>
          <template v-else>
            <span>{{ conferencia.resumo }}</span>
            <span v-if="conferencia.faltando?.length" class="faltando"><strong>Faltando:</strong> {{ conferencia.faltando.join(', ') }}.</span>
          </template>
        </div>
        <div v-if="g.proposta.observacoes" class="aviso observacoes"><strong v-if="!doLink">Da IA: </strong>{{ g.proposta.observacoes }}</div>

        <div class="destino">
          <label class="nome-baralho">
            <span class="rotulo-campo">Baralho</span>
            <select v-model="destino" class="campo" aria-label="Baralho">
              <option value="">Um baralho novo</option>
              <option v-if="g.baralho_id && !baralhos.some((b) => b.id === g.baralho_id)" :value="g.baralho_id">O baralho do pedido</option>
              <option v-for="b in baralhos" :key="b.id" :value="b.id">{{ b.nome }}</option>
            </select>
          </label>
          <label v-if="!destino" class="nome-baralho">
            <span class="rotulo-campo">Nome do baralho novo</span>
            <input v-model="nome" class="campo" aria-label="Nome do baralho novo" />
          </label>
        </div>
        <p v-if="atualizacoes" class="suave">
          {{ atualizacoes === 1 ? '1 cartão já existe no baralho e vai ser atualizado' : `${atualizacoes} cartões já existem no baralho e vão ser atualizados` }}, sem perder o histórico.
        </p>

        <div class="contagem">
          <p class="rotulo">{{ escolhidos.length }} de {{ cartoes.length }} {{ cartoes.length === 1 ? 'cartão marcado' : 'cartões marcados' }}</p>
          <div class="linha-acoes">
            <button v-if="comAviso" type="button" class="botao-link" @click="soComAviso = !soComAviso">
              {{ soComAviso ? 'Ver todos' : comAviso === 1 ? 'Ver o cartão com aviso' : `Ver os ${comAviso} com aviso` }}
            </button>
            <button v-if="cartoes.length > 1" type="button" class="botao-link" @click="marcarTodos">{{ todosMarcados ? 'Desmarcar todos' : 'Marcar todos' }}</button>
          </div>
        </div>
        <ul class="lista">
          <li v-for="(c, i) in cartoes" v-show="!soComAviso || c.avisos?.length" :key="i" class="proposto" :class="{ fora: !c.incluir }">
            <input v-model="c.incluir" type="checkbox" class="marcar" :aria-label="`Incluir o cartão ${i + 1}`" />
            <div class="campos">
              <div v-if="c.atualiza || c.fonte" class="chips">
                <span v-if="c.atualiza" class="chip atualiza">Atualiza o que já existe</span>
                <a v-if="c.fonte" class="chip" :href="c.fonte" target="_blank" rel="noopener">{{ c.secao || (doLivro(c.fonte) ? 'Trecho no livro' : 'Texto no Acervo') }}</a>
              </div>

              <ul v-if="c.avisos?.length" class="avisos-do-cartao">
                <li v-for="(a, j) in c.avisos" :key="j">{{ a }}</li>
              </ul>

              <template v-if="editandoCartao !== i">
                <div class="previa" :class="{ leitura: g.tipo === 'leitura' }">
                  <img v-if="c.imagem_frente_url" class="img-frente" :src="c.imagem_frente_url" alt="" loading="lazy" />
                  <div class="textos">
                    <p class="p-frente">{{ c.frente }}</p>
                    <p class="p-verso">{{ c.verso }}</p>
                    <audio v-if="c.audio_url" class="audio-previa" controls preload="none" :src="c.audio_url" :aria-label="`Áudio do cartão ${i + 1}`" />
                  </div>
                  <img v-if="c.imagem_url" class="img-verso" :src="c.imagem_url" alt="" loading="lazy" />
                </div>
                <button type="button" class="botao-link editar-cartao" @click="editandoCartao = i">Editar</button>
              </template>

              <template v-else>
                <span class="rotulo-campo">Frente</span>
                <textarea v-model="c.frente" class="campo texto-cartao" rows="2" :aria-label="`Frente do cartão ${i + 1}`" />
                <span class="rotulo-campo">Verso</span>
                <textarea v-model="c.verso" class="campo texto-cartao" :rows="g.tipo === 'leitura' ? 10 : 2" :aria-label="`Verso do cartão ${i + 1}`" />
                <input v-model="c.tags" class="campo tags" placeholder="tags" :aria-label="`Tags do cartão ${i + 1}`" />
                <div v-if="c.audio_url" class="audio">
                  <audio controls preload="none" :src="c.audio_url" :aria-label="`Áudio do cartão ${i + 1}`" />
                  <button type="button" class="botao" @click="c.audio_url = ''">Sem áudio</button>
                </div>
                <div v-for="lado in ['frente', 'verso']" :key="lado" class="imagem">
                  <img v-if="c[lado === 'frente' ? 'imagem_frente_url' : 'imagem_url']" :src="c[lado === 'frente' ? 'imagem_frente_url' : 'imagem_url']" alt="" loading="lazy" />
                  <span class="rotulo-campo lado">Imagem {{ lado === 'frente' ? 'da frente' : 'do verso' }}</span>
                  <div class="linha-acoes">
                    <button type="button" class="botao" @click="abrirBusca(i, lado)">
                      {{ c[lado === 'frente' ? 'imagem_frente_url' : 'imagem_url'] ? 'Trocar' : 'Buscar' }}
                    </button>
                    <button v-if="c[lado === 'frente' ? 'imagem_frente_url' : 'imagem_url']" type="button" class="botao" @click="c[lado === 'frente' ? 'imagem_frente_url' : 'imagem_url'] = ''">
                      Tirar
                    </button>
                  </div>
                </div>
                <BuscaDeImagem
                  v-if="buscandoEm?.i === i"
                  :termo="(buscandoEm.lado === 'frente' ? c.imagem_frente : c.imagem) || termoDaBusca(c.verso || c.frente)"
                  :contexto="contextoDoCartao(c)"
                  @escolher="trocarImagem"
                  @fechar="buscandoEm = null"
                />
                <button type="button" class="botao-link editar-cartao" @click="(editandoCartao = -1), (buscandoEm = null)">Pronto</button>
              </template>
            </div>
          </li>
        </ul>

        <div class="rodape vidro">
          <button class="botao principal" :disabled="enviando || !escolhidos.length || (!destino && !nome.trim())" @click="aprovar">
            {{
              enviando
                ? 'Criando…'
                : destino
                  ? atualizacoes === escolhidos.length
                    ? `Atualizar ${escolhidos.length} no baralho`
                    : `Adicionar ${escolhidos.length} ao baralho`
                  : `Criar baralho com ${escolhidos.length} ${escolhidos.length === 1 ? 'cartão' : 'cartões'}`
            }}
          </button>
          <button v-if="g.pode_refazer" class="botao" :disabled="enviando" @click="acao('refazer')">Refazer</button>
          <button class="botao perigo" :disabled="enviando" @click="excluir">Excluir</button>
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

.instrucao {
  margin: 0 0 8px;
}
.instrucao .rotulo-campo {
  display: block;
}
.quantos {
  display: grid;
  gap: 6px;
  max-width: 200px;
}

.caixa {
  position: relative;
}

.caixa textarea {
  display: block;
  width: 100%;
  padding-bottom: 52px;
}

.falar {
  position: absolute;
  right: 8px;
  bottom: 8px;
}

.entendimento {
  display: grid;
  gap: 4px;
  margin: 0 0 16px;
  padding: 12px 14px;
  border-left: 3px solid var(--marca);
  border-radius: var(--raio-p);
  background: var(--superficie);
  font-family: var(--leitura);
  line-height: 1.5;
}

.entendimento .rotulo-campo,
.conferencia .rotulo-campo {
  font-family: var(--interface);
}

.conferencia {
  display: grid;
  gap: 4px;
  margin: 0 0 16px;
  padding: 12px 14px;
  border-left: 3px solid var(--azul);
  border-radius: var(--raio-p);
  background: var(--superficie);
  line-height: 1.5;
}

.conferencia.falhou {
  border-left-color: var(--linha-forte);
  color: var(--texto-suave);
}

.faltando {
  color: var(--vermelho);
}

.avisos-do-cartao {
  display: grid;
  gap: 4px;
  margin: 0;
  padding: 8px 12px;
  border-radius: var(--raio-p);
  background: var(--vermelho-fundo);
  color: var(--vermelho);
  font-size: 0.85rem;
  line-height: 1.45;
  list-style: none;
}

.botao-link {
  padding: 0;
  border: 0;
  background: none;
  color: var(--marca);
  font: inherit;
  font-size: 0.82rem;
  font-weight: 500;
  cursor: pointer;
}

.botao-link:hover {
  text-decoration: underline;
}

.contagem {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-top: 20px;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

/* A prévia: como o cartão aparece no estudo, compacto. */
.previa {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}

.textos {
  flex: 1;
  min-width: 0;
}

.textos p {
  margin: 0;
  overflow-wrap: break-word;
  white-space: pre-wrap;
}

.p-frente {
  font-family: var(--leitura);
  font-size: 1.02rem;
  font-weight: 500;
  line-height: 1.4;
}

.p-verso {
  margin-top: 4px !important;
  color: var(--texto-suave);
  font-family: var(--leitura);
  font-size: 0.95rem;
  line-height: 1.5;
}

.previa.leitura .p-verso {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 6;
}

.img-frente,
.img-verso {
  flex-shrink: 0;
  border-radius: var(--raio-p);
  background: var(--superficie);
  object-fit: contain;
}

.img-frente {
  width: 96px;
  height: 64px;
  box-shadow: 0 0 0 1px var(--linha);
}

.img-verso {
  width: 64px;
  height: 64px;
}

.audio-previa {
  height: 34px;
  max-width: 100%;
  margin-top: 8px;
}

.editar-cartao {
  justify-self: start;
}

.lado {
  min-width: 110px;
}

.dica {
  margin: 0;
  font-size: 0.85rem;
  line-height: 1.5;
}

.atualiza,
a.chip {
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

.destino {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}

select.campo {
  appearance: auto;
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
  padding: 14px 0;
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
  align-items: center;
  gap: 10px;
}

.imagem img {
  width: 56px;
  height: 56px;
  border-radius: var(--raio-p);
  background: var(--superficie);
  object-fit: contain;
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
