<template>
  <div class="page-shell">
    <PageHeader title="控制中心" description="上传图片并查看近期处理结果" />

    <section class="dashboard-summary page-surface" aria-label="图片统计概览">
      <div v-for="item in dashboardSummaryItems" :key="item.label" class="dashboard-summary-item">
        <span class="dashboard-summary-icon" aria-hidden="true"><i :class="item.icon"></i></span>
        <span class="min-w-0">
          <span class="dashboard-summary-label">{{ item.label }}</span>
          <span class="dashboard-summary-value" :class="{ 'animate-pulse text-slate-300 dark:text-slate-700': statsLoading }">{{ item.value }}</span>
        </span>
      </div>
    </section>

    <div class="space-y-3 lg:space-y-3.5">
      <section class="space-y-3">
        <div class="content-panel home-panel-compact">
          <div class="mb-3 flex flex-col gap-2 border-b border-slate-200/70 pb-3 dark:border-white/10 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <h2 class="section-title mt-1 flex items-center gap-2 text-base font-semibold sm:text-lg">
                <i class="ri-upload-cloud-2-line text-primary"></i>
                图片上传
              </h2>
            </div>
            <div class="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
              <span class="rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 dark:border-white/10 dark:bg-slate-950">{{ presetBuckets.find(bucket => bucket.id == selectedBucket)?.name || '未选择存储' }}</span>
              <span class="rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 dark:border-white/10 dark:bg-slate-950">{{ selectedTags.length }} 个标签</span>
            </div>
          </div>

          <div 
          class="imageflow-dropzone upload-area relative cursor-pointer overflow-hidden"
          :class="{ 
            'scale-[1.005] border-primary bg-primary/5 shadow-[0_0_0_4px_rgba(37,99,235,0.1)] dark:bg-primary/10': isDragOver,
            'border-slate-200 bg-slate-50 dark:border-white/10 dark:bg-slate-900/40': !isDragOver && !isUploading,
            'border-primary/50 bg-primary/10 dark:bg-primary/10': isUploading
          }"
          role="button"
          tabindex="0"
          :aria-busy="isUploading"
          aria-label="选择或拖入图片上传"
          @drop="handleDrop"
          @dragover.prevent="handleDragOver"
          @dragenter.prevent="handleDragEnter"
          @dragleave="handleDragLeave"
          @click="triggerFileInput"
          @keydown.enter.prevent="triggerFileInput"
          @keydown.space.prevent="triggerFileInput"
        >
          <div v-if="!isUploading" class="upload-content py-6 text-center sm:py-7">
            <div class="upload-icon mb-2.5 text-4xl text-slate-900 transition-transform duration-200 dark:text-white sm:text-[42px]" :class="{ 'scale-110 text-primary': isDragOver }">
              <i class="ri-upload-cloud-line"></i>
            </div>
            <h3 class="mb-1.5 text-base font-semibold text-slate-900 dark:text-white">拖拽图片到此处，或点击选择图片</h3>
            <p class="mx-auto mb-3 max-w-md text-sm leading-5 text-slate-500 dark:text-slate-400">图片会先加入待上传列表，确认后统一上传。</p>
            <div class="flex flex-col items-stretch justify-center gap-2 sm:flex-row sm:flex-wrap sm:items-center">
            <button type="button" class="primary-button w-full px-4 py-2 sm:w-auto">
              <i class="ri-file-image-line"></i>
              选择图片
            </button>
            <button 
            @click.stop="uploadbyurlmodal"
            type="button"
            :disabled="isUploading"
            class="soft-button w-full border-slate-200 px-4 py-2 sm:w-auto">
              <i class="ri-links-line"></i>
              从URL上传
            </button>
            </div>
            <p class="paste-tip mt-2.5 text-center text-xs text-slate-500 dark:text-slate-400">
              支持 Ctrl+V 粘贴和直接拖入，最多 {{ MAX_UPLOAD_FILES }} 张
            </p>
          </div>

          <!-- 上传进度状态 -->
          <div v-else class="upload-progress px-3 py-8 text-center sm:px-4 sm:py-10">
            <div class="spinner w-10 h-10 border-4 border-primary/30 border-t-primary rounded-full animate-spin mx-auto mb-3"></div>
            <p class="text-secondary text-sm mb-3">
              {{ uploadPhase === 'processing' ? `正在处理 ${uploadingCount} 个文件` : `正在上传 ${uploadingCount} 个文件（${Math.round(uploadProgress)}%）` }}
            </p>
            <div class="progress-bar w-full max-w-md mx-auto h-2 bg-light-200 dark:bg-dark-100 rounded-full overflow-hidden">
              <div 
                class="progress-fill h-full bg-primary transition-[width] duration-300 ease-out"
                :style="{ width: uploadProgress + '%' }"
              ></div>
            </div>
          </div>
        </div>

        <section v-if="uploadQueue.length > 0" class="mt-3 border-t border-slate-200/80 pt-3 dark:border-white/10" aria-labelledby="upload-queue-title">
          <div class="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
              <h3 id="upload-queue-title" class="text-sm font-semibold text-slate-900 dark:text-white">待上传 {{ uploadQueue.length }}/{{ MAX_UPLOAD_FILES }}</h3>
              <span class="text-xs text-slate-500 dark:text-slate-400">共 {{ formatFileSize(uploadQueueTotalSize) }}</span>
            </div>
            <div class="flex items-center gap-2">
              <button type="button" class="soft-button min-h-9 px-3 py-2" :disabled="isUploading" @click="clearUploadQueue">
                <i class="ri-delete-bin-line"></i>
                清空
              </button>
              <button type="button" class="primary-button min-h-9 px-3 py-2" :disabled="isUploading || uploadQueue.length === 0" @click="uploadQueuedFiles">
                <i :class="isUploading ? 'ri-loader-4-line animate-spin' : 'ri-upload-cloud-2-line'"></i>
                {{ isUploading ? '上传中' : '上传全部' }}
              </button>
            </div>
          </div>

          <div class="upload-queue-grid">
            <article
              v-for="item in uploadQueue"
              :key="item.id"
              class="upload-queue-item"
              :class="{ 'upload-queue-item-error': item.status === 'failed' }"
            >
              <div class="upload-queue-preview">
                <img :src="item.previewUrl" :alt="item.file.name" class="h-full w-full object-cover">
                <button
                  type="button"
                  class="upload-queue-remove"
                  :disabled="isUploading"
                  :aria-label="`移除 ${item.file.name}`"
                  title="移除"
                  @click="removeQueuedFile(item.id)"
                >
                  <i class="ri-close-line"></i>
                </button>
              </div>
              <div class="min-w-0 px-2.5 py-2">
                <p class="truncate text-xs font-medium text-slate-800 dark:text-slate-100" :title="item.file.name">{{ item.file.name }}</p>
                <p class="mt-1 text-[11px] text-slate-500 dark:text-slate-400">{{ formatFileSize(item.file.size) }}</p>
                <p v-if="item.status === 'failed'" class="mt-1 line-clamp-2 text-[11px] leading-4 text-red-600 dark:text-red-300" :title="item.error">
                  {{ item.error }}
                </p>
              </div>
            </article>
          </div>
        </section>

        <input 
          ref="fileInput"
          type="file"
          multiple
          accept="image/*"
          @change="handleFileSelect"
          class="hidden"
        </div>

        <div class="content-panel home-panel-compact space-y-2.5">
          <div class="flex flex-col gap-2 border-b border-slate-200/70 pb-2.5 dark:border-white/10 md:flex-row md:items-end md:justify-between">
            <div>
              <h2 class="section-title mt-1 text-base font-semibold text-slate-900 dark:text-white sm:text-lg">上传设置</h2>
            </div>
          </div>

          <div class="grid gap-2.5 xl:grid-cols-[minmax(0,0.82fr)_minmax(0,1.18fr)]">
            <div class="control-group control-group-compact">
            <p class="control-group-title">选择存储桶</p>
            <select 
              class="input-modern mt-3"
              v-model="selectedBucket"
              :disabled="isUploading"
              @change="handleBucketChange"
            >
              <option 
                v-for="bucket in presetBuckets" 
                :key="bucket.id"
                :value="bucket.id"
              >{{ bucket.name }}  ({{ bucket.type }})</option>
            </select>
          </div>

            <div class="control-group control-group-compact">
            <p class="control-group-title">上传标签</p>
            <div class="mt-2.5">
              <TagSelector
                :model-value="selectedTags"
                :options="uploadTagOptions"
                :disabled="isUploading"
                :create-option="createUploadTag"
                multiple
                confirm
                allow-create
                empty-label="选择标签"
                @update:model-value="selectedTags = $event"
              />
            </div>
            </div>
          </div>
        </div>

        <div class="content-panel home-panel-compact">
          <div class="mb-3 flex flex-col gap-2 border-b border-slate-200/70 pb-3 dark:border-white/10 md:flex-row md:items-center md:justify-between">
            <div>
              <h2 class="section-title mt-1 flex items-center gap-2 text-base font-semibold sm:text-lg">
                <i class="ri-gallery-line text-primary"></i>
                最近上传
              </h2>
            </div>
            <span class="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-sm text-slate-600 dark:border-white/10 dark:bg-slate-950 dark:text-slate-300">{{ recentImages.length }} 张</span>
          </div>

      <TransitionGroup v-if="recentImages.length > 0" name="result-list" tag="div" class="result-stream">
        <div
          v-for="image in recentImages" 
          :key="image.id"
          class="result-card result-card-compact result-card-mobile-safe"
        >
          <div class="result-card-layout">
            <div class="result-card-media result-card-media-large">
            <div class="loading absolute inset-0 z-0 flex items-center justify-center bg-gray-100 text-slate-300 dark:bg-gray-800">
              <svg class="w-8 h-8 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="transform: scaleX(-1) scaleY(-1);">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </div>
            <img 
              :src="getFullUrl(image.thumbnail || image.url)"
              :alt="image.filename || '图片预览'" 
              class="recent-image h-full w-full object-cover opacity-0"
              loading="lazy"
              @load="handleImageLoad"
              @error="(e) => handleImageError(e, image)"
              @click.stop="previewImage(image)"
            />
            </div>

            <div class="min-w-0 flex-1 space-y-2">
              <div class="flex flex-col gap-2 sm:gap-2.5 lg:flex-row lg:items-start lg:justify-between">
                <div class="min-w-0">
                  <p class="truncate text-sm font-medium text-slate-900 dark:text-white">{{ image.filename }}</p>
                  <div class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
                    <span class="result-meta-pill">{{ formatFileSize(image.file_size) }}</span>
                    <span class="result-meta-pill">{{ image.width }}×{{ image.height }}</span>
                    <span
                      v-if="getStorageSyncSummary(image).total > 0"
                      class="inline-flex items-center gap-1 rounded-full border px-2 py-0.5"
                      :class="getStorageSyncSummary(image).badgeClass"
                    >
                      <i :class="getStorageSyncSummary(image).icon"></i>
                      {{ getStorageSyncSummary(image).label }}
                    </span>
                  </div>
                </div>
                <div class="flex items-center justify-end gap-1.5 sm:self-end lg:justify-end">
                  <button 
                    @click.stop="downloadImage(image)"
                    class="result-card-action"
                    title="下载图片"
                    aria-label="下载图片"
                  >
                    <i class="ri-download-line text-sm"></i>
                  </button>
                  <button 
                    @click.stop="deleteImage(image.id)"
                    class="result-card-action border-red-200 bg-red-50 text-red-500 hover:bg-red-100 dark:border-red-500/20 dark:bg-red-500/10 dark:text-red-300"
                    title="删除图片"
                    aria-label="删除图片"
                  >
                    <i class="ri-delete-bin-line text-sm"></i>
                  </button>
                </div>
              </div>

              <div class="result-links-grid result-links-grid-mobile">
            <div class="link-field cursor-pointer"
              @click.stop="copyImageLink(image, 'url')"
              @keydown.enter.stop="copyImageLink(image, 'url')"
              role="button"
              tabindex="0"
              title="点击复制URL"
            >
              <i class="ri-link text-sm text-slate-400"></i>
              <span class="w-8 shrink-0 font-medium text-slate-900 dark:text-white sm:w-10">URL</span>
              <span class="truncate">{{ getFullUrl(image.url) }}</span>
            </div>

            <div class="link-field cursor-pointer"
              @click.stop="copyImageLink(image, 'html')"
              @keydown.enter.stop="copyImageLink(image, 'html')"
              role="button"
              tabindex="0"
              title="点击复制HTML"
            >
              <i class="ri-code-line text-sm text-slate-400"></i>
              <span class="w-8 shrink-0 font-medium text-slate-900 dark:text-white sm:w-10">HTML</span>
              <span class="truncate">{{ getHtmlCode(image) }}</span>
            </div>

            <div class="link-field cursor-pointer"
              @click.stop="copyImageLink(image, 'markdown')"
              @keydown.enter.stop="copyImageLink(image, 'markdown')"
              role="button"
              tabindex="0"
              title="点击复制Markdown"
            >
              <i class="ri-markdown-line text-sm text-slate-400"></i>
              <span class="w-8 shrink-0 font-medium text-slate-900 dark:text-white sm:w-10">MD</span>
              <span class="truncate">{{ getMarkdownCode(image) }}</span>
            </div>
              </div>
            </div>
          </div>
        </div>
      </TransitionGroup>

      <!-- 无图片状态 -->
      <div v-else class="py-8 text-center">
        <div class="mb-2.5 text-5xl text-light-300 dark:text-dark-100">
          <i class="ri-image-line"></i>
        </div>
        <p class="mb-3 text-base text-secondary">暂无上传的图片</p>
      </div>
      </div>
      </section>
    </div>

    <AppDialog v-model="urlUploadOpen" title="从 URL 上传图片" width-class="max-w-md">
      <form id="url-upload-form" class="space-y-4" @submit.prevent="submitUrlUpload">
        <div>
          <label class="field-label" for="url-upload-input">图片链接</label>
          <input
            id="url-upload-input"
            v-model.trim="urlUploadUrl"
            type="url"
            class="input-modern"
            placeholder="https://example.com/image.jpg"
            required
            autofocus
          />
        </div>
        <div>
          <label class="field-label">标签</label>
          <TagSelector v-model="urlUploadTag" :options="urlTagOptions" empty-label="不添加标签" />
        </div>
        <div>
          <label class="field-label" for="url-upload-bucket">存储</label>
          <select id="url-upload-bucket" v-model="urlUploadBucket" class="input-modern" required>
            <option v-for="bucket in presetBuckets" :key="bucket.id" :value="bucket.id">
              {{ bucket.name }} ({{ bucket.type }})
            </option>
          </select>
        </div>
      </form>

      <template #footer>
        <button type="button" class="soft-button" :disabled="urlUploadLoading" @click="urlUploadOpen = false">取消</button>
        <button type="submit" form="url-upload-form" class="primary-button" :disabled="urlUploadLoading || !urlUploadUrl">
          <i v-if="urlUploadLoading" class="ri-loader-4-line animate-spin"></i>
          上传
        </button>
      </template>
    </AppDialog>
  </div>
</template>

<script setup>
import errorImg from '@/assets/images/error.webp';
import { computed, ref, onMounted, nextTick, onBeforeUnmount } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import AppDialog from '@/components/AppDialog.vue'
import PageHeader from '@/components/PageHeader.vue'
import TagSelector from '@/components/TagSelector.vue'
import Loading from '@/utils/loading.js'
import Message from '@/utils/message.js'
import PopupModal from '@/utils/popupModal.js'
import { getStorageSyncSummary } from '@/utils/storageStatus.js'

// ====================== 常量定义 ======================
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';
const ALLOWED_FILE_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp', 'image/svg+xml'];
const DEFAULT_MAX_FILE_SIZE = 10 * 1024 * 1024;
const MAX_UPLOAD_FILES = 10;

// ====================== 响应式数据 ======================
// 上传相关
const isDragOver = ref(false);
const isUploading = ref(false);
const uploadingCount = ref(0);
const uploadProgress = ref(0);
const uploadPhase = ref('idle');
const recentImages = ref([]);
const fileInput = ref(null);
const uploadQueue = ref([]);
const allowedFileTypes = ref([...ALLOWED_FILE_TYPES]);
const maxFileSize = ref(DEFAULT_MAX_FILE_SIZE);
const statsLoading = ref(true);
const dashboardStats = ref({ total_images: 0, total_size: 0, today_uploads: 0, month_uploads: 0 });

// 标签相关
const presetTags = ref([]);
const selectedTags = ref([]);
const urlUploadOpen = ref(false);
const urlUploadUrl = ref('');
const urlUploadTag = ref(0);
const urlUploadBucket = ref('1');
const urlUploadLoading = ref(false);

// 存储相关
const presetBuckets = ref([
  { id: "1", name: '默认存储', type: "default" },
]);
const selectedBucket = ref("1");

// 预览相关
const activeCopyMenu = ref(null);
let previewCopyMenu = false;
let currentPreviewImage = null;
let previewModalInstance = null;
let queueSequence = 0;

const uploadTagOptions = computed(() => presetTags.value.map(tag => ({ value: tag.name, label: tag.name })));
const urlTagOptions = computed(() => [
  { value: 0, label: '不添加标签' },
  ...presetTags.value.map(tag => ({ value: tag.id, label: tag.name }))
]);
const dashboardSummaryItems = computed(() => [
  { label: '图片总数', value: formatNumber(dashboardStats.value.total_images), icon: 'ri-image-line' },
  { label: '已用存储', value: formatFileSize(dashboardStats.value.total_size), icon: 'ri-hard-drive-2-line' },
  { label: '今日上传', value: formatNumber(dashboardStats.value.today_uploads), icon: 'ri-calendar-check-line' },
  { label: '本月上传', value: formatNumber(dashboardStats.value.month_uploads), icon: 'ri-calendar-line' },
]);
const uploadQueueTotalSize = computed(() => uploadQueue.value.reduce((total, item) => total + item.file.size, 0));

// ====================== 工具函数 ======================
function isAdmin() {
  const userInfo = JSON.parse(localStorage.getItem('userInfo') || '{}');
  return Number(userInfo?.role) === 1;
}

/**
 * 获取完整的图片URL
 */
const getFullUrl = (path) => {
  if (!path) return '';
  if (typeof window === 'undefined') return path;
  
  // 处理绝对路径和相对路径
  if (path.startsWith('http')) return path;
  return `${window.location.origin}${path}`;
};

/**
 * 格式化文件大小
 */
const formatFileSize = (bytes) => {
  if (!bytes || bytes < 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const formatNumber = value => Number(value || 0).toLocaleString('zh-CN');

/**
 * 格式化日期
 */
const formatDate = (dateString) => {
  if (!dateString) return '';
  try {
    const date = new Date(dateString);
    return date.toLocaleString('zh-CN');
  } catch (error) {
    console.error('日期格式化失败:', error);
    return dateString;
  }
};

/**
 * 获取复制类型文本
 */
const getTypeText = (type) => {
  const typeMap = {
    url: 'URL',
    html: 'HTML',
    markdown: 'Markdown'
  };
  return typeMap[type] || '';
};

const getImageAltText = (image) => image?.original_filename || image?.filename || '图片';
const escapeHtml = value => String(value ?? '').replace(/[&<>'"]/g, character => ({
  '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
})[character]);

/**
 * 生成HTML代码
 */
const getHtmlCode = (image) => {
  const url = getFullUrl(image.url);
  const alt = getImageAltText(image);
  return `<img src="${url}" alt="${alt}"/>`;
};

/**
 * 生成Markdown代码
 */
const getMarkdownCode = (image) => {
  const url = getFullUrl(image.url);
  const filename = getImageAltText(image);
  return `![${filename}](${url})`;
};

// ====================== API 请求函数 ======================
/**
 * 获取上传配置
 */
const getUploadConfig = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/uploadConfig`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    
    const result = await response.json();
    if (response.ok && result.code === 200) {
      presetTags.value = result.data?.tags || [];
      presetBuckets.value = result.data?.buckets || [];
      const bucketId = localStorage.getItem('currentBucket');
      if (bucketId != null){
        const num = parseInt(bucketId);
        selectedBucket.value = Number.isNaN(num) ? '1' : bucketId;
      } else {
        selectedBucket.value = result.data?.default_bucket || '1';
      }
      const configuredTypes = String(result.data?.allowed_types || '')
        .split(',')
        .map(type => type.trim())
        .filter(Boolean);
      allowedFileTypes.value = configuredTypes.length > 0 ? configuredTypes : [...ALLOWED_FILE_TYPES];
      const configuredMaxSize = Number(result.data?.max_file_size);
      maxFileSize.value = Number.isFinite(configuredMaxSize) && configuredMaxSize > 0
        ? configuredMaxSize
        : DEFAULT_MAX_FILE_SIZE;
    } else {
      throw new Error(result.message || '获取上传配置失败');
    }
  } catch (error) {
    console.error('获取上传配置失败:', error);
    Message.error(error.message || '获取上传配置失败');
  }
};

/**
 * 加载最近上传的图片
 */
const loadRecentImages = async () => {
  try {
    const params = new URLSearchParams({ limit: '12' });
    if (isAdmin()) {
      params.set('role', 'admin');
    }
    const response = await fetch(`${API_BASE_URL}/api/images?${params}`, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    
    if (response.ok) {
      const result = await response.json();
      recentImages.value = Array.isArray(result.data?.images) ? result.data.images : [];
    }
  } catch (error) {
    console.error('加载图片失败:', error);
    recentImages.value = [];
    Message.error(`加载图片失败: ${error.message}`, {
      duration: 3000,
      position: 'top-right',
      showClose: true
    });
  }
};

const loadDashboardStats = async () => {
  statsLoading.value = true;
  try {
    const response = await fetch(`${API_BASE_URL}/api/stats/dashboard`, {
      headers: { 'Authorization': `Bearer ${localStorage.getItem('authToken')}` }
    });
    const result = await response.json();
    if (!response.ok || result.code !== 200) throw new Error(result.message || '获取统计数据失败');
    dashboardStats.value = { ...dashboardStats.value, ...(result.data || {}) };
  } catch (error) {
    console.error('获取统计数据失败:', error);
  } finally {
    statsLoading.value = false;
  }
};

/**
 * 删除单张图片
 */
const deleteAsync = async (imageId) => { 
  let loadingInstance;
  try {
    loadingInstance = Loading.show({
      text: '删除中...',
      color: '#ff4d4f',
      mask: true
    });
    
    const response = await fetch(`${API_BASE_URL}/api/images/${imageId}`, {
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`,
        'Content-Type': 'application/json'
      }
    });
    
    if (response.ok) {
      Message.success('图片删除成功', {
        duration: 1500,
        position: 'top-right'
      });
      
      // 如果删除的是当前预览的图片，关闭预览弹窗
      if (currentPreviewImage?.id === imageId && previewModalInstance) {
        previewModalInstance.close();
        currentPreviewImage = null;
        previewModalInstance = null;
      }
      
      previewCopyMenu = false;
      activeCopyMenu.value = null;
      await loadRecentImages();
    } else {
      const result = await response.json();
      throw new Error(result.message || '删除失败');
    }
  } catch (error) {
    console.error('删除图片错误:', error);
    Message.error(`删除失败: ${error.message}`, {
      duration: 3000,
      position: 'top-right',
      showClose: true
    });
  } finally {
    if (loadingInstance) await loadingInstance.hide();
  }
};

/**
 * 添加标签到服务器
 */
const addTagToServer = async (tag) => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/tags`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: JSON.stringify({ name: tag })
    });
    
    const result = await response.json();
    if (response.ok && result.code === 200) {
      return result.data;
    } else {
      throw new Error(result.message || '添加标签失败');
    }
  } catch (error) {
    console.error('添加标签失败:', error);
    throw error;
  }
};

// ====================== 事件处理函数 ======================
/**
 * 拖拽相关处理
 */
const handleDragOver = () => {
  isDragOver.value = true;
};

const handleDragEnter = () => {
  isDragOver.value = true;
};

const handleDragLeave = (e) => {
  if (!e.currentTarget.contains(e.relatedTarget)) {
    isDragOver.value = false;
  }
};

const handleDrop = (e) => {
  e.preventDefault();
  isDragOver.value = false;
  addFilesToQueue(Array.from(e.dataTransfer.files));
};

/**
 * 文件选择处理
 */
const triggerFileInput = () => {
  if (!isUploading.value && fileInput.value) {
    fileInput.value.click();
  }
};

const handleFileSelect = (e) => {
  const files = Array.from(e.target.files);
  if (files.length > 0) {
    addFilesToQueue(files);
  }
  e.target.value = ''; // 清空文件选择
};

/**
 * 剪贴板粘贴处理
 */
const handlePaste = async (e) => {
  const items = e.clipboardData?.items;
  if (!items) return;
  
  const imageFiles = [];
  
  for (let item of items) {
    if (item.type.startsWith('image/')) {
      const file = item.getAsFile();
      if (file) {
        imageFiles.push(file);
      }
    }
  }
  
  if (imageFiles.length > 0) {
    e.preventDefault();
    addFilesToQueue(imageFiles);
  }
};

/**
 * 验证文件有效性
 */
const validateFiles = (files) => {
  const validFiles = [];
  
  files.forEach(file => {
    if (!file.type.startsWith('image/') || !allowedFileTypes.value.includes(file.type)) {
      Message.warning(`${file.name} 不是允许上传的图片格式`, {
        duration: 2000,
        position: 'top-right'
      });
      return;
    }
    if (file.size > maxFileSize.value) {
      Message.warning(`${file.name} 超过 ${formatFileSize(maxFileSize.value)} 大小限制`, {
        duration: 2500,
        position: 'top-right'
      });
      return;
    }
    
    validFiles.push(file);
  });
  
  return validFiles;
};

const getQueueFingerprint = file => [file.name, file.size, file.lastModified, file.type].join(':');

const addFilesToQueue = files => {
  if (isUploading.value || files.length === 0) return;

  const validFiles = validateFiles(files);
  const existingFingerprints = new Set(uploadQueue.value.map(item => item.fingerprint));
  let duplicateCount = 0;
  let overflowCount = 0;
  let addedCount = 0;

  validFiles.forEach(file => {
    const fingerprint = getQueueFingerprint(file);
    if (existingFingerprints.has(fingerprint)) {
      duplicateCount += 1;
      return;
    }
    if (uploadQueue.value.length >= MAX_UPLOAD_FILES) {
      overflowCount += 1;
      return;
    }

    existingFingerprints.add(fingerprint);
    queueSequence += 1;
    uploadQueue.value.push({
      id: `queued-${queueSequence}`,
      fingerprint,
      file,
      previewUrl: URL.createObjectURL(file),
      status: 'pending',
      error: '',
    });
    addedCount += 1;
  });

  if (addedCount > 0) Message.success(`已加入 ${addedCount} 张图片`);
  if (duplicateCount > 0) Message.warning(`已跳过 ${duplicateCount} 个队列内重复文件`);
  if (overflowCount > 0) Message.warning(`队列最多 ${MAX_UPLOAD_FILES} 张，已跳过 ${overflowCount} 张`);
};

const releaseQueueItem = item => {
  if (item?.previewUrl) URL.revokeObjectURL(item.previewUrl);
};

const removeQueuedFile = id => {
  if (isUploading.value) return;
  const item = uploadQueue.value.find(candidate => candidate.id === id);
  if (item) releaseQueueItem(item);
  uploadQueue.value = uploadQueue.value.filter(candidate => candidate.id !== id);
};

const clearUploadQueue = () => {
  if (isUploading.value) return;
  uploadQueue.value.forEach(releaseQueueItem);
  uploadQueue.value = [];
};

const showUploadResultMessage = (files = []) => {
  const duplicateCount = files.filter(file => file?.duplicate).length;
  const uploadedCount = files.length - duplicateCount;

  if (duplicateCount > 0 && uploadedCount > 0) {
    Message.warning(`上传完成，已跳过 ${duplicateCount} 张重复图片`, {
      duration: 3000,
      position: 'top-right'
    });
    return;
  }

  if (duplicateCount > 0) {
    Message.warning(`图片已存在，已跳过重复上传`, {
      duration: 3000,
      position: 'top-right'
    });
    return;
  }

  Message.success(`上传成功`, {
    duration: 2000,
    position: 'top-right'
  });
};

/**
 * 文件上传
 */
const sendQueuedUpload = formData => new Promise((resolve, reject) => {
  const request = new XMLHttpRequest();
  request.open('POST', `${API_BASE_URL}/api/upload/images`);
  request.setRequestHeader('Authorization', `Bearer ${localStorage.getItem('authToken')}`);
  request.upload.onprogress = event => {
    if (!event.lengthComputable) return;
    uploadProgress.value = Math.min(90, (event.loaded / event.total) * 90);
  };
  request.upload.onload = () => {
    uploadPhase.value = 'processing';
    uploadProgress.value = Math.max(uploadProgress.value, 95);
  };
  request.onerror = () => reject(new Error('网络连接失败'));
  request.onabort = () => reject(new Error('上传已取消'));
  request.onload = () => {
    let result;
    try {
      result = JSON.parse(request.responseText || '{}');
    } catch {
      reject(new Error('服务器返回了无法解析的结果'));
      return;
    }
    if (request.status < 200 || request.status >= 300) {
      reject(new Error(result.message || `请求失败（HTTP ${request.status}）`));
      return;
    }
    resolve(result);
  };
  request.send(formData);
});

const uploadQueuedFiles = async () => {
  if (isUploading.value || uploadQueue.value.length === 0) return;

  const queuedItems = [...uploadQueue.value];
  isUploading.value = true;
  uploadingCount.value = queuedItems.length;
  uploadProgress.value = 0;
  uploadPhase.value = 'uploading';
  queuedItems.forEach(item => {
    item.status = 'uploading';
    item.error = '';
  });
  
  try {
    const formData = new FormData();
    queuedItems.forEach(item => {
      formData.append('images[]', item.file);
    });
    
    // 携带标签数据
    if (selectedTags.value.length > 0) {
      formData.append('tags', JSON.stringify(selectedTags.value));
    }
    // 携带存储桶信息
    formData.append('bucket_id', selectedBucket.value || '1')
    
    const result = await sendQueuedUpload(formData);
    uploadProgress.value = 100;

    const uploadResults = Array.isArray(result.data?.files) ? result.data.files : [];
    const completedIDs = new Set();
    let uploadedCount = 0;
    let duplicateCount = 0;
    let failedCount = 0;

    queuedItems.forEach((item, index) => {
      const fileResult = uploadResults[index];
      if (fileResult?.success) {
        completedIDs.add(item.id);
        if (fileResult.duplicate) duplicateCount += 1;
        else uploadedCount += 1;
        return;
      }

      item.status = 'failed';
      item.error = fileResult?.message || result.message || '上传失败';
      failedCount += 1;
    });

    uploadQueue.value = uploadQueue.value.filter(item => {
      if (!completedIDs.has(item.id)) return true;
      releaseQueueItem(item);
      return false;
    });

    if (uploadedCount + duplicateCount > 0) {
      await Promise.all([loadRecentImages(), loadDashboardStats()]);
    }
    if (failedCount > 0) {
      Message.warning(`上传完成，成功 ${uploadedCount} 张，重复 ${duplicateCount} 张，失败 ${failedCount} 张`);
    } else if (duplicateCount > 0) {
      Message.warning(`上传完成，已跳过 ${duplicateCount} 张重复图片`);
    } else {
      Message.success(`成功上传 ${uploadedCount} 张图片`);
    }
  } catch (error) {
    console.error('上传错误:', error);
    queuedItems.forEach(item => {
      item.status = 'failed';
      item.error = error.message || '上传失败';
    });
    Message.error(`上传失败: ${error.message}`, {
      duration: 3000,
      position: 'top-right',
      showClose: true
    });
  } finally {
    isUploading.value = false;
    uploadingCount.value = 0;
    uploadProgress.value = 0;
    uploadPhase.value = 'idle';
  }
};

const createUploadTag = async name => {
  try {
    const newTag = await addTagToServer(name);
    if (!presetTags.value.some(tag => tag.id === newTag.id)) {
      presetTags.value.push(newTag);
    }
    Message.success('标签添加成功');
    return { value: newTag.name, label: newTag.name };
  } catch (error) {
    Message.error(error.message || '添加标签失败');
    throw error;
  }
};

/**
 * 图片相关操作
 */
const handleImageLoad = (e) => {
  e.target.classList.remove('opacity-0');
  const loadingEl = e.target.parentElement.querySelector('.loading');
  if (loadingEl) loadingEl.classList.add('hidden');
};

const handleImageError = (e) => {
  e.target.src = errorImg;
  const loadingEl = e.target.parentElement.querySelector('.loading');
  if (loadingEl) loadingEl.classList.add('hidden');
};

const copyImageLink = async (image, type) => {
  if (!image) return;
  
  const fullUrl = getFullUrl(image.url);
  const altText = getImageAltText(image);
  let copyText = '';
  
  switch (type) {
    case 'url':
      copyText = fullUrl;
      break;
    case 'html':
      copyText = `<img src="${fullUrl}" alt="${altText}" width="${image.width || ''}" height="${image.height || ''}">`;
      break;
    case 'markdown':
      copyText = `![${altText}](${fullUrl})`;
      break;
    default:
      copyText = fullUrl;
  }
  
  try {
    await navigator.clipboard.writeText(copyText);
    Message.success(`已复制${getTypeText(type)}格式`, {
      duration: 1500,
      position: 'top-right'
    });
  } catch (error) {
    // 降级处理
    const textArea = document.createElement('textarea');
    textArea.value = copyText;
    document.body.appendChild(textArea);
    textArea.select();
    document.execCommand('copy');
    document.body.removeChild(textArea);
    Message.success(`已复制${getTypeText(type)}格式`, {
      duration: 1500,
      position: 'top-right'
    });
  } finally {
    // 关闭所有下拉菜单
    nextTick(() => {
      previewCopyMenu = false;
      activeCopyMenu.value = null;
      
      // 关闭预览弹窗内的复制下拉框
      const dropdown = document.getElementById('previewCopyDropdown');
      const icon = document.getElementById('copyMenuIcon');
      if (dropdown && icon) {
        dropdown.classList.add('hidden', 'opacity-0', 'translate-y-[-5px]');
        dropdown.classList.remove('block', 'opacity-100', 'translate-y-0');
        icon.classList.remove('rotate-180');
      }
    });
  }
};

/**
 * 存储选择处理事件，设置后优先使用选择的存储
 */
const handleBucketChange = () => {
  const bucketId = selectedBucket.value;
  if (!bucketId) return;
  localStorage.setItem('currentBucket', bucketId);
};

const deleteImage = (imageId) => {
  const modal = new PopupModal({
    title: '确认删除',
    content: `
      <div class="flex gap-3">
        <i class="fa fa-exclamation-triangle text-warning text-xl mt-1"></i>
        <div>
          <p>确定要删除这张图片吗？</p>
          <p class="mt-1 text-secondary text-sm">删除后无法恢复，请谨慎操作</p>
        </div>
      </div>
    `,
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close()
      },
      {
        text: '确认删除',
        type: 'danger',
        callback: async (modal) => {
          modal.close();
          await deleteAsync(imageId);
        }
      }
    ],
    maskClose: true
  });
  modal.open();
};

const downloadImage = (image) => {
  if (!image || !image.url) {
    Message.error('图片信息不完整，无法下载', {
      duration: 2000,
      position: 'top-right'
    });
    return;
  }
  
  const fullUrl = getFullUrl(image.url);
  const link = document.createElement('a');
  link.href = fullUrl;
  link.download = image.filename || `image-${Date.now()}.png`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  
  Message.info('开始下载图片', {
    duration: 1500,
    position: 'top-right'
  });
  
  previewCopyMenu = false;
  activeCopyMenu.value = null;
};

/**
 * 图片预览功能
 */
const previewImage = (image) => {
  if (!image || !image.url) {
    Message.error('图片信息不完整，无法预览', {
      duration: 2000,
      position: 'top-right'
    });
    return;
  }
  
  currentPreviewImage = image;

  const imageTags = image.tags || [];
  const extraTagCount = Math.max(0, imageTags.length - 2);
  const tagsHtml = imageTags.map((tag, index) => `
    <div class="${index >= 2 ? 'home-preview-extra-tag hidden ' : ''}px-2 py-0.5 rounded bg-primary/10 dark:bg-primary/20 text-primary text-xs">
      <span>${escapeHtml(tag.name)}</span>
    </div>
  `).join('') || '';
  
  // 构建预览弹窗内容
  const previewContent = `
    <div class="image-preview-popup w-full max-w-5xl max-h-[85vh] flex flex-col overflow-hidden bg-white dark:bg-dark-200">
      <!-- 顶部操作栏 -->
      <div class="preview-header bg-light-50 pb-2 flex justify-between items-center">
          <h3 class="text-xs font-medium truncate max-w-[50%]">${image.filename}</h3>
          <div class="flex gap-1">
              <!-- 下载按钮 -->
              <button 
                  class="px-3 py-1.5 text-xs bg-light-100 dark:bg-dark-300 hover:bg-light-200 whitespace-nowrap dark:hover:bg-dark-400 text-secondary rounded-md transition-colors duration-200 flex items-center gap-1"
                  onclick="event.stopPropagation(); window.downloadPreviewImage()"
              >
                  <i class="ri-download-fill text-xs"></i>
                  下载
              </button>
              <!-- 删除按钮 -->
              <button 
                  class="px-3 py-1.5 text-xs bg-danger/10 hover:bg-danger/20 whitespace-nowrap text-danger rounded-md transition-colors duration-200 flex items-center gap-1"
                  onclick="event.stopPropagation(); window.deletePreviewImage(${image.id})"
              >
                  <i class="ri-delete-bin-fill text-xs"></i>
                  删除
              </button>
          </div>
      </div>
      
      <!-- 预览图片区域 -->
      <div class="max-h-[360px] flex-1 overflow-auto flex items-center justify-center">
          <a 
              class="spotlight min-w-full max-w-full min-h-[260px] block" 
              href="${getFullUrl(image.url)}" 
              data-description="尺寸: ${image.width || '未知'}×${image.height || '未知'} | 大小: ${formatFileSize(image.file_size || 0)} | 上传日期：${formatDate(image.created_at)} | 角色：${Number(image.uploader_role) === 1 ? '管理员' : Number(image.uploader_role) === 3 ? '普通用户' : '已删除用户'}"
          >
              <div class="relative max-w-full w-fill max-h-[360px] min-h-[260px] rounded-lg overflow-hidden animate-pulse flex items-center justify-center">
                  <div class="absolute inset-0 flex items-center justify-center">
                      <svg class="w-10 h-10 text-slate-300 animate-spin loading-svg" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="transform: scaleX(-1) scaleY(-1);">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                      </svg>
                  </div>
                  <img 
                      src="${getFullUrl(image.url)}"
                      alt="${image.filename}" 
                      class="max-w-full w-fill max-h-[360px] min-h-[260px] object-contain rounded-lg relative z-10 opacity-0 transition-opacity duration-300"
                      onload="this.classList.remove('opacity-0'); this.parentElement.classList.remove('animate-pulse'); this.parentElement.querySelector('.loading-svg').classList.add('hidden');"
                      onerror="this.parentElement.classList.remove('animate-pulse'); this.classList.remove('opacity-0'); this.src='${errorImg}';"
                  />
              </div>
          </a>
      </div>

      <!-- 图片复制区域 -->
      <div class="flex gap-1 flex-wrap items-center w-full mt-3 mb-3">
          <p class="mr-1 text-xs text-secondary font-semibold">复制：</p>
          <button 
              onclick="window.copyPreviewImageLink('url')"
              class="px-2 py-1 text-xs bg-primary shadow-md text-white dark:bg-dark-300 hover:bg-blue-800 rounded transition-colors duration-200">
              <i class="ri-link text-xs w-4 text-center text-white"></i> URL
          </button>

          <button 
              onclick="window.copyPreviewImageLink('html')"
              class="px-2 py-1 text-xs bg-primary shadow-md text-white dark:bg-dark-300 hover:bg-blue-800 rounded transition-colors duration-200">
              <i class="ri-code-fill text-xs w-4 text-center text-white"></i> HTML
          </button>

          <button 
              onclick="window.copyPreviewImageLink('markdown')"
              class="px-2 py-1 text-xs bg-primary shadow-md text-white dark:bg-dark-300 hover:bg-blue-800 rounded transition-colors duration-200">
              <i class="ri-markdown-fill text-xs w-4 text-center text-white"></i> Markdown
          </button>
      </div>

      <!-- 标签 -->
      <div class="pt-2 flex flex-wrap gap-2 items-center">
        <p class="mr-1 text-xs text-secondary font-semibold">标签：</p>
        ${tagsHtml}
        ${extraTagCount > 0 ? `<button
          type="button"
          onclick="window.toggleHomePreviewTags(this)"
          data-count="${extraTagCount}"
          class="px-2 py-0.5 rounded bg-slate-100 text-slate-500 text-xs hover:text-slate-900 dark:bg-white/10 dark:text-slate-300 dark:hover:text-white">
          +${extraTagCount}
        </button>` : ''}
      </div>
      
      <!-- 底部信息栏 -->
      <div class="pt-2 flex flex-wrap gap-2 text-xs text-secondary">
          <div class="flex items-center gap-1.5">
              <i class="ri-ruler-line w-3.5 text-center"></i>
              尺寸: ${image.width || '未知'}×${image.height || '未知'}
          </div>
          <div class="flex items-center gap-1.5">
              <i class="ri-image-line w-3.5 text-center"></i>
              大小: ${formatFileSize(image.file_size || 0)}
          </div>
          <div class="flex items-center gap-1.5">
              <i class="ri-hard-drive-3-line"></i>
              存储: ${(image.storage === 'default' ? '本地' : image.storage) || '未知'}
          </div>
      </div>
  </div>
  `;

  // 注册预览相关全局函数
  window.copyPreviewImageLink = (type) => copyImageLink(currentPreviewImage, type);
  window.downloadPreviewImage = () => downloadImage(currentPreviewImage);
  window.toggleHomePreviewTags = button => {
    const container = button.closest('.image-preview-popup');
    const extraTags = container?.querySelectorAll('.home-preview-extra-tag') || [];
    const shouldExpand = Array.from(extraTags).some(tag => tag.classList.contains('hidden'));
    extraTags.forEach(tag => tag.classList.toggle('hidden', !shouldExpand));
    button.textContent = shouldExpand ? '收起' : `+${button.dataset.count}`;
  };
  window.deletePreviewImage = () => {
    deleteImage(currentPreviewImage.id);
    closePreviewModal();
  };
  window.closePreviewModal = () => {
    if (previewModalInstance) {
      previewModalInstance.close();
      cleanupPreview();
    }
  };

  // 创建预览弹窗
  previewModalInstance = new PopupModal({
    title: '图片预览',
    content: previewContent,
    type: 'default',
    buttons: [{
      text: '确定',
      type: 'default',
      callback: (modal) => modal.close()
    }],
    maskClose: true,
    zIndex: 10000,
    maxHeight: '90vh',
    onClose: cleanupPreview
  });

  previewModalInstance.open();

  // 阻止弹窗内容冒泡
  nextTick(() => {
    const previewContent = document.querySelector('.image-preview-popup');
    if (previewContent) {
      previewContent.addEventListener('click', (e) => e.stopPropagation());
    }
  });
};

/**
 * 从URL上传图片
 */
const uploadbyurlmodal = () => {
  urlUploadUrl.value = '';
  urlUploadTag.value = 0;
  urlUploadBucket.value = selectedBucket.value || '1';
  urlUploadOpen.value = true;
}

const submitUrlUpload = async () => {
  if (!urlUploadUrl.value || urlUploadLoading.value) return;
  urlUploadLoading.value = true;
  try {
    const succeeded = await postuploadbyurl({
      url: urlUploadUrl.value,
      tag_id: urlUploadTag.value || 0,
      bucket_id: urlUploadBucket.value || '1'
    });
    if (succeeded) urlUploadOpen.value = false;
  } finally {
    urlUploadLoading.value = false;
  }
};

const postuploadbyurl = async (formData) => {
  try {
    const res = await fetch(`/api/images/url`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: JSON.stringify(formData)
    });
    const result = await res.json();
    if (res.ok && result.code === 200) {
      await loadRecentImages();
      showUploadResultMessage(result.data?.file ? [result.data.file] : []);
      return true;
    } else {
      throw new Error(result.message || '上传失败');
    }
  } catch (err) {
    console.error(err);
    Message.error(err.message || '上传失败');
    return false;
  }
}

/**
 * 清理预览相关资源
 */
const cleanupPreview = () => {
  // 清理全局函数
  window.copyPreviewImageLink = null;
  window.downloadPreviewImage = null;
  window.deletePreviewImage = null;
  window.closePreviewModal = null;
  window.toggleHomePreviewTags = null;
  
  // 重置状态
  currentPreviewImage = null;
  previewModalInstance = null;
  previewCopyMenu = false;
};

/**
 * 全局点击处理（关闭下拉菜单）
 */
const handleGlobalClick = (e) => {
  if (activeCopyMenu.value !== null) {
    const cardCopyMenus = document.querySelectorAll('.recent-item .relative.z-50');
    let isClickInside = false;
    
    cardCopyMenus.forEach(menu => {
      if (menu.contains(e.target)) {
        isClickInside = true;
      }
    });
    
    if (!isClickInside) {
      activeCopyMenu.value = null;
    }
  }
};

const shouldConfirmQueueExit = () => uploadQueue.value.length > 0 || isUploading.value;

const handleBeforeUnload = event => {
  if (!shouldConfirmQueueExit()) return;
  event.preventDefault();
  event.returnValue = '';
};

onBeforeRouteLeave(() => {
  if (!shouldConfirmQueueExit()) return true;
  return window.confirm('待上传列表中还有图片，离开后将清空。确认离开吗？');
});

// ====================== 生命周期 ======================
onMounted(() => {
  // 初始化数据
  getUploadConfig();
  loadDashboardStats();
  setTimeout(loadRecentImages, 100);
  
  // 注册全局事件
  document.addEventListener('paste', handlePaste);
  document.addEventListener('click', handleGlobalClick);
  window.addEventListener('beforeunload', handleBeforeUnload);
});

onBeforeUnmount(() => {
  // 移除事件监听
  document.removeEventListener('paste', handlePaste);
  document.removeEventListener('click', handleGlobalClick);
  window.removeEventListener('beforeunload', handleBeforeUnload);
  uploadQueue.value.forEach(releaseQueueItem);
  
  // 清理预览资源
  cleanupPreview();
  
  // 关闭所有消息提示
  Message.closeAll();
});
</script>

<style scoped>
.result-list-enter-active,
.result-list-leave-active,
.result-list-move {
  transition: opacity 200ms cubic-bezier(0.2, 0.8, 0.2, 1), transform 220ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.result-list-enter-from,
.result-list-leave-to {
  opacity: 0;
  transform: translateY(6px);
}

@media (prefers-reduced-motion: reduce) {
  .result-list-enter-from,
  .result-list-leave-to {
    transform: none;
  }
}
</style>
