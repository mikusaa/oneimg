<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type DashboardStats, type Image } from '@/api/client.ts'
import LinkBox from '@/components/media/LinkBox.vue'
import MediaThumb from '@/components/media/MediaThumb.vue'
import MediaViewer from '@/components/media/MediaViewer.vue'
import StorageTypeIcon from '@/components/storage/StorageTypeIcon.vue'
import TagPicker from '@/components/tags/TagPicker.vue'
import {
  confirm, toast, UiButton, UiDialog, UiField, UiInput, UiMenu, UiMenuLabel, UiMenuRadioGroup, UiMenuRadioItem, UiPopover, UiSelect, UiSwitch,
} from '@/components/ui'
import { LINK_FORMATS, useLinkFormat } from '@/composables/useLinkFormat'
import { useSession } from '@/composables/useSession'
import { loadTags, tagById, tagColor } from '@/composables/useTags'
import {
  addFiles, clearItems, importFromUrl, loadUploadOptions, removeItem, retryItem, startUpload, useUploadQueue, type UploadItem,
} from '@/composables/useUploadQueue'
import { formatBytes, formatCount } from '@/utils/format'
import { copyMediaText, isVideo, mediaLink, safeMediaURL } from '@/utils/media'
import { canManageImage, hasPermission, ROLE_ADMIN } from '@/utils/permissions.ts'

const { options, items, bucketId, tagIds, autoStart, queued, uploading, busy, completedBatches } = useUploadQueue()
const { user } = useSession()
const linkFormat = useLinkFormat()

const isAdmin = computed(() => Number(user.value.role) === ROLE_ADMIN)
const canCreateTag = computed(() => hasPermission('tag:create', user.value))
const pasteKey = /Mac|iPhone|iPad/.test(navigator.userAgent) ? '⌘ V' : 'Ctrl V'

/* ---------- 存储与标签 ---------- */
const storageMenuOpen = ref(false)
const tagPickerOpen = ref(false)
const currentStorage = computed(() => options.storages.find(storage => storage.id === bucketId.value))
const storageName = (id?: number) => options.storages.find(storage => storage.id === id)?.name
const bucketValue = computed({
  get: () => String(bucketId.value),
  set: (value: string) => { bucketId.value = Number(value) },
})
const selectedTags = computed(() => tagIds.value.map(tagById).filter(tag => !!tag))
const noStorage = computed(() => options.loaded && !options.storages.length)

const limits = computed(() => {
  const images = options.allowedTypes.filter(type => type.startsWith('image/')).map(type => type.slice(6).toUpperCase()).join(' / ')
  const parts = [`${images || '图片'} · 单张 ≤ ${formatBytes(options.maxImageSize)}`]
  if (options.videoEnabled) parts.push(`MP4 / WebM ≤ ${formatBytes(options.maxVideoSize)}`)
  return parts.join(' · ')
})
const accept = computed(() => [...options.allowedTypes.filter(type => type.startsWith('image/')), ...(options.videoEnabled ? ['.mp4', '.webm'] : [])].join(','))

/* ---------- 添加文件 ---------- */
const fileInput = ref<HTMLInputElement | null>(null)
const dragDepth = ref(0)
const pick = () => fileInput.value?.click()
const onPick = (event: Event) => {
  const target = event.target as HTMLInputElement
  if (target.files?.length) void addFiles(target.files)
  target.value = ''
}
const hasFiles = (event: DragEvent) => [...(event.dataTransfer?.types || [])].includes('Files')
const onDragEnter = (event: DragEvent) => { if (hasFiles(event)) dragDepth.value += 1 }
const onDragLeave = (event: DragEvent) => { if (hasFiles(event)) dragDepth.value = Math.max(0, dragDepth.value - 1) }
// 放下的文件由外壳统一加入队列（拖到页面其他位置也一样），这里只负责高亮
const onDrop = () => { dragDepth.value = 0 }

/* ---------- 上传列表 ---------- */
const done = computed(() => items.value.filter(item => item.status === 'done' && item.image))
const queuedSize = computed(() => queued.value.reduce((total, item) => total + item.size, 0))
const overall = computed(() => uploading.value.length ? Math.round(uploading.value.reduce((total, item) => total + item.progress, 0) / uploading.value.length) : 0)

const linkOf = (image: Image) => mediaLink(image, linkFormat.value, safeMediaURL(image.url))
const formatLabel = computed(() => LINK_FORMATS.find(item => item.value === linkFormat.value)?.label ?? 'URL')
const copyAll = async () => {
  const lines = done.value.map(item => linkOf(item.image!))
  try {
    await copyMediaText(lines.join('\n'))
    toast.success(`已复制 ${lines.length} 条 ${formatLabel.value} 链接`)
  } catch {
    toast.error('复制失败，请手动复制')
  }
}
const itemSize = (item: UploadItem) => item.image?.file_size ?? item.size
const savedLess = (item: UploadItem) => !!item.image && item.size > 0 && item.image.file_size < item.size

/* ---------- URL 导入 ---------- */
const urlDialogOpen = ref(false)
const importUrl = ref('')
const importError = ref('')
const importing = ref(false)
const openUrlDialog = () => {
  importUrl.value = ''
  importError.value = ''
  urlDialogOpen.value = true
}
const submitImport = async () => {
  const value = importUrl.value.trim()
  try {
    const url = new URL(value)
    if (!['http:', 'https:'].includes(url.protocol)) throw new Error()
  } catch {
    importError.value = '请输入以 http:// 或 https:// 开头的图片地址'
    return
  }
  importing.value = true
  urlDialogOpen.value = false
  try {
    await importFromUrl(value)
  } finally {
    importing.value = false
  }
}

/* ---------- 今日与最近上传 ---------- */
const stats = ref<DashboardStats | null>(null)
const recent = computed(() => (stats.value?.recent_images ?? []).slice(0, 6))
const loadStats = async () => {
  try {
    stats.value = await api.dashboardStats()
  } catch (error) {
    console.error('获取最近上传失败:', error)
  }
}

const copyRecent = async (image: Image) => {
  try {
    await copyMediaText(linkOf(image))
    toast.success(`已复制 ${formatLabel.value} 链接`)
  } catch {
    toast.error('复制失败，请手动复制')
  }
}

/* ---------- 查看与删除 ---------- */
const viewerOpen = ref(false)
const viewerIndex = ref(0)
const viewerItems = ref<Image[]>([])
// 在本次上传或最近上传的范围内左右切换
const preview = (image: Image, list: Image[]) => {
  viewerItems.value = list
  viewerIndex.value = Math.max(0, list.findIndex(item => item.id === image.id))
  viewerOpen.value = true
}
const canDelete = (image: Image) => canManageImage(image, 'image:delete', user.value)
const deleteImage = async (image: Image) => {
  const ok = await confirm({
    title: '删除这个文件？',
    description: '文件和它的外链都会删除，无法恢复。',
    danger: true,
    confirmText: '删除',
  })
  if (!ok) return
  try {
    await api.deleteImage(image.id)
    viewerItems.value = viewerItems.value.filter(item => item.id !== image.id)
    items.value.filter(item => item.image?.id === image.id).forEach(removeItem)
    toast.success('已删除')
    void loadStats()
  } catch (error) {
    toast.error(error instanceof Error && error.message ? error.message : '删除失败')
  }
}

onMounted(() => {
  loadUploadOptions().catch(() => toast.error('读取上传选项失败'))
  loadTags().catch(() => toast.error('读取标签失败'))
  void loadStats()
})
watch(completedBatches, () => { void loadStats() })
</script>

<template>
  <div class="grid flex-1 grid-cols-[minmax(0,1fr)_320px] font-oi text-sm text-oi-text max-[1280px]:grid-cols-1">
    <div class="mx-auto w-full min-w-0 max-w-[1000px] px-9 pb-12 pt-7 max-[1080px]:px-6 max-[900px]:px-4 max-[900px]:pb-8 max-[900px]:pt-4">
      <header class="mb-[22px]">
        <h1 class="text-2xl font-[680] leading-tight tracking-[-0.02em] text-oi-ink">上传</h1>
        <p class="mt-1.5 text-oi-muted">先确认存储和标签，再添加文件；上传完成后直接复制链接。</p>
      </header>

      <!-- 存储、标签、自动上传 -->
      <div class="mb-3.5 flex flex-wrap items-center gap-2">
        <UiMenu v-model:open="storageMenuOpen">
          <template #trigger>
            <button type="button" class="upload-opt" :disabled="busy || !options.storages.length" aria-label="选择存储">
              <StorageTypeIcon v-if="currentStorage" :type="currentStorage.type" size="sm" />
              <i v-else class="ri-database-2-line" aria-hidden="true" />
              <span class="text-[12.5px] text-oi-muted">存到</span>
              <span class="max-w-[200px] truncate">{{ currentStorage?.name || (options.loaded ? '没有可用的存储' : '读取中…') }}</span>
              <i class="ri-arrow-down-s-line -ml-0.5 text-base text-oi-faint" aria-hidden="true" />
            </button>
          </template>
          <UiMenuLabel>只显示你可用、未满且未停用的存储</UiMenuLabel>
          <UiMenuRadioGroup v-model="bucketValue">
            <UiMenuRadioItem v-for="storage in options.storages" :key="storage.id" :value="String(storage.id)">
              <template #leading><StorageTypeIcon :type="storage.type" size="sm" /></template>
              <span class="flex min-w-0 items-center gap-1.5">
                <span class="truncate">{{ storage.name }}</span>
                <span v-if="storage.id === options.defaultStorage" class="upload-pill">默认</span>
              </span>
            </UiMenuRadioItem>
          </UiMenuRadioGroup>
        </UiMenu>

        <UiPopover v-model:open="tagPickerOpen" label="选择上传标签">
          <template #trigger>
            <button type="button" class="upload-opt" aria-label="选择上传标签">
              <i class="ri-price-tag-3-line" aria-hidden="true" />
              <template v-if="selectedTags.length">
                <span class="inline-flex" aria-hidden="true">
                  <span
                    v-for="tag in selectedTags.slice(0, 4)"
                    :key="tag!.id"
                    class="-mr-0.5 h-2 w-2 rounded-full shadow-[0_0_0_2px_var(--oi-raised)]"
                    :style="{ background: tagColor(tag!.name) }"
                  />
                </span>
                <span class="max-w-[180px] truncate">{{ selectedTags.length === 1 ? selectedTags[0]!.name : `${selectedTags.length} 个标签` }}</span>
              </template>
              <span v-else class="text-[12.5px] text-oi-muted">不添加标签</span>
              <i class="ri-arrow-down-s-line -ml-0.5 text-base text-oi-faint" aria-hidden="true" />
            </button>
          </template>
          <TagPicker v-model="tagIds" :allow-create="canCreateTag" autofocus class="w-[min(300px,calc(100vw-32px))]" />
        </UiPopover>

        <span class="flex-1 max-[900px]:hidden" />
        <label class="inline-flex cursor-pointer select-none items-center gap-2 text-[13px] text-oi-text" title="关闭时文件先进入待上传列表，确认后再上传">
          <UiSwitch v-model="autoStart" aria-label="添加后立即上传" />添加后立即上传
        </label>
      </div>

      <p v-if="noStorage" class="mb-3.5 flex items-center gap-1.5 rounded-oi border border-transparent bg-oi-warn-soft px-3.5 py-2.5 text-[13px] text-oi-warn" role="alert">
        <i class="ri-error-warning-line" aria-hidden="true" />没有可以上传的存储，请联系管理员授权或启用存储。
      </p>

      <!-- 拖放区：没有文件时大，有文件后收起 -->
      <section
        :class="['upload-drop', { compact: items.length, over: dragDepth > 0 }]"
        role="button"
        tabindex="0"
        :aria-label="`选择或拖入${options.videoEnabled ? '图片或视频' : '图片'}`"
        @click="pick"
        @keydown.enter.self.prevent="pick"
        @keydown.space.self.prevent="pick"
        @dragenter="onDragEnter"
        @dragleave="onDragLeave"
        @drop="onDrop"
      >
        <template v-if="!items.length">
          <div class="upload-art" aria-hidden="true">
            <span class="t1"><i class="ri-image-2-line" /></span>
            <span class="t2"><i :class="options.videoEnabled ? 'ri-film-line' : 'ri-landscape-line'" /></span>
            <span class="t3"><i class="ri-arrow-up-line" /></span>
          </div>
          <h2 class="text-[19px] font-[650] tracking-[-0.01em] text-oi-ink">拖入{{ options.videoEnabled ? '图片或视频' : '图片' }}，或点击选择</h2>
          <p class="mt-1.5 flex flex-wrap items-center justify-center gap-1.5 text-oi-muted">
            <span class="inline-flex items-center gap-1.5 max-[900px]:hidden">也可以直接 <kbd class="upload-kbd">{{ pasteKey }}</kbd> 粘贴截图 ·</span>
            每次最多 {{ options.maxFiles }} 个
          </p>
          <div class="mt-5 flex flex-wrap justify-center gap-2" @click.stop>
            <UiButton variant="primary" :disabled="noStorage" @click="pick"><i class="ri-add-line" aria-hidden="true" />选择文件</UiButton>
            <UiButton :disabled="noStorage" @click="openUrlDialog"><i class="ri-link" aria-hidden="true" />从 URL 导入</UiButton>
          </div>
          <p class="mt-[18px] text-xs text-oi-faint">{{ limits }}</p>
        </template>
        <template v-else>
          <span class="grid h-[38px] w-[38px] flex-none place-items-center rounded-[11px] bg-oi-btn text-[19px] text-oi-btn-ink" aria-hidden="true"><i class="ri-add-line" /></span>
          <span class="min-w-0 flex-1">
            <b class="font-semibold text-oi-ink">继续添加</b>
            <span class="block text-[12.5px] text-oi-muted">拖入、粘贴或点击选择 · 待上传 {{ queued.length }} / {{ options.maxFiles }}</span>
          </span>
          <UiButton variant="ghost" size="sm" :disabled="noStorage" @click.stop="openUrlDialog">
            <i class="ri-link" aria-hidden="true" /><span class="max-[900px]:hidden">URL</span>
          </UiButton>
        </template>
        <input ref="fileInput" type="file" multiple :accept="accept" hidden @change="onPick">
      </section>

      <!-- 待上传 / 本次上传 -->
      <section v-if="items.length" class="mt-4 overflow-hidden rounded-oi-lg border border-oi-line bg-oi-raised" aria-labelledby="upload-list-title">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-oi-line py-3 pl-[18px] pr-3.5">
          <h2 id="upload-list-title" class="flex items-center gap-2 text-sm font-[620] text-oi-ink">
            <template v-if="queued.length">待上传 <span class="upload-pill">{{ queued.length }} 个 · {{ formatBytes(queuedSize) }}</span></template>
            <template v-else-if="busy">正在上传 <span class="upload-pill">{{ overall }}%</span></template>
            <template v-else>本次上传 <span class="upload-pill">{{ done.length }} 个完成</span></template>
          </h2>
          <div class="flex flex-wrap items-center gap-2">
            <UiButton variant="ghost" size="sm" :disabled="busy" title="只清空列表，不删除已上传的文件" @click="clearItems">清空列表</UiButton>
            <template v-if="done.length && !queued.length">
              <UiSelect v-model="linkFormat" size="sm" aria-label="链接格式">
                <option v-for="format in LINK_FORMATS" :key="format.value" :value="format.value">{{ format.label }}</option>
              </UiSelect>
              <UiButton variant="primary" size="sm" @click="copyAll"><i class="ri-file-copy-line" aria-hidden="true" />复制全部</UiButton>
            </template>
            <UiButton v-if="queued.length" variant="accent" size="sm" :disabled="busy || noStorage" @click="startUpload">
              <i class="ri-upload-2-line" aria-hidden="true" />上传全部
            </UiButton>
          </div>
        </div>

        <ul>
          <li v-for="item in items" :key="item.id" class="upload-row">
            <button
              v-if="item.image"
              type="button"
              class="upload-thumb group relative focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring"
              :aria-label="`预览 ${item.name}`"
              @click="preview(item.image, done.map(entry => entry.image!))"
            >
              <MediaThumb :src="item.image.thumbnail || item.image.url" :alt="item.name" :video="isVideo(item.image)" class="h-full w-full" />
              <i class="ri-zoom-in-line absolute inset-0 grid place-items-center bg-black/40 text-lg text-white opacity-0 transition-opacity group-hover:opacity-100" aria-hidden="true" />
            </button>
            <MediaThumb v-else :src="item.previewUrl" :alt="item.name" :video="item.kind === 'video'" class="upload-thumb" />

            <div class="min-w-0">
              <div class="truncate font-[560] text-oi-ink" :title="item.name">{{ item.name }}</div>
              <div class="mt-[3px] flex flex-wrap items-center gap-1.5 text-xs text-oi-muted">
                <span v-if="itemSize(item)">{{ formatBytes(itemSize(item)) }}</span>
                <template v-if="savedLess(item)"><span class="upload-sep" /><span>原 {{ formatBytes(item.size) }}</span></template>
                <template v-if="item.bucketId && storageName(item.bucketId)"><span class="upload-sep" /><span>{{ storageName(item.bucketId) }}</span></template>
                <span v-for="id in item.tagIds" :key="id" class="upload-pill !h-[18px] !text-[11px]">
                  <span class="h-1.5 w-1.5 rounded-full" :style="{ background: tagColor(tagById(id)?.name || '') }" aria-hidden="true" />{{ tagById(id)?.name }}
                </span>
              </div>
            </div>

            <div class="upload-link min-w-0">
              <div v-if="item.status === 'uploading'" role="status">
                <div class="mb-1.5 flex justify-between text-xs text-oi-muted">
                  <span>{{ item.file ? (item.processing ? '服务器处理中…' : '上传中…') : '服务器正在下载…' }}</span>
                  <span v-if="!item.processing">{{ item.progress }}%</span>
                </div>
                <div class="h-1 overflow-hidden rounded-full bg-oi-line">
                  <span
                    :class="['block h-full rounded-full bg-[linear-gradient(90deg,var(--oi-accent),color-mix(in_srgb,var(--oi-accent)_50%,var(--oi-sakura)))] transition-[width] duration-200', item.processing ? 'animate-pulse' : '']"
                    :style="{ width: `${item.processing ? 100 : item.progress}%` }"
                  />
                </div>
              </div>
              <div v-else-if="item.status === 'failed'" class="flex items-center gap-2">
                <p class="flex min-w-0 flex-1 items-center gap-1.5 text-[12.5px] text-oi-danger" role="alert">
                  <i class="ri-error-warning-line flex-none" aria-hidden="true" /><span class="min-w-0 break-words">{{ item.error }}</span>
                </p>
                <UiButton v-if="item.retryable && item.file" size="sm" :disabled="busy" @click="retryItem(item)">重试</UiButton>
              </div>
              <p v-else-if="item.status === 'queued'" class="flex items-center gap-1.5 text-xs text-oi-muted"><i class="ri-time-line" aria-hidden="true" />等待上传</p>
              <template v-else-if="item.image">
                <p v-if="item.duplicate" class="mb-1.5 flex items-center gap-1.5 text-xs text-oi-warn">
                  <i class="ri-file-copy-2-line" aria-hidden="true" />已存在相同文件，返回已有链接，没有重复存储
                </p>
                <LinkBox :value="linkOf(item.image)" :label="`${item.name} 的${formatLabel}链接`" />
              </template>
            </div>

            <div class="upload-act">
              <UiButton variant="ghost" size="sm" icon :disabled="item.status === 'uploading'" :aria-label="`从列表移除 ${item.name}`" title="从列表移除" @click="removeItem(item)">
                <i class="ri-close-line" aria-hidden="true" />
              </UiButton>
            </div>
          </li>
        </ul>
      </section>
    </div>

    <!-- 今日与最近上传 -->
    <aside class="flex flex-col gap-[26px] border-l border-oi-line bg-oi-panel px-[22px] py-7 max-[1280px]:grid max-[1280px]:grid-cols-2 max-[1280px]:border-l-0 max-[1280px]:border-t max-[1280px]:px-9 max-[1080px]:px-6 max-[900px]:grid-cols-1 max-[900px]:px-4 max-[900px]:pb-8 max-[900px]:pt-5">
      <section aria-labelledby="upload-today-title">
        <h2 id="upload-today-title" class="upload-side-title">今日 <RouterLink to="/stats">统计 <i class="ri-arrow-right-s-line" aria-hidden="true" /></RouterLink></h2>
        <div class="grid grid-cols-3 gap-2">
          <div class="upload-stat"><b>{{ stats ? formatCount(stats.today_image_uploads) : '—' }}</b><span>图片</span></div>
          <div class="upload-stat"><b>{{ stats ? formatCount(stats.today_video_uploads) : '—' }}</b><span>视频</span></div>
          <div class="upload-stat"><b>{{ stats ? formatCount(stats.total_images + stats.total_videos) : '—' }}</b><span>{{ isAdmin ? '全站文件' : '我的文件' }}</span></div>
        </div>
      </section>

      <section aria-labelledby="upload-recent-title">
        <h2 id="upload-recent-title" class="upload-side-title">最近上传 <RouterLink to="/gallery">图库 <i class="ri-arrow-right-s-line" aria-hidden="true" /></RouterLink></h2>
        <div v-if="recent.length" class="grid grid-cols-3 gap-2">
          <div v-for="image in recent" :key="image.id" class="upload-recent group">
            <button type="button" class="block h-full w-full focus-visible:outline-none" :aria-label="`预览 ${image.original_file_name || image.file_name}`" @click="preview(image, recent)">
              <MediaThumb :src="image.thumbnail || image.url" :alt="image.original_file_name || image.file_name" :video="isVideo(image)" class="h-full w-full transition-transform duration-300 group-hover:scale-[1.06]" />
            </button>
            <div class="upload-recent-ov">
              <button type="button" :aria-label="`复制 ${image.original_file_name || image.file_name} 的链接`" @click="copyRecent(image)">
                <i class="ri-file-copy-line" aria-hidden="true" />复制
              </button>
            </div>
          </div>
        </div>
        <p v-else-if="stats" class="rounded-xl border border-dashed border-oi-line-2 px-3 py-5 text-center text-[12.5px] text-oi-muted">还没有上传过文件</p>
      </section>
    </aside>

    <UiDialog v-model:open="urlDialogOpen" title="从 URL 导入" description="由服务器下载这张图片，再按当前选择的存储和标签保存。" icon="ri-link">
      <form id="url-import-form" class="flex flex-col gap-3.5" novalidate @submit.prevent="submitImport">
        <UiField label="图片地址" for="url-import-input" :error="importError">
          <UiInput
            id="url-import-input"
            v-model="importUrl"
            type="url"
            mono
            :invalid="!!importError"
            placeholder="https://example.com/photo.jpg"
            autocomplete="off"
            @input="importError = ''"
          />
        </UiField>
        <p class="text-xs text-oi-muted">
          存到「{{ currentStorage?.name || '默认存储' }}」{{ selectedTags.length ? ` · ${selectedTags.length} 个标签` : '' }} · 只支持图片
        </p>
      </form>
      <template #footer="{ close }">
        <UiButton variant="ghost" @click="close">取消</UiButton>
        <UiButton type="submit" form="url-import-form" variant="primary" :disabled="!importUrl.trim() || importing">导入</UiButton>
      </template>
    </UiDialog>

    <MediaViewer v-model:open="viewerOpen" v-model:index="viewerIndex" :items="viewerItems" :can-delete="canDelete" @delete="deleteImage" />
  </div>
</template>

<style scoped>
.upload-opt {
  display: inline-flex; align-items: center; gap: 8px; height: 38px; padding: 0 12px 0 10px; border-radius: 11px;
  border: 1px solid var(--oi-line-2); background: var(--oi-raised); box-shadow: var(--oi-shadow-sm);
  font-size: 13.5px; color: var(--oi-ink); transition: border-color .15s, box-shadow .15s;
}
.upload-opt:hover, .upload-opt[aria-expanded="true"] { border-color: var(--oi-faint); }
.upload-opt:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--oi-ring); }
.upload-opt:disabled { opacity: .55; pointer-events: none; }
.upload-opt > i:first-child { font-size: 16px; color: var(--oi-muted); }

.upload-pill {
  display: inline-flex; align-items: center; gap: 5px; height: 22px; padding: 0 8px; border-radius: 99px; white-space: nowrap;
  font-size: 12px; font-weight: 500; background: var(--oi-panel); color: var(--oi-text); border: 1px solid var(--oi-line);
}
.upload-kbd {
  display: inline-flex; align-items: center; height: 18px; padding: 0 5px; border-radius: 5px; font-size: 11px;
  font-family: var(--oi-mono); color: var(--oi-muted); background: var(--oi-raised); border: 1px solid var(--oi-line);
}
.upload-sep { width: 3px; height: 3px; border-radius: 50%; background: var(--oi-faint); }

.upload-drop {
  position: relative; border-radius: 22px; padding: 56px 24px; text-align: center; cursor: pointer; outline: none;
  background:
    radial-gradient(120% 90% at 0% 0%, color-mix(in srgb, var(--oi-accent) 9%, transparent), transparent 55%),
    radial-gradient(110% 90% at 100% 100%, color-mix(in srgb, var(--oi-sakura) 10%, transparent), transparent 55%),
    var(--oi-panel);
  border: 1.5px dashed var(--oi-line-2); transition: border-color .2s, padding .3s, transform .2s, box-shadow .2s;
}
.upload-drop:hover { border-color: color-mix(in srgb, var(--oi-accent) 55%, var(--oi-line-2)); }
.upload-drop:focus-visible { border-color: var(--oi-accent); box-shadow: 0 0 0 3px var(--oi-ring); }
.upload-drop.over { border-color: var(--oi-accent); border-style: solid; transform: scale(1.005); box-shadow: 0 0 0 6px var(--oi-ring); }
.upload-drop.compact { padding: 14px 16px; display: flex; align-items: center; gap: 14px; text-align: left; border-radius: 16px; }

.upload-art { position: relative; width: 112px; height: 84px; margin: 0 auto 20px; }
.upload-art span {
  position: absolute; width: 58px; height: 72px; border-radius: 12px; display: grid; place-items: center; font-size: 22px;
  box-shadow: var(--oi-shadow); border: 3px solid var(--oi-raised); transition: transform .35s cubic-bezier(.3, .7, .4, 1.3);
}
.upload-art .t1 { left: 4px; top: 8px; transform: rotate(-12deg); background: var(--oi-accent-soft); color: var(--oi-accent-2); }
.upload-art .t2 { right: 4px; top: 8px; transform: rotate(12deg); background: var(--oi-sakura-soft); color: var(--oi-sakura); }
.upload-art .t3 { left: 50%; top: 0; margin-left: -29px; background: var(--oi-btn); color: var(--oi-btn-ink); font-size: 26px; z-index: 1; }
.upload-drop:hover .t1, .upload-drop.over .t1 { transform: rotate(-18deg) translate(-8px, 2px); }
.upload-drop:hover .t2, .upload-drop.over .t2 { transform: rotate(18deg) translate(8px, 2px); }
.upload-drop:hover .t3, .upload-drop.over .t3 { transform: translateY(-4px); }

.upload-row {
  display: grid; grid-template-columns: 52px minmax(0, 1fr) minmax(0, 1.25fr) auto; gap: 14px; align-items: center;
  padding: 12px 14px 12px 18px; border-bottom: 1px solid var(--oi-line); animation: upload-row-in .3s cubic-bezier(.3, .7, .4, 1);
}
.upload-row:last-child { border-bottom: 0; }
.upload-thumb { width: 52px; height: 52px; border-radius: 10px; overflow: hidden; border: 1px solid var(--oi-line); background: var(--oi-panel); }
@keyframes upload-row-in { from { opacity: 0; transform: translateY(6px); } }

.upload-side-title {
  display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px;
  font-size: 12.5px; font-weight: 600; color: var(--oi-ink);
}
.upload-side-title a { display: inline-flex; align-items: center; font-size: 12px; font-weight: 500; color: var(--oi-muted); }
.upload-side-title a:hover { color: var(--oi-accent); }
.upload-stat { background: var(--oi-raised); border: 1px solid var(--oi-line); border-radius: 12px; padding: 10px 12px; }
.upload-stat b { display: block; font-size: 18px; font-weight: 650; color: var(--oi-ink); font-variant-numeric: tabular-nums; letter-spacing: -.02em; }
.upload-stat span { font-size: 11.5px; color: var(--oi-muted); }

.upload-recent { position: relative; aspect-ratio: 1; border-radius: 11px; overflow: hidden; background: var(--oi-line); }
.upload-recent:focus-within { box-shadow: 0 0 0 3px var(--oi-ring); }
.upload-recent-ov {
  position: absolute; inset: auto 0 0; display: flex; justify-content: center; padding: 18px 6px 6px; pointer-events: none;
  background: linear-gradient(to top, rgba(0, 0, 0, .6), transparent); opacity: 0; transition: opacity .15s;
}
.upload-recent:hover .upload-recent-ov, .upload-recent:focus-within .upload-recent-ov { opacity: 1; }
.upload-recent-ov button {
  pointer-events: auto; height: 24px; padding: 0 8px; border-radius: 7px; background: rgba(255, 255, 255, .92); color: #131518;
  font-size: 11.5px; font-weight: 600; display: inline-flex; gap: 4px; align-items: center;
}

@media (max-width: 900px) {
  .upload-drop { padding: 36px 16px; }
  .upload-row { grid-template-columns: 48px minmax(0, 1fr) auto; gap: 10px 12px; padding: 12px 14px; }
  .upload-row .upload-thumb { width: 48px; height: 48px; }
  .upload-link { grid-column: 1 / -1; }
  .upload-act { grid-row: 1; grid-column: 3; }
}
@media (hover: none) {
  .upload-recent-ov { opacity: 1; }
}
@media (prefers-reduced-motion: reduce) {
  .upload-row { animation: none; }
  .upload-drop, .upload-art span { transition: none; }
}
</style>
