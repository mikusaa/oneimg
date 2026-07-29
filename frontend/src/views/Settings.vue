<template>
    <div class="page-shell text-gray-800 dark:text-gray-200">
        <section class="page-header border-b border-slate-200/70 pb-4 dark:border-white/10">
            <h1 class="page-title">系统设置</h1>

            <div class="settings-status-list" aria-label="当前设置状态">
                <div class="settings-status-item">
                    <span class="settings-status-icon"><i class="ri-database-2-line"></i></span>
                    <span>
                        <span class="settings-status-label">默认存储</span>
                        <span class="settings-status-value">{{ currentDefaultBucket?.name || '未选择' }}</span>
                    </span>
                </div>
                <div class="settings-status-item">
                    <span class="settings-status-icon"><i class="ri-code-box-line"></i></span>
                    <span>
                        <span class="settings-status-label">API 状态</span>
                        <span class="settings-status-value">{{ systemSettings.start_api ? '已启用' : '未启用' }}</span>
                    </span>
                </div>
                <div class="settings-status-item">
                    <span class="settings-status-icon"><i class="ri-file-upload-line"></i></span>
                    <span>
                        <span class="settings-status-label">最大文件</span>
                        <span class="settings-status-value">{{ maxFileSizeReadable }}</span>
                    </span>
                </div>
            </div>
        </section>

        <nav
            v-if="availableSettingTabs.length"
            class="settings-tabs"
            aria-label="设置分类"
        >
            <button
                v-for="tab in availableSettingTabs"
                :key="tab.key"
                :ref="element => setTabRef(tab.key, element)"
                type="button"
                class="settings-tab"
                :class="{ 'settings-tab-active': activeSettingTab === tab.key }"
                @click="selectSettingTab(tab.key)"
            >
                <i :class="tab.icon"></i>
                <span>{{ tab.label }}</span>
            </button>
        </nav>

        <div class="pb-8 md:pb-10">
            <div v-if="activeSettingTab" class="page-surface settings-panel">
                <header class="settings-panel-header">
                    <span class="panel-icon text-2xl"><i :class="activeSettingTabIcon"></i></span>
                    <h2 class="text-lg font-semibold text-slate-900 dark:text-white sm:text-xl">
                        {{ activeSettingTabLabel }}
                    </h2>
                    <span class="sr-only" aria-live="polite">{{ latestSaveMessage }}</span>
                </header>

                <div class="divide-y divide-slate-200/80 dark:divide-white/10">
                    <template v-if="activeSettingTab === 'storage'">
                        <section class="settings-section">
                            <div class="settings-section-heading">
                                <h3>存储策略</h3>
                            </div>
                            <div class="settings-field-grid">
                                <div class="setting-group">
                                    <label class="field-label" for="default_storage">系统默认存储</label>
                                    <select
                                        id="default_storage"
                                        v-model="systemSettings.default_storage"
                                        :data-save-state="saveStates.default_storage"
                                        class="input-modern"
                                        @change="handleSelectChange('default_storage', systemSettings.default_storage)"
                                    >
                                        <option
                                            v-for="bucket in presetBuckets"
                                            :key="bucket.id"
                                            :value="bucket.id"
                                        >
                                            {{ bucket.name }} ({{ bucket.type }})
                                        </option>
                                    </select>
                                    <p class="field-hint">普通用户始终可使用默认存储，其他存储按用户权限开放。</p>
                                </div>

                                <div class="settings-toggle-field">
                                    <div>
                                        <p class="setting-row-title">多存储后台同步</p>
                                        <p class="setting-row-hint">先保存到本地，再按用户权限异步同步到远端存储。</p>
                                    </div>
                                    <label class="settings-switch" title="多存储后台同步">
                                        <input
                                            v-model="systemSettings.multi_storage_sync"
                                            :data-save-state="saveStates.multi_storage_sync"
                                            type="checkbox"
                                            class="sr-only peer"
                                            @change="handleSwitchChange('multi_storage_sync', systemSettings.multi_storage_sync)"
                                        >
                                        <span class="switch-track"></span>
                                        <span class="switch-thumb"></span>
                                    </label>
                                </div>
                            </div>
                        </section>

                        <section class="settings-section">
                            <div class="settings-section-heading">
                                <h3>图片直链</h3>
                            </div>
                            <div class="settings-field-grid">
                                <div class="setting-group">
                                    <label class="field-label" for="public_image_domain">图片直链域名</label>
                                    <input
                                        id="public_image_domain"
                                        v-model="systemSettings.public_image_domain"
                                        :data-save-state="saveStates.public_image_domain"
                                        type="text"
                                        class="input-modern"
                                        :class="{ 'cursor-not-allowed opacity-60': publicImageDomainInputDisabled }"
                                        :disabled="publicImageDomainInputDisabled"
                                        placeholder="例如 https://img.example.com"
                                        @blur="handleFieldBlur('public_image_domain', systemSettings.public_image_domain)"
                                    >
                                    <p
                                        class="field-hint"
                                        :class="{ 'text-amber-600 dark:text-amber-300': publicImageDomainUnavailable || hasPublicImageDomain }"
                                    >
                                        {{ publicImageDomainHint }}
                                    </p>
                                </div>
                            </div>
                        </section>
                    </template>

                    <template v-else-if="activeSettingTab === 'upload'">
                        <section class="settings-section">
                            <div class="settings-section-heading">
                                <h3>文件组织</h3>
                            </div>
                            <div class="settings-field-grid">
                                <div class="setting-group">
                                    <label class="field-label" for="default_path">默认存储路径</label>
                                    <input
                                        id="default_path"
                                        v-model="systemSettings.default_path"
                                        :data-save-state="saveStates.default_path"
                                        type="text"
                                        class="input-modern"
                                        placeholder="例如 /uploads/{year}/{month}/{day}"
                                        @blur="handleFieldBlur('default_path', systemSettings.default_path)"
                                    >
                                    <p class="field-hint">
                                        支持 {year}、{month}、{day}、{hour}、{minute}、{random}、{uuid} 和 {role}。
                                    </p>
                                </div>

                                <div class="setting-group">
                                    <label class="field-label" for="file_name">文件名模板</label>
                                    <input
                                        id="file_name"
                                        v-model="systemSettings.file_name"
                                        :data-save-state="saveStates.file_name"
                                        type="text"
                                        class="input-modern"
                                        :class="{ 'cursor-not-allowed opacity-60': systemSettings.save_original_name }"
                                        :disabled="systemSettings.save_original_name"
                                        placeholder="例如 {random}"
                                        @blur="handleFieldBlur('file_name', systemSettings.file_name)"
                                    >
                                    <p class="field-hint">
                                        支持 {random}、{year}、{month}、{day}、{hour}、{minute} 和 {second}。
                                    </p>
                                </div>
                            </div>

                            <div class="settings-toggle-row mt-5">
                                <div>
                                    <p class="setting-row-title">保留原文件名</p>
                                    <p class="setting-row-hint">使用上传文件的基础名称，扩展名仍按最终图片格式调整；启用后文件名模板不生效。</p>
                                </div>
                                <label class="settings-switch" title="保留原文件名">
                                    <input
                                        v-model="systemSettings.save_original_name"
                                        :data-save-state="saveStates.save_original_name"
                                        type="checkbox"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('save_original_name', systemSettings.save_original_name)"
                                    >
                                    <span class="switch-track"></span>
                                    <span class="switch-thumb"></span>
                                </label>
                            </div>
                        </section>

                        <section class="settings-section">
                            <div class="settings-section-heading">
                                <h3>上传限制</h3>
                            </div>
                            <div class="settings-field-grid">
                                <div class="setting-group">
                                    <label class="field-label" for="max_file_size">最大文件大小</label>
                                    <div class="settings-input-suffix">
                                        <input
                                            id="max_file_size"
                                            v-model.number="systemSettings.max_file_size"
                                            :data-save-state="saveStates.max_file_size"
                                            type="number"
                                            min="1"
                                            class="input-modern pr-16"
                                            @blur="handleFieldBlur('max_file_size', systemSettings.max_file_size)"
                                        >
                                        <span>字节</span>
                                    </div>
                                    <p class="field-hint">默认 10485760 字节（10 MB）。</p>
                                </div>

                                <div class="setting-group">
                                    <label class="field-label" for="allowed_types">允许上传的图片类型</label>
                                    <textarea
                                        id="allowed_types"
                                        v-model="systemSettings.allowed_types"
                                        :data-save-state="saveStates.allowed_types"
                                        class="input-modern min-h-[88px] leading-6"
                                        rows="2"
                                        placeholder="image/jpeg,image/png,image/webp"
                                        @blur="handleFieldBlur('allowed_types', systemSettings.allowed_types)"
                                    ></textarea>
                                    <p class="field-hint">使用英文逗号分隔 MIME 类型。</p>
                                </div>
                            </div>
                        </section>
                    </template>

                    <template v-else-if="activeSettingTab === 'image'">
                        <section class="settings-section">
                            <div class="settings-section-heading">
                                <h3>主图保存</h3>
                            </div>

                            <div class="settings-toggle-list">
                                <div class="settings-toggle-row">
                                    <div>
                                        <p class="setting-row-title">保存原图</p>
                                        <p class="setting-row-hint">主图保持上传时的字节、格式和扩展名，不压缩、不转换。</p>
                                    </div>
                                    <label class="settings-switch" title="保存原图">
                                        <input
                                            v-model="systemSettings.original_image"
                                            :data-save-state="saveStates.original_image"
                                            type="checkbox"
                                            class="sr-only peer"
                                            @change="handleSwitchChange('original_image', systemSettings.original_image)"
                                        >
                                        <span class="switch-track"></span>
                                        <span class="switch-thumb"></span>
                                    </label>
                                </div>

                                <div class="settings-toggle-row" :class="{ 'opacity-50': systemSettings.original_image }">
                                    <div>
                                        <p class="setting-row-title">保存 WebP 格式</p>
                                    </div>
                                    <label
                                        class="settings-switch"
                                        :class="systemSettings.original_image ? 'cursor-not-allowed' : ''"
                                        title="保存 WebP 格式"
                                    >
                                        <input
                                            v-model="systemSettings.save_webp"
                                            :data-save-state="saveStates.save_webp"
                                            type="checkbox"
                                            class="sr-only peer"
                                            :disabled="systemSettings.original_image"
                                            @change="handleSwitchChange('save_webp', systemSettings.save_webp)"
                                        >
                                        <span class="switch-track"></span>
                                        <span class="switch-thumb"></span>
                                    </label>
                                </div>
                            </div>

                            <p v-if="systemSettings.original_image" class="settings-notice mt-4">
                                保存原图已启用，WebP、压缩质量和跳过压缩格式暂不参与主图处理。
                            </p>

                            <div class="settings-field-grid mt-5">
                                <div class="setting-group" :class="{ 'opacity-50': systemSettings.original_image }">
                                    <label class="field-label" for="main_image_quality">主图压缩质量</label>
                                    <input
                                        id="main_image_quality"
                                        v-model.number="systemSettings.main_image_quality"
                                        :data-save-state="saveStates.main_image_quality"
                                        type="number"
                                        min="0"
                                        max="100"
                                        class="input-modern"
                                        :disabled="systemSettings.original_image"
                                        placeholder="85"
                                        @blur="handleFieldBlur('main_image_quality', systemSettings.main_image_quality)"
                                    >
                                    <p class="field-hint">范围 0-100，数值越高画质越好、体积越大。</p>
                                </div>

                                <div class="setting-group" :class="{ 'opacity-50': systemSettings.original_image }">
                                    <label class="field-label" for="skip_compress_formats">跳过压缩格式</label>
                                    <input
                                        id="skip_compress_formats"
                                        v-model="systemSettings.skip_compress_formats"
                                        :data-save-state="saveStates.skip_compress_formats"
                                        type="text"
                                        class="input-modern"
                                        :disabled="systemSettings.original_image"
                                        placeholder="image/gif,image/svg+xml,image/webp"
                                        @blur="handleFieldBlur('skip_compress_formats', systemSettings.skip_compress_formats)"
                                    >
                                    <p class="field-hint">支持 MIME 或扩展名，使用英文逗号分隔；命中后主图保持原格式。</p>
                                </div>
                            </div>
                        </section>

                        <section class="settings-section">
                            <div class="settings-section-heading settings-section-heading-inline">
                                <div>
                                    <h3>缩略图</h3>
                                    <p>用于图库快速预览，不影响对外主图链接。</p>
                                </div>
                                <label class="settings-switch" title="生成缩略图">
                                    <input
                                        v-model="systemSettings.thumbnail"
                                        :data-save-state="saveStates.thumbnail"
                                        type="checkbox"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('thumbnail', systemSettings.thumbnail)"
                                    >
                                    <span class="switch-track"></span>
                                    <span class="switch-thumb"></span>
                                </label>
                            </div>
                        </section>

                    </template>

                    <template v-else-if="activeSettingTab === 'security'">
                        <section class="settings-section">
                            <div class="settings-section-heading">
                                <h3>账号访问</h3>
                            </div>
                            <div class="settings-toggle-list">
                                <div class="settings-toggle-row">
                                    <div>
                                        <p class="setting-row-title">开放用户注册</p>
                                        <p class="setting-row-hint">新账号固定为普通用户，注册后需要自行登录。</p>
                                    </div>
                                    <label class="settings-switch" title="开放用户注册">
                                        <input
                                            v-model="systemSettings.start_register"
                                            :data-save-state="saveStates.start_register"
                                            type="checkbox"
                                            class="sr-only peer"
                                            @change="handleSwitchChange('start_register', systemSettings.start_register)"
                                        >
                                        <span class="switch-track"></span>
                                        <span class="switch-thumb"></span>
                                    </label>
                                </div>

                            </div>
                        </section>

                        <section class="settings-section">
                            <div class="settings-section-heading settings-section-heading-inline">
                                <div>
                                    <h3>图片来源保护</h3>
                                    <p v-if="hasPublicImageDomain" class="text-amber-600 dark:text-amber-300">
                                        已配置图片直链域名，图片不会经过系统代理，来源白名单无法生效。
                                    </p>
                                </div>
                                <label
                                    class="settings-switch"
                                    :class="hasPublicImageDomain ? 'cursor-not-allowed opacity-60' : ''"
                                    title="来源白名单"
                                >
                                    <input
                                        v-model="systemSettings.referer_white_enable"
                                        :data-save-state="saveStates.referer_white_enable"
                                        type="checkbox"
                                        class="sr-only peer"
                                        :disabled="hasPublicImageDomain"
                                        @change="handleSwitchChange('referer_white_enable', systemSettings.referer_white_enable)"
                                    >
                                    <span class="switch-track"></span>
                                    <span class="switch-thumb"></span>
                                </label>
                            </div>

                            <div class="setting-group mt-5" :class="{ 'opacity-50': refererListDisabled }">
                                <label class="field-label" for="referer_white_list">允许的来源域名</label>
                                <textarea
                                    id="referer_white_list"
                                    v-model="systemSettings.referer_white_list"
                                    :data-save-state="saveStates.referer_white_list"
                                    class="input-modern min-h-[112px] leading-6"
                                    :disabled="refererListDisabled"
                                    placeholder="example.com,images.example.com"
                                    rows="4"
                                    @blur="handleFieldBlur('referer_white_list', systemSettings.referer_white_list)"
                                ></textarea>
                                <p class="field-hint">只填写域名，多个域名使用英文逗号分隔；直接打开图片不受限制。</p>
                            </div>
                        </section>
                    </template>

                    <template v-else-if="activeSettingTab === 'integrations'">
                        <section v-if="hasSettingPermission('setting:api')" class="settings-section">
                            <div class="settings-section-heading settings-section-heading-inline">
                                <div>
                                    <h3>上传 API</h3>
                                    <p>通过 API Token 调用图片上传接口。</p>
                                </div>
                                <label class="settings-switch" title="上传 API">
                                    <input
                                        v-model="systemSettings.start_api"
                                        :data-save-state="saveStates.start_api"
                                        type="checkbox"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('start_api', systemSettings.start_api)"
                                    >
                                    <span class="switch-track"></span>
                                    <span class="switch-thumb"></span>
                                </label>
                            </div>

                            <div class="setting-group mt-5">
                                <label class="field-label" for="api_token">API Token</label>
                                <div class="settings-token-row">
                                    <input
                                        id="api_token"
                                        v-model="systemSettings.api_token"
                                        :data-save-state="saveStates.api_token"
                                        type="text"
                                        class="input-modern min-w-0"
                                        :placeholder="systemSettings.api_token_configured ? '已配置，输入新 Token 可替换' : '请输入或生成 API Token'"
                                        @blur="handleFieldBlur('api_token', systemSettings.api_token)"
                                    >
                                    <button type="button" class="soft-button shrink-0" @click="generateApiToken">
                                        <i class="ri-refresh-line"></i>
                                        生成
                                    </button>
                                </div>
                                <p class="field-hint">
                                    请求头使用 <code>Authorization: oneimg_token=&lt;API Token&gt;</code>。新 Token 仅在本次页面中显示。
                                </p>
                            </div>
                        </section>
                    </template>

                    <template v-else-if="activeSettingTab === 'site'">
                        <section class="settings-section">
                            <div class="settings-section-heading">
                                <h3>站点信息</h3>
                            </div>
                            <div class="settings-field-grid">
                                <div class="setting-group">
                                    <label class="field-label" for="seo_title">网站标题</label>
                                    <input
                                        id="seo_title"
                                        v-model="systemSettings.seo_title"
                                        :data-save-state="saveStates.seo_title"
                                        type="text"
                                        class="input-modern"
                                        @blur="handleFieldBlur('seo_title', systemSettings.seo_title)"
                                    >
                                </div>

                                <div class="setting-group">
                                    <label class="field-label" for="seo_icon">网站图标</label>
                                    <input
                                        id="seo_icon"
                                        v-model="systemSettings.seo_icon"
                                        :data-save-state="saveStates.seo_icon"
                                        type="text"
                                        class="input-modern"
                                        placeholder="https://example.com/favicon.ico"
                                        @blur="handleFieldBlur('seo_icon', systemSettings.seo_icon)"
                                    >
                                </div>

                                <div class="setting-group md:col-span-2">
                                    <label class="field-label" for="seo_description">网站描述</label>
                                    <textarea
                                        id="seo_description"
                                        v-model="systemSettings.seo_description"
                                        :data-save-state="saveStates.seo_description"
                                        class="input-modern min-h-[96px] leading-6"
                                        rows="3"
                                        @blur="handleFieldBlur('seo_description', systemSettings.seo_description)"
                                    ></textarea>
                                </div>

                                <div class="setting-group md:col-span-2">
                                    <label class="field-label" for="seo_keywords">网站关键词</label>
                                    <textarea
                                        id="seo_keywords"
                                        v-model="systemSettings.seo_keywords"
                                        :data-save-state="saveStates.seo_keywords"
                                        class="input-modern min-h-[88px] leading-6"
                                        rows="2"
                                        @blur="handleFieldBlur('seo_keywords', systemSettings.seo_keywords)"
                                    ></textarea>
                                </div>
                            </div>
                        </section>

                        <section class="settings-section">
                            <div class="settings-section-heading">
                                <h3>备案信息</h3>
                            </div>
                            <div class="settings-field-grid">
                                <div class="setting-group">
                                    <label class="field-label" for="seo_icp">ICP备案号</label>
                                    <input
                                        id="seo_icp"
                                        v-model="systemSettings.seo_icp"
                                        :data-save-state="saveStates.seo_icp"
                                        type="text"
                                        class="input-modern"
                                        @blur="handleFieldBlur('seo_icp', systemSettings.seo_icp)"
                                    >
                                    <p class="field-hint">填写后显示在页面底部。</p>
                                </div>

                                <div class="setting-group">
                                    <label class="field-label" for="public_security">公安备案号</label>
                                    <input
                                        id="public_security"
                                        v-model="systemSettings.public_security"
                                        :data-save-state="saveStates.public_security"
                                        type="text"
                                        class="input-modern"
                                        @blur="handleFieldBlur('public_security', systemSettings.public_security)"
                                    >
                                    <p class="field-hint">填写后显示在页面底部。</p>
                                </div>
                            </div>
                        </section>
                    </template>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import message from '@/utils/message.js'

const route = useRoute()
const router = useRouter()

const presetBuckets = ref([
    { id: '1', name: '默认存储', type: 'default' },
])

const systemSettings = reactive({
    id: 1,
    original_image: false,
    save_webp: false,
    thumbnail: false,
    start_register: false,
    referer_white_list: '',
    referer_white_enable: false,
    seo_title: '',
    seo_description: '',
    seo_keywords: '',
    seo_icp: '',
    public_security: '',
    seo_icon: '',
    api_token: '',
    api_token_configured: false,
    start_api: false,
    save_original_name: false,
    default_storage: 1,
    multi_storage_sync: false,
    max_file_size: 10485760,
    allowed_types: 'image/jpeg,image/png,image/gif,image/webp,image/svg+xml',
    main_image_quality: 85,
    skip_compress_formats: 'image/gif,image/svg+xml,image/webp',
    default_path: '/uploads/{year}/{moon}',
    file_name: '{random}',
    public_image_domain: '',
})

const SETTING_TABS = [
    { key: 'storage', permissions: ['setting:upload'], label: '存储设置', icon: 'ri-database-2-line' },
    { key: 'upload', permissions: ['setting:upload'], label: '上传规则', icon: 'ri-upload-cloud-2-line' },
    { key: 'image', permissions: ['setting:image'], label: '图片处理', icon: 'ri-image-edit-line' },
    { key: 'security', permissions: ['setting:security'], label: '访问安全', icon: 'ri-shield-keyhole-line' },
    { key: 'integrations', permissions: ['setting:api'], label: '集成服务', icon: 'ri-plug-line' },
    { key: 'site', permissions: ['setting:seo'], label: '站点信息', icon: 'ri-global-line' },
]

const settingPermissions = ref([])
const activeSettingTab = ref('')
const updateSetting = reactive({})
const saveStates = reactive({})
const saveTimers = new Map()
const saveQueues = new Map()
const saveStateTimers = new Map()
const latestSaveMessage = ref('')
const tabElements = new Map()

const setTabRef = (key, element) => {
    if (element) tabElements.set(key, element)
    else tabElements.delete(key)
}

const revealActiveTab = async (key) => {
    await nextTick()
    tabElements.get(key)?.scrollIntoView({
        behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
        block: 'nearest',
        inline: 'center',
    })
}

const hasSettingPermission = (permission) => settingPermissions.value.includes(permission)
const availableSettingTabs = computed(() => SETTING_TABS.filter(tab => tab.permissions.some(hasSettingPermission)))
const currentSettingTab = computed(() => availableSettingTabs.value.find(tab => tab.key === activeSettingTab.value))
const activeSettingTabLabel = computed(() => currentSettingTab.value?.label || '系统设置')
const activeSettingTabIcon = computed(() => currentSettingTab.value?.icon || 'ri-settings-4-line')

const publicDomainStorageTypes = ['s3', 'r2']
const publicDomainAffectedSettings = [
    'referer_white_enable',
    'referer_white_list',
]

const currentDefaultBucket = computed(() => {
    return presetBuckets.value.find(bucket => String(bucket.id) === String(systemSettings.default_storage))
})

const maxFileSizeReadable = computed(() => {
    const bytes = Number(systemSettings.max_file_size)
    if (!Number.isFinite(bytes) || bytes <= 0) return '--'

    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    let value = bytes
    let unitIndex = 0
    while (value >= 1024 && unitIndex < units.length - 1) {
        value /= 1024
        unitIndex += 1
    }
    const digits = value >= 10 || Number.isInteger(value) ? 0 : 1
    return `${value.toFixed(digits)} ${units[unitIndex]}`
})

const supportsPublicImageDomain = computed(() => {
    return publicDomainStorageTypes.includes(currentDefaultBucket.value?.type)
})

const hasPublicImageDomain = computed(() => String(systemSettings.public_image_domain || '').trim() !== '')
const publicImageDomainUnavailable = computed(() => !supportsPublicImageDomain.value)
const publicImageDomainInputDisabled = computed(() => publicImageDomainUnavailable.value && !hasPublicImageDomain.value)

const publicImageDomainHint = computed(() => {
    if (!supportsPublicImageDomain.value) {
        return '当前默认存储不支持图片直链域名，仅 S3/R2 存储可用。'
    }
    if (hasPublicImageDomain.value) {
        return '图片链接将直接使用该域名，来源白名单等代理能力不会生效。'
    }
    return '填写 S3/R2 绑定的直链域名后，返回链接将直接使用该域名。'
})

const refererListDisabled = computed(() => {
    return hasPublicImageDomain.value || !systemSettings.referer_white_enable
})

const bucketSupportsPublicImageDomain = (bucketId) => {
    const bucket = presetBuckets.value.find(item => String(item.id) === String(bucketId))
    return publicDomainStorageTypes.includes(bucket?.type)
}

const disabledByPublicImageDomain = (key) => {
    return hasPublicImageDomain.value && publicDomainAffectedSettings.includes(key)
}

const normalizeDomain = (value) => {
    let domain = String(value || '').trim()
    if (domain === '') return ''
    if (!domain.includes('://')) domain = `https://${domain}`
    return domain.replace(/\/+$/, '')
}

const getRequestHeaders = () => ({
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${localStorage.getItem('authToken')}`,
})

const valuesMatch = (left, right) => String(left) === String(right)

const persistSetting = async (key, value) => {
    const previousValue = updateSetting[key]
    saveStates[key] = 'saving'
    latestSaveMessage.value = '正在保存设置'
    const existingStateTimer = saveStateTimers.get(key)
    if (existingStateTimer) clearTimeout(existingStateTimer)
    try {
        const response = await fetch('/api/settings/update', {
            method: 'POST',
            headers: getRequestHeaders(),
            body: JSON.stringify({ key, value }),
        })
        const result = await response.json()

        if (!response.ok || result.code !== 200) {
            throw new Error(result.message || '未知错误')
        }

        if (key === 'api_token') {
            systemSettings.api_token_configured = value !== '' || systemSettings.api_token_configured
            updateSetting.api_token_configured = systemSettings.api_token_configured
        } else {
            updateSetting[key] = value
        }
        saveStates[key] = 'success'
        latestSaveMessage.value = '设置已保存'
        saveStateTimers.set(key, setTimeout(() => {
            if (saveStates[key] === 'success') saveStates[key] = ''
            saveStateTimers.delete(key)
        }, 1600))
    } catch (error) {
        if (valuesMatch(systemSettings[key], value) && Object.prototype.hasOwnProperty.call(updateSetting, key)) {
            systemSettings[key] = previousValue
        }
        saveStates[key] = 'error'
        latestSaveMessage.value = '设置保存失败'
        console.error('保存失败:', error)
        message.error(`更新失败：${error.message || '网络异常'}`)
    }
}

const enqueueSettingSave = (key, value) => {
    const previousQueue = saveQueues.get(key) || Promise.resolve()
    const nextQueue = previousQueue
        .catch(() => undefined)
        .then(() => persistSetting(key, value))
    saveQueues.set(key, nextQueue)
    nextQueue.finally(() => {
        if (saveQueues.get(key) === nextQueue) saveQueues.delete(key)
    })
}

const saveSetting = (key, value) => {
    if (valuesMatch(updateSetting[key], value) && key !== 'api_token') return

    const existingTimer = saveTimers.get(key)
    if (existingTimer) clearTimeout(existingTimer)
    saveTimers.set(key, setTimeout(() => {
        saveTimers.delete(key)
        enqueueSettingSave(key, value)
    }, 300))
}

const getBucketsList = async () => {
    try {
        const response = await fetch('/api/buckets/list', {
            method: 'GET',
            headers: getRequestHeaders(),
        })
        const result = await response.json()
        if (!response.ok || result.code !== 200) {
            throw new Error(result.message || '获取存储列表失败')
        }
        presetBuckets.value = result.data || []
    } catch (error) {
        console.error('获取存储列表失败:', error)
        message.error(error.message || '获取存储列表失败')
    }
}

const generateApiToken = () => {
    const chars = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz'
    let token = ''
    for (let i = 0; i < 32; i += 1) {
        token += chars[Math.floor(Math.random() * chars.length)]
    }
    systemSettings.api_token = token
    saveSetting('api_token', token)
}

const handleSwitchChange = (key, value) => {
    if (disabledByPublicImageDomain(key)) {
        message.warning('已配置图片直链域名，该设置不会生效')
        systemSettings[key] = updateSetting[key]
        return
    }

    if (key === 'start_api' && value) {
        if (systemSettings.api_token === '' && !systemSettings.api_token_configured) {
            message.warning('请先配置 API Token')
            systemSettings.start_api = false
            return
        }
    }

    saveSetting(key, value)
}

const handleFieldBlur = (key, rawValue) => {
    if (disabledByPublicImageDomain(key)) {
        message.warning('已配置图片直链域名，该设置不会生效')
        systemSettings[key] = updateSetting[key]
        return
    }

    let value = rawValue
    if (key === 'public_image_domain') {
        value = normalizeDomain(value)
        systemSettings[key] = value

        if (key === 'public_image_domain' && publicImageDomainUnavailable.value && value !== '') {
            message.warning('当前默认存储不支持图片直链域名')
            systemSettings.public_image_domain = updateSetting.public_image_domain || ''
            return
        }
    }

    if (key === 'main_image_quality') {
        const quality = Number(value)
        if (!Number.isInteger(quality) || quality < 0 || quality > 100) {
            message.warning('主图压缩质量必须是 0-100 的整数')
            systemSettings.main_image_quality = updateSetting.main_image_quality ?? 85
            return
        }
        value = quality
        systemSettings.main_image_quality = quality
    }

    if (key === 'skip_compress_formats') {
        value = String(value || '').trim()
        if (value === '') {
            message.warning('跳过压缩格式不能为空')
            systemSettings.skip_compress_formats = updateSetting.skip_compress_formats || 'image/gif,image/svg+xml,image/webp'
            return
        }
        systemSettings.skip_compress_formats = value
    }

    const allowEmptyKeys = ['public_image_domain']
    if (value === '' && !allowEmptyKeys.includes(key)) {
        if (Object.prototype.hasOwnProperty.call(updateSetting, key)) {
            systemSettings[key] = updateSetting[key]
        }
        return
    }
    if (valuesMatch(value, updateSetting[key])) return

    if (key === 'api_token' && value === '' && systemSettings.start_api && !systemSettings.api_token_configured) {
        systemSettings.start_api = false
        saveSetting('start_api', false)
    }
    saveSetting(key, value)
}

const handleSelectChange = (key, value) => {
    if (key === 'default_storage' && hasPublicImageDomain.value && !bucketSupportsPublicImageDomain(value)) {
        message.warning('图片直链域名仅支持 S3/R2 存储，请先清空该配置')
        systemSettings.default_storage = updateSetting.default_storage
        return
    }

    if (disabledByPublicImageDomain(key)) {
        message.warning('已配置图片直链域名，该设置不会生效')
        systemSettings[key] = updateSetting[key]
        return
    }
    saveSetting(key, value)
}

const syncActiveTab = () => {
    const requestedTab = String(route.query.tab || '')
    const nextTab = availableSettingTabs.value.some(tab => tab.key === requestedTab)
        ? requestedTab
        : availableSettingTabs.value[0]?.key || ''

    activeSettingTab.value = nextTab
    if (nextTab) void revealActiveTab(nextTab)
    if (nextTab && requestedTab !== nextTab) {
        void router.replace({ query: { ...route.query, tab: nextTab } })
    }
}

const selectSettingTab = (tab) => {
    if (!availableSettingTabs.value.some(item => item.key === tab)) return
    activeSettingTab.value = tab
    void revealActiveTab(tab)
    if (route.query.tab !== tab) {
        void router.replace({ query: { ...route.query, tab } })
    }
}

const getSettings = async () => {
    try {
        const response = await fetch('/api/settings/get', {
            method: 'GET',
            headers: getRequestHeaders(),
        })
        if (!response.ok) throw new Error(`请求失败：${response.status}`)

        const result = await response.json()
        if (result.code !== 200 || !result.data) {
            throw new Error(result.message || '获取设置失败：无数据')
        }

        settingPermissions.value = Array.isArray(result.data.setting_permissions)
            ? result.data.setting_permissions
            : []
        Object.assign(systemSettings, result.data)
        Object.assign(updateSetting, result.data)
        syncActiveTab()
    } catch (error) {
        console.error('获取设置失败:', error)
        message.error(error.message || '获取设置失败：网络异常')
    }
}

watch(() => route.query.tab, syncActiveTab)

onMounted(() => {
    getSettings()
    getBucketsList()
})

onBeforeUnmount(() => {
    saveTimers.forEach(timer => clearTimeout(timer))
    saveStateTimers.forEach(timer => clearTimeout(timer))
})
</script>
