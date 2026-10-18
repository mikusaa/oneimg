import { reactive } from 'vue'

export interface ConfirmOptions {
  title: string
  description?: string
  /** 影响范围列表，危险操作时用红色圆点 */
  points?: string[]
  /** Remix Icon 类名，默认按 danger 选择 */
  icon?: string
  danger?: boolean
  confirmText?: string
  cancelText?: string
  /** 需要输入这段文字才能确认，用于删除存储等不可撤销的操作 */
  requireText?: string
}

interface ConfirmState {
  open: boolean
  options: ConfirmOptions | null
  resolve: ((value: boolean) => void) | null
}

/** 由 UiConfirmHost 渲染 */
export const confirmState = reactive<ConfirmState>({ open: false, options: null, resolve: null })

/** 打开确认弹窗；确认返回 true，取消、Esc 或点遮罩返回 false */
export function confirm(options: ConfirmOptions): Promise<boolean> {
  confirmState.resolve?.(false)
  return new Promise(resolve => {
    confirmState.options = options
    confirmState.resolve = resolve
    confirmState.open = true
  })
}

export function settleConfirm(value: boolean) {
  const resolve = confirmState.resolve
  confirmState.resolve = null
  confirmState.open = false
  resolve?.(value)
}
