<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import { api } from './api.js'

// Importar uma lista: uma linha por cartão, "frente;verso;tags". Serve para colar de uma planilha
// (tabulação) ou do "Exportar notas em texto" do Anki.
const props = defineProps({ id: { type: String, required: true } })
const recado = inject('recado')

const baralho = ref(null)
const texto = ref('')
const tipo = ref('basico')
const resultado = ref(null)
const importando = ref(false)
const erro = ref('')

const linhas = computed(() => texto.value.split('\n').filter((l) => l.trim() && !l.startsWith('#')).length)

async function lerArquivo(ev) {
  const arquivo = ev.target.files[0]
  if (arquivo) texto.value = await arquivo.text()
}

async function importar() {
  importando.value = true
  resultado.value = null
  try {
    resultado.value = await api(`/api/baralhos/${props.id}/importar`, { metodo: 'POST', corpo: { texto: texto.value, tipo: tipo.value } })
    if (resultado.value.importadas) texto.value = ''
  } catch (e) {
    recado(e.message, true)
  } finally {
    importando.value = false
  }
}

onMounted(async () => {
  try {
    baralho.value = await api(`/api/baralhos/${props.id}`)
  } catch (e) {
    erro.value = e.status === 0 ? 'Sem conexão: importar só funciona na rede de casa.' : e.message
  }
})
</script>

<template>
  <div class="pagina">
    <a class="voltar" :href="`#/baralho/${id}`">{{ baralho?.nome || 'Baralho' }}</a>
    <h1>Importar</h1>
    <div v-if="erro" class="aviso erro">{{ erro }}</div>

    <form v-else class="formulario" @submit.prevent="importar">
      <p class="suave explicacao">
        Uma linha por cartão: <code>frente;verso;tags</code>. Também vale separar com tabulação (colando de uma planilha) ou vírgula. Linhas com
        <code>#</code> no começo são ignoradas, e frentes que já estão no baralho são puladas.
      </p>

      <div class="pilulas" role="radiogroup" aria-label="Tipo de cartão">
        <button type="button" class="pilula" role="radio" :class="{ ativa: tipo === 'basico' }" :aria-checked="tipo === 'basico'" @click="tipo = 'basico'">
          Frente e verso
        </button>
        <button type="button" class="pilula" role="radio" :class="{ ativa: tipo === 'invertido' }" :aria-checked="tipo === 'invertido'" @click="tipo = 'invertido'">
          Nos dois sentidos
        </button>
      </div>

      <textarea v-model="texto" class="campo lista-texto" rows="10" placeholder="casa;house&#10;cachorro;dog;animal" aria-label="Linhas para importar" />
      <div class="linha-acoes">
        <button class="botao principal" :disabled="importando || !linhas">
          {{ importando ? 'Importando…' : linhas === 1 ? 'Importar 1 linha' : `Importar ${linhas} linhas` }}
        </button>
        <label class="botao">
          Abrir arquivo
          <input type="file" accept=".csv,.tsv,.txt,text/plain,text/csv" hidden @change="lerArquivo" />
        </label>
      </div>
    </form>

    <div v-if="resultado" class="aviso ok resultado" role="status">
      {{ resultado.importadas === 1 ? '1 nota importada' : `${resultado.importadas} notas importadas` }}<template v-if="resultado.repetidas">;
        {{ resultado.repetidas }} já {{ resultado.repetidas === 1 ? 'estava' : 'estavam' }} no baralho</template
      ><template v-if="resultado.ignoradas">; {{ resultado.ignoradas }} sem frente ou verso</template>.
      <a :href="`#/baralho/${id}`">Voltar ao baralho</a>
    </div>
  </div>
</template>

<style scoped>
.voltar {
  display: inline-block;
  margin-bottom: 10px;
  color: var(--texto-suave);
  font-size: 0.85rem;
  text-decoration: none;
}

.voltar::before {
  content: '‹ ';
}

.formulario {
  display: grid;
  gap: 14px;
}

.explicacao {
  margin: 0;
  font-size: 0.88rem;
  line-height: 1.55;
}

code {
  padding: 1px 4px;
  border-radius: 4px;
  background: var(--superficie);
  font-size: 0.85em;
}

.pilulas {
  display: flex;
  gap: 4px;
}

.lista-texto {
  font-family: ui-monospace, 'SF Mono', Menlo, monospace;
  font-size: 0.85rem;
  line-height: 1.5;
  resize: vertical;
}

.resultado {
  margin-top: 16px;
}

.resultado a {
  margin-left: 6px;
  font-weight: 600;
}
</style>
