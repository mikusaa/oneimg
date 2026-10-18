<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { safeMediaURL } from '@/utils/media'

const props = defineProps<{
  /** 缩略图或原图地址；本地预览可以传 blob: 地址 */
  src?: string
  alt: string
  video?: boolean
}>()

const failed = ref(false)
const url = computed(() => props.src?.startsWith('blob:') ? props.src : safeMediaURL(props.src))
watch(url, () => { failed.value = false })
</script>

<template>
  <!-- 缩略图：视频没有封面时显示图标，图片加载失败时显示失败状态 -->
  <span
    v-if="failed || !url || video"
    class="grid place-items-center overflow-hidden bg-oi-panel text-oi-faint"
    :title="failed ? `加载失败：${alt}` : alt"
  >
    <i :class="failed || (!url && !video) ? 'ri-image-off-line' : 'ri-film-line'" class="text-xl" aria-hidden="true" />
    <span v-if="failed" class="sr-only">加载失败：{{ alt }}</span>
  </span>
  <img v-else :src="url" :alt="alt" loading="lazy" decoding="async" class="object-cover" @error="failed = true">
</template>
