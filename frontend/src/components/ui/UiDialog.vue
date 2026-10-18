<script setup lang="ts">
import { computed, useSlots } from 'vue'
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle, DialogTrigger } from 'reka-ui'
import UiButton from './UiButton.vue'

const props = withDefaults(defineProps<{
  title: string
  description?: string
  /** 标题左侧的 Remix Icon 类名 */
  icon?: string
  tone?: 'default' | 'danger'
  wide?: boolean
  /** 右上角显示关闭按钮；Esc 和点遮罩始终可以关闭 */
  closeButton?: boolean
}>(), { tone: 'default', wide: false, closeButton: false })

const open = defineModel<boolean>('open', { default: false })
const slots = useSlots()
const hasDescription = computed(() => !!props.description || !!slots.description)
const close = () => { open.value = false }
</script>

<template>
  <DialogRoot v-model:open="open">
    <DialogTrigger v-if="$slots.trigger" as-child>
      <slot name="trigger" />
    </DialogTrigger>
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-[100] bg-oi-scrim backdrop-blur-[3px] data-[state=open]:animate-oi-fade motion-reduce:animate-none" />
      <DialogContent
        v-bind="hasDescription ? {} : { 'aria-describedby': undefined }"
        :class="[
          'fixed left-1/2 top-1/2 z-[101] max-h-[calc(100dvh-32px)] -translate-x-1/2 -translate-y-1/2 overflow-auto',
          'rounded-[20px] border border-oi-line bg-oi-raised font-oi text-oi-text shadow-oi-lg outline-none',
          'data-[state=open]:animate-oi-dialog-in motion-reduce:animate-none',
          wide ? 'w-[min(620px,calc(100vw-32px))]' : 'w-[min(480px,calc(100vw-32px))]',
        ]"
      >
        <div :class="['flex items-start gap-3 px-[22px] pt-5', closeButton ? 'pr-12' : '']">
          <span
            v-if="icon"
            :class="['grid h-[38px] w-[38px] flex-none place-items-center rounded-xl text-[19px]', tone === 'danger' ? 'bg-oi-danger-soft text-oi-danger' : 'bg-oi-panel text-oi-ink']"
            aria-hidden="true"
          ><i :class="icon" /></span>
          <div class="min-w-0 flex-1">
            <DialogTitle class="text-[17px] font-[650] leading-snug text-oi-ink">{{ title }}</DialogTitle>
            <DialogDescription v-if="hasDescription" class="mt-1 text-[13px] text-oi-muted">
              <slot name="description">{{ description }}</slot>
            </DialogDescription>
            <slot name="header" />
          </div>
        </div>
        <div v-if="$slots.default" class="flex flex-col gap-3.5 px-[22px] pt-[18px]">
          <slot :close="close" />
        </div>
        <div v-if="$slots.footer" class="flex flex-wrap justify-end gap-2 px-[22px] pb-[18px] pt-3.5">
          <slot name="footer" :close="close" />
        </div>
        <div v-else class="pb-5" />
        <!-- 放在最后，打开弹窗时焦点先落到内容里的第一个输入项 -->
        <DialogClose v-if="closeButton" as-child>
          <UiButton variant="ghost" size="sm" icon aria-label="关闭" class="absolute right-3.5 top-3.5"><i class="ri-close-line" /></UiButton>
        </DialogClose>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
