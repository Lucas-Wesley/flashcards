<script setup>
import Sugestoes from './Sugestoes.vue'

defineProps({ tela: { type: String, default: '' } })
</script>

<template>
  <div class="pagina">
    <h1>Ajustes</h1>

    <p class="rotulo">Sugestões de melhoria</p>
    <p class="suave explicacao">Anote o que pode melhorar no Flashcards. Depois é só pedir ao Claude para ver as sugestões: ele faz as mudanças e responde aqui.</p>
    <Sugestoes :tela="tela" />

    <p class="rotulo">Sobre a repetição espaçada</p>
    <p class="suave explicacao">
      O agendamento usa o FSRS-6, o mesmo algoritmo do Anki, com os pesos padrão. Cartões novos passam por dois passos (1 e 10 minutos) antes de
      ganhar um intervalo em dias; um erro na revisão volta o cartão por 10 minutos. O botão “Amanhã” conta como erro, mas tira o
      cartão do dia: ele volta só no dia seguinte. O dia de estudo vira às 4h. A retenção desejada fica nas
      opções de cada baralho.
    </p>
  </div>
</template>

<style scoped>
.rotulo:first-of-type {
  margin-top: 8px;
}

.explicacao {
  margin: 0 0 14px;
  font-size: 0.88rem;
  line-height: 1.5;
}
</style>
