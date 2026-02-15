import axios from 'axios'

const api = axios.create({
  baseURL: '',
  timeout: 30000,
})

api.interceptors.response.use(
  (response) => response,
  (error) => Promise.reject(error),
)

export async function getStatus() {
  const { data } = await api.get('/api/v1/status')
  return data
}

export async function queryClassrooms(params) {
  const { data } = await api.post('/api/v1/query', params)
  return data
}

export async function queryFullDayStatus(params) {
  const { data } = await api.post('/api/v1/query-full-day', params)
  return data
}

export function getErrorMessage(error, fallback = '请求失败，请稍后重试') {
  return error?.response?.data?.error || fallback
}
