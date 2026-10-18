<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  /** 多行输入 */
  multiline?: boolean
  /** 等宽字体，适合路径、域名、密钥 */
  mono?: boolean
  invalid?: boolean
}>(), { multiline: false, mono: false, invalid: false })

const model = defineModel<string | number>()

const classes = computed(() => [
  'w-full rounded-[10px] border bg-oi-raised px-3 text-[13.5px] text-oi-ink outline-none transition-[border-color,box-shadow] duration-150',
  'placeholder:text-oi-faint focus:shadow-[0_0_0_3px_var(--oi-ring)] focus-visible:outline-none disabled:opacity-60',
  props.multiline ? 'min-h-[76px] resize-y py-[9px] leading-normal' : 'h-9',
  props.mono ? 'font-oi-mono text-[12.5px]' : '',
  props.invalid ? 'border-oi-danger focus:border-oi-danger' : 'border-oi-line-2 focus:border-oi-accent',
])
</script>

<template>
  <textarea v-if="multiline" v-model="model" :class="classes" :aria-invalid="invalid || undefined" />
  <input v-else v-model="model" :class="classes" :aria-invalid="invalid || undefined" />
</template>
