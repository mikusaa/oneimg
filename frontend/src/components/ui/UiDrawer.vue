<script setup lang="ts">
import { computed, useSlots } from 'vue'
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle, DialogTrigger } from 'reka-ui'
import UiButton from './UiButton.vue'

const props = defineProps<{
  title: string
  description?: string
}>()

const open = defineModel<boolean>('open', { default: false })
const slots = useSlots()
const hasDescription = computed(() => !!props.description || !!slots.description)
const close = () => { open.value = false }
</script>

<template>
  <!-- 从右侧滑出的面板，行为与弹窗相同：焦点锁定、Esc 和点遮罩关闭 -->
  <DialogRoot v-model:open="open">
    <DialogTrigger v-if="$slots.trigger" as-child>
      <slot name="trigger" />
    </DialogTrigger>
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-[100] bg-oi-scrim data-[state=open]:animate-oi-fade motion-reduce:animate-none" />
      <DialogContent
        v-bind="hasDescription ? {} : { 'aria-describedby': undefined }"
        class="fixed bottom-2 right-2 top-2 z-[101] flex w-[min(460px,calc(100vw-16px))] flex-col rounded-[20px] border border-oi-line bg-oi-raised font-oi text-oi-text shadow-oi-lg outline-none data-[state=open]:animate-oi-drawer-in motion-reduce:animate-none"
      >
        <div class="flex items-center gap-3 border-b border-oi-line px-5 py-[18px]">
          <slot name="leading" />
          <div class="min-w-0 flex-1">
            <DialogTitle class="truncate text-[15px] font-[650] text-oi-ink">{{ title }}</DialogTitle>
            <DialogDescription v-if="hasDescription" class="mt-0.5 text-[12.5px] text-oi-muted">
              <slot name="description">{{ description }}</slot>
            </DialogDescription>
          </div>
          <DialogClose as-child>
            <UiButton variant="ghost" size="sm" icon aria-label="关闭"><i class="ri-close-line" /></UiButton>
          </DialogClose>
        </div>
        <div class="flex flex-1 flex-col gap-5 overflow-auto px-5 py-[18px] [&>*]:shrink-0">
          <slot :close="close" />
        </div>
        <div v-if="$slots.footer" class="flex flex-wrap justify-end gap-2 border-t border-oi-line px-5 py-3.5">
          <slot name="footer" :close="close" />
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
