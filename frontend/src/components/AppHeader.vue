<script setup>
import { useRouter } from 'vue-router'

const props = defineProps({
  title: {
    type: String,
    required: true,
  },
  showBack: {
    type: Boolean,
    default: false,
  },
})

const router = useRouter()

function goBack() {
  if (window.history.length > 1) {
    router.back()
    return
  }
  router.push('/')
}
</script>

<template>
  <header class="bg-white/80 backdrop-blur-md sticky top-0 z-50 border-b border-gray-200 px-4 pt-[env(safe-area-inset-top)]">
    <div class="h-14 flex items-center justify-between relative">
      <button
        v-if="props.showBack"
        type="button"
        class="text-gray-500 hover:text-gray-700"
        @click="goBack"
      >
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
        </svg>
      </button>
      <div v-else class="w-6" aria-hidden="true"></div>

      <h1 class="text-lg font-semibold absolute left-1/2 -translate-x-1/2">{{ props.title }}</h1>
      <div class="w-6" aria-hidden="true"></div>
    </div>
  </header>
</template>
