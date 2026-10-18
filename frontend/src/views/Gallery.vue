<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter, type LocationQuery } from 'vue-router'
import { api, type Image } from '@/api/client.ts'
import BatchCopyDialog from '@/components/gallery/BatchCopyDialog.vue'
import BatchTagDialog from '@/components/gallery/BatchTagDialog.vue'
import GalleryFilters from '@/components/gallery/GalleryFilters.vue'
import GalleryInspector from '@/components/gallery/GalleryInspector.vue'
import { SORT_OPTIONS, type GalleryFilter, type GallerySort, type MediaKind, type MediaStorage, type UploaderRole } from '@/components/gallery/types'
import MediaThumb from '@/components/media/MediaThumb.vue'
import MediaViewer from '@/components/media/MediaViewer.vue'
import TagManagerDialog from '@/components/tags/TagManagerDialog.vue'
import { confirm, toast, UiButton, UiDrawer, UiSegmented } from '@/components/ui'
import { LINK_FORMATS, useLinkFormat } from '@/composables/useLinkFormat'
import { useSession } from '@/composables/useSession'
import { loadTags, tagById, tagColor } from '@/composables/useTags'
import { formatBytes, formatCount, formatDateTime, formatDay } from '@/utils/format'
import { copyMediaText, isVideo, mediaLink, safeMediaURL } from '@/utils/media'
import { canManageImage, hasPermission, ROLE_ADMIN } from '@/utils/permissions.ts'

const PAGE_SIZE = 20
const VIEW_KEY = 'gallery-view'

const route = useRoute()
const router = useRouter()
const { user } = useSession()
const linkFormat = useLinkFormat()

const isAdmin = computed(() => Number(user.value.role) === ROLE_ADMIN)
const can = (image: Image, permission: string) => canManageImage(image, permission, user.value)
const canCreateTag = computed(() => hasPermission('tag:create', user.value))
const tagPermissions = computed(() => ({
  create: hasPermission('tag:create', user.value),
  update: hasPermission('tag:update', user.value),
  delete: hasPermission('tag:delete', user.value),
}))
const canManageTags = computed(() => tagPermissions.value.create || tagPermissions.value.update || tagPermissions.value.delete)

/* ---------- 筛选条件（同步到地址栏，书签和「在图库中查看」可用） ---------- */
const first = (value: LocationQuery[string]) => (Array.isArray(value) ? value[0] : value) ?? ''
const positiveInt = (value: string) => { const n = Number(value); return Number.isInteger(n) && n > 0 ? n : null }
const pick = <T extends string>(value: string, allowed: readonly T[], fallback: T) => (allowed as readonly string[]).includes(value) ? value as T : fallback

const filterFromQuery = (query: LocationQuery): GalleryFilter => ({
  type: pick<MediaKind>(first(query.type), ['all', 'image', 'video'], 'all'),
  tags: first(query.tags).split(',').map(positiveInt).filter((id): id is number => id !== null),
  untagged: first(query.untagged) === '1',
  storage: positiveInt(first(query.storage)),
  role: pick<UploaderRole>(first(query.role), ['all', 'admin', 'user'], 'all'),
  q: first(query.q),
  sort: pick<GallerySort>(first(query.sort), ['new', 'old', 'big', 'name'], 'new'),
})
const filter = reactive<GalleryFilter>(filterFromQuery(route.query))
const page = ref(positiveInt(first(route.query.page)) ?? 1)

const toQuery = () => {
  const query: Record<string, string> = {}
  if (filter.type !== 'all') query.type = filter.type
  if (filter.tags.length) query.tags = filter.tags.join(',')
  if (filter.untagged) query.untagged = '1'
  if (filter.storage) query.storage = String(filter.storage)
  if (isAdmin.value && filter.role !== 'all') query.role = filter.role
  if (filter.q.trim()) query.q = filter.q.trim()
  if (filter.sort !== 'new') query.sort = filter.sort
  if (page.value > 1) query.page = String(page.value)
  return query
}

/* ---------- 列表 ---------- */
const images = ref<Image[]>([])
const total = ref(0)
const totalPages = ref(1)
const loading = ref(true)
const loadError = ref('')
let requestSeq = 0

const loadImages = async () => {
  const seq = ++requestSeq
  loading.value = true
  loadError.value = ''
  const sort = SORT_OPTIONS.find(option => option.value === filter.sort) ?? SORT_OPTIONS[0]
  const params = new URLSearchParams({ page: String(page.value), page_size: String(PAGE_SIZE), sort: sort.sort, order: sort.order })
  if (filter.type !== 'all') params.set('media_type', filter.type)
  if (filter.tags.length) params.set('tag_ids', filter.tags.join(','))
  if (filter.untagged) params.set('untagged', 'true')
  if (filter.storage) params.set('bucket_id', String(filter.storage))
  if (isAdmin.value && filter.role !== 'all') params.set('uploader_role', filter.role === 'admin' ? '1' : '3')
  if (filter.q.trim()) params.set('q', filter.q.trim())
  try {
    const result = await api.listImagesPage(params.toString())
    if (seq !== requestSeq) return
    images.value = Array.isArray(result.data) ? result.data : []
    total.value = result.meta?.pagination?.total ?? images.value.length
    totalPages.value = Math.max(1, result.meta?.pagination?.total_pages ?? 1)
    // 删除后当前页可能已经超出范围
    if (page.value > totalPages.value) { page.value = totalPages.value; return }
    const ids = new Set(images.value.map(image => image.id))
    selected.value = selected.value.filter(id => ids.has(id))
  } catch (error) {
    if (seq !== requestSeq) return
    loadError.value = error instanceof Error && error.message ? error.message : '加载失败'
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
// 筛选变化回到第一页；搜索词稍等再请求
watch(() => [filter.type, filter.tags.join(','), filter.untagged, filter.storage, filter.role, filter.sort], () => {
  if (page.value !== 1) page.value = 1
  else void loadImages()
})
watch(() => filter.q, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    if (page.value !== 1) page.value = 1
    else void loadImages()
  }, 300)
})
watch(page, () => {
  void loadImages()
  window.scrollTo({ top: 0, behavior: 'smooth' })
})
watch([() => ({ ...filter, tags: [...filter.tags] }), page], () => {
  void router.replace({ query: toQuery() })
}, { deep: true })

/* ---------- 存储 ---------- */
const storages = ref<MediaStorage[]>([])
const loadStorages = async () => {
  try {
    const list = await api.listImageStorages()
    storages.value = Array.isArray(list) ? list : []
    if (filter.storage && !storages.value.some(storage => storage.id === filter.storage)) filter.storage = null
  } catch (error) {
    console.error('读取存储列表失败:', error)
  }
}

/* ---------- 显示 ---------- */
const readView = () => { try { return localStorage.getItem(VIEW_KEY) === 'list' ? 'list' : 'grid' } catch { return 'grid' } }
const view = ref<string>(readView())
watch(view, value => { try { localStorage.setItem(VIEW_KEY, value) } catch { /* 不保存也不影响使用 */ } })
const viewOptions = [{ value: 'grid', icon: 'ri-layout-grid-line', title: '网格' }, { value: 'list', icon: 'ri-list-check', title: '列表' }]

// 按最新上传排序时按天分组
const groups = computed(() => {
  if (filter.sort !== 'new') return [{ key: '', items: images.value }]
  const result: { key: string; items: Image[] }[] = []
  for (const image of images.value) {
    const key = formatDay(image.created_at)
    let group = result.find(item => item.key === key)
    if (!group) result.push(group = { key, items: [] })
    group.items.push(image)
  }
  return result
})
const name = (image: Image) => image.original_file_name || image.file_name || '文件'
const subtitle = (image: Image) => `${formatBytes(image.file_size)} · ${isVideo(image) ? (image.mime_type.split('/')[1] || '').toUpperCase() : `${image.width}×${image.height}`}`
const hasFilters = computed(() => filter.type !== 'all' || filter.tags.length > 0 || filter.untagged || !!filter.storage || filter.role !== 'all' || !!filter.q.trim())
const clearFilters = () => Object.assign(filter, { type: 'all', tags: [], untagged: false, storage: null, role: 'all', q: '' })
const removeTagFilter = (id: number) => { filter.tags = filter.tags.filter(item => item !== id) }

/* ---------- 复制 ---------- */
const formatLabel = computed(() => LINK_FORMATS.find(item => item.value === linkFormat.value)?.label ?? 'URL')
const copiedId = ref<number | null>(null)
let copiedTimer: ReturnType<typeof setTimeout> | undefined
const copyLink = async (image: Image) => {
  try {
    await copyMediaText(mediaLink(image, linkFormat.value, safeMediaURL(image.url)))
    toast.success(`已复制 ${formatLabel.value} 链接`)
    copiedId.value = image.id
    clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => { copiedId.value = null }, 1400)
  } catch {
    toast.error('复制失败，请手动复制')
  }
}

/* ---------- 选择与批量操作 ---------- */
const selected = ref<number[]>([])
const selecting = computed(() => selected.value.length > 0)
const selectedImages = computed(() => images.value.filter(image => selected.value.includes(image.id)))
const allSelected = computed(() => images.value.length > 0 && selected.value.length === images.value.length)
const toggleSelected = (id: number) => {
  selected.value = selected.value.includes(id) ? selected.value.filter(item => item !== id) : [...selected.value, id]
}
const toggleAll = () => { selected.value = allSelected.value ? [] : images.value.map(image => image.id) }
const batchCopyOpen = ref(false)
const batchTagOpen = ref(false)

/* ---------- 详情与查看器 ---------- */
const inspectId = ref<number | null>(null)
const inspectOpen = ref(false)
const inspected = computed(() => images.value.find(image => image.id === inspectId.value) ?? null)
const inspect = (image: Image) => {
  inspectId.value = image.id
  inspectOpen.value = true
}
watch(inspectOpen, value => { if (!value) inspectId.value = null })

const viewerOpen = ref(false)
const viewerIndex = ref(0)
const openViewer = (image: Image) => {
  viewerIndex.value = Math.max(0, images.value.findIndex(item => item.id === image.id))
  viewerOpen.value = true
}

const onTileClick = (image: Image, event: MouseEvent) => {
  if (selecting.value || event.metaKey || event.ctrlKey || event.shiftKey) toggleSelected(image.id)
  else inspect(image)
}
const onUpdated = (image: Image) => {
  images.value = images.value.map(item => item.id === image.id ? image : item)
}

/* ---------- 删除 ---------- */
const deleteImages = async (targets: Image[]) => {
  const allowed = targets.filter(image => can(image, 'image:delete'))
  if (!allowed.length) {
    toast.error('没有可删除的文件（无权限）')
    return
  }
  const skipped = targets.length - allowed.length
  const ok = await confirm({
    title: allowed.length === 1 ? `删除「${name(allowed[0])}」？` : `删除 ${allowed.length} 个文件？`,
    description: `文件会从存储中移除，已发布的外链将失效，无法撤销。${skipped ? `另有 ${skipped} 个文件无权删除，将跳过。` : ''}`,
    danger: true,
    confirmText: '删除',
  })
  if (!ok) return
  let failed = 0
  const queue = [...allowed]
  // 没有批量删除接口，同时最多发 3 个请求
  await Promise.all(Array.from({ length: Math.min(3, queue.length) }, async () => {
    for (let image = queue.shift(); image; image = queue.shift()) {
      try { await api.deleteImage(image.id) } catch { failed += 1 }
    }
  }))
  const deleted = allowed.length - failed
  if (deleted) toast.success(`已删除 ${deleted} 个文件`)
  if (failed) toast.error(`${failed} 个文件删除失败`)
  if (inspectId.value && allowed.some(image => image.id === inspectId.value)) inspectOpen.value = false
  await loadImages()
  void loadTags(true).catch(() => undefined)
  void loadStorages()
}

/* ---------- 标签管理 ---------- */
const tagManagerOpen = ref(false)
const showTag = (id: number) => Object.assign(filter, { tags: [id], untagged: false })

/* ---------- 手机端筛选抽屉 ---------- */
const filtersOpen = ref(false)

// Esc：先取消选择；弹窗打开时由弹窗处理
const onKeydown = (event: KeyboardEvent) => {
  if (event.key !== 'Escape' || !selecting.value || event.defaultPrevented) return
  if (document.querySelector('[role="dialog"], [role="alertdialog"]')) return
  selected.value = []
}

onMounted(() => {
  void loadImages()
  void loadStorages()
  loadTags(true).catch(() => toast.error('读取标签失败'))
  // 旧地址 /tags 跳转到 /gallery?view=tags：打开标签管理
  if (route.query.view === 'tags') {
    if (canManageTags.value) tagManagerOpen.value = true
    void router.replace({ query: toQuery() })
  }
  window.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  clearTimeout(searchTimer)
  clearTimeout(copiedTimer)
})
</script>

<template>
  <div class="grid flex-1 grid-cols-[228px_minmax(0,1fr)] font-oi text-sm text-oi-text max-[900px]:grid-cols-1">
    <!-- 桌面端筛选栏 -->
    <aside class="border-r border-oi-line bg-oi-panel max-[900px]:hidden" aria-label="筛选">
      <div class="sticky top-0 max-h-dvh overflow-auto px-3 py-5">
        <GalleryFilters v-model="filter" :storages="storages" :is-admin="isAdmin" :can-manage-tags="canManageTags" @manage-tags="tagManagerOpen = true" />
      </div>
    </aside>

    <div class="flex min-w-0 flex-col">
      <header class="px-[26px] pt-[22px] max-[900px]:px-4 max-[900px]:pt-4">
        <div class="flex items-baseline gap-2.5">
          <h1 class="text-[22px] font-[680] tracking-[-0.02em] text-oi-ink">图库</h1>
          <span class="text-[13px] text-oi-muted" role="status">{{ isAdmin ? '全站' : '我的' }} {{ loading && !images.length ? '…' : formatCount(total) }} 个文件</span>
        </div>
        <div v-if="filter.tags.length || filter.untagged" class="mt-2.5 flex flex-wrap items-center gap-1.5 text-[12.5px]">
          <span class="text-oi-muted">标签含任一：</span>
          <span v-for="id in filter.tags" :key="id" class="inline-flex h-6 items-center gap-[5px] rounded-full border border-oi-line bg-oi-panel pl-2 pr-1 text-xs text-oi-ink">
            <span class="h-[7px] w-[7px] rounded-full" :style="{ background: tagColor(tagById(id)?.name || '') }" aria-hidden="true" />{{ tagById(id)?.name ?? `#${id}` }}
            <button type="button" class="grid h-[18px] w-[18px] place-items-center rounded-full text-[13px] text-oi-muted hover:bg-oi-line hover:text-oi-ink" :aria-label="`取消筛选 ${tagById(id)?.name ?? id}`" @click="removeTagFilter(id)"><i class="ri-close-line" aria-hidden="true" /></button>
          </span>
          <span v-if="filter.untagged" class="inline-flex h-6 items-center gap-[5px] rounded-full border border-oi-line bg-oi-panel pl-2 pr-1 text-xs text-oi-ink">
            未加标签
            <button type="button" class="grid h-[18px] w-[18px] place-items-center rounded-full text-[13px] text-oi-muted hover:bg-oi-line hover:text-oi-ink" aria-label="取消筛选未加标签" @click="filter.untagged = false"><i class="ri-close-line" aria-hidden="true" /></button>
          </span>
          <button type="button" class="text-oi-accent-2 hover:underline" @click="filter.tags = []; filter.untagged = false">清除</button>
        </div>
      </header>

      <div class="flex flex-wrap items-center gap-2 border-b border-oi-line px-[26px] py-3.5 max-[900px]:px-4">
        <label class="relative min-w-[180px] max-w-[360px] flex-1">
          <i class="ri-search-line pointer-events-none absolute left-[11px] top-1/2 -translate-y-1/2 text-base text-oi-faint" aria-hidden="true" />
          <input v-model="filter.q" type="search" placeholder="原始文件名 / 文件名 / URL / 哈希" aria-label="搜索文件" class="gallery-input w-full !pl-[34px]">
        </label>
        <UiButton icon class="min-[901px]:hidden" aria-label="筛选" :aria-expanded="filtersOpen" @click="filtersOpen = true">
          <i class="ri-filter-3-line" aria-hidden="true" />
        </UiButton>
        <span class="flex-1" />
        <select v-model="filter.sort" class="gallery-input" aria-label="排序">
          <option v-for="option in SORT_OPTIONS" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <select v-model="linkFormat" class="gallery-input" aria-label="复制的链接格式" title="复制的链接格式">
          <option v-for="format in LINK_FORMATS" :key="format.value" :value="format.value">{{ format.label }}</option>
        </select>
        <UiSegmented v-model="view" :options="viewOptions" aria-label="显示方式" />
      </div>

      <div class="relative min-h-[60vh] px-[26px] pb-28 pt-[18px] max-[900px]:px-4">
        <div v-if="loadError" class="py-16 text-center text-oi-muted" role="alert">
          <i class="ri-error-warning-line mb-2.5 block text-[34px] text-oi-danger" aria-hidden="true" />
          <h2 class="mb-1 text-[15px] font-semibold text-oi-ink">加载失败</h2>
          <p class="mb-4">{{ loadError }}</p>
          <UiButton @click="loadImages">重试</UiButton>
        </div>

        <div v-else-if="loading && !images.length" class="grid grid-cols-[repeat(auto-fill,minmax(176px,1fr))] gap-x-3.5 gap-y-4" aria-busy="true" aria-label="正在加载">
          <div v-for="n in 8" :key="n" class="aspect-[4/3] animate-pulse rounded-[14px] bg-oi-panel" />
        </div>

        <div v-else-if="!images.length" class="py-16 text-center text-oi-muted">
          <i class="ri-image-line mb-2.5 block text-[34px] text-oi-faint" aria-hidden="true" />
          <h2 class="mb-1 text-[15px] font-semibold text-oi-ink">{{ hasFilters ? '没有符合条件的文件' : '还没有文件' }}</h2>
          <p>
            <template v-if="hasFilters"><button type="button" class="text-oi-accent-2 hover:underline" @click="clearFilters">清除筛选</button>，或 </template>
            <RouterLink to="/upload" class="text-oi-accent-2 hover:underline">去上传</RouterLink>
          </p>
        </div>

        <template v-else-if="view === 'grid'">
          <section v-for="group in groups" :key="group.key" :aria-label="group.key || '文件'" class="[&:not(:first-child)]:mt-[22px]">
            <h2 v-if="group.key" class="mb-2.5 mt-1 flex items-center gap-2 text-[12.5px] font-semibold text-oi-muted">
              {{ group.key }} <span class="font-normal">{{ group.items.length }}</span>
            </h2>
            <ul :class="['grid grid-cols-[repeat(auto-fill,minmax(176px,1fr))] gap-x-3.5 gap-y-4 max-[480px]:grid-cols-2 max-[480px]:gap-x-2.5', selecting && 'is-selecting']">
              <li v-for="image in group.items" :key="image.id" :class="['gallery-tile group relative', selected.includes(image.id) && 'is-selected', inspectId === image.id && 'is-active']">
                <div class="tile-img relative aspect-[4/3] overflow-hidden rounded-[14px] border border-oi-line bg-oi-panel">
                  <button
                    type="button"
                    class="block h-full w-full focus-visible:outline-none"
                    :aria-label="`${selecting ? '选择' : '查看详情'} ${name(image)}`"
                    @click="onTileClick(image, $event)"
                    @dblclick="openViewer(image)"
                    @keydown.space.prevent="toggleSelected(image.id)"
                  >
                    <MediaThumb :src="image.thumbnail || image.url" :alt="name(image)" :video="isVideo(image)" class="h-full w-full transition-transform duration-[450ms] ease-[cubic-bezier(.3,.7,.4,1)] group-hover:scale-[1.04]" />
                  </button>
                  <button
                    type="button"
                    role="checkbox"
                    :aria-checked="selected.includes(image.id)"
                    :aria-label="`选择 ${name(image)}`"
                    class="tile-check absolute left-2 top-2 z-[2]"
                    @click="toggleSelected(image.id)"
                  >
                    <span :class="['grid h-[18px] w-[18px] place-items-center rounded-md border-[1.5px] text-[13px] transition-colors', selected.includes(image.id) ? 'border-oi-accent bg-oi-accent text-white' : 'border-black/15 bg-white/90 text-transparent']"><i class="ri-check-line" aria-hidden="true" /></span>
                  </button>
                  <span v-if="isVideo(image)" class="pointer-events-none absolute right-2 top-2 inline-flex h-[22px] items-center gap-[3px] rounded-[7px] bg-black/55 px-[7px] text-[11px] font-semibold text-white backdrop-blur-[6px]">
                    <i class="ri-play-fill" aria-hidden="true" />{{ (image.mime_type.split('/')[1] || '').toUpperCase() }}
                  </span>
                  <div class="tile-quick absolute bottom-2 right-2 z-[2] flex gap-1">
                    <button type="button" class="tile-quick-btn w-7 !px-0" :aria-label="`放大查看 ${name(image)}`" title="放大查看" @click="openViewer(image)"><i class="ri-zoom-in-line" aria-hidden="true" /></button>
                    <button type="button" :class="['tile-quick-btn', copiedId === image.id && '!text-[#0b8352]']" :aria-label="`复制 ${name(image)} 的${formatLabel}链接`" @click="copyLink(image)">
                      <i :class="copiedId === image.id ? 'ri-check-line' : 'ri-file-copy-line'" aria-hidden="true" />{{ copiedId === image.id ? '已复制' : '复制' }}
                    </button>
                  </div>
                </div>
                <div class="px-0.5 pt-2">
                  <div class="truncate text-[13px] font-[540] text-oi-ink" :title="name(image)">{{ name(image) }}</div>
                  <div class="mt-px flex items-center gap-1.5 text-[11.5px] text-oi-muted">
                    <span class="truncate">{{ subtitle(image) }}</span>
                    <span class="ml-auto inline-flex gap-[3px]">
                      <span v-for="tag in image.tags.slice(0, 6)" :key="tag.id" class="h-[7px] w-[7px] rounded-full" :style="{ background: tagColor(tag.name) }" :title="tag.name" />
                    </span>
                  </div>
                </div>
              </li>
            </ul>
          </section>
        </template>

        <div v-else class="-mx-3 overflow-x-auto">
          <table class="w-full border-separate border-spacing-0 text-[13px]">
            <thead>
              <tr class="[&>th]:border-b [&>th]:border-oi-line [&>th]:px-3 [&>th]:py-2 [&>th]:text-left [&>th]:text-xs [&>th]:font-[550] [&>th]:text-oi-muted">
                <th class="w-9">
                  <button type="button" role="checkbox" :aria-checked="allSelected" aria-label="全选本页" @click="toggleAll">
                    <span :class="['grid h-[18px] w-[18px] place-items-center rounded-md border-[1.5px] text-[13px]', allSelected ? 'border-oi-accent bg-oi-accent text-white' : 'border-oi-line-2 bg-oi-raised text-transparent']"><i class="ri-check-line" aria-hidden="true" /></span>
                  </button>
                </th>
                <th>文件</th>
                <th class="max-[900px]:hidden">标签</th>
                <th class="max-[900px]:hidden">存储</th>
                <th class="max-[900px]:hidden">大小</th>
                <th class="max-[900px]:hidden">上传</th>
                <th class="text-right">链接</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="image in images"
                :key="image.id"
                :class="['[&>td]:border-b [&>td]:border-oi-line [&>td]:px-3 [&>td]:py-[9px]', selected.includes(image.id) ? '[&>td]:bg-oi-accent-soft' : 'hover:[&>td]:bg-oi-panel']"
              >
                <td>
                  <button type="button" role="checkbox" :aria-checked="selected.includes(image.id)" :aria-label="`选择 ${name(image)}`" @click="toggleSelected(image.id)">
                    <span :class="['grid h-[18px] w-[18px] place-items-center rounded-md border-[1.5px] text-[13px]', selected.includes(image.id) ? 'border-oi-accent bg-oi-accent text-white' : 'border-oi-line-2 bg-oi-raised text-transparent']"><i class="ri-check-line" aria-hidden="true" /></span>
                  </button>
                </td>
                <td>
                  <div class="flex min-w-0 items-center gap-3">
                    <button type="button" class="h-10 w-10 flex-none overflow-hidden rounded-lg border border-oi-line" :aria-label="`放大查看 ${name(image)}`" @click="openViewer(image)">
                      <MediaThumb :src="image.thumbnail || image.url" :alt="name(image)" :video="isVideo(image)" class="h-full w-full" />
                    </button>
                    <button type="button" class="min-w-0 text-left" :aria-label="`查看详情 ${name(image)}`" @click="inspect(image)">
                      <span class="block max-w-[260px] truncate font-[540] text-oi-ink">{{ name(image) }}</span>
                      <span class="block max-w-[260px] truncate font-oi-mono text-[11.5px] text-oi-muted">{{ image.file_name }}</span>
                    </button>
                  </div>
                </td>
                <td class="max-[900px]:hidden">
                  <div class="flex flex-wrap gap-1">
                    <span v-for="tag in image.tags" :key="tag.id" class="inline-flex h-[22px] items-center gap-[5px] whitespace-nowrap rounded-full border border-oi-line bg-oi-panel px-2 text-xs">
                      <span class="h-1.5 w-1.5 rounded-full" :style="{ background: tagColor(tag.name) }" aria-hidden="true" />{{ tag.name }}
                    </span>
                  </div>
                </td>
                <td class="whitespace-nowrap max-[900px]:hidden">
                  {{ storages.find(storage => storage.id === image.bucket_id)?.name ?? image.storage }}<span v-if="storages.find(storage => storage.id === image.bucket_id)?.disabled" class="text-oi-muted">（已停用）</span>
                </td>
                <td class="whitespace-nowrap tabular-nums max-[900px]:hidden">{{ formatBytes(image.file_size) }}</td>
                <td class="whitespace-nowrap tabular-nums text-oi-muted max-[900px]:hidden">{{ formatDateTime(image.created_at) }}</td>
                <td class="text-right">
                  <UiButton size="sm" :aria-label="`复制 ${name(image)} 的${formatLabel}链接`" @click="copyLink(image)">
                    <i :class="copiedId === image.id ? 'ri-check-line' : 'ri-file-copy-line'" aria-hidden="true" />{{ copiedId === image.id ? '已复制' : '复制' }}
                  </UiButton>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <nav v-if="images.length && totalPages > 1" class="flex items-center justify-center gap-3 pb-1 pt-6 text-[12.5px] text-oi-muted" aria-label="分页">
          <UiButton variant="ghost" size="sm" :disabled="page <= 1 || loading" @click="page -= 1"><i class="ri-arrow-left-s-line" aria-hidden="true" />上一页</UiButton>
          <span class="tabular-nums">第 {{ page }} / {{ totalPages }} 页 · 每页 {{ PAGE_SIZE }} 个</span>
          <UiButton variant="ghost" size="sm" :disabled="page >= totalPages || loading" @click="page += 1">下一页<i class="ri-arrow-right-s-line" aria-hidden="true" /></UiButton>
        </nav>
      </div>
    </div>

    <!-- 批量操作栏 -->
    <Transition
      enter-from-class="opacity-0 translate-y-3"
      leave-to-class="opacity-0 translate-y-3"
      enter-active-class="transition duration-200 ease-[cubic-bezier(.3,.7,.4,1)]"
      leave-active-class="transition duration-200 ease-[cubic-bezier(.3,.7,.4,1)]"
    >
      <div
        v-if="selecting"
        class="fixed bottom-6 left-1/2 z-[70] flex -translate-x-1/2 items-center gap-0.5 rounded-[14px] bg-oi-btn py-1.5 pl-3.5 pr-1.5 text-oi-btn-ink shadow-oi-lg max-[900px]:bottom-[calc(74px+env(safe-area-inset-bottom))] min-[900px]:ml-[106px] min-[1081px]:ml-[118px]"
        role="toolbar"
        aria-label="批量操作"
      >
        <span class="mr-2 whitespace-nowrap text-[13px] font-semibold">已选 {{ selected.length }}</span>
        <button type="button" class="selbar-btn" @click="toggleAll"><i class="ri-checkbox-multiple-line" aria-hidden="true" /><span class="max-[600px]:hidden">{{ allSelected ? '取消全选' : '全选本页' }}</span></button>
        <span class="mx-1 h-[18px] w-px bg-[color-mix(in_srgb,var(--oi-btn-ink)_22%,transparent)]" aria-hidden="true" />
        <button type="button" class="selbar-btn" aria-label="复制链接" @click="batchCopyOpen = true"><i class="ri-file-copy-line" aria-hidden="true" /><span class="max-[600px]:hidden">复制链接</span></button>
        <button type="button" class="selbar-btn" aria-label="编辑标签" @click="batchTagOpen = true"><i class="ri-price-tag-3-line" aria-hidden="true" /><span class="max-[600px]:hidden">标签</span></button>
        <button type="button" class="selbar-btn !text-[#ff8a8e]" aria-label="删除所选" @click="deleteImages(selectedImages)"><i class="ri-delete-bin-6-line" aria-hidden="true" /><span class="max-[600px]:hidden">删除</span></button>
        <span class="mx-1 h-[18px] w-px bg-[color-mix(in_srgb,var(--oi-btn-ink)_22%,transparent)]" aria-hidden="true" />
        <button type="button" class="selbar-btn" aria-label="取消选择" title="取消选择 (Esc)" @click="selected = []"><i class="ri-close-line" aria-hidden="true" /></button>
      </div>
    </Transition>

    <UiDrawer v-model:open="filtersOpen" title="筛选">
      <GalleryFilters v-model="filter" :storages="storages" :is-admin="isAdmin" :can-manage-tags="canManageTags" @manage-tags="filtersOpen = false; tagManagerOpen = true" />
      <template #footer>
        <UiButton variant="ghost" :disabled="!hasFilters" @click="clearFilters">清除筛选</UiButton>
        <UiButton variant="primary" @click="filtersOpen = false">完成</UiButton>
      </template>
    </UiDrawer>

    <GalleryInspector
      v-model:open="inspectOpen"
      :image="inspected"
      :storages="storages"
      :is-admin="isAdmin"
      :can="can"
      :can-create-tag="canCreateTag"
      @view="openViewer"
      @delete="image => deleteImages([image])"
      @updated="onUpdated"
    />
    <MediaViewer
      v-model:open="viewerOpen"
      v-model:index="viewerIndex"
      :items="images"
      :can-delete="image => can(image, 'image:delete')"
      @delete="image => deleteImages([image])"
    />
    <BatchCopyDialog v-model:open="batchCopyOpen" :images="selectedImages" />
    <BatchTagDialog v-model:open="batchTagOpen" :images="selectedImages" :can="can" :can-create-tag="canCreateTag" @done="loadImages" />
    <TagManagerDialog
      v-model:open="tagManagerOpen"
      :can-create="tagPermissions.create"
      :can-update="tagPermissions.update"
      :can-delete="tagPermissions.delete"
      @show-tag="showTag"
    />
  </div>
</template>

<style scoped>
.gallery-input {
  height: 36px; border-radius: 10px; border: 1px solid var(--oi-line-2); background: var(--oi-raised); padding-inline: 10px;
  font-size: 13.5px; color: var(--oi-ink); outline: none;
}
.gallery-input:focus { border-color: var(--oi-accent); box-shadow: 0 0 0 3px var(--oi-ring); }

.tile-img { transition: box-shadow .2s; }
.gallery-tile:hover .tile-img { box-shadow: var(--oi-shadow); }
.gallery-tile.is-selected .tile-img, .gallery-tile.is-active .tile-img { box-shadow: 0 0 0 2.5px var(--oi-accent), var(--oi-shadow); border-color: transparent; }
.gallery-tile:has(button:first-child:focus-visible) .tile-img { box-shadow: 0 0 0 2.5px var(--oi-accent); }

.tile-check { opacity: 0; transition: opacity .15s; }
.tile-check:focus-visible { opacity: 1; outline: none; }
.tile-check:focus-visible > span { box-shadow: 0 0 0 3px var(--oi-ring); }
.gallery-tile:hover .tile-check, .gallery-tile.is-selected .tile-check, .is-selecting .tile-check { opacity: 1; }

.tile-quick { opacity: 0; transform: translateY(4px); transition: opacity .18s, transform .18s; }
.gallery-tile:hover .tile-quick, .gallery-tile:focus-within .tile-quick { opacity: 1; transform: none; }
.is-selecting .tile-quick { display: none; }
.tile-quick-btn {
  display: inline-flex; align-items: center; justify-content: center; gap: 4px; height: 28px; padding: 0 9px; border-radius: 8px;
  background: rgba(255, 255, 255, .94); color: #131518; font-size: 12px; font-weight: 600; box-shadow: 0 2px 8px rgba(0, 0, 0, .15);
}
.tile-quick-btn:hover { background: #fff; }
.tile-quick-btn:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--oi-ring); }

.selbar-btn {
  display: inline-flex; align-items: center; gap: 6px; height: 32px; padding: 0 10px; border-radius: 9px;
  font-size: 13px; font-weight: 520; color: inherit; opacity: .88;
}
.selbar-btn:hover { background: color-mix(in srgb, var(--oi-btn-ink) 14%, transparent); opacity: 1; }
.selbar-btn:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--oi-ring); }

/* 触屏没有悬停：勾选框和快捷按钮始终显示 */
@media (hover: none) {
  .tile-check, .tile-quick { opacity: 1; transform: none; }
}
@media (prefers-reduced-motion: reduce) {
  .tile-quick, .tile-check { transition: none; }
}
</style>
