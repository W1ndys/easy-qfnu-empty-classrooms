<script setup>
import QRCodeCard from '@/components/QRCodeCard.vue'
import AppFooter from '@/components/AppFooter.vue'
</script>

<template>
  <div class="min-h-screen bg-gray-50 text-[#1C1C1E] font-sans antialiased pb-10">
    <main class="px-4 py-6 space-y-6 max-w-xl mx-auto">
      <div class="text-center py-8">
        <h2 class="text-2xl font-bold text-primary">QFNU 教室查询系统</h2>
      </div>

      <div class="space-y-4">
        <RouterLink
          to="/empty-classroom"
          class="block bg-white rounded-2xl p-6 shadow-sm border border-gray-100 hover:shadow-md transition-all active:scale-95"
        >
          <div class="flex items-center space-x-4">
            <div class="w-14 h-14 bg-green-100 rounded-xl flex items-center justify-center flex-shrink-0">
              <svg class="w-7 h-7 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                />
              </svg>
            </div>
            <div class="flex-1">
              <h3 class="text-lg font-semibold text-gray-800">空教室查询</h3>
              <p class="text-sm text-gray-500 mt-1">查询指定时间段内的空闲教室</p>
            </div>
            <svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
            </svg>
          </div>
        </RouterLink>

        <RouterLink
          to="/full-day-status"
          class="block bg-white rounded-2xl p-6 shadow-sm border border-gray-100 hover:shadow-md transition-all active:scale-95"
        >
          <div class="flex items-center space-x-4">
            <div class="w-14 h-14 bg-primary/10 rounded-xl flex items-center justify-center flex-shrink-0">
              <svg class="w-7 h-7 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2"
                />
              </svg>
            </div>
            <div class="flex-1">
              <h3 class="text-lg font-semibold text-gray-800">教室全天状态</h3>
              <p class="text-sm text-gray-500 mt-1">查看指定教学楼全天各节次的占用情况</p>
            </div>
            <svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
            </svg>
          </div>
        </RouterLink>
      </div>

      <QRCodeCard />
      <AppFooter />
    </main>
  </div>
</template>
