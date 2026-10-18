<script setup lang="ts">
import { computed } from 'vue'
import { DropdownMenuItem } from 'reka-ui'
import type { RouteLocationRaw } from 'vue-router'

const props = defineProps<{
  /** Remix Icon 类名 */
  icon?: string
  danger?: boolean
  disabled?: boolean
  /** 传入时渲染为站内链接 */
  to?: RouteLocationRaw
}>()

const emit = defineEmits<{ select: [event: Event] }>()

const classes = computed(() => [
  'flex min-h-[34px] w-full cursor-default select-none items-center gap-2.5 rounded-[9px] px-2.5 py-1.5 text-left text-[13.5px] outline-none transition-colors duration-150',
  'data-[highlighted]:bg-oi-hover data-[disabled]:pointer-events-none data-[disabled]:opacity-50 [&>i]:text-base',
  props.danger
    ? 'text-oi-danger [&>i]:text-oi-danger'
    : 'text-oi-text data-[highlighted]:text-oi-ink [&>i]:text-oi-muted',
])
</script>

<template>
  <DropdownMenuItem v-if="to" as-child :disabled="disabled" @select="emit('select', $event)">
    <RouterLink :to="to" :class="classes">
      <i v-if="icon" :class="icon" aria-hidden="true" />
      <span class="min-w-0 flex-1 truncate"><slot /></span>
      <slot name="end" />
    </RouterLink>
  </DropdownMenuItem>
  <DropdownMenuItem v-else :disabled="disabled" :class="classes" @select="emit('select', $event)">
    <i v-if="icon" :class="icon" aria-hidden="true" />
    <span class="min-w-0 flex-1 truncate"><slot /></span>
    <slot name="end" />
  </DropdownMenuItem>
</template>
