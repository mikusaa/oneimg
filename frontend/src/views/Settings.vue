<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { api, errorMessage, fieldErrors, type StorageBucket } from '@/api/client.ts'
import SettingsImageTab from '@/components/settings/SettingsImageTab.vue'
import SettingsSecurityTab from '@/components/settings/SettingsSecurityTab.vue'
import SettingsSiteTab from '@/components/settings/SettingsSiteTab.vue'
import SettingsUploadTab, { type SampleStorage } from '@/components/settings/SettingsUploadTab.vue'
import {
  FIELD_GROUP, FIELD_LABEL, fromServer, sameValue, SETTINGS_KEYS, toServer, validateDraft,
  type SettingsDraft, type SettingsGroup, type SettingsKey,
} from '@/components/settings/settingsModel'
import { storageAccess } from '@/components/storage/storageInfo'
import { confirm, toast, UiButton } from '@/components/ui'
import { applySiteConfig, useSite } from '@/composables/useSite'
import { loadUploadOptions } from '@/composables/useUploadQueue'

/*
 * 系统设置：各分区的改动先存在草稿里，由底部保存栏统一提交。
 * 只提交改过的字段，放在一次 PATCH 里；后端在一个事务中保存，任一字段无效时全部不保存，
 * 返回的 errors[].field 用来切换到对应分区并标出字段。
 */

type TabKey = 'upload' | 'image' | 'security' | 'site'
const TABS: Array<{ key: TabKey; group: SettingsGroup; label: string; icon: string }> = [
  { key: 'upload', group: 'upload', label: '上传规则', icon: 'ri-upload-cloud-2-line' },
  { key: 'image', group: 'image', label: '图片处理', icon: 'ri-image-edit-line' },
  { key: 'security', group: 'security', label: '访问安全', icon: 'ri-shield-keyhole-line' },
  { key: 'site', group: 'seo', label: '站点信息', icon: 'ri-global-line' },
]
const tabOfGroup = (group: SettingsGroup) => TABS.find(tab => tab.group === group)?.key

const route = useRoute()
const router = useRouter()
const site = useSite()
// 表单值只有数字、字符串、布尔和字符串数组；草稿里的数组是响应式代理，不能用 structuredClone
const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value))

const groups = ref<SettingsGroup[]>([])
const tabs = computed(() => TABS.filter(tab => groups.value.includes(tab.group)))
const saved = reactive({}) as Partial<SettingsDraft>
const draft = reactive({}) as SettingsDraft
const loading = ref(true)
const loadError = ref('')
const saving = ref(false)
/** 服务器返回的字段错误，修改该字段后清除 */
const serverErrors = reactive<Record<string, string>>({})

const keys = computed(() => SETTINGS_KEYS.filter(key => key in saved))
const isDirty = (key: SettingsKey) => key in saved && !sameValue(draft[key], saved[key])
const dirtyKeys = computed(() => keys.value.filter(isDirty))
const tabDirty = (tab: TabKey) => dirtyKeys.value.some(key => tabOfGroup(FIELD_GROUP[key]) === tab)

const clientErrors = computed(() => validateDraft(draft, isDirty))
const errors = computed<Record<string, string>>(() => ({ ...clientErrors.value, ...serverErrors }))
const blockers = computed(() => [...new Set(Object.keys(clientErrors.value).map(key => FIELD_LABEL[key] ?? key))])

// 修改字段后去掉它的服务器错误
const lastSubmitted: Record<string, unknown> = {}
watch(() => keys.value.map(key => draft[key]), () => {
  for (const key of Object.keys(serverErrors)) {
    if (key in saved && !sameValue(draft[key as SettingsKey], lastSubmitted[key])) delete serverErrors[key]
  }
}, { deep: true })

/* ---------- 分区（同步到地址栏 ?tab=） ---------- */
const active = ref<TabKey>('upload')
const syncTab = () => {
  // 离开本页时路由已经变了，不要把 tab 参数带到下一个页面
  if (route.name !== 'Settings') return
  // 旧版的「存储设置」分区已并入存储页
  const requested = route.query.tab === 'storage' ? 'upload' : String(route.query.tab ?? '')
  const next = tabs.value.find(tab => tab.key === requested)?.key ?? tabs.value[0]?.key
  if (!next) return
  active.value = next
  if (route.query.tab !== next) void router.replace({ query: { ...route.query, tab: next } })
}
const selectTab = (key: TabKey) => {
  active.value = key
  if (route.query.tab !== key) void router.replace({ query: { ...route.query, tab: key } })
}
watch(() => route.query.tab, syncTab)

/* ---------- 读取 ---------- */
const sampleStorage = ref<SampleStorage | null>(null)
const scope = ref<{ proxied: string[]; direct: string[] } | null>(null)

const load = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const data = await api.getSettings()
    groups.value = (Array.isArray(data.groups) ? data.groups : []) as SettingsGroup[]
    const values = fromServer(data)
    for (const key of Object.keys(saved)) delete saved[key as SettingsKey]
    Object.assign(saved, clone(values))
    Object.assign(draft, clone(values))
    syncTab()
    void loadStorages(String(data.cdn_domain ?? ''), 'cdn_domain' in data)
  } catch (error) {
    loadError.value = errorMessage(error, '读取设置失败')
  } finally {
    loading.value = false
  }
}

// 示例链接和白名单生效范围需要存储列表；没有存储权限时退回到上传选项，只显示经 OneImg 的示例
const loadStorages = async (cdnDomain: string, knowsCdn: boolean) => {
  let options: Record<string, unknown> = {}
  try { options = await api.uploadOptions() } catch { /* 示例里不写存储名称 */ }
  const defaultId = Number(options.default_storage) || 0
  let list: StorageBucket[] | null = null
  try { list = await api.listBuckets() } catch { list = null }
  if (list) {
    const access = (storage: StorageBucket) => storageAccess(storage, knowsCdn ? cdnDomain : '')
    const target = list.find(storage => storage.id === defaultId)
    if (target) sampleStorage.value = { name: target.name, type: target.type, ...access(target) }
    scope.value = knowsCdn
      ? { proxied: list.filter(storage => !access(storage).direct).map(storage => storage.name), direct: list.filter(storage => access(storage).direct).map(storage => storage.name) }
      : null
    return
  }
  const fallback = (Array.isArray(options.storage_buckets) ? options.storage_buckets : []) as Record<string, unknown>[]
  const target = fallback.find(storage => Number(storage.id) === defaultId)
  if (target) sampleStorage.value = { name: String(target.name ?? ''), type: String(target.type ?? ''), domain: '', stripUploads: false }
}

onMounted(load)

/* ---------- 保存与放弃 ---------- */
const discard = () => {
  Object.assign(draft, clone(saved))
  for (const key of Object.keys(serverErrors)) delete serverErrors[key]
}

const save = async () => {
  if (saving.value || !dirtyKeys.value.length) return
  if (blockers.value.length) {
    const first = Object.keys(clientErrors.value)[0] as SettingsKey
    const tab = tabOfGroup(FIELD_GROUP[first])
    if (tab) selectTab(tab)
    toast.error(`请先修正${blockers.value.join('和')}里的错误`)
    return
  }
  const changed = [...dirtyKeys.value]
  const patch: Record<string, unknown> = {}
  for (const key of changed) {
    patch[key] = toServer(key, draft[key])
    lastSubmitted[key] = clone(draft[key])
  }
  saving.value = true
  for (const key of Object.keys(serverErrors)) delete serverErrors[key]
  try {
    const result = fromServer(await api.updateSettings(patch))
    // 只更新本页能看到的字段；没改动的字段也用服务器的值，以便看到别人刚保存的结果
    for (const key of keys.value) {
      if (!(key in result)) continue
      saved[key] = clone(result[key]) as never
      if (changed.includes(key)) draft[key] = clone(result[key]) as never
    }
    toast.success(`已保存 ${changed.length} 项设置`)
    if (changed.some(key => FIELD_GROUP[key] === 'seo')) {
      applySiteConfig({
        title: saved.seo_title ?? '', description: saved.seo_description ?? '', keywords: saved.seo_keywords ?? '',
        icon: saved.seo_icon ?? '', icp: saved.seo_icp ?? '', publicSecurity: saved.public_security ?? '',
      })
      document.title = `${route.meta.title} - ${site.title}`
    }
    if (changed.some(key => FIELD_GROUP[key] === 'upload')) void loadUploadOptions(true).catch(() => undefined)
  } catch (error) {
    const fields = fieldErrors(error)
    Object.assign(serverErrors, fields)
    const field = Object.keys(fields)[0]
    if (field) {
      const group = FIELD_GROUP[field as SettingsKey]
      const tab = group ? tabOfGroup(group) : undefined
      if (tab) selectTab(tab)
      toast.error(`${FIELD_LABEL[field] ?? field}：${fields[field]}。所有更改都没有保存`)
    } else {
      toast.error(`${errorMessage(error, '保存失败')}。所有更改都没有保存`)
    }
  } finally {
    saving.value = false
  }
}

/* ---------- 离开前提醒 ---------- */
const beforeUnload = (event: BeforeUnloadEvent) => {
  if (!dirtyKeys.value.length) return
  event.preventDefault()
  event.returnValue = ''
}
window.addEventListener('beforeunload', beforeUnload)
onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
onBeforeRouteLeave(async () => {
  if (!dirtyKeys.value.length) return true
  return confirm({
    title: '放弃未保存的更改？',
    description: `还有 ${dirtyKeys.value.length} 项设置没有保存，离开后会丢失。`,
    danger: true,
    cancelText: '留在本页',
    confirmText: '放弃更改',
  })
})

const saveStatus = computed(() => blockers.value.length ? `${blockers.value.join('和')}有错误，修正后才能保存` : `${dirtyKeys.value.length} 项更改未保存`)
</script>

<template>
  <div class="mx-auto w-full max-w-[1100px] font-oi text-sm text-oi-text">
    <header class="mb-[22px]">
      <h1 class="text-2xl font-[680] leading-tight tracking-[-0.02em] text-oi-ink">系统设置</h1>
      <p class="mt-1.5 text-oi-muted">
        对全站生效的上传、处理与访问规则。默认存储和链接域名在 <RouterLink to="/storage" class="text-oi-accent-2 hover:underline">存储</RouterLink> 中设置。
      </p>
    </header>

    <div v-if="loadError" class="mb-4 flex flex-wrap items-center gap-3 rounded-oi border border-oi-line bg-oi-danger-soft px-4 py-3 text-[13px] text-oi-danger" role="alert">
      <i class="ri-error-warning-line" aria-hidden="true" />
      <span class="min-w-0 flex-1">{{ loadError }}</span>
      <UiButton size="sm" @click="load">重试</UiButton>
    </div>

    <div v-if="loading && !tabs.length" class="grid grid-cols-1 gap-4 min-[900px]:grid-cols-[200px_minmax(0,1fr)] min-[900px]:gap-8" aria-busy="true">
      <div class="h-[152px] animate-pulse rounded-oi bg-oi-panel max-[899px]:h-9" />
      <div class="h-[420px] animate-pulse rounded-oi-lg bg-oi-panel" />
    </div>

    <div v-else-if="tabs.length" class="grid grid-cols-1 items-start gap-4 min-[900px]:grid-cols-[200px_minmax(0,1fr)] min-[900px]:gap-8">
      <nav class="-mx-4 flex gap-0.5 overflow-x-auto px-4 [scrollbar-width:none] min-[900px]:sticky min-[900px]:top-4 min-[900px]:mx-0 min-[900px]:flex-col min-[900px]:px-0" aria-label="设置分区">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          :aria-current="active === tab.key ? 'page' : undefined"
          :class="[
            'flex h-9 flex-none items-center gap-2.5 rounded-[9px] px-3 text-left font-[520] transition-colors [&>i]:text-[17px] focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring',
            active === tab.key ? 'bg-oi-panel text-oi-ink shadow-[inset_0_0_0_1px_var(--oi-line)] [&>i]:text-oi-accent' : 'text-oi-text hover:bg-oi-hover [&>i]:text-oi-muted',
          ]"
          @click="selectTab(tab.key)"
        >
          <i :class="tab.icon" aria-hidden="true" />{{ tab.label }}
          <span v-if="tabDirty(tab.key)" class="ml-auto h-1.5 w-1.5 rounded-full bg-oi-warn" title="有未保存的更改" />
          <span v-if="tabDirty(tab.key)" class="sr-only">（有未保存的更改）</span>
        </button>
      </nav>

      <div class="min-w-0">
        <SettingsUploadTab v-if="active === 'upload'" :draft="draft" :errors="errors" :dirty="isDirty" :sample-storage="sampleStorage" />
        <SettingsImageTab v-else-if="active === 'image'" :draft="draft" :errors="errors" :dirty="isDirty" />
        <SettingsSecurityTab v-else-if="active === 'security'" :draft="draft" :errors="errors" :dirty="isDirty" :scope="scope" />
        <SettingsSiteTab v-else-if="active === 'site'" :draft="draft" :errors="errors" :dirty="isDirty" />

        <div
          v-if="dirtyKeys.length"
          class="sticky bottom-[calc(74px+env(safe-area-inset-bottom))] z-20 mt-2 flex items-center gap-2.5 rounded-[14px] bg-oi-btn py-2.5 pl-4 pr-2.5 text-oi-btn-ink shadow-oi-lg min-[900px]:bottom-4"
          role="region"
          aria-label="未保存的更改"
        >
          <span class="flex min-w-0 flex-1 items-center gap-2 text-[13px] font-[520]" aria-live="polite">
            <i class="ri-error-warning-fill text-base text-oi-warn" aria-hidden="true" />{{ saveStatus }}
          </span>
          <UiButton size="sm" variant="ghost" class="!text-oi-btn-ink opacity-80 hover:!bg-[color-mix(in_srgb,var(--oi-btn-ink)_12%,transparent)] hover:opacity-100" :disabled="saving" @click="discard">放弃</UiButton>
          <UiButton size="sm" variant="accent" :disabled="saving || blockers.length > 0" @click="save">
            <i v-if="saving" class="ri-loader-4-line animate-spin" aria-hidden="true" />保存更改
          </UiButton>
        </div>
      </div>
    </div>
  </div>
</template>
