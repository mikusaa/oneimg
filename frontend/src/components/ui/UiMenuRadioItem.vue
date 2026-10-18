<script setup lang="ts">
import { DropdownMenuItemIndicator, DropdownMenuRadioItem } from 'reka-ui'

defineProps<{
  value: string
  disabled?: boolean
}>()
</script>

<template>
  <!-- 放在 UiMenuRadioGroup 里使用；选中项右侧显示对勾 -->
  <DropdownMenuRadioItem
    :value="value"
    :disabled="disabled"
    class="flex min-h-[34px] w-full cursor-default select-none items-center gap-2.5 rounded-[9px] px-2.5 py-1.5 text-left text-[13.5px] text-oi-text outline-none transition-colors duration-150 data-[disabled]:pointer-events-none data-[highlighted]:bg-oi-hover data-[highlighted]:text-oi-ink data-[disabled]:opacity-50"
  >
    <slot name="leading" />
    <span class="min-w-0 flex-1"><slot /></span>
    <DropdownMenuItemIndicator class="flex-none text-base text-oi-accent">
      <i class="ri-check-line" aria-hidden="true" />
    </DropdownMenuItemIndicator>
  </DropdownMenuRadioItem>
</template>
