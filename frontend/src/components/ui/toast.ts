import { reactive } from 'vue'

export type ToastType = 'success' | 'error' | 'info'

export interface ToastItem {
  id: number
  message: string
  type: ToastType
  duration: number
  open: boolean
}

const MAX_VISIBLE = 3

/** 由 UiToaster 渲染；页面里只需调用 toast() */
export const toastState = reactive<{ items: ToastItem[] }>({ items: [] })

let seq = 0

function push(message: string, type: ToastType, duration?: number) {
  // 同一条消息连续触发时只保留一条
  toastState.items = toastState.items.filter(item => !(item.open && item.message === message && item.type === type))
  toastState.items.push({ id: ++seq, message, type, duration: duration ?? (type === 'error' ? 4000 : 2200), open: true })
  const visible = toastState.items.filter(item => item.open)
  visible.slice(0, Math.max(0, visible.length - MAX_VISIBLE)).forEach(item => { item.open = false })
}

export function removeToast(id: number) {
  toastState.items = toastState.items.filter(item => item.id !== id)
}

export const toast = Object.assign(
  (message: string, type: ToastType = 'success', duration?: number) => push(message, type, duration),
  {
    success: (message: string, duration?: number) => push(message, 'success', duration),
    error: (message: string, duration?: number) => push(message, 'error', duration),
    info: (message: string, duration?: number) => push(message, 'info', duration),
  },
)
