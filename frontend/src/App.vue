<script setup>
import { computed, onBeforeUnmount, onMounted, provide, ref } from 'vue'
import Ajustes from './Ajustes.vue'
import Baralho from './Baralho.vue'
import Baralhos from './Baralhos.vue'
import Cartoes from './Cartoes.vue'
import Editor from './Editor.vue'
import Estudo from './Estudo.vue'
import Geracao from './Geracao.vue'
import Gerar from './Gerar.vue'
import Icone from './Icone.vue'
import Importar from './Importar.vue'
import Instrucao from './Instrucao.vue'
import { api } from './api.js'

// Rotas no hash, como na Biblioteca e no Acervo: o "voltar" do celular volta para a tela anterior.
const hash = ref(location.hash)
const recado = ref(null) // { texto, erro } — aviso rápido
const versao = ref('')
const pendentes = ref(0)

const rota = computed(() => {
  const h = decodeURIComponent(hash.value.replace(/^#\/?/, ''))
  const [nome, ...resto] = h.split('/')
  return { nome: nome || 'baralhos', param: resto.join('/') }
})

const MENU = [
  { rota: 'baralhos', nome: 'Baralhos', icone: 'baralhos', contador: true },
  { rota: 'adicionar', nome: 'Adicionar', icone: 'mais' },
  { rota: 'cartoes', nome: 'Cartões', icone: 'cartoes' },
  { rota: 'ajustes', nome: 'Ajustes', icone: 'ajustes' },
]

const secao = computed(() => {
  const n = rota.value.nome
  if (['baralho', 'importar', 'gerar', 'geracao'].includes(n)) return 'baralhos'
  if (n === 'nota') return 'cartoes'
  return n
})

function ir(destino) {
  location.hash = `#/${destino}`
}

// A tela de onde se foi para Ajustes: vai junto com a sugestão, para dar contexto.
const telaAnterior = ref('')

function aoMudarHash() {
  if (location.hash.startsWith('#/ajustes') && !hash.value.startsWith('#/ajustes')) telaAnterior.value = hash.value || '#/baralhos'
  hash.value = location.hash
  atualizarPendentes()
}

// O contador do menu: revisões e cartões aprendendo para hoje, somando os baralhos.
let relogio
async function atualizarPendentes() {
  if (document.visibilityState !== 'visible') return
  try {
    const baralhos = await api('/api/baralhos', { tempo: 5000 })
    pendentes.value = baralhos.reduce((soma, b) => soma + b.contagem.revisar + b.contagem.aprendendo, 0)
  } catch {
    // Sem rede: o contador fica como estava.
  }
}

let tempoDoRecado
function mostrarRecado(texto, erro = false) {
  recado.value = { texto, erro }
  clearTimeout(tempoDoRecado)
  tempoDoRecado = setTimeout(() => (recado.value = null), 4000)
}
provide('recado', mostrarRecado)

onMounted(async () => {
  window.addEventListener('hashchange', aoMudarHash)
  document.addEventListener('visibilitychange', atualizarPendentes)
  relogio = setInterval(atualizarPendentes, 60000)
  atualizarPendentes()
  try {
    versao.value = (await api('/api/health', { tempo: 4000 })).version
  } catch {
    versao.value = ''
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('hashchange', aoMudarHash)
  document.removeEventListener('visibilitychange', atualizarPendentes)
  clearInterval(relogio)
})
</script>

<template>
  <div class="app" :class="{ estudando: rota.nome === 'estudar' }">
    <aside class="menu" aria-label="Menu">
      <div class="marca">Flashcards</div>
      <nav>
        <button
          v-for="item in MENU"
          :key="item.rota"
          class="item"
          :class="{ ativo: secao === item.rota }"
          :aria-current="secao === item.rota ? 'page' : undefined"
          @click="ir(item.rota)"
        >
          <Icone :nome="item.icone" />
          <span class="nome">{{ item.nome }}</span>
          <span v-if="item.contador && pendentes" class="contador" :aria-label="`${pendentes} para revisar`">{{ pendentes }}</span>
        </button>
      </nav>
      <div class="rodape">{{ versao ? `versão ${versao.slice(0, 7)}` : 'sem conexão' }}</div>
    </aside>

    <main class="conteudo">
      <Estudo v-if="rota.nome === 'estudar'" :key="rota.param" :id="rota.param" />
      <Baralho v-else-if="rota.nome === 'baralho'" :key="rota.param" :id="rota.param" />
      <Importar v-else-if="rota.nome === 'importar'" :key="rota.param" :id="rota.param" />
      <Editor v-else-if="rota.nome === 'adicionar'" :key="'adicionar' + rota.param" :baralho="rota.param" />
      <Editor v-else-if="rota.nome === 'nota'" :key="rota.param" :id="rota.param" />
      <Cartoes v-else-if="rota.nome === 'cartoes'" :key="rota.param" :baralho="rota.param" />
      <Gerar v-else-if="rota.nome === 'gerar'" :key="'gerar' + rota.param" :baralho="rota.param" />
      <Geracao v-else-if="rota.nome === 'geracao'" :key="rota.param" :id="rota.param" />
      <Instrucao v-else-if="rota.nome === 'ajustes' && rota.param.startsWith('instrucoes/')" :key="rota.param" :arquivo="rota.param.slice('instrucoes/'.length)" />
      <Ajustes v-else-if="rota.nome === 'ajustes'" :tela="telaAnterior" />
      <Baralhos v-else />
    </main>

    <Transition name="recado">
      <div v-if="recado" class="recado vidro" :class="{ erro: recado.erro }" role="status">{{ recado.texto }}</div>
    </Transition>
  </div>
</template>

<style scoped>
.app {
  min-height: 100vh;
}

.menu {
  position: fixed;
  inset: 0 auto 0 0;
  z-index: 20;
  display: flex;
  flex-direction: column;
  width: 216px;
  padding: max(28px, env(safe-area-inset-top)) 12px 16px;
  border-right: 1px solid var(--linha);
  background: var(--fundo);
}

.marca {
  padding: 0 10px 22px;
  font-family: var(--leitura);
  font-size: 1.2rem;
  font-weight: 600;
  letter-spacing: -0.01em;
}

nav {
  display: grid;
  gap: 1px;
}

.item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 34px;
  padding: 0 10px;
  border: 0;
  border-radius: var(--raio-p);
  background: transparent;
  color: var(--texto-suave);
  font-size: 0.9rem;
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.item:hover {
  background: var(--superficie);
  color: var(--texto);
}

.item.ativo {
  background: var(--superficie);
  color: var(--texto);
  font-weight: 600;
}

.contador {
  margin-left: auto;
  min-width: 18px;
  padding: 0 6px;
  border-radius: 999px;
  background: var(--terracota);
  color: var(--destaque-texto);
  font-size: 0.7rem;
  font-weight: 600;
  line-height: 18px;
  text-align: center;
}

.rodape {
  margin-top: auto;
  padding: 0 10px;
  color: var(--texto-fraco);
  font-size: 0.72rem;
}

.conteudo {
  margin-left: 216px;
  min-height: 100vh;
}

/* Estudando, o menu some: a tela é do cartão. A barra do estudo tem o voltar. */
.estudando .menu {
  display: none;
}

.estudando .conteudo {
  margin-left: 0;
}

.recado {
  position: fixed;
  left: 50%;
  bottom: calc(24px + env(safe-area-inset-bottom));
  z-index: 40;
  max-width: calc(100% - 32px);
  padding: 10px 16px;
  border: 1px solid var(--vidro-borda);
  border-radius: 999px;
  box-shadow: var(--sombra);
  font-size: 0.88rem;
  transform: translateX(-50%);
}

.recado.erro {
  color: var(--terracota);
}

.recado-enter-active,
.recado-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.recado-enter-from,
.recado-leave-to {
  opacity: 0;
  transform: translate(-50%, 8px);
}

/* Celular: o menu vira uma barra de abas de vidro no pé da tela. */
@media (max-width: 820px) {
  .menu {
    inset: auto 0 0 0;
    width: auto;
    padding: 6px 8px max(6px, env(safe-area-inset-bottom));
    border-right: 0;
    border-top: 1px solid var(--linha);
    background: var(--vidro);
    -webkit-backdrop-filter: var(--blur);
    backdrop-filter: var(--blur);
  }

  .marca,
  .rodape {
    display: none;
  }

  nav {
    grid-template-columns: repeat(4, 1fr);
  }

  .item {
    position: relative;
    flex-direction: column;
    justify-content: center;
    gap: 2px;
    min-height: 48px;
    padding: 4px 0 2px;
    font-size: 0.66rem;
    font-weight: 500;
  }

  .item.ativo,
  .item:hover {
    background: transparent;
    font-weight: 600;
  }

  .contador {
    position: absolute;
    top: 2px;
    left: calc(50% + 6px);
    margin: 0;
  }

  .conteudo {
    margin-left: 0;
  }

  .recado {
    bottom: calc(80px + env(safe-area-inset-bottom));
  }

  .estudando .recado {
    bottom: calc(150px + env(safe-area-inset-bottom));
  }
}
</style>
