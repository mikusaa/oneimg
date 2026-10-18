<script setup lang="ts">
import { useId } from 'vue'

// OneImg Logo：数字 1 加一个太阳圆点，意为「一张图」；源文件 assets/images/logo.svg
withDefaults(defineProps<{ size?: number }>(), { size: 32 })
const gradientId = `oi-logo-${useId()}`
</script>

<template>
  <svg class="block" :width="size" :height="size" viewBox="0 0 32 32" aria-hidden="true">
    <defs>
      <linearGradient :id="gradientId" x1="0" y1="0" x2="1" y2="1">
        <stop offset="0" stop-color="#19b072" />
        <stop offset="1" stop-color="#0b8352" />
      </linearGradient>
    </defs>
    <rect width="32" height="32" rx="9" :fill="`url(#${gradientId})`" />
    <path d="M10.5 12.2 16 8v16" fill="none" stroke="#fff" stroke-width="3.4" stroke-linecap="round" stroke-linejoin="round" />
    <circle cx="22.6" cy="11" r="2.6" fill="#fff" />
  </svg>
</template>
