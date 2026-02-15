<script setup>
const props = defineProps({
  text: {
    type: String,
    default: '暂无数据',
  },
})
</script>

<template>
  <div class="py-10 flex flex-col items-center justify-center text-gray-400">
    <svg class="w-16 h-16 mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="1.5"
        d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
    <p>{{ props.text }}</p>
  </div>
</template>
