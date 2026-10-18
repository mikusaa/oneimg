<script setup lang="ts">
import { ToastDescription, ToastProvider, ToastRoot, ToastViewport } from 'reka-ui'
import { removeToast, toastState, type ToastType } from './toast'

const icons: Record<ToastType, string> = {
  success: 'ri-checkbox-circle-fill text-oi-accent',
  error: 'ri-error-warning-fill text-oi-danger',
  info: 'ri-information-fill text-oi-btn-ink',
}

// 等退出动画结束再移除
const onOpenChange = (id: number, open: boolean) => {
  if (!open) setTimeout(() => removeToast(id), 200)
}
</script>

<template>
  <!-- 整个应用只挂载一次 -->
  <ToastProvider label="通知" swipe-direction="up">
    <ToastRoot
      v-for="item in toastState.items"
      :key="item.id"
      v-model:open="item.open"
      :duration="item.duration"
      :type="item.type === 'error' ? 'foreground' : 'background'"
      class="pointer-events-auto flex max-w-full items-center gap-2 rounded-xl bg-oi-btn py-[9px] pl-3 pr-3.5 font-oi text-[13.5px] font-[520] text-oi-btn-ink shadow-oi-lg data-[state=open]:animate-oi-toast-in data-[state=closed]:animate-oi-toast-out motion-reduce:animate-none"
      @update:open="open => onOpenChange(item.id, open)"
    >
      <i :class="['flex-none text-[17px]', icons[item.type]]" aria-hidden="true" />
      <ToastDescription class="min-w-0">{{ item.message }}</ToastDescription>
    </ToastRoot>
    <ToastViewport
      class="pointer-events-none fixed left-1/2 top-[18px] z-[200] flex w-max max-w-[calc(100vw-32px)] -translate-x-1/2 flex-col items-center gap-2 outline-none max-md:bottom-[calc(80px+env(safe-area-inset-bottom))] max-md:top-auto"
    />
  </ToastProvider>
</template>
