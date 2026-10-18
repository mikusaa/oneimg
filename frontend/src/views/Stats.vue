<template>
  <div class="page-shell">
    <PageHeader title="统计" :description="description" />

    <!-- 原控制中心顶部的统计概览，统计页本身在后续步骤重做 -->
    <section class="dashboard-summary page-surface" aria-label="文件统计概览">
      <div v-for="item in summaryItems" :key="item.label" class="dashboard-summary-item">
        <span class="dashboard-summary-icon" aria-hidden="true"><i :class="item.icon"></i></span>
        <span class="min-w-0">
          <span class="dashboard-summary-label">{{ item.label }}</span>
          <span class="dashboard-summary-value" :class="{ 'animate-pulse text-slate-300 dark:text-slate-700': loading }">{{ item.value }}</span>
          <span v-if="item.detail" class="block text-xs text-slate-500 dark:text-slate-400">{{ item.detail }}</span>
        </span>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type DashboardStats } from '@/api/client.ts'
import PageHeader from '@/components/PageHeader.vue'
import { useSession } from '@/composables/useSession'
import { ROLE_ADMIN } from '@/utils/permissions.ts'

const { user } = useSession()
const description = computed(() => Number(user.value.role) === ROLE_ADMIN ? '全站的文件数量、体积和近期上传' : '你上传的文件数量、体积和近期上传')

const loading = ref(true)
const stats = ref<Partial<DashboardStats>>({})

const formatNumber = (value?: number) => Number(value || 0).toLocaleString('zh-CN')
const formatFileSize = (bytes?: number) => {
  if (!bytes || bytes < 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1)
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const summaryItems = computed(() => {
  const s = stats.value
  return [
    { label: '图片总数', value: `${formatNumber(s.total_images)} 张`, icon: 'ri-image-line', detail: '' },
    { label: '视频总数', value: `${formatNumber(s.total_videos)} 个`, icon: 'ri-video-line', detail: '' },
    { label: '图片体积', value: formatFileSize(s.image_size), icon: 'ri-image-2-line', detail: '不含缩略图' },
    { label: '视频体积', value: formatFileSize(s.video_size), icon: 'ri-film-line', detail: '原文件大小' },
    { label: '今日上传', value: formatNumber(s.today_uploads), icon: 'ri-calendar-check-line', detail: `图片 ${s.today_image_uploads ?? 0} 张 · 视频 ${s.today_video_uploads ?? 0} 个` },
    { label: '本月上传', value: formatNumber(s.month_uploads), icon: 'ri-calendar-line', detail: `图片 ${s.month_image_uploads ?? 0} 张 · 视频 ${s.month_video_uploads ?? 0} 个` },
  ]
})

onMounted(async () => {
  try {
    stats.value = await api.dashboardStats()
  } catch (error) {
    console.error('获取统计数据失败:', error)
  } finally {
    loading.value = false
  }
})
</script>
