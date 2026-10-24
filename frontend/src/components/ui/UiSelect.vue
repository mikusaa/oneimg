<script setup lang="ts">
import { computed, ref } from 'vue'

// 原生下拉框，保留系统选项列表和键盘行为，只统一外观；选项写在默认插槽里
const props = withDefaults(defineProps<{
  size?: 'md' | 'sm'
  invalid?: boolean
}>(), { size: 'md', invalid: false })

const model = defineModel<string | number>()
const el = ref<HTMLSelectElement | null>(null)

defineExpose({ el })

const classes = computed(() => [
  'appearance-none border bg-oi-raised bg-oi-caret bg-no-repeat text-oi-ink outline-none transition-[border-color,box-shadow] duration-150',
  'focus:shadow-[0_0_0_3px_var(--oi-ring)] focus-visible:outline-none disabled:opacity-60',
  props.size === 'sm'
    ? 'h-[30px] rounded-lg bg-[length:16px] bg-[position:right_6px_center] pl-2 pr-7 text-[12.5px]'
    : 'h-9 rounded-[10px] bg-[length:18px] bg-[position:right_8px_center] pl-3 pr-8 text-[13.5px]',
  props.invalid ? 'border-oi-danger focus:border-oi-danger' : 'border-oi-line-2 focus:border-oi-accent',
])
</script>

<template>
  <select ref="el" v-model="model" :class="classes" :aria-invalid="invalid || undefined">
    <slot />
  </select>
</template>
