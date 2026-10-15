<template>
  <AppDialog :model-value="true" :title="media.original_file_name || media.file_name || '图片预览'" width-class="max-w-5xl" @close="emit('close')">
    <div class="image-preview-popup space-y-4">
      <button type="button" class="block w-full rounded-lg bg-slate-100 dark:bg-slate-950" aria-label="放大原图" :disabled="!url" @click="enlarge">
        <img v-if="url" :src="url" :alt="media.original_file_name || media.file_name" class="mx-auto max-h-[55vh] max-w-full object-contain" @error="failed = true" />
      </button>
      <p v-if="failed || !url" role="alert" class="text-sm text-red-500">图片加载失败或链接不受支持</p>
      <div class="flex flex-wrap gap-2">
        <button v-for="[format, label] in formats" :key="format" class="soft-button" type="button" @click="copy(format)">{{ label }}</button>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <span v-for="tag in media.tags || []" :key="tag.id" class="result-meta-pill">
          {{ tag.name }}
          <button v-if="canDeleteTag" type="button" :aria-label="`移除标签 ${tag.name}`" @click="emit('remove-tag', tag.id)">×</button>
        </span>
        <button v-if="canAddTag" type="button" class="soft-button" @click="emit('add-tag')">添加标签</button>
      </div>
      <dl class="grid gap-2 break-words text-xs text-slate-500 sm:grid-cols-2 dark:text-slate-400">
        <div>原始文件名: {{ media.original_file_name || media.file_name || '未知' }}</div>
        <div>尺寸: {{ media.width }} × {{ media.height }}</div>
        <div>大小: {{ media.file_size }} 字节</div>
        <div>存储: {{ storageName || media.storage || '未知' }}</div>
        <div>角色: {{ Number(media.uploader_role) === 1 ? '管理员' : Number(media.uploader_role) === 3 ? '普通用户' : '已删除用户' }}</div>
        <div>上传时间: {{ media.created_at || '未知' }}</div>
      </dl>
    </div>
    <template #footer>
      <button v-if="canDelete" type="button" class="danger-button" @click="emit('delete')">删除</button>
      <a v-if="url" :href="url" :download="safeDownloadName(media.file_name)" class="soft-button">下载</a>
      <button type="button" class="primary-button" @click="emit('close')">关闭</button>
    </template>
  </AppDialog>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Image } from '@/api/client'
import AppDialog from './AppDialog.vue'
import { safeMediaURL, safeDownloadName, mediaLink, copyMediaText } from '@/utils/media'
import Message from '@/utils/message'

const props = defineProps<{ media: Image; storageName?: string; canDelete?: boolean; canAddTag?: boolean; canDeleteTag?: boolean }>()
const emit = defineEmits<{ close: []; delete: []; 'add-tag': []; 'remove-tag': [id: number] }>()
const url = computed(() => safeMediaURL(props.media.url))
const failed = ref(false)
const formats = [['url', '复制 URL'], ['html', 'HTML'], ['markdown', 'Markdown']]
const copy = async (format: string) => {
  if (!url.value) return
  try { await copyMediaText(mediaLink(props.media, format, url.value)); Message.success('已复制') }
  catch { Message.error('复制失败') }
}
const enlarge = () => {
  if (url.value) window.Spotlight?.show([{ src: url.value }])
}
</script>
