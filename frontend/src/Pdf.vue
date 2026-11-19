<script setup>
import { inject, onMounted, ref } from 'vue'
import AbasAdicionar from './AbasAdicionar.vue'
import { api, lembrar } from './api.js'

// Cartões de um PDF que já os traz (F46): frases com tradução, frente e verso, listas de um curso.
// O servidor tira o texto do PDF e a IA passa os cartões como estão; vira uma proposta para revisar.
const recado = inject('recado')

const baralhos = ref([])
const arquivo = ref(null)
const destino = ref('')
const nomeNovo = ref('')
const tipo = ref('basico')
const instrucao = ref('')
const enviando = ref(false)

function escolher(ev) {
  arquivo.value = ev.target.files[0] || null
}

async function enviar() {
  if (!arquivo.value || enviando.value) return
  enviando.value = true
  const corpo = new FormData()
  corpo.append('arquivo', arquivo.value)
  corpo.append('baralho_id', destino.value)
  corpo.append('baralho', nomeNovo.value)
  corpo.append('tipo', tipo.value)
  corpo.append('instrucao', instrucao.value)
  try {
    const resposta = await fetch('/api/pdfs', { method: 'POST', body: corpo, signal: AbortSignal.timeout(180000) })
    const r = await resposta.json().catch(() => ({}))
    if (!resposta.ok) throw new Error(r.erro || `Erro ${resposta.status}`)
    lembrar('baralho-pdf', destino.value || 'novo')
    location.hash = `#/geracao/${r.id}`
  } catch (e) {
    recado(e.message === 'Failed to fetch' ? 'Sem conexão com o servidor.' : e.message, true)
  } finally {
    enviando.value = false
  }
}

onMounted(async () => {
  try {
    baralhos.value = await api('/api/baralhos')
  } catch {
    // Sem a lista, vai para um baralho novo.
  }
  const ultimo = lembrar('baralho-pdf')
  if (ultimo === 'novo') return
  const semAcento = (s) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').trim().toLowerCase()
  destino.value = (baralhos.value.find((b) => b.id === ultimo) ?? baralhos.value.find((b) => semAcento(b.nome) === 'ingles'))?.id ?? ''
})
</script>

<template>
  <div class="pagina">
    <h1>Adicionar</h1>
    <AbasAdicionar atual="pdf" />
    <p class="suave explicacao">
      Suba um PDF que já traz cartões: frases com a tradução, frente e verso, listas de um curso. A IA passa os cartões como estão no PDF (a
      nota que explica uma frase vai no verso dela), sem inventar nada, e você revisa antes de entrarem no baralho. PDF escaneado (só imagem)
      não dá para ler.
    </p>

    <form class="formulario" @submit.prevent="enviar">
      <label>
        <span class="rotulo-campo">Arquivo PDF</span>
        <input class="campo" type="file" accept="application/pdf,.pdf" aria-label="Arquivo PDF" @change="escolher" />
      </label>
      <div class="dupla">
        <label>
          <span class="rotulo-campo">Para qual baralho</span>
          <select v-model="destino" class="campo" aria-label="Baralho de destino">
            <option value="">Um baralho novo</option>
            <option v-for="b in baralhos" :key="b.id" :value="b.id">{{ b.nome }}</option>
          </select>
        </label>
        <label v-if="!destino">
          <span class="rotulo-campo">Nome do baralho novo</span>
          <input v-model="nomeNovo" class="campo" aria-label="Nome do baralho novo" />
        </label>
        <div v-else class="tipo">
          <span class="rotulo-campo">Tipo</span>
          <div class="pilulas" role="radiogroup" aria-label="Tipo de cartão">
            <button type="button" class="pilula" role="radio" :class="{ ativa: tipo === 'basico' }" :aria-checked="tipo === 'basico'" @click="tipo = 'basico'">
              Frente e verso
            </button>
            <button type="button" class="pilula" role="radio" :class="{ ativa: tipo === 'invertido' }" :aria-checked="tipo === 'invertido'" @click="tipo = 'invertido'">
              Nos dois sentidos
            </button>
          </div>
        </div>
      </div>
      <label>
        <span class="rotulo-campo">Algo a mais (opcional)</span>
        <textarea v-model="instrucao" class="campo" rows="2" maxlength="3000" placeholder="Ex.: só as frases da lição; sem os exemplos das notas" aria-label="Algo a mais" />
      </label>
      <div class="linha-acoes">
        <button class="botao principal" :disabled="enviando || !arquivo || (!destino && !nomeNovo.trim())">{{ enviando ? 'Lendo o PDF…' : 'Tirar os cartões' }}</button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.explicacao {
  margin: 0 0 20px;
  font-size: 0.9rem;
  line-height: 1.55;
}

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

@media (max-width: 820px) {
  .dupla {
    grid-template-columns: 1fr;
  }
}
</style>
