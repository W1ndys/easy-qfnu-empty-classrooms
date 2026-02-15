export const WEEK_DAYS = ['日', '一', '二', '三', '四', '五', '六']

export function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value))
}

export function formatDateYMD(date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function buildDatePreview(offset) {
  const target = new Date()
  target.setDate(target.getDate() + offset)
  return `目标日期: ${formatDateYMD(target)} (星期${WEEK_DAYS[target.getDay()]})`
}
