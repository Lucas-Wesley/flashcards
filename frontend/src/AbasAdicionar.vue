<script setup>
// As formas de adicionar cartões (e um baralho novo), no topo de cada uma.
defineProps({ atual: { type: String, required: true } })

const ABAS = [
  { rota: 'adicionar', nome: 'Escrever' },
  { rota: 'link', nome: 'Do link' },
  { rota: 'gerar', nome: 'Com IA' },
  { rota: 'novo-baralho', nome: 'Baralho novo' },
]
</script>

<template>
  <nav class="abas pilulas" aria-label="Como adicionar">
    <a v-for="a in ABAS" :key="a.rota" class="pilula" :class="{ ativa: atual === a.rota }" :href="`#/${a.rota}`" :aria-current="atual === a.rota ? 'page' : undefined">
      {{ a.nome }}
    </a>
  </nav>
</template>

<style scoped>
.abas {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin: -6px 0 22px;
}

.pilula {
  display: inline-flex;
  align-items: center;
  text-decoration: none;
}
</style>
