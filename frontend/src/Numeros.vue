<script setup>
// Os números de hoje, como no Anki: novos em azul, aprendendo em cinza, para revisar em vermelho.
// Os zerados não aparecem; sem nada, um "em dia".
defineProps({ contagem: { type: Object, required: true } })
</script>

<template>
  <span class="numeros" :aria-label="`${contagem.novos} novos, ${contagem.aprendendo} aprendendo, ${contagem.revisar} para revisar`">
    <template v-if="contagem.novos + contagem.aprendendo + contagem.revisar">
      <span v-if="contagem.novos" class="novos">{{ contagem.novos }}</span>
      <span v-if="contagem.aprendendo" class="aprendendo">{{ contagem.aprendendo }}</span>
      <span v-if="contagem.revisar" class="revisar">{{ contagem.revisar }}</span>
    </template>
    <span v-else class="em-dia">em dia</span>
  </span>
</template>

<style scoped>
.numeros {
  display: inline-flex;
  flex: none;
  gap: 10px;
  font-size: 0.9rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.novos {
  color: var(--azul);
}

.aprendendo {
  color: var(--texto-suave);
}

.revisar {
  color: var(--vermelho);
}

.em-dia {
  color: var(--texto-fraco);
  font-size: 0.8rem;
  font-weight: 500;
}
</style>
