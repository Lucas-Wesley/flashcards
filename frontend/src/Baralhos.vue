<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from './api.js'

// Só os baralhos e o Estudar. Criar um baralho fica em Adicionar; os pedidos à IA, em Pedidos.
const baralhos = ref(null)
const erro = ref('')

async function carregar() {
  try {
    baralhos.value = await api('/api/baralhos')
    erro.value = ''
  } catch (e) {
    erro.value = e.status === 0 ? 'Sem conexão: os baralhos só abrem na rede de casa.' : e.message
  }
}

const total = (b) => b.contagem.novos + b.contagem.aprendendo + b.contagem.revisar
// O "Estudar" do topo junta todos os baralhos numa fila só.
const totalGeral = computed(() => (baralhos.value || []).reduce((soma, b) => soma + total(b), 0))

onMounted(carregar)
</script>

<template>
  <div class="pagina">
    <div class="topo">
      <h1>Baralhos</h1>
      <!-- Sempre à vista; sem nada para hoje, fica discreto e abre o "Pronto por hoje". -->
      <a
        v-if="baralhos?.length"
        class="botao estudar-tudo"
        :class="{ principal: totalGeral }"
        href="#/estudar"
        :title="totalGeral ? 'Todos os baralhos, misturados' : 'Nada para estudar agora'"
        >Estudar <span class="total">{{ totalGeral }}</span></a
      >
    </div>

    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <template v-if="baralhos">
      <ul v-if="baralhos.length" class="lista">
        <li v-for="b in baralhos" :key="b.id">
          <a class="baralho" :href="`#/baralho/${b.id}`">
            <span class="info">
              <span class="nome">{{ b.nome }}</span>
              <span class="meta">
                {{ b.cartoes === 1 ? '1 cartão' : `${b.cartoes} cartões` }}<template v-if="b.feitos_hoje"> · {{ b.feitos_hoje }} {{ b.feitos_hoje === 1 ? 'revisão' : 'revisões' }} hoje</template>
              </span>
              <!-- Cada número com o nome ao lado; os zerados não aparecem. -->
              <span class="numeros" :aria-label="`${b.contagem.novos} novos, ${b.contagem.aprendendo} aprendendo, ${b.contagem.revisar} para revisar`">
                <span v-if="b.contagem.novos" class="numero novos">{{ b.contagem.novos }} {{ b.contagem.novos === 1 ? 'novo' : 'novos' }}</span>
                <span v-if="b.contagem.aprendendo" class="numero aprendendo">{{ b.contagem.aprendendo }} aprendendo</span>
                <span v-if="b.contagem.revisar" class="numero revisar">{{ b.contagem.revisar }} para revisar</span>
                <span v-if="!total(b) && b.cartoes" class="nada">Nada para hoje</span>
              </span>
            </span>
          </a>
          <a v-if="total(b)" class="botao estudar" :href="`#/estudar/${b.id}`">Estudar</a>
        </li>
      </ul>
      <p v-else class="vazio">
        Nenhum baralho ainda.<br />
        <a href="#/novo-baralho">Crie o primeiro</a>, um por assunto, como “Inglês” ou “História”, ou <a href="#/gerar">peça à IA</a> um pronto.
      </p>

    </template>
  </div>
</template>

<style scoped>
.topo {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 20px;
}

.topo h1 {
  margin-bottom: 0;
}

.estudar-tudo {
  flex-shrink: 0;
}

.estudar-tudo .total {
  font-variant-numeric: tabular-nums;
  opacity: 0.75;
}

.lista li {
  display: flex;
  align-items: center;
  gap: 12px;
}

.baralho {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 16px;
  min-width: 0;
  padding: 16px 0;
  color: inherit;
  text-decoration: none;
}

.info {
  display: grid;
  flex: 1;
  gap: 3px;
  min-width: 0;
}

.nome {
  overflow: hidden;
  font-size: 1.06rem;
  font-weight: 650;
  letter-spacing: -0.012em;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color 0.15s ease;
}

.baralho:hover .nome {
  color: var(--marca);
}

.meta {
  color: var(--texto-fraco);
  font-size: 0.8rem;
}

.numeros {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 5px;
  font-variant-numeric: tabular-nums;
}

.numero {
  padding: 2px 9px;
  border-radius: 999px;
  font-size: 0.78rem;
  font-weight: 600;
}

/* Novos em azul (informação); revisões vencidas em vermelho (pedem atenção). */
.numero.novos {
  background: var(--azul-fundo);
  color: var(--azul);
}

.numero.aprendendo {
  background: var(--superficie-forte);
  color: var(--texto-suave);
}

.numero.revisar {
  background: var(--vermelho-fundo);
  color: var(--vermelho);
}

.nada {
  color: var(--texto-fraco);
  font-size: 0.78rem;
}

.aviso {
  margin-bottom: 16px;
}

@media (max-width: 820px) {
  .estudar {
    display: none;
  }
}
</style>
