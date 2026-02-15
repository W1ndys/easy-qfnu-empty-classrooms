<script setup>
import { computed } from 'vue'

const props = defineProps({
  type: {
    type: String,
    default: 'warning',
    validator: (value) => ['error', 'warning'].includes(value),
  },
  title: {
    type: String,
    required: true,
  },
  message: {
    type: String,
    required: true,
  },
})

const classes = computed(() => {
  if (props.type === 'error') {
    return {
      wrapper: 'bg-red-50 border-red-200',
      icon: 'text-red-500',
      title: 'text-red-800',
      message: 'text-red-600',
    }
  }

  return {
    wrapper: 'bg-amber-50 border-amber-200',
    icon: 'text-amber-500',
    title: 'text-amber-800',
    message: 'text-amber-600',
  }
})
</script>

<template>
  <div :class="['border rounded-2xl p-4 shadow-sm', classes.wrapper]">
    <div class="flex items-center space-x-3">
      <div class="flex-shrink-0">
        <svg class="w-6 h-6" :class="classes.icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
      </div>
      <div>
        <h3 :class="['font-semibold', classes.title]">{{ props.title }}</h3>
        <p :class="['text-sm mt-1', classes.message]">{{ props.message }}</p>
      </div>
    </div>
  </div>
</template>
