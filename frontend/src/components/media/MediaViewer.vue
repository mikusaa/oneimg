<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import type { Image } from '@/api/client.ts'
import { toast } from '@/components/ui'
import { LINK_FORMATS, useLinkFormat } from '@/composables/useLinkFormat'
import { formatBytes } from '@/utils/format'
import { copyMediaText, isVideo, mediaLink, safeDownloadName, safeMediaURL } from '@/utils/media'
import MediaThumb from './MediaThumb.vue'

// 全屏查看器：图片可缩放（滚轮、双指、双击、+ / - / 0）和拖动，左右切换；视频用浏览器原生控件播放

const props = defineProps<{
  items: Image[]
  /** 返回 true 时显示删除按钮；删除由父组件确认并执行 */
  canDelete?: (image: Image) => boolean
}>()
const emit = defineEmits<{ delete: [image: Image] }>()

const open = defineModel<boolean>('open', { default: false })
const index = defineModel<number>('index', { default: 0 })
const linkFormat = useLinkFormat()

const total = computed(() => props.items.length)
const file = computed(() => props.items[Math.min(Math.max(index.value, 0), total.value - 1)])
const url = computed(() => file.value ? safeMediaURL(file.value.url) : '')
const video = computed(() => !!file.value && isVideo(file.value))
const name = (image: Image) => image.original_file_name || image.file_name || '文件'

const zoom = reactive({ scale: 1, x: 0, y: 0, animate: true })
const natural = reactive({ width: 0, height: 0 })
const loading = ref(true)
const failed = ref(false)
const stage = ref<HTMLElement | null>(null)
const img = ref<HTMLImageElement | null>(null)
const strip = ref<HTMLElement | null>(null)

const reset = () => Object.assign(zoom, { scale: 1, x: 0, y: 0, animate: true })
watch(() => file.value?.id, () => {
  reset()
  loading.value = true
  failed.value = false
  nextTick(() => strip.value?.querySelector('[aria-current="true"]')?.scrollIntoView({ block: 'nearest', inline: 'center' }))
})
// 列表变短（如删除后）时保持下标有效，列表为空时关闭
watch(total, value => {
  if (!value) open.value = false
  else if (index.value >= value) index.value = value - 1
})

const onLoad = (event: Event) => {
  const target = event.target as HTMLImageElement
  natural.width = target.naturalWidth
  natural.height = target.naturalHeight
  loading.value = false
}

// 「100%」对应的缩放倍数：图片按原始像素显示
const actualScale = () => img.value?.clientWidth ? natural.width / img.value.clientWidth : 1
const maxScale = () => Math.max(6, actualScale() * 3)
const percent = computed(() => img.value?.clientWidth && natural.width ? Math.round(zoom.scale * img.value.clientWidth / natural.width * 100) : 100)

const zoomTo = (next: number, clientX?: number, clientY?: number) => {
  if (!stage.value) return
  const scale = Math.min(maxScale(), Math.max(1, next))
  if (scale === 1) { Object.assign(zoom, { scale: 1, x: 0, y: 0 }); return }
  const rect = stage.value.getBoundingClientRect()
  const ox = (clientX ?? rect.left + rect.width / 2) - (rect.left + rect.width / 2)
  const oy = (clientY ?? rect.top + rect.height / 2) - (rect.top + rect.height / 2)
  zoom.x = ox - (ox - zoom.x) * (scale / zoom.scale)
  zoom.y = oy - (oy - zoom.y) * (scale / zoom.scale)
  zoom.scale = scale
}
const zoomBy = (factor: number) => { zoom.animate = true; zoomTo(zoom.scale * factor) }
const toggleActual = (clientX?: number, clientY?: number) => {
  zoom.animate = true
  const target = actualScale() > 1.05 ? actualScale() : 2.5
  zoomTo(zoom.scale > 1.01 ? 1 : target, clientX, clientY)
}
const onWheel = (event: WheelEvent) => {
  if (video.value || failed.value) return
  event.preventDefault()
  zoom.animate = false
  zoomTo(zoom.scale * Math.exp(-event.deltaY * 0.0018), event.clientX, event.clientY)
}

const go = (delta: number) => {
  if (total.value < 2) return
  index.value = (index.value + delta + total.value) % total.value
}

// 拖动、双指缩放；未放大时左右滑动切换
const pointers = new Map<number, { x: number; y: number }>()
let start: { x: number; y: number; zx: number; zy: number } | null = null
let pinch: { distance: number; scale: number; cx: number; cy: number } | null = null
let moved = false
let clickTimer: ReturnType<typeof setTimeout> | undefined

const onPointerDown = (event: PointerEvent) => {
  if (video.value || failed.value || !stage.value) return
  if ((event.target as HTMLElement).closest('button')) return
  stage.value.setPointerCapture?.(event.pointerId)
  pointers.set(event.pointerId, { x: event.clientX, y: event.clientY })
  moved = false
  zoom.animate = false
  if (pointers.size === 1) start = { x: event.clientX, y: event.clientY, zx: zoom.x, zy: zoom.y }
  if (pointers.size === 2) {
    const [a, b] = [...pointers.values()]
    pinch = { distance: Math.hypot(a.x - b.x, a.y - b.y), scale: zoom.scale, cx: (a.x + b.x) / 2, cy: (a.y + b.y) / 2 }
  }
}
const onPointerMove = (event: PointerEvent) => {
  if (!pointers.has(event.pointerId)) return
  pointers.set(event.pointerId, { x: event.clientX, y: event.clientY })
  if (pinch && pointers.size === 2) {
    const [a, b] = [...pointers.values()]
    zoomTo(pinch.scale * Math.hypot(a.x - b.x, a.y - b.y) / pinch.distance, pinch.cx, pinch.cy)
    moved = true
    return
  }
  if (!start) return
  const dx = event.clientX - start.x
  const dy = event.clientY - start.y
  if (Math.abs(dx) + Math.abs(dy) > 4) moved = true
  if (zoom.scale > 1) { zoom.x = start.zx + dx; zoom.y = start.zy + dy } else zoom.x = dx * 0.6
}
const onPointerUp = (event: PointerEvent) => {
  pointers.delete(event.pointerId)
  if (pointers.size < 2) pinch = null
  if (!pointers.size && start) {
    const dx = event.clientX - start.x
    if (zoom.scale <= 1.001) {
      zoom.animate = true
      if (Math.abs(dx) > 70) go(dx < 0 ? 1 : -1)
      zoom.x = 0
      zoom.y = 0
    }
    start = null
  }
}
// 未放大时点图片外的空白处关闭（等一下，避免和双击冲突）
const onStageClick = (event: MouseEvent) => {
  if (!stage.value || moved || event.target !== stage.value || zoom.scale > 1.001) return
  // 图片不接收指针事件，点在图片上的也会落到舞台上
  const rect = img.value?.getBoundingClientRect()
  if (rect && !failed.value && event.clientX >= rect.left && event.clientX <= rect.right && event.clientY >= rect.top && event.clientY <= rect.bottom) return
  clearTimeout(clickTimer)
  clickTimer = setTimeout(() => { open.value = false }, 220)
}
const onStageDoubleClick = (event: MouseEvent) => {
  clearTimeout(clickTimer)
  if (!video.value && !failed.value) toggleActual(event.clientX, event.clientY)
}

const onKeydown = (event: KeyboardEvent) => {
  if ((event.target as HTMLElement).closest('input, textarea, select, video')) return
  const key = event.key
  if (key === 'ArrowRight') go(1)
  else if (key === 'ArrowLeft') go(-1)
  else if (!video.value && (key === '+' || key === '=')) zoomBy(1.4)
  else if (!video.value && key === '-') zoomBy(1 / 1.4)
  else if (!video.value && key === '0') reset()
  else return
  event.preventDefault()
}

const formatLabel = computed(() => LINK_FORMATS.find(item => item.value === linkFormat.value)?.label ?? 'URL')
const copyLink = async () => {
  if (!file.value || !url.value) return
  try {
    await copyMediaText(mediaLink(file.value, linkFormat.value, url.value))
    toast.success(`已复制 ${formatLabel.value} 链接`)
  } catch {
    toast.error('复制失败，请手动复制')
  }
}

// 预加载前后两张图片
const neighbors = computed(() => total.value < 2 ? [] : [1, -1]
  .map(delta => props.items[(index.value + delta + total.value) % total.value])
  .filter(image => image && !isVideo(image))
  .map(image => safeMediaURL(image.url))
  .filter(Boolean))

const summary = computed(() => {
  const image = file.value
  if (!image) return ''
  const kind = video.value ? image.mime_type.split('/')[1]?.toUpperCase() : `${image.width} × ${image.height}`
  return [kind, formatBytes(image.file_size), total.value > 1 ? `${index.value + 1} / ${total.value}` : ''].filter(Boolean).join(' · ')
})
</script>

<template>
  <DialogRoot v-model:open="open">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-[180] bg-[rgba(8,9,10,0.96)] backdrop-blur-[6px] data-[state=open]:animate-oi-fade motion-reduce:animate-none" />
      <DialogContent
        v-if="file"
        class="fixed inset-0 z-[181] flex flex-col font-oi text-[#f1f2f0] outline-none data-[state=open]:animate-oi-fade motion-reduce:animate-none"
        @keydown="onKeydown"
      >
        <div class="flex flex-none items-center justify-between gap-4 py-2.5 pl-5 pr-3.5 max-[900px]:py-2 max-[900px]:pl-3.5 max-[900px]:pr-1.5">
          <div class="flex min-w-0 flex-col">
            <DialogTitle class="truncate font-semibold">{{ name(file) }}</DialogTitle>
            <DialogDescription class="text-xs tabular-nums text-white/55">{{ summary }}</DialogDescription>
          </div>
          <div class="flex flex-none items-center gap-0.5">
            <template v-if="!video">
              <button type="button" class="viewer-btn max-[900px]:hidden" aria-label="缩小" title="缩小 (-)" :disabled="zoom.scale <= 1" @click="zoomBy(1 / 1.4)"><i class="ri-zoom-out-line" aria-hidden="true" /></button>
              <button type="button" class="h-[30px] min-w-[56px] rounded-lg bg-white/[0.08] px-2 text-[12.5px] tabular-nums text-white/85 hover:bg-white/[0.14] max-[900px]:hidden" title="在适应窗口和 100% 之间切换" @click="toggleActual()">
                {{ zoom.scale <= 1.001 ? '适应' : `${percent}%` }}
              </button>
              <button type="button" class="viewer-btn max-[900px]:hidden" aria-label="放大" title="放大 (+)" @click="zoomBy(1.4)"><i class="ri-zoom-in-line" aria-hidden="true" /></button>
              <span class="mx-1.5 h-5 w-px bg-white/15 max-[900px]:hidden" aria-hidden="true" />
            </template>
            <button type="button" class="viewer-btn" :aria-label="`复制 ${formatLabel} 链接`" :title="`复制 ${formatLabel} 链接`" @click="copyLink"><i class="ri-file-copy-line" aria-hidden="true" /></button>
            <a v-if="url" class="viewer-btn" :href="url" :download="safeDownloadName(file.file_name)" aria-label="下载" title="下载"><i class="ri-download-2-line" aria-hidden="true" /></a>
            <a v-if="url" class="viewer-btn max-[900px]:hidden" :href="url" target="_blank" rel="noopener noreferrer" aria-label="在新标签页打开原文件" title="在新标签页打开原文件"><i class="ri-external-link-line" aria-hidden="true" /></a>
            <button v-if="canDelete?.(file)" type="button" class="viewer-btn" aria-label="删除" title="删除" @click="emit('delete', file)"><i class="ri-delete-bin-6-line" aria-hidden="true" /></button>
            <button type="button" class="viewer-btn" aria-label="关闭" title="关闭 (Esc)" @click="open = false"><i class="ri-close-line" aria-hidden="true" /></button>
          </div>
        </div>

        <div
          ref="stage"
          :class="[
            'relative flex min-h-0 flex-1 items-center justify-center overflow-hidden px-16 py-2 max-[900px]:p-0',
            video ? 'cursor-default' : zoom.scale > 1.001 ? 'cursor-grab touch-none active:cursor-grabbing' : 'cursor-zoom-in touch-none',
          ]"
          @wheel="onWheel"
          @pointerdown="onPointerDown"
          @pointermove="onPointerMove"
          @pointerup="onPointerUp"
          @pointercancel="onPointerUp"
          @click="onStageClick"
          @dblclick="onStageDoubleClick"
        >
          <template v-if="video">
            <video
              v-if="!failed && url"
              :key="file.id"
              :src="url"
              controls
              autoplay
              playsinline
              class="max-h-full max-w-full rounded-lg bg-black shadow-[0_20px_60px_rgba(0,0,0,0.5)]"
              @error="failed = true"
            />
            <div v-else class="flex flex-col items-center gap-1.5 px-6 text-center text-white/70" role="alert">
              <i class="ri-film-line text-[40px] text-white/35" aria-hidden="true" />
              <b>无法播放此视频</b>
              <span class="text-[12.5px] text-white/45">可能是浏览器不支持它的编码、文件已损坏或网络连接失败。可以在新标签页打开原文件，或下载后播放。</span>
            </div>
          </template>
          <template v-else>
            <i v-if="loading && !failed && url" class="ri-loader-4-line absolute animate-spin text-[28px] text-white/50" aria-hidden="true" />
            <div v-if="failed || !url" class="flex flex-col items-center gap-1.5 px-6 text-center text-white/70" role="alert">
              <i class="ri-image-off-line text-[40px] text-white/35" aria-hidden="true" />
              <b>图片加载失败</b>
              <span class="text-[12.5px] text-white/45">存储不可达、文件已被删除，或链接不受支持</span>
            </div>
            <img
              v-if="url"
              v-show="!failed"
              ref="img"
              :key="file.id"
              :src="url"
              :alt="name(file)"
              draggable="false"
              class="pointer-events-none max-h-full max-w-full select-none object-contain shadow-[0_20px_60px_rgba(0,0,0,0.5)] [will-change:transform] max-[900px]:shadow-none"
              :style="{ transform: `translate(${zoom.x}px, ${zoom.y}px) scale(${zoom.scale})`, transition: zoom.animate ? 'transform .22s cubic-bezier(.3,.7,.4,1)' : 'none' }"
              @load="onLoad"
              @error="failed = true; loading = false"
            >
          </template>
          <template v-if="total > 1">
            <button type="button" class="viewer-nav left-3" aria-label="上一个" title="上一个 (←)" @click.stop="go(-1)"><i class="ri-arrow-left-s-line" aria-hidden="true" /></button>
            <button type="button" class="viewer-nav right-3" aria-label="下一个" title="下一个 (→)" @click.stop="go(1)"><i class="ri-arrow-right-s-line" aria-hidden="true" /></button>
          </template>
          <p v-if="!video && !failed && zoom.scale <= 1.001" class="pointer-events-none absolute bottom-3 left-1/2 -translate-x-1/2 whitespace-nowrap text-[11.5px] text-white/40 max-[900px]:hidden">
            滚轮或双指缩放 · 双击切换 100% · 拖动平移
          </p>
        </div>

        <div
          v-if="total > 1"
          ref="strip"
          class="flex flex-none gap-1.5 overflow-x-auto px-4 pb-3.5 pt-2.5 [justify-content:safe_center] [scrollbar-width:none] max-[900px]:justify-start max-[900px]:pb-[calc(12px+env(safe-area-inset-bottom))]"
        >
          <button
            v-for="(item, itemIndex) in items"
            :key="item.id"
            type="button"
            :aria-label="name(item)"
            :aria-current="itemIndex === index ? 'true' : undefined"
            :class="[
              'h-[42px] w-14 flex-none overflow-hidden rounded-[7px] outline outline-2 outline-offset-1 transition-[opacity,outline-color] hover:opacity-80',
              itemIndex === index ? 'opacity-100 outline-oi-accent' : 'opacity-45 outline-transparent',
            ]"
            @click="index = itemIndex"
          >
            <MediaThumb :src="item.thumbnail || item.url" :alt="name(item)" :video="isVideo(item)" class="h-full w-full" />
          </button>
        </div>
        <img v-for="src in neighbors" :key="src" :src="src" alt="" hidden>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<style scoped>
.viewer-btn {
  display: grid; place-items: center; width: 38px; height: 38px; border-radius: 10px; font-size: 19px; color: rgba(255, 255, 255, .85);
}
.viewer-btn:hover { background: rgba(255, 255, 255, .1); color: #fff; }
.viewer-btn:disabled { opacity: .3; pointer-events: none; }
.viewer-btn:focus-visible, .viewer-nav:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--oi-ring); }
.viewer-nav {
  position: absolute; top: 50%; transform: translateY(-50%); display: grid; place-items: center; width: 44px; height: 64px;
  border-radius: 12px; font-size: 28px; color: rgba(255, 255, 255, .75); background: rgba(255, 255, 255, .06); transition: background .15s;
}
.viewer-nav:hover { background: rgba(255, 255, 255, .14); color: #fff; }
@media (max-width: 900px) {
  .viewer-nav { display: none; }
}
</style>
