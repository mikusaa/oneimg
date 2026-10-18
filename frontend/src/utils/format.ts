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
