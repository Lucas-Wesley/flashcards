<script setup>
defineProps({ nome: { type: String, required: true } })
</script>

<template>
  <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path v-if="nome === 'voltar'" d="M15 5l-7 7 7 7" />
    <path v-else-if="nome === 'avancar'" d="M9 5l7 7-7 7" />
    <path v-else-if="nome === 'fechar'" d="M6 6l12 12M18 6L6 18" />
    <path v-else-if="nome === 'mais'" d="M12 5v14M5 12h14" />
    <path v-else-if="nome === 'desfazer'" d="M9 14L4 9l5-5M4 9h10.5a5.5 5.5 0 010 11H11" />
    <template v-else-if="nome === 'som'">
      <path d="M4 9.5h3.5L12 5.5v13l-4.5-4H4z" />
      <path d="M15.5 9a4 4 0 010 6M18 6.5a7.5 7.5 0 010 11" />
    </template>
    <path v-else-if="nome === 'cartoes'" d="M9 6.5h11M9 12h11M9 17.5h11M4.5 6.5h.01M4.5 12h.01M4.5 17.5h.01" />
    <template v-else-if="nome === 'baralhos'">
      <path d="M7 4h10a2 2 0 012 2v12a2 2 0 01-2 2H7a2 2 0 01-2-2V6a2 2 0 012-2zM5 12h14" />
      <path d="M10.4 7.3a1.7 1.7 0 013.2.5c0 1-1.6 1.2-1.6 2M8.5 15.3h7M8.5 17.3h4" />
    </template>
    <template v-else-if="nome === 'pedidos'">
      <path d="M4 13.5L6.5 5.5h11l2.5 8v5a1.5 1.5 0 01-1.5 1.5h-13A1.5 1.5 0 014 18.5z" />
      <path d="M4 13.5h4.5l1 2h5l1-2H20" />
    </template>
    <template v-else-if="nome === 'ajustes'">
      <path d="M4 7.5h9M17 7.5h3M4 16.5h3M11 16.5h9" />
      <circle cx="15" cy="7.5" r="2" />
      <circle cx="9" cy="16.5" r="2" />
    </template>
    <template v-else-if="nome === 'tv'">
      <rect x="3" y="4.5" width="18" height="12.5" rx="2" />
      <path d="M8.5 20.5h7M12 17v3.5" />
    </template>
    <template v-else-if="nome === 'info'">
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 11v5M12 8h.01" />
    </template>
    <template v-else-if="nome === 'aleatorio'">
      <path d="M4 7h3.5c2 0 3.2 1.2 4.5 3.2l1 1.6c1.3 2 2.5 3.2 4.5 3.2H20M17 12.5l3 2.5-3 2.5" />
      <path d="M4 17h3.5c1.3 0 2.3-.5 3.1-1.4M13.4 8.4c.8-.9 1.8-1.4 3.1-1.4H20M17 4.5L20 7l-3 2.5" />
    </template>
    <template v-else-if="nome === 'pesquisar'">
      <circle cx="11" cy="11" r="6.5" />
      <path d="M16 16l4.5 4.5" />
    </template>
  </svg>
</template>
