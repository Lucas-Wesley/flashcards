<script setup>
import Icone from './Icone.vue'
import { ondeAbre } from './fonte.js'

// O cartão na tela, no estudo e no aleatório: a frente e, mostrada a resposta, o verso, a foto e o
// link da fonte. O verso de um cartão de leitura (F38) é um texto para ler: alinhado à esquerda,
// em letra de leitura, como nas Referências do Acervo; o de pergunta é curto e centralizado.
const props = defineProps({
  cartao: { type: Object, required: true },
  mostrando: { type: Boolean, default: false },
  audioNaPergunta: { type: Boolean, default: false },
})
const emit = defineEmits(['tocar'])
const leitura = () => props.cartao.tipo === 'leitura'
</script>

<template>
  <section class="face" :class="{ leitura: leitura() }">
    <div class="frente texto-cartao">{{ cartao.frente }}</div>
    <button v-if="audioNaPergunta" class="botao icone som" aria-label="Ouvir de novo" title="Ouvir de novo (R)" @click="emit('tocar')"><Icone nome="som" /></button>
    <template v-if="mostrando">
      <hr />
      <div class="verso texto-cartao">{{ cartao.verso }}</div>
      <button v-if="cartao.audio && !audioNaPergunta" class="botao icone som" aria-label="Ouvir de novo" title="Ouvir de novo (R)" @click="emit('tocar')">
        <Icone nome="som" />
      </button>
      <img v-if="cartao.imagem" class="foto" :src="`/api/imagens/${cartao.imagem}`" alt="" />
      <a v-if="cartao.fonte" class="fonte suave" :href="cartao.fonte" target="_blank" rel="noopener">Ver {{ ondeAbre(cartao.fonte) }}</a>
    </template>
  </section>
</template>

<style scoped>
.face {
  max-width: 720px;
  margin: 0 auto;
  padding: clamp(32px, 9vh, 96px) 20px 32px;
  text-align: center;
}

.texto-cartao {
  font-family: var(--leitura);
  line-height: 1.5;
  overflow-wrap: break-word;
  white-space: pre-wrap;
}

.frente {
  font-size: clamp(1.5rem, 4vw, 2.1rem);
  font-weight: 500;
}

.verso {
  font-size: clamp(1.25rem, 3.2vw, 1.6rem);
  color: var(--texto);
}

/* Leitura: a pergunta fica no alto, e o texto corre como numa página. */
.leitura {
  padding-top: clamp(24px, 5vh, 56px);
}

.leitura .frente {
  font-size: clamp(1.3rem, 3.4vw, 1.75rem);
  line-height: 1.35;
}

.leitura .verso {
  max-width: 64ch;
  margin: 0 auto;
  font-size: 1.1rem;
  line-height: 1.72;
  text-align: left;
}

.leitura hr {
  margin: 22px auto 26px;
}

.som {
  margin: 14px auto 0;
}

.foto {
  display: block;
  max-width: min(100%, 420px);
  max-height: 40vh;
  margin: 24px auto 0;
  border-radius: var(--raio);
  object-fit: contain;
}

.fonte {
  display: block;
  width: fit-content;
  margin: 20px auto 0;
  font-size: 0.85rem;
}

hr {
  width: min(320px, 70%);
  margin: 28px auto;
  border: 0;
  border-top: 1px solid var(--linha-forte);
}
</style>
