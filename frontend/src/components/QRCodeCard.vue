<script setup>
import qrcodeUrl from '@/assets/images/qrcode.png'
</script>

<template>
  <div class="bg-white rounded-2xl p-6 shadow-sm mt-4 text-center">
    <h3 class="text-primary font-semibold mb-3">扫码关注公众号</h3>
    <p class="text-gray-600 text-sm mb-4 px-2">
      绩点计算、考试安排、选课结果、预选课查询等更多便捷工具，欢迎关注微信公众号
    </p>
    <div class="flex justify-center">
      <img :src="qrcodeUrl" alt="微信公众号二维码" class="w-48 h-48 rounded-lg shadow-sm" />
    </div>
    <p class="text-gray-400 text-xs mt-3">获取最新空教室信息与校园服务</p>
  </div>
</template>
