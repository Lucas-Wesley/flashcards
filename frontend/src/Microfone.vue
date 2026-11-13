<script setup>
import { inject, onBeforeUnmount, ref } from 'vue'
import Icone from './Icone.vue'

// Falar em vez de escrever (F39): grava no aparelho e o servidor transcreve com o Whisper do Mac
// mini. O texto volta para o campo, onde dá para conferir e corrigir antes de mandar à IA.
const props = defineProps({ contexto: { type: String, default: '' } }) // o assunto, para o Whisper acertar os nomes (F43)
const emit = defineEmits(['texto'])
const recado = inject('recado')

const estado = ref('parado') // parado | gravando | transcrevendo
const segundos = ref(0)
let gravador, pedacos, relogio

const suportado = typeof window !== 'undefined' && !!navigator.mediaDevices?.getUserMedia && typeof MediaRecorder !== 'undefined'

async function alternar() {
  if (estado.value === 'gravando') return gravador.stop()
  if (estado.value !== 'parado') return
  let fluxo
  try {
    fluxo = await navigator.mediaDevices.getUserMedia({ audio: true })
  } catch {
    recado('Sem acesso ao microfone. Permita o microfone para este site no Chrome.', true)
    return
  }
  pedacos = []
  gravador = new MediaRecorder(fluxo)
  gravador.ondataavailable = (ev) => ev.data.size && pedacos.push(ev.data)
  gravador.onstop = () => {
    fluxo.getTracks().forEach((t) => t.stop())
    clearInterval(relogio)
    transcrever(new Blob(pedacos, { type: gravador.mimeType || 'audio/webm' }))
  }
  gravador.start()
  segundos.value = 0
  relogio = setInterval(() => segundos.value++, 1000)
  estado.value = 'gravando'
}

async function transcrever(audio) {
  if (!audio.size) {
    estado.value = 'parado'
    return
  }
  estado.value = 'transcrevendo'
  const corpo = new FormData()
  corpo.append('audio', audio, audio.type.includes('mp4') ? 'fala.mp4' : 'fala.webm')
  if (props.contexto) corpo.append('contexto', props.contexto)
  try {
    const resposta = await fetch('/api/transcrever', { method: 'POST', body: corpo, signal: AbortSignal.timeout(300000) })
    const r = await resposta.json().catch(() => ({}))
    if (!resposta.ok) throw new Error(r.erro || `Erro ${resposta.status}`)
    emit('texto', r.texto)
  } catch (e) {
    recado(e.name === 'TypeError' ? 'Sem conexão com o servidor.' : e.message, true)
  } finally {
    estado.value = 'parado'
  }
}

const tempo = () => `${Math.floor(segundos.value / 60)}:${String(segundos.value % 60).padStart(2, '0')}`

onBeforeUnmount(() => {
  clearInterval(relogio)
  if (gravador?.state === 'recording') {
    gravador.onstop = () => gravador.stream.getTracks().forEach((t) => t.stop())
    gravador.stop()
  }
})
</script>

<template>
  <button
    v-if="suportado"
    type="button"
    class="microfone"
    :class="estado"
    :disabled="estado === 'transcrevendo'"
    :aria-label="estado === 'gravando' ? 'Parar e transcrever' : 'Falar em vez de escrever'"
    :title="estado === 'gravando' ? 'Parar e transcrever' : 'Falar em vez de escrever'"
    @click="alternar"
  >
    <span v-if="estado === 'transcrevendo'" class="girando" aria-hidden="true" />
    <Icone v-else :nome="estado === 'gravando' ? 'parar' : 'microfone'" />
    <span v-if="estado === 'gravando'" class="tempo">{{ tempo() }}</span>
    <span v-else-if="estado === 'transcrevendo'" class="tempo">Transcrevendo…</span>
  </button>
</template>

<style scoped>
.microfone {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 40px;
  height: 40px;
  padding: 0 9px;
  border: 0;
  border-radius: 999px;
  background: var(--superficie);
  color: var(--texto-suave);
  font: inherit;
  font-size: 0.82rem;
  cursor: pointer;
  transition:
    background 0.15s ease,
    color 0.15s ease;
}

.microfone:hover {
  color: var(--marca);
}

.microfone.gravando {
  padding-right: 14px;
  background: var(--perigo-fundo);
  color: var(--perigo);
  animation: pulso 1.6s ease-in-out infinite;
}

.microfone.transcrevendo {
  padding-right: 14px;
  cursor: progress;
}

.tempo {
  font-variant-numeric: tabular-nums;
  font-weight: 500;
}

.girando {
  width: 18px;
  height: 18px;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: giro 0.8s linear infinite;
}

@keyframes pulso {
  50% {
    box-shadow: 0 0 0 6px color-mix(in srgb, var(--perigo) 15%, transparent);
  }
}

@keyframes giro {
  to {
    transform: rotate(360deg);
  }
}
</style>
