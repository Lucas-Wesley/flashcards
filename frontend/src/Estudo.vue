<script setup>
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import Face from './Face.vue'
import Icone from './Icone.vue'
import { APRENDENDO, NOVO, REAPRENDENDO, REVISAO, api, lembrar } from './api.js'
import { hora, intervalo } from './tempo.js'

// A sessão de estudo: frente, "Mostrar resposta", verso e as notas. Teclado: espaço ou Enter
// mostra a resposta (e depois vale "Bom"), 1 a 4 respondem, 5 é "Amanhã", R repete o áudio, Z desfaz,
// Esc volta. O áudio é a pronúncia da frente da nota: toca sozinho quando ela aparece (na pergunta,
// ou na resposta do cartão invertido).
// "Amanhã" conta como erro para o algoritmo, mas o cartão sai do dia em vez de voltar em minutos.
// Os botões são grandes de propósito: a mesma tela serve no celular e no navegador da TV.
// O escopo é um baralho (o id), um tema ("tema/Inglês", F38) ou tudo ("tudo"). Com tema ou tudo, os
// baralhos se misturam: cada cartão mostra de que baralho é, e a resposta conta no baralho dele, como
// se tivesse sido estudado por lá.
// "Mostrar na TV" (F37): o celular continua sendo o estudo e manda cada cena à página da TV
// (http://192.168.1.20:8453), que só mostra. A escolha fica guardada para a próxima vez.
// Sem aviso do endereço na tela: atrapalhava o estudo.
const props = defineProps({ escopo: { type: String, required: true } })
const tema = computed(() => (props.escopo.startsWith('tema/') ? props.escopo.slice('tema/'.length) : ''))
const todos = computed(() => props.escopo === 'tudo')
const id = computed(() => (todos.value || tema.value ? '' : props.escopo))
const doTema = computed(() => (tema.value ? `?tema=${encodeURIComponent(tema.value)}` : ''))
const recado = inject('recado')

const estudo = ref(null)
const erro = ref('')
const mostrando = ref(false)
const enviando = ref(false)
const botaoMostrar = ref(null)
let mostradoEm = Date.now()
let espera

const naTV = ref(lembrar('tv') === '1')

const NOTAS = [
  { valor: 1, nome: 'Errei', classe: 'errei', dica: 'Errei: rever daqui a pouco (1)' },
  { valor: 5, nome: 'Amanhã', classe: 'errei', dica: 'Errei: rever só amanhã (5)' },
  { valor: 2, nome: 'Difícil', classe: '', dica: 'Difícil (2)' },
  { valor: 3, nome: 'Bom', classe: 'principal', dica: 'Bom (3, espaço ou Enter)' },
  { valor: 4, nome: 'Fácil', classe: '', dica: 'Fácil (4)' },
]

const cartao = computed(() => estudo.value?.cartao)
const player = typeof Audio === 'undefined' ? null : new Audio()
// Onde o áudio aparece: com a pergunta (sentido normal) ou com a resposta (invertido).
const audioNaPergunta = computed(() => !!cartao.value?.audio && cartao.value.sentido === 0)
const audioVisivel = computed(() => !!cartao.value?.audio && (audioNaPergunta.value || mostrando.value))

const textoDoFim = computed(() => {
  const e = estudo.value
  if (!e) return ''
  const feitos = e.feitos_hoje
    ? `${e.feitos_hoje} ${e.feitos_hoje === 1 ? 'revisão' : 'revisões'} hoje.`
    : 'Nada para estudar agora.'
  return e.espera_ate ? `${feitos} Um cartão em aprendizado volta às ${hora(e.espera_ate)}.` : feitos
})

// enviarCena: o que a TV mostra agora. Uma falha não atrapalha o estudo no celular.
function enviarCena() {
  if (!naTV.value || !estudo.value) return
  const c = cartao.value
  const cena = {
    ativo: true,
    todos: !id.value,
    baralho: estudo.value.baralho?.nome || '',
    contagem: estudo.value.contagem,
    fila: fila.value,
    ...(c ? { cartao: c.id, frente: c.frente, verso: c.verso, imagem: c.imagem || '', mostrando: mostrando.value } : { fim: textoDoFim.value }),
  }
  api('/api/tv', { metodo: 'PUT', corpo: cena }).catch(() => {})
}

function alternarTV() {
  naTV.value = !naTV.value
  lembrar('tv', naTV.value ? '1' : '0')
  if (naTV.value) enviarCena()
  else api('/api/tv', { metodo: 'PUT', corpo: { ativo: false } }).catch(() => {})
}

function tocar() {
  if (!player || !cartao.value?.audio) return
  player.src = `/api/audios/${cartao.value.audio}`
  // Sem um toque antes, o navegador pode bloquear o som automático: aí fica o botão.
  player.play().catch(() => {})
}
const fila = computed(() => {
  const e = cartao.value?.estado
  if (e === NOVO) return 'novos'
  if (e === APRENDENDO || e === REAPRENDENDO) return 'aprendendo'
  if (e === REVISAO) return 'revisar'
  return ''
})

async function aplicar(dados) {
  estudo.value = dados
  mostrando.value = false
  mostradoEm = Date.now()
  player?.pause()
  if (audioNaPergunta.value) tocar()
  enviarCena()
  clearTimeout(espera)
  // Um cartão aprendendo volta ainda hoje: a tela se atualiza sozinha quando ele vencer.
  if (!dados.cartao && dados.espera_ate) {
    const ms = new Date(dados.espera_ate) - Date.now()
    espera = setTimeout(carregar, Math.max(1000, Math.min(ms - 20 * 60000 + 1000, 2 ** 31 - 1)))
  }
  await nextTick()
  botaoMostrar.value?.focus()
}

async function carregar() {
  try {
    await aplicar(await api(id.value ? `/api/baralhos/${id.value}/estudo` : `/api/estudo${doTema.value}`))
    erro.value = ''
  } catch (e) {
    erro.value = e.status === 0 ? 'Sem conexão: o estudo só funciona na rede de casa.' : e.message
  }
}

function mostrar() {
  mostrando.value = true
  enviarCena()
  if (cartao.value?.audio && !audioNaPergunta.value) tocar()
}

async function responder(nota) {
  if (!cartao.value || !mostrando.value || enviando.value) return
  enviando.value = true
  try {
    await aplicar(
      await api(`/api/cartoes/${cartao.value.id}/responder`, {
        metodo: 'POST',
        corpo: { nota, repeticoes: cartao.value.repeticoes, duracao_ms: Date.now() - mostradoEm, todos: todos.value, tema: tema.value },
      }),
    )
  } catch (e) {
    recado(e.message, true)
    if (e.status === 409) carregar()
  } finally {
    enviando.value = false
  }
}

async function desfazer() {
  if (enviando.value) return
  enviando.value = true
  try {
    await aplicar(await api(id.value ? `/api/baralhos/${id.value}/desfazer` : `/api/estudo/desfazer${doTema.value}`, { metodo: 'POST' }))
    recado('Resposta desfeita.')
  } catch (e) {
    recado(e.status === 404 ? 'Nada para desfazer.' : e.message, e.status !== 404)
  } finally {
    enviando.value = false
  }
}

function voltar() {
  location.hash = id.value ? `#/baralho/${id.value}` : '#/estudar'
}

function aoTeclar(ev) {
  if (ev.target.closest('input, textarea, select') || ev.altKey || ev.metaKey) return
  if (ev.key === 'Escape') return voltar()
  if (ev.key.toLowerCase() === 'z' && !ev.shiftKey) {
    ev.preventDefault()
    return desfazer()
  }
  if (ev.ctrlKey || !cartao.value) return
  if (ev.key.toLowerCase() === 'r' && audioVisivel.value) return tocar()
  if (ev.key === ' ' || ev.key === 'Enter') {
    ev.preventDefault()
    return mostrando.value ? responder(3) : mostrar()
  }
  if (mostrando.value && ['1', '2', '3', '4', '5'].includes(ev.key)) responder(Number(ev.key))
}

onMounted(() => {
  window.addEventListener('keydown', aoTeclar)
  carregar()
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', aoTeclar)
  clearTimeout(espera)
  player?.pause()
  // Fora do estudo, a TV volta para a espera.
  if (naTV.value) api('/api/tv', { metodo: 'PUT', corpo: { ativo: false } }).catch(() => {})
})
</script>

<template>
  <div class="estudo">
    <header class="barra vidro">
      <button class="botao icone" aria-label="Voltar" title="Voltar (Esc)" @click="voltar"><Icone nome="voltar" /></button>
      <div class="onde">
        <span class="baralho"
          ><span v-if="!id" class="modo">{{ tema || 'Todos' }}<template v-if="estudo?.baralho.nome"> · </template></span>{{ estudo?.baralho.nome }}</span
        >
        <span v-if="estudo" class="contagem" aria-label="Para hoje">
          <span class="novos" :class="{ atual: fila === 'novos' }" title="Novos">{{ estudo.contagem.novos }}</span>
          <span class="aprendendo" :class="{ atual: fila === 'aprendendo' }" title="Aprendendo">{{ estudo.contagem.aprendendo }}</span>
          <span class="revisar" :class="{ atual: fila === 'revisar' }" title="Para revisar">{{ estudo.contagem.revisar }}</span>
        </span>
      </div>
      <button
        class="botao icone"
        :class="{ ligado: naTV }"
        :aria-pressed="naTV"
        :aria-label="naTV ? 'Parar de mostrar na TV' : 'Mostrar na TV'"
        :title="naTV ? 'Parar de mostrar na TV' : 'Mostrar na TV'"
        @click="alternarTV"
      >
        <Icone nome="tv" />
      </button>
      <button class="botao icone" aria-label="Desfazer a última resposta" title="Desfazer (Z)" :disabled="!estudo?.feitos_hoje" @click="desfazer">
        <Icone nome="desfazer" />
      </button>
    </header>

    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <Face v-if="cartao" :cartao="cartao" :mostrando="mostrando" :audio-na-pergunta="audioNaPergunta" :aria-busy="enviando" @tocar="tocar" />

    <div v-else-if="estudo" class="fim">
      <p class="fim-titulo">Pronto por hoje.</p>
      <p class="suave">{{ textoDoFim }}</p>
      <!-- Acabada a fila, dá para seguir no aleatório, que não mexe no agendamento. -->
      <div class="linha-acoes centro">
        <a class="botao principal" :href="`#/aleatorio/${escopo}`"><Icone nome="aleatorio" /> Aleatório</a>
        <button class="botao" @click="voltar">Voltar</button>
      </div>
    </div>

    <footer v-if="cartao" class="respostas vidro">
      <button v-if="!mostrando" ref="botaoMostrar" class="botao principal mostrar" @click="mostrar">Mostrar resposta</button>
      <div v-else class="notas">
        <button
          v-for="n in NOTAS"
          :key="n.valor"
          class="botao nota"
          :class="n.classe"
          :disabled="enviando"
          :title="n.dica"
          @click="responder(n.valor)"
        >
          <span class="quando">{{ intervalo(estudo.previsoes[n.valor]) }}</span>
          <span>{{ n.nome }}</span>
        </button>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.estudo {
  min-height: 100vh;
  padding-bottom: calc(140px + env(safe-area-inset-bottom));
}

.barra {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: max(8px, env(safe-area-inset-top)) 12px 8px;
  border-bottom: 1px solid var(--linha);
}

.onde {
  display: flex;
  flex: 1;
  align-items: baseline;
  justify-content: center;
  gap: 14px;
  min-width: 0;
}

.baralho {
  overflow: hidden;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.modo {
  color: var(--texto-fraco);
  font-weight: 500;
}

.contagem {
  display: flex;
  gap: 10px;
  color: var(--texto-fraco);
  font-size: 0.85rem;
  font-variant-numeric: tabular-nums;
}

.contagem .novos {
  color: var(--azul);
}

.contagem .revisar {
  color: var(--vermelho);
}

/* O número da fila do cartão atual fica sublinhado. */
.contagem .atual {
  font-weight: 700;
  text-decoration: underline;
  text-underline-offset: 4px;
}

.ligado {
  color: var(--marca);
  background: var(--marca-fundo);
}

.aviso {
  max-width: 720px;
  margin: 24px auto;
}









.fim {
  max-width: 480px;
  margin: 0 auto;
  padding: 80px 20px;
  text-align: center;
  line-height: 1.55;
}

.fim-titulo {
  margin: 0 0 8px;
  font-family: var(--leitura);
  font-size: 1.6rem;
  font-weight: 600;
}

.fim p {
  margin: 0 0 6px;
}

.centro {
  justify-content: center;
  margin-top: 24px;
}

.respostas {
  position: fixed;
  inset: auto 0 0 0;
  z-index: 10;
  padding: 12px 16px max(12px, env(safe-area-inset-bottom));
  border-top: 1px solid var(--linha);
}

.mostrar,
.notas {
  display: flex;
  width: 100%;
  max-width: 640px;
  margin: 0 auto;
}

.mostrar {
  justify-content: center;
  min-height: 52px;
  font-size: 0.95rem;
}

.notas {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 6px;
}

.nota {
  flex-direction: column;
  justify-content: center;
  gap: 1px;
  min-height: 52px;
  padding: 4px 6px;
  font-size: 0.9rem;
}

.quando {
  color: var(--texto-fraco);
  font-size: 0.72rem;
  font-weight: 500;
}

.nota.principal .quando {
  color: inherit;
  opacity: 0.7;
}

.nota.errei {
  color: var(--vermelho);
}
</style>
