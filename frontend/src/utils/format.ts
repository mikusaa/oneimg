const KB = 1024
const MB = KB * 1024
const GB = MB * 1024

/** 文件大小，如 820 KB、3.4 MB */
export const formatBytes = (bytes?: number | null) => {
  if (bytes == null || !Number.isFinite(bytes) || bytes < 0) return '—'
  if (bytes < KB) return `${bytes} B`
  if (bytes < MB) return `${Math.round(bytes / KB)} KB`
  if (bytes < GB) return `${parseFloat((bytes / MB).toFixed(1))} MB`
  return `${parseFloat((bytes / GB).toFixed(2))} GB`
}

export const formatCount = (value?: number | null) => Number(value || 0).toLocaleString('zh-CN')

const sameDay = (a: Date, b: Date) => a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()

/** 按天分组的标题：今天、昨天，今年的只写月日，往年加年份 */
export const formatDay = (value: string | Date) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '日期未知'
  const today = new Date()
  const yesterday = new Date(today)
  yesterday.setDate(today.getDate() - 1)
  if (sameDay(date, today)) return '今天'
  if (sameDay(date, yesterday)) return '昨天'
  const options: Intl.DateTimeFormatOptions = { month: 'long', day: 'numeric', weekday: 'short' }
  if (date.getFullYear() !== today.getFullYear()) options.year = 'numeric'
  return date.toLocaleDateString('zh-CN', options)
}

/** 完整的日期时间，24 小时制 */
export const formatDateTime = (value: string | Date) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '未知' : date.toLocaleString('zh-CN', { hour12: false })
}

/** 视频时长，如 1:05、1:02:03 */
export const formatDuration = (seconds: number) => {
  const total = Math.round(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const rest = String(total % 60).padStart(2, '0')
  return hours ? `${hours}:${String(minutes).padStart(2, '0')}:${rest}` : `${minutes}:${rest}`
}
