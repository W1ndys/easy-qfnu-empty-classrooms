<script setup>
const props = defineProps({
  text: {
    type: String,
    default: '加载中...',
  },
})
</script>

<template>
  <div class="bg-white rounded-2xl p-6 shadow-sm text-center">
    <svg class="animate-spin h-8 w-8 text-primary mx-auto mb-3" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
    </svg>
    <p class="text-gray-500">{{ props.text }}</p>
  </div>
</template>
