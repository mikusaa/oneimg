<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { api, type Image } from '@/api/client.ts'
import LinkBox from '@/components/media/LinkBox.vue'
import MediaThumb from '@/components/media/MediaThumb.vue'
import TagPicker from '@/components/tags/TagPicker.vue'
import { toast, UiButton, UiDrawer } from '@/components/ui'
import { LINK_FORMATS } from '@/composables/useLinkFormat'
import { loadTags, tagColor } from '@/composables/useTags'
import { formatBytes, formatDateTime, formatDuration } from '@/utils/format'
import { isVideo, mediaLink, safeDownloadName, safeMediaURL } from '@/utils/media'
import type { MediaStorage } from './types'

// 文件详情：从右侧滑出，列出各格式链接、标签和文件信息

const props = defineProps<{
  image: Image | null
  storages: MediaStorage[]
  isAdmin: boolean
  /** 当前用户能否对这个文件执行某项操作（image:delete、image:tag:add、image:tag:delete） */
  can: (image: Image, permission: string) => boolean
  canCreateTag: boolean
}>()
const emit = defineEmits<{
  view: [image: Image]
  delete: [image: Image]
  /** 标签变化后返回更新过的文件 */
  updated: [image: Image]
}>()
const open = defineModel<boolean>('open', { default: false })

const url = computed(() => props.image ? safeMediaURL(props.image.url) : '')
const video = computed(() => !!props.image && isVideo(props.image))
const name = computed(() => props.image ? props.image.original_file_name || props.image.file_name || '文件' : '')
const formats = computed(() => LINK_FORMATS.filter(format => format.value !== 'gif' || video.value))
const storage = computed(() => props.storages.find(item => item.id === props.image?.bucket_id))
const roleLabel = computed(() => {
  const role = Number(props.image?.uploader_role)
  return role === 1 ? '管理员' : role === 3 ? '普通用户' : '已删除用户'
})
const canAddTag = computed(() => !!props.image && props.can(props.image, 'image:tag:add'))
const canRemoveTag = computed(() => !!props.image && props.can(props.image, 'image:tag:delete'))

// 视频的时长和分辨率由浏览器读取元数据后显示
const videoMeta = reactive<{ loaded: boolean; duration: number | null; width: number; height: number }>({ loaded: false, duration: null, width: 0, height: 0 })
const onVideoMeta = (event: Event | null) => {
  const target = event?.target as HTMLVideoElement | undefined
  Object.assign(videoMeta, {
    loaded: true,
    duration: target && Number.isFinite(target.duration) ? target.duration : null,
    width: target?.videoWidth || 0,
    height: target?.videoHeight || 0,
  })
}
watch(() => props.image?.id, () => {
  Object.assign(videoMeta, { loaded: false, duration: null, width: 0, height: 0 })
  picking.value = false
})

/* ---------- 标签 ---------- */
const picking = ref(false)
const saving = ref(false)
const tagIds = computed(() => props.image?.tags.map(tag => tag.id) ?? [])

const updateTags = async (next: number[]) => {
  const image = props.image
  if (!image || saving.value) return
  const added = next.filter(id => !tagIds.value.includes(id))
  const removed = tagIds.value.filter(id => !next.includes(id))
  if ((added.length && !canAddTag.value) || (removed.length && !canRemoveTag.value)) {
    toast.error('没有修改这个文件标签的权限')
    return
  }
  saving.value = true
  try {
    await Promise.all([
      ...added.map(id => api.addImageTag(image.id, id)),
      ...removed.map(id => api.removeImageTag(image.id, id)),
    ])
    emit('updated', await api.getImage(image.id))
    void loadTags(true).catch(() => undefined)
  } catch (error) {
    toast.error(error instanceof Error && error.message ? error.message : '修改标签失败')
    // 部分请求可能已经成功，重新读取以显示实际状态
    api.getImage(image.id).then(fresh => emit('updated', fresh)).catch(() => undefined)
  } finally {
    saving.value = false
  }
}
const removeTag = (id: number) => updateTags(tagIds.value.filter(item => item !== id))
</script>

<template>
  <UiDrawer v-model:open="open" :title="name" description="文件详情">
    <template v-if="image">
      <div class="flex flex-wrap gap-2">
        <UiButton size="sm" @click="emit('view', image)">
          <i :class="video ? 'ri-play-fill' : 'ri-fullscreen-line'" aria-hidden="true" />{{ video ? '播放' : '放大查看' }}
        </UiButton>
        <UiButton v-if="url" as="a" size="sm" variant="ghost" :href="url" target="_blank" rel="noopener noreferrer">
          <i class="ri-external-link-line" aria-hidden="true" />打开原文件
        </UiButton>
        <UiButton v-if="url" as="a" size="sm" variant="ghost" :href="url" :download="safeDownloadName(image.file_name)">
          <i class="ri-download-2-line" aria-hidden="true" />下载
        </UiButton>
      </div>

      <button
        type="button"
        class="grid max-h-[280px] place-items-center overflow-hidden rounded-[14px] border border-oi-line bg-[repeating-conic-gradient(var(--oi-panel)_0_25%,var(--oi-hover)_0_50%)] bg-[length:16px_16px] focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring"
        :aria-label="video ? `播放 ${name}` : `放大查看 ${name}`"
        @click="emit('view', image)"
      >
        <MediaThumb :src="video ? '' : image.url" :alt="name" :video="video" :class="video ? 'h-40 w-full' : 'max-h-[280px] w-full !object-contain'" />
      </button>

      <section aria-labelledby="inspector-links">
        <h3 id="inspector-links" class="inspector-title">链接</h3>
        <div class="flex flex-col gap-1.5">
          <div v-for="format in formats" :key="format.value" class="grid grid-cols-[76px_minmax(0,1fr)] items-center gap-2">
            <span class="text-xs font-[550] text-oi-muted">{{ format.value === 'gif' ? '动图式' : format.label }}</span>
            <LinkBox :value="mediaLink(image, format.value, url)" :label="`${format.label} 链接`" />
          </div>
        </div>
      </section>

      <section aria-labelledby="inspector-tags">
        <h3 id="inspector-tags" class="inspector-title">标签 <span class="font-normal">{{ image.tags.length }} 个</span></h3>
        <div class="flex flex-wrap gap-1.5">
          <span v-for="tag in image.tags" :key="tag.id" class="inline-flex h-6 max-w-full items-center gap-[5px] whitespace-nowrap rounded-full border border-oi-line bg-oi-panel pl-2 pr-1 text-xs text-oi-ink">
            <span class="h-[7px] w-[7px] flex-none rounded-full" :style="{ background: tagColor(tag.name) }" aria-hidden="true" />
            <span class="truncate">{{ tag.name }}</span>
            <button
              v-if="canRemoveTag"
              type="button"
              class="grid h-[18px] w-[18px] flex-none place-items-center rounded-full text-[13px] text-oi-muted hover:bg-oi-line hover:text-oi-ink disabled:opacity-50"
              :disabled="saving"
              :aria-label="`移除标签 ${tag.name}`"
              @click="removeTag(tag.id)"
            ><i class="ri-close-line" aria-hidden="true" /></button>
            <span v-else class="w-1" />
          </span>
          <button
            v-if="canAddTag"
            type="button"
            class="inline-flex h-[26px] items-center gap-1 rounded-full border border-dashed border-oi-line-2 pl-1.5 pr-2.5 text-xs text-oi-muted hover:border-oi-accent hover:text-oi-accent-2"
            :aria-expanded="picking"
            @click="picking = !picking"
          >
            <i :class="picking ? 'ri-check-line' : 'ri-add-line'" aria-hidden="true" />{{ picking ? '完成' : '添加标签' }}
          </button>
          <span v-if="!image.tags.length && !canAddTag" class="text-[12.5px] text-oi-muted">没有标签</span>
        </div>
        <div v-if="picking" class="mt-2.5 overflow-hidden rounded-xl border border-oi-line-2">
          <TagPicker :model-value="tagIds" :allow-create="canCreateTag" hide-selected autofocus @update:model-value="updateTags" />
        </div>
      </section>

      <section aria-labelledby="inspector-info">
        <h3 id="inspector-info" class="inspector-title">信息</h3>
        <dl class="grid grid-cols-[76px_minmax(0,1fr)] gap-x-2.5 gap-y-[7px] text-[13px] [&_dd]:min-w-0 [&_dd]:truncate [&_dd]:text-oi-ink [&_dt]:text-oi-muted">
          <dt>原始文件名</dt><dd :title="image.original_file_name">{{ image.original_file_name || '—' }}</dd>
          <dt>存储文件名</dt><dd class="font-oi-mono text-xs" :title="image.file_name">{{ image.file_name }}</dd>
          <template v-if="video">
            <dt>时长</dt>
            <dd>
              <video v-if="url" :key="image.id" :src="url" preload="metadata" muted class="hidden" @loadedmetadata="onVideoMeta" @error="onVideoMeta(null)" />
              {{ videoMeta.loaded ? (videoMeta.duration == null ? '时长未知' : formatDuration(videoMeta.duration)) : '正在读取视频信息…' }}
            </dd>
            <dt>分辨率</dt><dd>{{ videoMeta.width ? `${videoMeta.width} × ${videoMeta.height}` : '—' }}</dd>
          </template>
          <template v-else><dt>尺寸</dt><dd>{{ image.width }} × {{ image.height }}</dd></template>
          <dt>大小</dt>
          <dd>
            {{ formatBytes(image.file_size) }}<span v-if="image.original_file_size > image.file_size" class="text-oi-muted"> · 原 {{ formatBytes(image.original_file_size) }}</span>
          </dd>
          <dt>类型</dt><dd>{{ image.mime_type || '—' }}</dd>
          <dt>存储</dt>
          <dd>
            {{ storage?.name || image.storage || '未知' }}
            <span v-if="storage?.disabled" class="ml-1.5 rounded-full bg-oi-warn-soft px-1.5 text-[11px] text-oi-warn" title="不接受新上传，这个文件的链接照常可用">已停用</span>
          </dd>
          <template v-if="isAdmin"><dt>上传者</dt><dd>{{ roleLabel }}</dd></template>
          <dt>上传时间</dt><dd>{{ formatDateTime(image.created_at) }}</dd>
        </dl>
      </section>

      <UiButton v-if="can(image, 'image:delete')" variant="danger" class="self-start" @click="emit('delete', image)">
        <i class="ri-delete-bin-6-line" aria-hidden="true" />删除文件
      </UiButton>
    </template>
  </UiDrawer>
</template>

<style scoped>
.inspector-title {
  display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px;
  font-size: 12px; font-weight: 600; color: var(--oi-muted);
}
</style>
