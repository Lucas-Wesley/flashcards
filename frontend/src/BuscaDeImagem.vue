<script setup>
import { nextTick, onMounted, ref } from 'vue'
import { api } from './api.js'

// Busca de imagens na Wikipédia: o servidor procura os artigos e devolve a foto principal de cada
// um. Escolher só devolve a URL; quem chama decide quando baixar (o editor baixa na hora; a
// proposta da IA, só ao aprovar).
const props = defineProps({ termo: { type: String, default: '' } })
const emit = defineEmits(['escolher', 'fechar'])

const busca = ref(props.termo)
const achadas = ref(null)
const buscando = ref(false)
const erro = ref('')
const campo = ref(null)

async function buscar() {
  if (!busca.value.trim()) return
  buscando.value = true
  erro.value = ''
  try {
    achadas.value = await api(`/api/imagens?q=${encodeURIComponent(busca.value)}`, { tempo: 30000 })
  } catch (e) {
    erro.value = e.message
  } finally {
    buscando.value = false
  }
}

onMounted(async () => {
  await nextTick()
  campo.value?.focus()
  if (busca.value.trim()) buscar()
})
</script>

<template>
  <div class="busca-imagem">
    <form class="linha" @submit.prevent="buscar">
      <input ref="campo" v-model="busca" class="campo" placeholder="Ex.: Luiz Inácio Lula da Silva" aria-label="Buscar imagem na Wikipédia" />
      <button class="botao" :disabled="buscando || !busca.trim()">{{ buscando ? 'Buscando…' : 'Buscar' }}</button>
      <button type="button" class="botao" @click="emit('fechar')">Fechar</button>
    </form>
    <div v-if="erro" class="aviso erro">{{ erro }}</div>
    <p v-else-if="achadas && !achadas.length" class="suave nada">Nenhum artigo com foto. Tente o nome completo ou em inglês.</p>
    <div v-if="achadas?.length" class="grade">
      <button v-for="a in achadas" :key="a.url" type="button" class="opcao" :title="a.titulo" @click="emit('escolher', a)">
        <img :src="a.url" :alt="a.titulo" loading="lazy" />
        <span>{{ a.titulo }}</span>
      </button>
    </div>
    <p class="fonte">Fotos da Wikipédia (Wikimedia Commons).</p>
  </div>
</template>

<style scoped>
.busca-imagem {
  display: grid;
  gap: 10px;
  padding: 12px;
  border-radius: var(--raio);
  background: var(--superficie);
}

.linha {
  display: flex;
  gap: 6px;
}

.linha .botao {
  min-height: 38px;
}

.grade {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
  gap: 8px;
}

.opcao {
  display: grid;
  gap: 4px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--texto-suave);
  font-size: 0.72rem;
  text-align: left;
  cursor: pointer;
}

.opcao img {
  width: 100%;
  aspect-ratio: 1;
  border-radius: var(--raio-p);
  background: var(--superficie-forte);
  object-fit: cover;
  transition: opacity 0.15s ease;
}

.opcao:hover img {
  opacity: 0.8;
}

.opcao span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nada,
.fonte {
  margin: 0;
  font-size: 0.78rem;
}

.fonte {
  color: var(--texto-fraco);
}
</style>
