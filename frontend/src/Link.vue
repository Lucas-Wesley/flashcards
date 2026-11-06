<script setup>
import { inject, onMounted, ref } from 'vue'
import AbasAdicionar from './AbasAdicionar.vue'
import { api, lembrar } from './api.js'

// Cartões a partir de um link (por enquanto, do site do Mairo Vergara): o servidor lê as frases
// de exemplo com a tradução e o áudio e monta uma proposta para revisar. Chega aqui também pelo
// "Compartilhar" do celular, com o link já preenchido.
const props = defineProps({ link: { type: String, default: '' } })
const recado = inject('recado')

const baralhos = ref([])
const url = ref(props.link)
const destino = ref('')
const tipo = ref('basico')
const lendo = ref(false)

async function ler() {
  if (!url.value.trim() || lendo.value) return
  lendo.value = true
  try {
    const g = await api('/api/links', { metodo: 'POST', corpo: { url: url.value.trim(), baralho_id: destino.value, tipo: tipo.value }, tempo: 30000 })
    lembrar('baralho-link', destino.value || 'novo')
    location.hash = `#/geracao/${g.id}`
  } catch (e) {
    recado(e.message, true)
  } finally {
    lendo.value = false
  }
}

onMounted(async () => {
  try {
    baralhos.value = await api('/api/baralhos')
  } catch {
    // Sem a lista, vai para um baralho novo.
  }
  const ultimo = lembrar('baralho-link')
  if (baralhos.value.some((b) => b.id === ultimo)) destino.value = ultimo
})
</script>

<template>
  <div class="pagina">
    <h1>Adicionar</h1>
    <AbasAdicionar atual="link" />
    <p class="suave explicacao">
      Cole o link de um post do <strong>Mairo Vergara</strong>. Das frases de exemplo, as <strong>3 mais didáticas</strong> viram cartões (quando há
      mais que isso, a IA escolhe, sem mudar nada nelas): a frase em inglês na frente, com o áudio, e a tradução no verso. Você revisa antes de entrarem no baralho. No celular, dá para usar o <strong>Compartilhar</strong> do navegador direto
      para o app.
    </p>

    <form class="formulario" @submit.prevent="ler">
      <label>
        <span class="rotulo-campo">Link</span>
        <input v-model="url" class="campo" type="url" inputmode="url" placeholder="https://www.mairovergara.com/…" aria-label="Link" />
      </label>
      <div class="dupla">
        <label>
          <span class="rotulo-campo">Para qual baralho</span>
          <select v-model="destino" class="campo" aria-label="Baralho de destino">
            <option value="">Um baralho novo</option>
            <option v-for="b in baralhos" :key="b.id" :value="b.id">{{ b.nome }}</option>
          </select>
        </label>
        <div class="tipo">
          <span class="rotulo-campo">Tipo</span>
          <div class="pilulas" role="radiogroup" aria-label="Tipo de cartão">
            <button type="button" class="pilula" role="radio" :class="{ ativa: tipo === 'basico' }" :aria-checked="tipo === 'basico'" @click="tipo = 'basico'">
              Inglês → português
            </button>
            <button type="button" class="pilula" role="radio" :class="{ ativa: tipo === 'invertido' }" :aria-checked="tipo === 'invertido'" @click="tipo = 'invertido'">
              Nos dois sentidos
            </button>
          </div>
        </div>
      </div>
      <div class="linha-acoes">
        <button class="botao principal" :disabled="lendo || !url.trim()">{{ lendo ? 'Lendo a página…' : 'Montar cartões' }}</button>
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
