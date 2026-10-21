<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { resolveApiURL } from '@/api/client.ts'

const props = withDefaults(defineProps<{
  name?: string
  /** 用户信息里的 avatar_url；为空或加载失败时显示首字母 */
  src?: string | null
  size?: 'sm' | 'lg'
}>(), {
  name: '',
  src: null,
  size: 'sm',
})

const failed = ref(false)
watch(() => props.src, () => { failed.value = false })

const initial = computed(() => [...props.name][0]?.toUpperCase() ?? '?')
const url = computed(() => (props.src && !failed.value ? resolveApiURL(props.src) : ''))
</script>

<template>
  <span
    :class="[
      'relative grid flex-none place-items-center overflow-hidden rounded-full bg-[linear-gradient(135deg,#7c8cf8,#ec7598)] font-semibold text-white',
      size === 'lg' ? 'h-16 w-16 text-2xl' : 'h-8 w-8 text-[13px]',
    ]"
    aria-hidden="true"
  >
    {{ initial }}
    <img v-if="url" :src="url" alt="" class="absolute inset-0 h-full w-full object-cover" decoding="async" @error="failed = true">
  </span>
</template>
