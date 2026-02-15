<script setup>
import { computed, reactive, ref } from 'vue'
import { getErrorMessage, queryClassrooms } from '@/api'
import { useSystemStatus } from '@/composables/useSystemStatus'
import AppFooter from '@/components/AppFooter.vue'
import AppHeader from '@/components/AppHeader.vue'
import DateSelector from '@/components/DateSelector.vue'
import EmptyState from '@/components/EmptyState.vue'
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import QRCodeCard from '@/components/QRCodeCard.vue'
import StatusWarning from '@/components/StatusWarning.vue'

const { statusLoading, inTeachingCalendar, hasPermission, currentWeek, currentTerm } = useSystemStatus()

const loading = ref(false)
const hasSearched = ref(false)
const results = ref([])
const resultInfo = ref(null)
const displayLimit = ref(100)

const form = reactive({
  building: '',
  offset: 0,
  start: '01',
  end: '11',
})

const nodeOptions = Array.from({ length: 11 }, (_, index) => String(index + 1).padStart(2, '0'))

const displayedResults = computed(() => results.value.slice(0, displayLimit.value))

async function search() {
  if (!form.building.trim()) {
    alert('请输入教学楼')
    return
  }

  loading.value = true
  displayLimit.value = 100
  hasSearched.value = false
  results.value = []
  resultInfo.value = null

  try {
    const data = await queryClassrooms({
      building: form.building,
      start_node: form.start,
      end_node: form.end,
      date_offset: form.offset,
    })

    results.value = data.classrooms || []
    resultInfo.value = {
      date: data.date,
      week: data.week,
      day: data.day_of_week,
    }
    hasSearched.value = true
  } catch (error) {
    console.error(error)
    alert(getErrorMessage(error, '查询出错，请重试'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen bg-[var(--color-bg-page)] text-[#1C1C1E] font-sans antialiased pb-10">
    <AppHeader title="空教室查询" showBack />

    <main class="px-4 py-4 space-y-4 max-w-xl mx-auto">
      <StatusWarning
        v-if="!hasPermission && !statusLoading"
        type="error"
        title="权限不足"
        message="当前账号无权限访问教务系统查询接口，请检查账号状态。"
      />

      <StatusWarning
        v-if="!inTeachingCalendar && !statusLoading"
        type="warning"
        title="提示"
        message="当前日期不在教学周历内，查询结果可能不准确。"
      />

      <LoadingSpinner v-if="statusLoading" text="正在检查系统状态..." />

      <div v-else class="bg-white rounded-2xl p-4 shadow-sm space-y-4">
        <div v-if="inTeachingCalendar" class="bg-green-50 border border-green-200 rounded-xl p-3 text-sm">
          <div class="flex items-center space-x-2 text-green-700">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span>当前：<strong>{{ currentTerm }}</strong> 第<strong>{{ currentWeek }}</strong>周</span>
          </div>
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-500 mb-1.5 ml-1">教学楼</label>
          <div class="relative">
            <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
                />
              </svg>
            </span>
            <input
              v-model="form.building"
              type="text"
              class="w-full bg-[#E5E5EA] rounded-xl py-3 pl-10 pr-4 text-[15px] focus:outline-none focus:ring-2 focus:ring-primary/20 transition-all"
              placeholder="例如：老文史楼"
            />
          </div>
        </div>

        <DateSelector v-model="form.offset" />

        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-sm font-medium text-gray-500 mb-1.5 ml-1">起始节次</label>
            <select
              v-model="form.start"
              class="w-full bg-[#E5E5EA] rounded-xl py-3 px-4 text-[15px] appearance-none focus:outline-none"
            >
              <option v-for="value in nodeOptions" :key="`start-${value}`" :value="value">{{ value }}</option>
            </select>
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-500 mb-1.5 ml-1">终止节次</label>
            <select
              v-model="form.end"
              class="w-full bg-[#E5E5EA] rounded-xl py-3 px-4 text-[15px] appearance-none focus:outline-none"
            >
              <option v-for="value in nodeOptions" :key="`end-${value}`" :value="value">{{ value }}</option>
            </select>
          </div>
        </div>

        <button
          type="button"
          :disabled="loading"
          class="w-full bg-primary text-white font-semibold py-3.5 rounded-xl btn-active transition-all disabled:opacity-70 disabled:cursor-not-allowed flex items-center justify-center shadow-lg shadow-primary/20"
          @click="search"
        >
          <span v-if="!loading">查询空闲教室</span>
          <span v-else class="flex items-center">
            <svg class="animate-spin -ml-1 mr-2 h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
            </svg>
            查询中...
          </span>
        </button>
      </div>

      <div v-if="resultInfo" class="px-2 flex items-center justify-between text-xs text-gray-500">
        <span>{{ resultInfo.date }} (第{{ resultInfo.week }}周 星期{{ resultInfo.day }})</span>
        <span>共 {{ results.length }} 间</span>
      </div>

      <div v-if="results.length > 0" class="space-y-3">
        <div class="grid grid-cols-2 gap-3">
          <div
            v-for="(room, index) in displayedResults"
            :key="`${room}-${index}`"
            class="bg-white p-4 rounded-xl shadow-sm border border-gray-100 flex items-center justify-center text-center"
          >
            <span class="text-primary font-semibold text-lg">{{ room }}</span>
          </div>
        </div>

        <div v-if="results.length > displayLimit" class="mt-4 text-center">
          <button type="button" class="text-primary text-sm font-medium hover:underline py-2" @click="displayLimit += 100">
            加载更多 (显示 {{ displayedResults.length }} / {{ results.length }})
          </button>
        </div>
      </div>

      <EmptyState v-if="hasSearched && results.length === 0 && !loading" text="该时间段暂无空闲教室" />

      <QRCodeCard />
    </main>

    <AppFooter />
  </div>
</template>
