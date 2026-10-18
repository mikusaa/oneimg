<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type DashboardStats, type TrendPeriod, type TrendPoint } from '@/api/client.ts'
import UploadTrendChart from '@/components/stats/UploadTrendChart.vue'
import { UiButton, UiSegmented } from '@/components/ui'
import { useSession } from '@/composables/useSession'
import { formatBytes, formatCount } from '@/utils/format'
import { ROLE_ADMIN } from '@/utils/permissions.ts'

const { user } = useSession()
const isAdmin = computed(() => Number(user.value.role) === ROLE_ADMIN)

/* ---------- 概览、格式和大小分布 ---------- */
const stats = ref<DashboardStats | null>(null)
const statsError = ref('')
const loadStats = async () => {
  statsError.value = ''
  try {
    stats.value = await api.dashboardStats()
  } catch (error) {
    statsError.value = error instanceof Error ? error.message : '读取统计数据失败'
  }
}

const show = (value?: number, unit = '') => (stats.value ? `${formatCount(value)}${unit}` : '—')
const tiles = computed(() => {
  const s = stats.value
  return [
    { key: 'images', label: '图片', icon: 'ri-image-line', value: show(s?.total_images), unit: '张', detail: s ? `${formatBytes(s.image_size)} · 不含缩略图` : '' },
    { key: 'videos', label: '视频', icon: 'ri-film-line', value: show(s?.total_videos), unit: '个', detail: s ? `${formatBytes(s.video_size)} · 原文件` : '' },
    { key: 'today', label: '今日上传', icon: 'ri-calendar-check-line', value: show(s?.today_uploads), unit: '', detail: s ? `图片 ${formatCount(s.today_image_uploads)} · 视频 ${formatCount(s.today_video_uploads)}` : '' },
    { key: 'month', label: '本月上传', icon: 'ri-calendar-line', value: show(s?.month_uploads), unit: '', detail: s ? `图片 ${formatCount(s.month_image_uploads)} · 视频 ${formatCount(s.month_video_uploads)}` : '' },
  ]
})

// 与后端 models.VideoSQL 一致：只有这两种 MIME 算视频
const VIDEO_TYPES = ['video/mp4', 'video/webm']
const FORMAT_NAMES: Record<string, string> = {
  'image/jpeg': 'JPEG', 'image/png': 'PNG', 'image/gif': 'GIF', 'image/webp': 'WebP', 'image/svg+xml': 'SVG',
  'video/mp4': 'MP4', 'video/webm': 'WebM',
}
const formats = computed(() => {
  const merged = new Map<string, { count: number; size: number }>()
  for (const item of stats.value?.format_stats ?? []) {
    const key = item.format.trim().toLowerCase()
    const entry = merged.get(key) ?? { count: 0, size: 0 }
    entry.count += item.count
    entry.size += item.size
    merged.set(key, entry)
  }
  const total = [...merged.values()].reduce((sum, item) => sum + item.count, 0) || 1
  return [...merged.entries()]
    .filter(([, item]) => item.count > 0)
    .sort((a, b) => b[1].count - a[1].count)
    .map(([mime, item]) => ({
      mime,
      name: FORMAT_NAMES[mime] ?? (mime || '未知格式'),
      video: VIDEO_TYPES.includes(mime),
      count: item.count,
      size: item.size,
      share: item.count / total * 100,
    }))
})

const SIZE_LABELS: Record<string, string> = {
  '< 100KB': '< 100 KB', '100KB - 500KB': '100 – 500 KB', '500KB - 1MB': '500 KB – 1 MB',
  '1MB - 5MB': '1 – 5 MB', '5MB - 10MB': '5 – 10 MB', '>= 10MB': '≥ 10 MB',
}
const sizes = computed(() => {
  const rows = stats.value?.size_distribution ?? []
  const max = Math.max(1, ...rows.map(row => row.count))
  const total = rows.reduce((sum, row) => sum + row.count, 0) || 1
  return rows.map(row => ({ label: SIZE_LABELS[row.range] ?? row.range, count: row.count, width: row.count / max * 100, share: row.count / total * 100 }))
})
const hasFiles = computed(() => !!stats.value && stats.value.total_images + stats.value.total_videos > 0)
const percent = (value: number) => (value > 0 && value < 1 ? '< 1%' : `${Math.round(value)}%`)

/* ---------- 上传趋势 ---------- */
const PERIOD_KEY = 'stats-period'
const PERIODS: { value: TrendPeriod; label: string; range: string }[] = [
  { value: 'day', label: '日', range: '最近 30 天' },
  { value: 'week', label: '周', range: '最近 12 周' },
  { value: 'month', label: '月', range: '最近 12 个月' },
  { value: 'year', label: '年', range: '最近 5 年' },
]
const readPeriod = (): TrendPeriod => {
  try {
    const saved = localStorage.getItem(PERIOD_KEY)
    return PERIODS.some(item => item.value === saved) ? saved as TrendPeriod : 'day'
  } catch { return 'day' }
}
const period = ref<TrendPeriod>(readPeriod())
const periodModel = computed({
  get: () => period.value,
  set: (value: string) => { period.value = value as TrendPeriod },
})
const periodRange = computed(() => PERIODS.find(item => item.value === period.value)?.range ?? '')
const trend = ref<TrendPoint[]>([])
const trendLoading = ref(false)
const trendError = ref('')
const showTable = ref(false)
let trendSeq = 0

const loadTrend = async () => {
  const seq = ++trendSeq
  trendLoading.value = true
  trendError.value = ''
  try {
    const points = await api.imageStats(period.value)
    if (seq === trendSeq) trend.value = points
  } catch (error) {
    if (seq === trendSeq) trendError.value = error instanceof Error ? error.message : '读取上传趋势失败'
  } finally {
    if (seq === trendSeq) trendLoading.value = false
  }
}
watch(period, value => {
  try { localStorage.setItem(PERIOD_KEY, value) } catch { /* 不保存也不影响使用 */ }
  void loadTrend()
})

onMounted(() => {
  void loadStats()
  void loadTrend()
})
</script>

<template>
  <div class="mx-auto w-full max-w-[1240px] font-oi text-sm text-oi-text">
    <header class="mb-[22px]">
      <h1 class="text-2xl font-[680] leading-tight tracking-[-0.02em] text-oi-ink">统计</h1>
      <p class="mt-1.5 text-oi-muted">{{ isAdmin ? '全站所有用户的上传数据。' : '你自己上传的文件数据。' }}</p>
    </header>

    <div v-if="statsError" class="mb-4 flex flex-wrap items-center gap-3 rounded-oi border border-oi-line bg-oi-danger-soft px-4 py-3 text-[13px] text-oi-danger" role="alert">
      <i class="ri-error-warning-line" aria-hidden="true" />
      <span class="min-w-0 flex-1">读取统计数据失败：{{ statsError }}</span>
      <UiButton size="sm" @click="loadStats">重试</UiButton>
    </div>

    <!-- 概览 -->
    <section class="mb-4 grid grid-cols-4 gap-3 max-[1280px]:grid-cols-2 max-[900px]:gap-2" aria-label="概览">
      <div v-for="tile in tiles" :key="tile.key" class="rounded-oi-lg border border-oi-line bg-oi-raised px-[18px] py-4 max-[900px]:p-3.5">
        <p class="flex items-center gap-1.5 text-[12.5px] text-oi-muted"><i :class="[tile.icon, 'text-[15px]']" aria-hidden="true" />{{ tile.label }}</p>
        <p class="mt-1.5 text-[26px] font-[680] leading-tight tracking-[-0.03em] text-oi-ink max-[900px]:text-[22px]" :class="!stats && !statsError && 'animate-pulse text-oi-faint'">
          {{ tile.value }}<small v-if="stats && tile.unit" class="ml-[3px] text-sm font-[550] tracking-normal text-oi-muted">{{ tile.unit }}</small>
        </p>
        <p class="mt-0.5 min-h-[18px] text-xs text-oi-muted">{{ tile.detail }}</p>
      </div>
    </section>

    <!-- 上传趋势 -->
    <section class="mb-4 rounded-oi-lg border border-oi-line bg-oi-raised" aria-labelledby="stats-trend-title">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-oi-line px-[18px] py-3.5">
        <div class="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
          <h2 id="stats-trend-title" class="text-sm font-[620] text-oi-ink">上传趋势</h2>
          <span class="text-xs text-oi-muted">{{ periodRange }}</span>
          <span class="flex items-center gap-3 text-xs text-oi-muted">
            <span class="inline-flex items-center gap-1.5"><span class="h-2 w-2 rounded-[3px] bg-oi-chart-image" aria-hidden="true" />图片</span>
            <span class="inline-flex items-center gap-1.5"><span class="h-2 w-2 rounded-[3px] bg-oi-chart-video" aria-hidden="true" />视频</span>
          </span>
        </div>
        <div class="flex items-center gap-2">
          <UiButton
            variant="ghost"
            size="sm"
            icon
            :aria-pressed="showTable"
            :aria-label="showTable ? '以图表查看' : '以表格查看'"
            :title="showTable ? '以图表查看' : '以表格查看'"
            @click="showTable = !showTable"
          >
            <i :class="showTable ? 'ri-bar-chart-2-line' : 'ri-table-line'" aria-hidden="true" />
          </UiButton>
          <UiSegmented v-model="periodModel" :options="PERIODS.map(({ value, label }) => ({ value, label, title: `按${label}统计` }))" aria-label="统计粒度" />
        </div>
      </div>
      <div class="p-[18px]">
        <div v-if="trendError" class="flex min-h-[200px] flex-col items-center justify-center gap-3 text-[13px] text-oi-muted" role="alert">
          <span><i class="ri-error-warning-line mr-1 text-oi-danger" aria-hidden="true" />读取上传趋势失败：{{ trendError }}</span>
          <UiButton size="sm" @click="loadTrend">重试</UiButton>
        </div>
        <UploadTrendChart v-else :points="trend" :period="period" :loading="trendLoading" :table="showTable" />
      </div>
    </section>

    <div class="grid grid-cols-[1.6fr_1fr] gap-4 max-[900px]:grid-cols-1">
      <!-- 格式分布 -->
      <section class="rounded-oi-lg border border-oi-line bg-oi-raised" aria-labelledby="stats-format-title">
        <div class="border-b border-oi-line px-[18px] py-3.5">
          <h2 id="stats-format-title" class="text-sm font-[620] text-oi-ink">格式分布</h2>
        </div>
        <div class="p-[18px]">
          <ul v-if="formats.length" class="flex flex-col gap-3.5">
            <li v-for="item in formats" :key="item.mime" class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2.5 gap-y-1">
              <span class="flex min-w-0 items-center gap-2 text-[13px] font-[520] text-oi-ink">
                {{ item.name }}
                <span v-if="item.mime && item.name !== item.mime" class="truncate font-oi-mono text-[11.5px] font-normal text-oi-faint">{{ item.mime }}</span>
              </span>
              <span class="text-[12.5px] tabular-nums text-oi-muted">{{ formatCount(item.count) }} 个 · {{ percent(item.share) }} · {{ formatBytes(item.size) }}</span>
              <div class="col-span-2 h-1.5 overflow-hidden rounded-full bg-oi-line" aria-hidden="true">
                <span class="block h-full rounded-full" :class="item.video ? 'bg-oi-chart-video' : 'bg-oi-chart-image'" :style="{ width: `${item.share}%` }" />
              </div>
            </li>
          </ul>
          <p v-else-if="stats" class="py-6 text-center text-[12.5px] text-oi-muted">还没有上传过文件</p>
          <p v-else class="py-6 text-center text-[12.5px] text-oi-muted">{{ statsError ? '暂无数据' : '正在读取…' }}</p>
        </div>
      </section>

      <!-- 大小分布 -->
      <section class="rounded-oi-lg border border-oi-line bg-oi-raised" aria-labelledby="stats-size-title">
        <div class="flex items-baseline justify-between gap-3 border-b border-oi-line px-[18px] py-3.5">
          <h2 id="stats-size-title" class="text-sm font-[620] text-oi-ink">大小分布</h2>
          <span class="text-xs text-oi-muted">图片与视频合计</span>
        </div>
        <div class="p-[18px]">
          <ul v-if="hasFiles" class="flex flex-col gap-3.5">
            <li v-for="row in sizes" :key="row.label" class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2.5 gap-y-1">
              <span class="text-[13px] font-[520] text-oi-ink">{{ row.label }}</span>
              <span class="text-[12.5px] tabular-nums text-oi-muted">{{ formatCount(row.count) }} 个{{ row.count ? ` · ${percent(row.share)}` : '' }}</span>
              <div class="col-span-2 h-1.5 overflow-hidden rounded-full bg-oi-line" aria-hidden="true">
                <span class="block h-full rounded-full bg-oi-accent" :style="{ width: `${row.width}%` }" />
              </div>
            </li>
          </ul>
          <p v-else-if="stats" class="py-6 text-center text-[12.5px] text-oi-muted">还没有上传过文件</p>
          <p v-else class="py-6 text-center text-[12.5px] text-oi-muted">{{ statsError ? '暂无数据' : '正在读取…' }}</p>
        </div>
      </section>
    </div>
  </div>
</template>
