<script setup>
import { computed, onMounted, ref } from 'vue'
import Numeros from './Numeros.vue'
import { api } from './api.js'
import { agrupar, totalDe } from './temas.js'

// Os baralhos, agrupados por tema (F38), e o Estudar, que abre a escolha do tema. Criar um baralho
// fica em Adicionar; os pedidos à IA, em Pedidos.
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

// Os temas primeiro, cada um com os baralhos dele; os sem tema, juntos no fim.
const secoes = computed(() => {
  const grupos = agrupar(baralhos.value || [])
  const soltos = grupos.filter((g) => !g.tema).flatMap((g) => g.baralhos)
  return [...grupos.filter((g) => g.tema), ...(soltos.length ? [{ nome: '', escopo: 'soltos', baralhos: soltos }] : [])]
})
const totalGeral = computed(() => (baralhos.value || []).reduce((soma, b) => soma + totalDe(b.contagem), 0))

onMounted(carregar)
</script>

<template>
  <div class="pagina">
    <div class="topo">
      <h1>Baralhos</h1>
      <a v-if="baralhos?.length" class="botao estudar" :class="{ principal: totalGeral }" href="#/estudar">
        Estudar <span v-if="totalGeral" class="total">{{ totalGeral }}</span>
      </a>
    </div>

    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <template v-if="baralhos">
      <section v-for="s in secoes" :key="s.escopo" class="secao">
        <h2 v-if="s.nome" class="tema">{{ s.nome }}</h2>
        <h2 v-else-if="secoes.length > 1" class="tema sem-tema">Sem tema</h2>
        <ul class="lista">
          <li v-for="b in s.baralhos" :key="b.id">
            <a class="baralho" :href="`#/baralho/${b.id}`">
              <span class="nome">{{ b.nome }}</span>
              <Numeros v-if="b.cartoes" :contagem="b.contagem" />
              <span v-else class="vazio-baralho">vazio</span>
            </a>
          </li>
        </ul>
      </section>
      <p v-if="!baralhos.length" class="vazio">Nenhum baralho ainda.<br /><a href="#/novo-baralho">Crie o primeiro</a> ou <a href="#/gerar">peça à IA</a>.</p>
    </template>
  </div>
</template>

<style scoped>
.topo {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.topo h1 {
  margin-bottom: 0;
}

.estudar {
  flex-shrink: 0;
}

.total {
  font-variant-numeric: tabular-nums;
  opacity: 0.75;
}

.secao {
  margin-top: 20px;
}

.tema {
  margin: 0 0 6px;
  color: var(--texto-suave);
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.sem-tema {
  color: var(--texto-fraco);
}

.baralho {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 14px 0;
  color: inherit;
  text-decoration: none;
}

.nome {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  font-size: 1.02rem;
  font-weight: 600;
  letter-spacing: -0.01em;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color 0.15s ease;
}

.baralho:hover .nome {
  color: var(--marca);
}

.vazio-baralho {
  color: var(--texto-fraco);
  font-size: 0.8rem;
}

.aviso {
  margin-bottom: 16px;
}
</style>
