<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { TrendPeriod, TrendPoint } from '@/api/client.ts'
import { formatCount } from '@/utils/format'

const props = defineProps<{
  points: TrendPoint[]
  period: TrendPeriod
  /** 重新读取时保留上一次的图形，只降低不透明度 */
  loading?: boolean
  /** 以表格显示同一组数据 */
  table?: boolean
}>()

/* ---------- 刻度 ---------- */
const PLOT_HEIGHT = 168
const compact = new Intl.NumberFormat('zh-CN', { notation: 'compact', maximumFractionDigits: 1 })

// 步长取 1、2、5 × 10ⁿ（至少为 1），最多四格覆盖最大值
const scale = computed(() => {
  const max = Math.max(0, ...props.points.map(point => point.count))
  if (!max) return { top: 4, ticks: [0, 2, 4] }
  const raw = max / 4
  const pow = 10 ** Math.floor(Math.log10(raw))
  const step = Math.max(1, [1, 2, 5, 10].map(n => n * pow).find(n => n >= raw) ?? 10 * pow)
  const top = step * Math.ceil(max / step)
  const ticks: number[] = []
  for (let value = 0; value <= top; value += step) ticks.push(value)
  return { top, ticks }
})
const empty = computed(() => props.points.every(point => !point.count))

const px = (value: number) => (value ? Math.max(2, value / scale.value.top * PLOT_HEIGHT) : 0)
const bars = computed(() => props.points.map(point => {
  const image = px(point.image_count)
  const video = px(point.video_count)
  return { image, video, height: image + video + (image && video ? 2 : 0) }
}))

/* ---------- 时间标签 ---------- */
const CURRENT: Record<TrendPeriod, string> = { day: '今天', week: '本周', month: '本月', year: '今年' }

const parse = (value: string) => {
  const [year, month = 1, day = 1] = value.split('-').map(Number)
  return new Date(year, month - 1, day)
}
const monthDay = (date: Date) => `${date.getMonth() + 1}月${date.getDate()}日`

const shortLabel = (point: TrendPoint, index: number) => {
  if (index === props.points.length - 1) return CURRENT[props.period]
  const date = parse(point.date)
  if (props.period === 'year') return `${date.getFullYear()}年`
  if (props.period === 'month') return `${date.getFullYear()}年${date.getMonth() + 1}月`
  return monthDay(date)
}

const longLabel = (point: TrendPoint, index: number) => {
  const date = parse(point.date)
  let text: string
  if (props.period === 'year') text = `${date.getFullYear()}年`
  else if (props.period === 'month') text = `${date.getFullYear()}年${date.getMonth() + 1}月`
  else if (props.period === 'week') {
    const end = new Date(date)
    end.setDate(date.getDate() + 6)
    text = `${monthDay(date)} – ${monthDay(end)}`
  } else text = date.toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'short' })
  return index === props.points.length - 1 ? `${text}（${CURRENT[props.period]}）` : text
}

// 横轴只标首、中、尾，数量少时全部标出
const axisLabels = computed(() => {
  const count = props.points.length
  if (!count) return []
  const indexes = count <= 6 ? props.points.map((_, index) => index) : [0, Math.floor((count - 1) / 2), count - 1]
  return indexes.map(index => ({ index, text: shortLabel(props.points[index], index) }))
})
const axisLabelStyle = (index: number) => {
  const count = props.points.length
  const left = (index + 0.5) / count * 100
  if (index === 0 && count > 6) return { left: '0' }
  if (index === count - 1 && count > 6) return { right: '0' }
  return { left: `${left}%`, transform: 'translateX(-50%)' }
}

/* ---------- 悬停与键盘 ---------- */
const active = ref<number | null>(null)
const plot = ref<HTMLElement | null>(null)
watch(() => [props.points, props.period], () => { active.value = null })

const indexAt = (event: PointerEvent) => {
  const rect = plot.value?.getBoundingClientRect()
  if (!rect?.width || !props.points.length) return null
  const ratio = (event.clientX - rect.left) / rect.width
  return Math.min(props.points.length - 1, Math.max(0, Math.floor(ratio * props.points.length)))
}
const onPointerMove = (event: PointerEvent) => { active.value = indexAt(event) }
const onPointerLeave = (event: PointerEvent) => { if (event.pointerType === 'mouse') active.value = null }
const onFocus = () => { if (active.value == null && props.points.length) active.value = props.points.length - 1 }
const onKeydown = (event: KeyboardEvent) => {
  const last = props.points.length - 1
  if (last < 0) return
  const current = active.value ?? last
  const next = { ArrowLeft: current - 1, ArrowRight: current + 1, Home: 0, End: last }[event.key]
  if (next == null) return
  event.preventDefault()
  active.value = Math.min(last, Math.max(0, next))
}

const activePoint = computed(() => (active.value == null ? null : props.points[active.value] ?? null))
// 提示框放在当前柱子旁边，不遮住它
const tooltipStyle = computed(() => {
  if (active.value == null) return {}
  const count = props.points.length
  const center = (active.value + 0.5) / count * 100
  return center <= 50 ? { left: `calc(${center}% + 14px)` } : { right: `calc(${100 - center}% + 14px)` }
})
const announcement = computed(() => {
  const point = activePoint.value
  if (!point || active.value == null) return ''
  return `${longLabel(point, active.value)}：图片 ${formatCount(point.image_count)}，视频 ${formatCount(point.video_count)}`
})

const tableRows = computed(() => props.points.map((point, index) => ({ point, label: longLabel(point, index) })).reverse())
</script>

<template>
  <div v-if="table" class="max-h-[264px] overflow-auto rounded-oi border border-oi-line" :class="loading && 'opacity-60'">
    <table class="w-full border-collapse text-[13px]">
      <caption class="sr-only">上传趋势数据</caption>
      <thead class="sticky top-0 bg-oi-panel text-xs text-oi-muted">
        <tr>
          <th scope="col" class="px-3 py-2 text-left font-[550]">时间</th>
          <th scope="col" class="px-3 py-2 text-right font-[550]">图片</th>
          <th scope="col" class="px-3 py-2 text-right font-[550]">视频</th>
          <th scope="col" class="px-3 py-2 text-right font-[550]">合计</th>
        </tr>
      </thead>
      <tbody class="tabular-nums">
        <tr v-for="row in tableRows" :key="row.point.date" class="border-t border-oi-line">
          <th scope="row" class="px-3 py-1.5 text-left font-normal text-oi-text">{{ row.label }}</th>
          <td class="px-3 py-1.5 text-right text-oi-text">{{ formatCount(row.point.image_count) }}</td>
          <td class="px-3 py-1.5 text-right text-oi-text">{{ formatCount(row.point.video_count) }}</td>
          <td class="px-3 py-1.5 text-right font-[600] text-oi-ink">{{ formatCount(row.point.count) }}</td>
        </tr>
      </tbody>
    </table>
  </div>

  <div v-else class="relative pl-9 transition-opacity duration-150" :class="loading && points.length ? 'opacity-50' : ''">
    <!-- 刻度与网格线 -->
    <div class="relative" :style="{ height: `${PLOT_HEIGHT}px` }" aria-hidden="true">
      <div
        v-for="tick in scale.ticks"
        :key="tick"
        class="absolute inset-x-0 border-t border-oi-line"
        :style="{ bottom: `${tick / scale.top * PLOT_HEIGHT}px` }"
      >
        <span class="absolute -left-9 w-7 -translate-y-1/2 text-right text-[11px] tabular-nums leading-none text-oi-faint">{{ compact.format(tick) }}</span>
      </div>
    </div>

    <!-- 柱子：整列都是命中区域 -->
    <div
      ref="plot"
      class="absolute inset-y-0 left-9 right-0 flex touch-pan-y items-end rounded-[6px] focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring"
      :style="{ height: `${PLOT_HEIGHT}px` }"
      :tabindex="points.length && !empty ? 0 : -1"
      role="group"
      aria-roledescription="柱状图"
      aria-label="上传趋势，用左右方向键查看每一段的数量"
      @pointermove="onPointerMove"
      @pointerdown="onPointerMove"
      @pointerleave="onPointerLeave"
      @focus="onFocus"
      @blur="active = null"
      @keydown="onKeydown"
    >
      <div
        v-for="(bar, index) in bars"
        :key="points[index].date"
        class="flex h-full flex-1 items-end justify-center rounded-[4px] transition-colors duration-100"
        :class="active === index && 'bg-oi-hover'"
      >
        <div class="flex w-[min(24px,70%)] flex-col gap-[2px]" :class="active === index && 'brightness-110'" :style="{ height: `${bar.height}px` }">
          <span v-if="bar.video" class="block flex-none rounded-t-[4px] bg-oi-chart-video" :style="{ height: `${bar.video}px` }" />
          <span v-if="bar.image" class="block flex-none bg-oi-chart-image" :class="!bar.video && 'rounded-t-[4px]'" :style="{ height: `${bar.image}px` }" />
        </div>
      </div>

      <p v-if="!points.length && loading" class="absolute inset-0 grid place-items-center text-[12.5px] text-oi-muted">正在读取…</p>
      <p v-else-if="empty && !loading" class="absolute inset-0 grid place-items-center text-[12.5px] text-oi-muted">这段时间没有上传</p>

      <div
        v-if="activePoint && active != null"
        class="pointer-events-none absolute top-1 z-10 min-w-[128px] rounded-oi border border-oi-line bg-oi-raised px-3 py-2 shadow-oi"
        :style="tooltipStyle"
        aria-hidden="true"
      >
        <p class="whitespace-nowrap text-[11.5px] text-oi-muted">{{ longLabel(activePoint, active) }}</p>
        <p class="mt-1 flex items-center gap-2 text-[12.5px] text-oi-muted">
          <span class="h-[2px] w-2.5 rounded-full bg-oi-chart-image" /><b class="font-[650] tabular-nums text-oi-ink">{{ formatCount(activePoint.image_count) }}</b>图片
        </p>
        <p class="flex items-center gap-2 text-[12.5px] text-oi-muted">
          <span class="h-[2px] w-2.5 rounded-full bg-oi-chart-video" /><b class="font-[650] tabular-nums text-oi-ink">{{ formatCount(activePoint.video_count) }}</b>视频
        </p>
      </div>
    </div>

    <div class="relative mt-2 h-4 text-[11px] text-oi-faint" aria-hidden="true">
      <span v-for="label in axisLabels" :key="label.index" class="absolute whitespace-nowrap" :style="axisLabelStyle(label.index)">{{ label.text }}</span>
    </div>
    <p class="sr-only" aria-live="polite">{{ announcement }}</p>
  </div>
</template>
