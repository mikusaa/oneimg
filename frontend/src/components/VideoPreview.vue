<template>
  <AppDialog :model-value="true" :title="media.original_file_name || media.file_name || '视频预览'" width-class="max-w-5xl" @close="close">
    <video ref="player" :src="url" controls playsinline preload="metadata" class="max-h-[60vh] min-h-48 w-full rounded-lg bg-black" @loadedmetadata="readMetadata" @error="failed = true"></video>
    <p v-if="failed" role="alert" class="mt-3 text-sm text-red-500">无法播放此视频，可能是浏览器不支持其编码、文件损坏或网络连接失败。可打开原文件或下载后播放。</p>
    <p v-else-if="!metadata" class="mt-3 text-sm text-slate-500" role="status">正在读取视频信息…</p>
    <p v-else class="mt-3 text-sm text-slate-500">{{ metadata }}</p>
    <div class="mt-3 flex flex-wrap gap-2">
      <button v-for="[format, label] in formats" :key="format" class="soft-button" type="button" @click="copy(format)">{{ label }}</button>
    </div>
    <slot />
    <template #footer>
      <slot name="actions" />
      <a :href="url" target="_blank" rel="noopener noreferrer" class="soft-button">打开原文件</a>
      <a :href="url" :download="media.file_name" class="soft-button">下载</a>
      <button class="primary-button" type="button" @click="close">关闭</button>
    </template>
  </AppDialog>
</template>
<script setup lang="ts">
import { ref, onBeforeUnmount, watch } from 'vue'
import AppDialog from './AppDialog.vue'
import { mediaLink, copyMediaText, type MediaRecord } from '@/utils/media'
import Message from '@/utils/message'
const props = defineProps<{ media: MediaRecord; url: string }>()
const emit = defineEmits(['close'])
const player = ref<HTMLVideoElement | null>(null)
const failed = ref(false)
const metadata = ref('')
const formats = [['url', '复制 URL'], ['html', 'HTML'], ['markdown', 'Markdown'], ['gif', '动图式 HTML']]
const release = () => { const video = player.value; if (video) { video.pause(); video.removeAttribute('src'); video.load() } }
const close = () => { release(); emit('close') }
const readMetadata = () => {
  const video = player.value
  if (!video) return
  const duration = Number.isFinite(video.duration) ? `${Math.floor(video.duration / 60)}:${String(Math.floor(video.duration % 60)).padStart(2, '0')}` : '时长未知'
  metadata.value = `${duration}${video.videoWidth && video.videoHeight ? ` · ${video.videoWidth} × ${video.videoHeight}` : ''}`
}
const copy = async (format: string) => {
  try { await copyMediaText(mediaLink(props.media, format, props.url)); Message.success('已复制') } catch { Message.error('复制失败') }
}
watch(() => props.url, () => { release(); failed.value = false; metadata.value = '' }, { flush: 'pre' })
onBeforeUnmount(release)
</script>
