<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import Face from './Face.vue'
import Icone from './Icone.vue'
import { api } from './api.js'

// Estudar aleatório (F38): cartões sorteados do baralho, do tema ou de todos, a qualquer hora. Não há
// nota: "Próximo" só passa o cartão, e o agendamento do FSRS fica como está. Acabada uma leva, vem
// outra. Teclado: espaço ou Enter mostra a resposta e depois passa; R repete o áudio; Esc volta.
const props = defineProps({ escopo: { type: String, required: true } }) // id, "tema/<nome>" ou "tudo"
const tema = computed(() => (props.escopo.startsWith('tema/') ? props.escopo.slice('tema/'.length) : ''))
const id = computed(() => (props.escopo === 'tudo' || tema.value ? '' : props.escopo))

const leva = ref([])
const pos = ref(0)
const vistos = ref(0)
const mostrando = ref(false)
const erro = ref('')
const carregado = ref(false)
const botao = ref(null)

const cartao = computed(() => leva.value[pos.value])
const player = typeof Audio === 'undefined' ? null : new Audio()
const audioNaPergunta = computed(() => !!cartao.value?.audio && cartao.value.sentido === 0)

function tocar() {
  if (!player || !cartao.value?.audio) return
  player.src = `/api/audios/${cartao.value.audio}`
  player.play().catch(() => {})
}

async function sortear() {
  const q = id.value ? `?baralho=${encodeURIComponent(id.value)}` : tema.value ? `?tema=${encodeURIComponent(tema.value)}` : ''
  try {
    leva.value = await api(`/api/aleatorio${q}`)
    pos.value = 0
    erro.value = ''
  } catch (e) {
    erro.value = e.status === 0 ? 'Sem conexão: o estudo só funciona na rede de casa.' : e.message
  }
  carregado.value = true
  aoMudar()
}

async function aoMudar() {
  mostrando.value = false
  player?.pause()
  if (audioNaPergunta.value) tocar()
  await nextTick()
  botao.value?.focus()
}

function mostrar() {
  mostrando.value = true
  if (cartao.value?.audio && !audioNaPergunta.value) tocar()
}

function proximo() {
  vistos.value++
  if (pos.value + 1 < leva.value.length) {
    pos.value++
    aoMudar()
  } else {
    sortear()
  }
}

function voltar() {
  location.hash = id.value ? `#/baralho/${id.value}` : '#/estudar'
}

function aoTeclar(ev) {
  if (ev.target.closest('input, textarea, select') || ev.altKey || ev.metaKey || ev.ctrlKey) return
  if (ev.key === 'Escape') return voltar()
  if (!cartao.value) return
  if (ev.key.toLowerCase() === 'r') return tocar()
  if (ev.key === ' ' || ev.key === 'Enter') {
    ev.preventDefault()
    mostrando.value ? proximo() : mostrar()
  }
}

onMounted(() => {
  window.addEventListener('keydown', aoTeclar)
  sortear()
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', aoTeclar)
  player?.pause()
})
</script>

<template>
  <div class="aleatorio">
    <header class="barra vidro">
      <button class="botao icone" aria-label="Voltar" title="Voltar (Esc)" @click="voltar"><Icone nome="voltar" /></button>
      <div class="onde">
        <span class="modo"><Icone nome="aleatorio" /> Aleatório</span>
        <span v-if="cartao" class="baralho">{{ cartao.baralho }}</span>
      </div>
      <span class="vistos" :aria-label="`${vistos} vistos`">{{ vistos }}</span>
    </header>

    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <Face v-if="cartao" :cartao="cartao" :mostrando="mostrando" :audio-na-pergunta="audioNaPergunta" @tocar="tocar" />
    <p v-else-if="carregado && !erro" class="vazio">Nenhum cartão aqui.</p>

    <footer v-if="cartao" class="respostas vidro">
      <button v-if="!mostrando" ref="botao" class="botao principal grande" @click="mostrar">Mostrar resposta</button>
      <button v-else ref="botao" class="botao grande" @click="proximo">Próximo</button>
    </footer>
  </div>
</template>

<style scoped>
.aleatorio {
  min-height: 100vh;
  padding-bottom: calc(110px + env(safe-area-inset-bottom));
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
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-width: 0;
}

.modo {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 5px;
  color: var(--marca);
  font-size: 0.85rem;
  font-weight: 600;
}

.modo :deep(svg) {
  width: 17px;
  height: 17px;
}

.baralho {
  overflow: hidden;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.vistos {
  width: 34px;
  color: var(--texto-fraco);
  font-size: 0.85rem;
  font-variant-numeric: tabular-nums;
  text-align: center;
}

.aviso {
  max-width: 720px;
  margin: 24px auto;
}

.respostas {
  position: fixed;
  inset: auto 0 0 0;
  z-index: 10;
  padding: 12px 16px max(12px, env(safe-area-inset-bottom));
  border-top: 1px solid var(--linha);
}

.grande {
  display: flex;
  justify-content: center;
  width: 100%;
  max-width: 640px;
  min-height: 52px;
  margin: 0 auto;
  font-size: 0.95rem;
}
</style>
