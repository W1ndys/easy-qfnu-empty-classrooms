<script setup>
import { watch } from 'vue'
import { useDateSelection } from '@/composables/useDateSelection'

const props = defineProps({
  modelValue: {
    type: Number,
    default: 0,
  },
})

const emit = defineEmits(['update:modelValue'])

const {
  quickDateLabels,
  useCustomDate,
  customOffset,
  dateOffset,
  customDatePreview,
  setOffset,
  setQuickDate,
  toggleCustomDate,
  updateCustomOffset,
} = useDateSelection(props.modelValue)

watch(
  () => props.modelValue,
  (nextValue) => {
    if (nextValue !== dateOffset.value) {
      setOffset(nextValue)
    }
  },
)

watch(dateOffset, (nextValue) => {
  emit('update:modelValue', nextValue)
})

function handleCustomOffsetInput() {
  updateCustomOffset(customOffset.value)
}
</script>

<template>
  <div>
    <label class="block text-sm font-medium text-gray-500 mb-1.5 ml-1">日期</label>

    <div class="bg-[#E5E5EA] p-1 rounded-xl flex mb-2">
      <button
        v-for="(label, idx) in quickDateLabels"
        :key="idx"
        type="button"
        class="flex-1 py-1.5 text-[13px] font-medium rounded-lg transition-all duration-200"
        :class="dateOffset === idx && !useCustomDate ? 'bg-white text-black shadow-sm' : 'text-gray-500 hover:text-gray-700'"
        @click="setQuickDate(idx)"
      >
        {{ label }}
      </button>

      <button
        type="button"
        class="flex-1 py-1.5 text-[13px] font-medium rounded-lg transition-all duration-200"
        :class="useCustomDate ? 'bg-white text-black shadow-sm' : 'text-gray-500 hover:text-gray-700'"
        @click="toggleCustomDate"
      >
        自定义
      </button>
    </div>

    <div v-if="useCustomDate" class="space-y-2">
      <div class="flex items-center space-x-2">
        <div class="flex-1 relative">
          <input
            v-model.number="customOffset"
            type="number"
            min="0"
            max="180"
            class="w-full bg-[#E5E5EA] rounded-xl py-3 px-4 text-[15px] focus:outline-none focus:ring-2 focus:ring-primary/20 transition-all"
            placeholder="输入天数"
            @input="handleCustomOffsetInput"
          />
        </div>
        <span class="text-gray-500 text-sm whitespace-nowrap">天后</span>
      </div>
      <p class="text-xs text-gray-400 ml-1">{{ customDatePreview }}</p>
    </div>
  </div>
</template>
