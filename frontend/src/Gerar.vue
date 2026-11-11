<script setup>
import { computed, inject, nextTick, onMounted, ref } from 'vue'
import AbasAdicionar from './AbasAdicionar.vue'
import Info from './Info.vue'
import Microfone from './Microfone.vue'
import { api } from './api.js'

// Pedir um baralho à IA. O pedido entra na fila; a proposta volta para revisão em Geracao.vue.
// O pedido pode ser falado (F39); a quantidade é opcional: em branco, a IA decide pelo pedido.
const props = defineProps({ baralho: { type: String, default: '' } })
const recado = inject('recado')

const baralhos = ref([])
const pedido = ref('')
const quantidade = ref('')
const tipo = ref('basico')
const destino = ref(props.baralho)
const enviando = ref(false)
const campo = ref(null)

const resumoDasOpcoes = computed(() =>
  [tipo.value === 'invertido' ? 'nos dois sentidos' : 'frente e verso', quantidade.value ? `cerca de ${quantidade.value}` : 'a IA decide quantos'].join(' · '),
)

function ouvido(texto) {
  pedido.value = pedido.value.trim() ? `${pedido.value.trim()} ${texto}` : texto
  nextTick(() => campo.value?.focus())
}

async function pedir() {
  if (!pedido.value.trim() || enviando.value) return
  enviando.value = true
  try {
    const g = await api('/api/geracoes', {
      metodo: 'POST',
      corpo: { pedido: pedido.value, quantidade: Number(quantidade.value) || 0, tipo: tipo.value, baralho_id: destino.value },
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

    <form class="formulario" @submit.prevent="pedir">
      <div class="pedido">
        <span class="rotulo-campo"
          >O que você quer estudar?
          <Info
            >Escreva ou fale (no microfone) o assunto e, se quiser, como é cada cartão: o que vai na frente, o que vai no verso. A IA pesquisa, monta os cartões e devolve uma
            proposta para você revisar antes de qualquer cartão entrar no baralho. Leva alguns minutos.
            <a href="#/ajustes/instrucoes/gerar-baralho.md">Ver as instruções que a IA segue</a></Info
          ></span
        >
        <div class="caixa">
          <textarea
            ref="campo"
            v-model="pedido"
            class="campo"
            rows="5"
            placeholder="Ex.: as bandeiras dos países do mundo. Na frente, a bandeira; no verso, o nome do país e um mapa de onde ele fica."
            aria-label="Pedido"
            @keydown.ctrl.enter="pedir"
            @keydown.meta.enter="pedir"
          />
          <Microfone class="falar" @texto="ouvido" />
        </div>
      </div>

      <label class="destino">
        <span class="rotulo-campo">Baralho</span>
        <select v-model="destino" class="campo" aria-label="Baralho de destino">
          <option value="">Um baralho novo</option>
          <option v-for="b in baralhos" :key="b.id" :value="b.id">{{ b.nome }}</option>
        </select>
      </label>

      <details class="opcoes">
        <summary>
          Opções <span class="resumo">{{ resumoDasOpcoes }}</span>
        </summary>
        <div class="dentro">
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
          <label class="quantos">
            <span class="rotulo-campo"
              >Quantos cartões
              <Info
                >Opcional. Em branco, a IA decide pelo pedido: num conjunto completo (os estados, os países), todos; num assunto aberto, o que vale lembrar. Com um número, ela o usa
                como referência, mas não deixa um conjunto pela metade por causa dele.</Info
              ></span
            >
            <input v-model="quantidade" class="campo" type="number" min="1" max="300" placeholder="A IA decide" aria-label="Quantidade" />
          </label>
        </div>
      </details>

      <div>
        <button class="botao principal" :disabled="enviando || !pedido.trim()">{{ enviando ? 'Enviando…' : 'Pedir à IA' }}</button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.formulario {
  display: grid;
  gap: 18px;
  max-width: 640px;
}

.pedido,
.destino,
.tipo,
.quantos {
  display: grid;
  gap: 6px;
}

.rotulo-campo {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-height: 24px;
  color: var(--texto-suave);
  font-size: 0.82rem;
  font-weight: 500;
}

.caixa {
  position: relative;
}

.caixa textarea {
  display: block;
  width: 100%;
  padding-bottom: 52px;
  font-family: var(--leitura);
  font-size: 1.02rem;
  line-height: 1.55;
  resize: vertical;
  field-sizing: content;
  min-height: 9em;
}

.falar {
  position: absolute;
  right: 8px;
  bottom: 8px;
}

select.campo {
  appearance: auto;
}

.opcoes summary {
  display: flex;
  align-items: baseline;
  gap: 8px;
  width: fit-content;
  color: var(--texto-suave);
  font-size: 0.85rem;
  font-weight: 500;
  cursor: pointer;
  list-style: none;
}

.opcoes summary::-webkit-details-marker {
  display: none;
}

.opcoes summary::before {
  content: '›';
  display: inline-block;
  transition: transform 0.15s ease;
}

.opcoes[open] summary::before {
  transform: rotate(90deg);
}

.resumo {
  color: var(--texto-fraco);
  font-weight: 400;
}

.dentro {
  display: grid;
  grid-template-columns: auto minmax(0, 180px);
  gap: 16px;
  margin-top: 14px;
}

.pilulas {
  display: flex;
  gap: 4px;
}

@media (max-width: 600px) {
  .dentro {
    grid-template-columns: 1fr;
  }
}
</style>
