<script setup lang="ts">
import { Label } from 'reka-ui'

defineProps<{
  label?: string
  /** 关联的输入框 id，点击标签时聚焦 */
  for?: string
  hint?: string
  /** 有值时显示为错误提示，代替 hint */
  error?: string
}>()
</script>

<template>
  <div class="flex flex-col gap-1.5">
    <Label v-if="label || $slots.label" :for="$props.for" class="text-[12.5px] font-[550] text-oi-text">
      <slot name="label">{{ label }}</slot>
    </Label>
    <slot />
    <p v-if="error" class="flex items-center gap-1 text-xs text-oi-danger" role="alert">
      <i class="ri-error-warning-line" aria-hidden="true" />{{ error }}
    </p>
    <p v-else-if="hint || $slots.hint" class="text-xs text-oi-muted">
      <slot name="hint">{{ hint }}</slot>
    </p>
  </div>
</template>
