<script setup>
import { inject, nextTick, onMounted, ref } from 'vue'
import AbasAdicionar from './AbasAdicionar.vue'
import { api } from './api.js'

// Pedir um baralho à IA. O pedido entra na fila; a proposta volta para revisão em Geracao.vue.
const props = defineProps({ baralho: { type: String, default: '' } })
const recado = inject('recado')

const baralhos = ref([])
const pedido = ref('')
const quantidade = ref(20)
const tipo = ref('basico')
const destino = ref(props.baralho)
const enviando = ref(false)
const campo = ref(null)

async function pedir() {
  if (!pedido.value.trim() || enviando.value) return
  enviando.value = true
  try {
    const g = await api('/api/geracoes', {
      metodo: 'POST',
      corpo: { pedido: pedido.value, quantidade: Number(quantidade.value), tipo: tipo.value, baralho_id: destino.value },
    })
    location.hash = `#/geracao/${g.id}`
  } catch (e) {
    recado(e.message, true)
  } finally {
    enviando.value = false
  }
}

onMounted(async () => {
  await nextTick()
  campo.value?.focus()
  try {
    baralhos.value = await api('/api/baralhos')
  } catch {
    // Sem a lista, o pedido vai para um baralho novo.
  }
})
</script>

<template>
  <div class="pagina">
    <h1>Adicionar</h1>
    <AbasAdicionar atual="gerar" />
    <p class="suave explicacao">
      Diga o que quer estudar. A IA pesquisa, monta os cartões e devolve uma proposta: você edita, tira o que não quiser e aprova antes de
      qualquer cartão entrar no baralho. Leva alguns minutos.
    </p>

    <form class="formulario" @submit.prevent="pedir">
      <label>
        <span class="rotulo-campo">O que você quer estudar?</span>
        <textarea
          ref="campo"
          v-model="pedido"
          class="campo"
          rows="4"
          placeholder="Ex.: os presidentes do Brasil no século 21: quem foi, quando governou e um fato marcante de cada um"
          aria-label="Pedido"
          @keydown.ctrl.enter="pedir"
          @keydown.meta.enter="pedir"
        />
      </label>

      <div class="dupla">
        <label>
          <span class="rotulo-campo">Quantos cartões, mais ou menos</span>
          <input v-model="quantidade" class="campo" type="number" min="1" max="100" aria-label="Quantidade" />
        </label>
        <label>
          <span class="rotulo-campo">Para qual baralho</span>
          <select v-model="destino" class="campo" aria-label="Baralho de destino">
            <option value="">Um baralho novo</option>
            <option v-for="b in baralhos" :key="b.id" :value="b.id">{{ b.nome }}</option>
          </select>
        </label>
      </div>

      <div class="tipo">
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

      <div class="linha-acoes">
        <button class="botao principal" :disabled="enviando || !pedido.trim()">{{ enviando ? 'Enviando…' : 'Pedir à IA' }}</button>
        <a class="suave instrucoes" href="#/ajustes/instrucoes/gerar-baralho.md">Ver as instruções que a IA segue</a>
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

.formulario > label,
.tipo {
  display: grid;
  gap: 6px;
}

.formulario textarea {
  line-height: 1.5;
  resize: vertical;
}

.dupla {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.dupla label {
  display: grid;
  gap: 6px;
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
  gap: 4px;
}

.instrucoes {
  font-size: 0.82rem;
}

@media (max-width: 820px) {
  .dupla {
    grid-template-columns: 1fr;
  }
}
</style>
