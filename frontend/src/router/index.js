import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'
import EmptyClassroomView from '@/views/EmptyClassroomView.vue'
import FullDayStatusView from '@/views/FullDayStatusView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/empty-classroom', name: 'empty-classroom', component: EmptyClassroomView },
    { path: '/full-day-status', name: 'full-day-status', component: FullDayStatusView },
  ],
})

export default router
