import { onMounted, ref } from 'vue'
import { getStatus } from '@/api'

export function useSystemStatus(autoCheck = true) {
  const statusLoading = ref(true)
  const inTeachingCalendar = ref(true)
  const hasPermission = ref(true)
  const currentWeek = ref(0)
  const currentTerm = ref('')

  async function checkStatus() {
    statusLoading.value = true
    try {
      const data = await getStatus()
      inTeachingCalendar.value = !!data.in_teaching_calendar
      currentWeek.value = data.current_week || 0
      currentTerm.value = data.current_term || ''
      hasPermission.value = data.has_permission !== false
    } catch (error) {
      console.error('Failed to check status:', error)
      inTeachingCalendar.value = false
      hasPermission.value = true
    } finally {
      statusLoading.value = false
    }
  }

  if (autoCheck) {
    onMounted(checkStatus)
  }

  return {
    statusLoading,
    inTeachingCalendar,
    hasPermission,
    currentWeek,
    currentTerm,
    checkStatus,
  }
}
