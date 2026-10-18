<script setup lang="ts">
import { DropdownMenuContent, DropdownMenuPortal, DropdownMenuRoot, DropdownMenuTrigger } from 'reka-ui'

withDefaults(defineProps<{
  side?: 'top' | 'bottom' | 'left' | 'right'
  align?: 'start' | 'center' | 'end'
  sideOffset?: number
  /** 菜单与触发按钮同宽，如侧栏底部的头像菜单 */
  matchTriggerWidth?: boolean
}>(), {
  side: 'bottom',
  align: 'start',
  sideOffset: 6,
  matchTriggerWidth: false,
})

const open = defineModel<boolean>('open', { default: false })
</script>

<template>
  <!-- 下拉菜单：方向键移动、回车选择、Esc 关闭并把焦点还给触发按钮 -->
  <DropdownMenuRoot v-model:open="open">
    <DropdownMenuTrigger as-child>
      <slot name="trigger" />
    </DropdownMenuTrigger>
    <DropdownMenuPortal>
      <DropdownMenuContent
        :side="side"
        :align="align"
        :side-offset="sideOffset"
        :collision-padding="8"
        :class="[
          'z-[60] min-w-[220px] rounded-[14px] border border-oi-line bg-oi-raised p-1.5 font-oi text-oi-text shadow-oi-lg outline-none',
          'data-[state=open]:animate-oi-pop-in motion-reduce:animate-none',
          matchTriggerWidth && 'w-[var(--reka-dropdown-menu-trigger-width)]',
        ]"
      >
        <slot />
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>
