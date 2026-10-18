<script setup lang="ts">
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui'

withDefaults(defineProps<{
  side?: 'top' | 'bottom' | 'left' | 'right'
  align?: 'start' | 'center' | 'end'
  sideOffset?: number
  /** 气泡的无障碍名称，内容没有标题时提供 */
  label?: string
}>(), {
  side: 'bottom',
  align: 'start',
  sideOffset: 6,
})

const open = defineModel<boolean>('open', { default: false })
const close = () => { open.value = false }
</script>

<template>
  <!-- 气泡面板：用于标签选择这类需要输入的内容；Esc 或点外部关闭，关闭后焦点回到触发按钮 -->
  <PopoverRoot v-model:open="open">
    <PopoverTrigger as-child>
      <slot name="trigger" />
    </PopoverTrigger>
    <PopoverPortal>
      <PopoverContent
        :side="side"
        :align="align"
        :side-offset="sideOffset"
        :collision-padding="8"
        :aria-label="label"
        class="z-[60] overflow-hidden rounded-[14px] border border-oi-line bg-oi-raised font-oi text-oi-text shadow-oi-lg outline-none data-[state=open]:animate-oi-pop-in motion-reduce:animate-none"
      >
        <slot :close="close" />
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>
