<script setup lang="ts">
import { RadioGroupItem, RadioGroupRoot } from 'reka-ui'

export interface SegmentedOption {
  value: string
  label?: string
  /** Remix Icon 类名，如 ri-sun-line */
  icon?: string
  /** 只有图标时作为无障碍名称和悬停提示 */
  title?: string
}

defineProps<{
  options: SegmentedOption[]
  disabled?: boolean
}>()

const model = defineModel<string>()
</script>

<template>
  <!-- 单选语义：方向键切换，Tab 只停在当前项；组本身请传 aria-label -->
  <RadioGroupRoot
    v-model="model"
    :disabled="disabled"
    orientation="horizontal"
    class="inline-flex gap-0.5 rounded-[10px] border border-oi-line bg-oi-panel p-[3px]"
  >
    <RadioGroupItem
      v-for="option in options"
      :key="option.value"
      :value="option.value"
      :title="option.title || option.label"
      :aria-label="option.label ? undefined : option.title"
      :class="[
        'inline-flex h-7 items-center justify-center gap-[5px] whitespace-nowrap rounded-[7px] text-[12.5px] font-[550] text-oi-muted transition-all duration-150',
        'hover:text-oi-ink focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring disabled:opacity-50',
        'data-[state=checked]:bg-oi-raised data-[state=checked]:text-oi-ink data-[state=checked]:shadow-[var(--oi-shadow-sm),0_0_0_1px_var(--oi-line)]',
        option.label ? 'px-[11px]' : 'w-[30px] text-base',
      ]"
    >
      <i v-if="option.icon" :class="option.icon" aria-hidden="true" />
      <span v-if="option.label">{{ option.label }}</span>
    </RadioGroupItem>
  </RadioGroupRoot>
</template>
