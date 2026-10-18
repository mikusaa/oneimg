<script setup lang="ts">
import { DropdownMenuRadioGroup, DropdownMenuRadioItem } from 'reka-ui'
import type { SegmentedOption } from './UiSegmented.vue'

defineProps<{ options: SegmentedOption[] }>()

const model = defineModel<string>()
</script>

<template>
  <!-- 菜单里的单选分段控件，如头像菜单的主题三选一；选择后菜单保持打开。组本身请传 aria-label -->
  <div class="px-1.5 pb-1.5 pt-0.5">
    <DropdownMenuRadioGroup v-model="model" class="flex gap-0.5 rounded-[10px] border border-oi-line bg-oi-panel p-[3px]">
      <DropdownMenuRadioItem
        v-for="option in options"
        :key="option.value"
        :value="option.value"
        :title="option.title || option.label"
        :aria-label="option.label ? undefined : option.title"
        :class="[
          'inline-flex h-7 flex-1 cursor-default select-none items-center justify-center gap-[5px] rounded-[7px] text-[12.5px] font-[550] text-oi-muted outline-none transition-all duration-150',
          'data-[highlighted]:text-oi-ink data-[highlighted]:ring-2 data-[highlighted]:ring-oi-ring',
          'data-[state=checked]:bg-oi-raised data-[state=checked]:text-oi-ink data-[state=checked]:shadow-[var(--oi-shadow-sm),0_0_0_1px_var(--oi-line)]',
          option.label ? 'px-[11px]' : 'text-base',
        ]"
        @select="event => event.preventDefault()"
      >
        <i v-if="option.icon" :class="option.icon" aria-hidden="true" />
        <span v-if="option.label">{{ option.label }}</span>
      </DropdownMenuRadioItem>
    </DropdownMenuRadioGroup>
  </div>
</template>
