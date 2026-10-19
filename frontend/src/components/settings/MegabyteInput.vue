<script setup lang="ts">
import { computed } from 'vue'
import { MiB } from './settingsModel'

// 以 MB 显示和输入、以字节保存；没改动时保持原来的字节数
const props = defineProps<{ id: string; invalid?: boolean; min?: number; max?: number; describedby?: string }>()
const bytes = defineModel<number>({ required: true })

const megabytes = computed({
  get: () => Math.round((bytes.value / MiB) * 100) / 100,
  set: (value: number | string) => {
    const number = typeof value === 'number' ? value : Number(value)
    bytes.value = value === '' || !Number.isFinite(number) ? Number.NaN : Math.round(number * MiB)
  },
})
const minMB = computed(() => props.min ?? 0.01)
</script>

<template>
  <div class="relative w-40">
    <input
      :id="id"
      v-model="megabytes"
      type="number"
      inputmode="decimal"
      :min="minMB"
      :max="max ?? 1024"
      step="any"
      :aria-invalid="invalid || undefined"
      :aria-describedby="describedby"
      :class="[
        'h-9 w-full rounded-[10px] border bg-oi-raised pl-3 pr-11 text-[13.5px] tabular-nums text-oi-ink outline-none transition-[border-color,box-shadow] focus:shadow-[0_0_0_3px_var(--oi-ring)]',
        invalid ? 'border-oi-danger focus:border-oi-danger' : 'border-oi-line-2 focus:border-oi-accent',
      ]"
    >
    <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-[12.5px] text-oi-muted">MB</span>
  </div>
</template>
