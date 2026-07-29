<template>
    <div class="page-shell text-gray-800 dark:text-gray-200">
        <section class="page-header">
            <div>
                <h1 class="page-title">系统统计</h1>
            </div>
            <div class="flex flex-wrap items-center gap-3">
                <div class="stat-tile min-w-[160px] p-4">
                    <p class="text-xs font-medium text-slate-500 dark:text-slate-400">总图片</p>
                    <p class="mt-2 text-base font-semibold text-slate-900 dark:text-white">{{ formatNumber(stats.total_images) }}</p>
                </div>
                <div class="stat-tile min-w-[160px] p-4">
                    <p class="text-xs font-medium text-slate-500 dark:text-slate-400">本月上传</p>
                    <p class="mt-2 text-base font-semibold text-slate-900 dark:text-white">{{ formatNumber(stats.month_uploads) }}</p>
                </div>
            </div>
        </section>

        <div class="section-card">
            
            <!-- 加载状态 -->
            <div v-if="loading" class="loading-container flex flex-col items-center justify-center py-20">
                <div class="spinner w-10 h-10 border-4 border-gray-200 dark:border-gray-700 border-t-primary dark:border-t-primary rounded-full animate-spin mb-4"></div>
                <p class="text-gray-600 dark:text-gray-400">加载统计数据中...</p>
            </div>
            
            <!-- 统计卡片 -->
            <div v-else class="stats-grid grid grid-cols-1 gap-3 lg:grid-cols-3">
                <!-- 总图片数 -->
                <div class="stat-card rounded-lg border border-slate-200/80 bg-slate-50 p-5 dark:border-white/10 dark:bg-slate-950 flex items-center gap-4">
                    <div class="stat-icon flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-blue-100 text-2xl text-blue-600 dark:bg-blue-500/15 dark:text-blue-300">
                        <i class="ri-image-circle-line"></i>
                    </div>
                    <div class="stat-content">
                        <h3 class="stat-number text-xl font-semibold">{{ formatNumber(stats.total_images) }}</h3>
                        <p class="stat-label mt-1 text-sm text-gray-600 dark:text-gray-400">总图片数</p>
                    </div>
                </div>
                
                <!-- 总存储空间 -->
                <div class="stat-card rounded-lg border border-slate-200/80 bg-slate-50 p-5 dark:border-white/10 dark:bg-slate-950 flex items-center gap-4">
                    <div class="stat-icon flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-emerald-100 text-2xl text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-300">
                        <i class="ri-folder-3-line"></i>
                    </div>
                    <div class="stat-content">
                        <h3 class="stat-number text-xl font-semibold">{{ formatFileSize(stats.total_size) }}</h3>
                        <p class="stat-label mt-1 text-sm text-gray-600 dark:text-gray-400">总存储空间</p>
                    </div>
                </div>
                
                <!-- 本月上传 -->
                <div class="stat-card rounded-lg border border-slate-200/80 bg-slate-50 p-5 dark:border-white/10 dark:bg-slate-950 flex items-center gap-4">
                    <div class="stat-icon flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-amber-100 text-2xl text-amber-600 dark:bg-amber-500/15 dark:text-amber-300">
                        <i class="ri-calendar-line"></i>
                    </div>
                    <div class="stat-content">
                        <h3 class="stat-number text-xl font-semibold">{{ formatNumber(stats.month_uploads) }}</h3>
                        <p class="stat-label mt-1 text-sm text-gray-600 dark:text-gray-400">本月上传</p>
                    </div>
                </div>
            </div>
        </div>
        
    </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import message from '@/utils/message.js'

// 响应式数据
const loading = ref(false)
const stats = ref({
    total_images: 0,
    total_size: 0,
    today_uploads: 0,
    month_uploads: 0,
    average_size: 0,
    max_size: 0,
    upload_trend: []
})

// 加载统计数据
const loadStats = async () => {
    loading.value = true
    
    try {
        const response = await fetch('/api/stats/dashboard', {
            headers: {
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            }
        })
        
        if (!response.ok) {
            // 未授权处理
            if (response.status === 401) {
                localStorage.removeItem('authToken')
                window.location.href = '/login'
                message.error('登录已过期，请重新登录')
                return
            }
            throw new Error('加载统计数据失败')
        }
        
        const result = await response.json()
        stats.value = { ...stats.value, ...(result.data || {}) }
    } catch (error) {
        console.error('加载统计数据错误:', error)
        message.error('加载统计数据失败: ' + error.message)
    } finally {
        loading.value = false
    }
}

// 工具函数
/** 格式化数字为千分位 */
const formatNumber = (num) => {
    return num ? num.toLocaleString('zh-CN') : '0'
}

/** 格式化文件大小 */
const formatFileSize = (bytes) => {
    if (!bytes) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 生命周期
onMounted(() => {
    loadStats()
})
</script>
