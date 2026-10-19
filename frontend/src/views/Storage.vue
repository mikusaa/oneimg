<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, ApiError, type StorageBucket } from '@/api/client.ts'
import StorageCard from '@/components/storage/StorageCard.vue'
import StorageDeleteDialog from '@/components/storage/StorageDeleteDialog.vue'
import StorageDomainDialog from '@/components/storage/StorageDomainDialog.vue'
import StorageFormDialog from '@/components/storage/StorageFormDialog.vue'
import { storageAccess } from '@/components/storage/storageInfo'
import { confirm, toast, UiButton } from '@/components/ui'
import { useSession } from '@/composables/useSession'
import { loadUploadOptions } from '@/composables/useUploadQueue'
import { hasPermission } from '@/utils/permissions.ts'

const { user } = useSession()
const can = (code: string) => hasPermission(code, user.value)
const canCreate = computed(() => can('storage:create'))
const canUpdate = computed(() => can('storage:update'))
const canDelete = computed(() => can('storage:delete'))
// 默认存储和本地 CDN 域名仍是系统设置（setting:upload）
const canSettings = computed(() => can('setting:upload'))

const storages = ref<StorageBucket[]>([])
const defaultStorage = ref(0)
// 只有 setting:upload 能读取；读不到时为 null，本地存储卡片不判断是否直连
const cdnDomain = ref<string | null>(null)
const loading = ref(true)
const loadError = ref('')
const busyId = ref<number | null>(null)

const load = async () => {
  loadError.value = ''
  try {
    const [list, options, settings] = await Promise.all([
      api.listBuckets(),
      api.uploadOptions(),
      canSettings.value ? api.getSettings('upload') : Promise.resolve(null),
    ])
    storages.value = list
    defaultStorage.value = Number(options.default_storage) || 0
    selectedDefault.value = defaultStorage.value
    cdnDomain.value = settings ? String(settings.cdn_domain ?? '') : null
  } catch (error) {
    loadError.value = error instanceof ApiError ? error.problem?.detail || '读取存储列表失败' : '读取存储列表失败'
  } finally {
    loading.value = false
  }
}
// 上传页的存储列表跟着刷新
const refresh = async () => {
  await load()
  void loadUploadOptions(true).catch(() => undefined)
}
onMounted(load)

/* ---------- 默认存储 ---------- */
const selectedDefault = ref(0)
const savingDefault = ref(false)
const defaultSelect = ref<HTMLSelectElement | null>(null)
const saveDefault = async () => {
  if (savingDefault.value || selectedDefault.value === defaultStorage.value) return
  savingDefault.value = true
  try {
    await api.updateSettings({ default_storage: selectedDefault.value })
    toast.success('默认存储已更新')
    await refresh()
  } catch (error) {
    toast.error(error instanceof ApiError ? error.problem?.detail || '保存失败' : '保存失败，请稍后重试')
  } finally {
    savingDefault.value = false
  }
}

// 当前默认存储不能停用或删除（后端返回 409 storage_is_upload_default）
const explainDefault = async (storage: StorageBucket, action: '停用' | '删除') => {
  if (!canSettings.value) {
    toast.info(`「${storage.name}」是当前默认存储，${action}前需要有「存储与上传」设置权限的管理员先切换默认存储`)
    return
  }
  const ok = await confirm({
    title: `无法${action}默认存储`,
    description: `「${storage.name}」是当前的默认存储，${action}后上传默认值会失效。请先在本页顶部切换默认存储，再${action}它。`,
    icon: 'ri-information-line',
    cancelText: '知道了',
    confirmText: '去切换',
  })
  if (!ok) return
  // 弹窗关闭时会把焦点还给触发按钮，等它卸载后再移动焦点
  await dialogsClosed()
  defaultSelect.value?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  defaultSelect.value?.focus()
}
const dialogsClosed = () => new Promise<void>(resolve => {
  const started = performance.now()
  const check = () => {
    if (!document.querySelector('[role="dialog"]') || performance.now() - started > 1000) setTimeout(resolve)
    else requestAnimationFrame(check)
  }
  check()
})

/* ---------- 停用 / 启用、测试连接 ---------- */
const setDisabled = async (storage: StorageBucket, disabled: boolean, skipConfirm = false) => {
  if (disabled && storage.id === defaultStorage.value) return explainDefault(storage, '停用')
  if (disabled && !skipConfirm) {
    const ok = await confirm({
      title: `停用存储「${storage.name}」？`,
      description: '停用后：',
      icon: 'ri-pause-circle-line',
      points: ['不再出现在上传选项中，上传和 URL 导入都会被拒绝', '已有文件、外链、用户授权和远端文件保持不变', '可以随时重新启用'],
      confirmText: '停用',
    })
    if (!ok) return
  }
  busyId.value = storage.id
  try {
    await api.setBucketDisabled(storage.id, disabled)
    toast.success(disabled ? `「${storage.name}」已停用` : `「${storage.name}」已启用`)
    await refresh()
  } catch (error) {
    toast.error(error instanceof ApiError ? error.problem?.detail || '操作失败' : '操作失败，请稍后重试')
  } finally {
    busyId.value = null
  }
}

const testConnection = async (storage: StorageBucket) => {
  busyId.value = storage.id
  toast.info(`正在测试「${storage.name}」…`)
  try {
    const result = await api.testBucketConnection({ id: storage.id, name: storage.name, type: storage.type, capacity_bytes: storage.capacity_bytes, config: {} })
    toast.success(`「${storage.name}」${result?.detail || '连接正常'}`)
  } catch (error) {
    toast.error(`「${storage.name}」${error instanceof ApiError ? error.problem?.detail || '连接测试失败' : '连接测试失败'}`)
  } finally {
    busyId.value = null
  }
}

/* ---------- 弹窗 ---------- */
const formOpen = ref(false)
const formStorage = ref<StorageBucket | null>(null)
const openForm = (storage: StorageBucket | null) => {
  formStorage.value = storage
  formOpen.value = true
}

const domainOpen = ref(false)
const domainStorage = ref<StorageBucket | null>(null)
const openDomain = (storage: StorageBucket) => {
  domainStorage.value = storage
  domainOpen.value = true
}

const deleteOpen = ref(false)
const deleteStorage = ref<StorageBucket | null>(null)
const openDelete = (storage: StorageBucket) => {
  if (storage.id === defaultStorage.value) return void explainDefault(storage, '删除')
  deleteStorage.value = storage
  deleteOpen.value = true
}

const directCount = computed(() => storages.value.filter(storage => storageAccess(storage, cdnDomain.value ?? '').direct).length)
</script>

<template>
  <div class="mx-auto w-full max-w-[1240px] font-oi text-sm text-oi-text">
    <header class="mb-[22px] flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="text-2xl font-[680] leading-tight tracking-[-0.02em] text-oi-ink">存储</h1>
        <p class="mt-1.5 text-oi-muted">文件存在哪里、链接用什么域名，都在这里设置。</p>
      </div>
      <UiButton v-if="canCreate" variant="primary" @click="openForm(null)"><i class="ri-add-line" aria-hidden="true" />添加存储</UiButton>
    </header>

    <div v-if="loadError" class="mb-4 flex flex-wrap items-center gap-3 rounded-oi border border-oi-line bg-oi-danger-soft px-4 py-3 text-[13px] text-oi-danger" role="alert">
      <i class="ri-error-warning-line" aria-hidden="true" />
      <span class="min-w-0 flex-1">{{ loadError }}</span>
      <UiButton size="sm" @click="load">重试</UiButton>
    </div>

    <section
      v-if="storages.length"
      class="flex items-center justify-between gap-5 rounded-oi-lg border border-oi-line bg-oi-raised px-[18px] py-4 max-[760px]:flex-col max-[760px]:items-stretch"
      aria-labelledby="storage-default-title"
    >
      <div class="flex min-w-0 items-start gap-3.5">
        <span class="grid h-[38px] w-[38px] flex-none place-items-center rounded-xl bg-oi-panel text-[19px] text-oi-ink" aria-hidden="true"><i class="ri-star-line" /></span>
        <div class="min-w-0">
          <h2 id="storage-default-title" class="font-[620] text-oi-ink">默认存储</h2>
          <p class="mt-0.5 text-[12.5px] text-oi-muted">上传页默认选中它。普通用户始终可以使用默认存储，其他存储需要在「用户」中单独授权。默认存储不能停用或删除。</p>
        </div>
      </div>
      <div class="flex flex-none gap-2">
        <select
          ref="defaultSelect"
          v-model.number="selectedDefault"
          :disabled="!canSettings || savingDefault"
          :title="canSettings ? undefined : '需要「存储与上传」设置权限'"
          aria-labelledby="storage-default-title"
          class="h-9 w-[220px] rounded-[10px] border border-oi-line-2 bg-oi-raised px-2.5 text-[13.5px] text-oi-ink outline-none focus:border-oi-accent focus:shadow-[0_0_0_3px_var(--oi-ring)] disabled:opacity-60 max-[760px]:w-auto max-[760px]:flex-1"
        >
          <option v-for="storage in storages" :key="storage.id" :value="storage.id" :disabled="storage.disabled">
            {{ storage.disabled ? `${storage.name}（已停用）` : storage.name }}
          </option>
        </select>
        <UiButton v-if="canSettings" variant="primary" :disabled="selectedDefault === defaultStorage || savingDefault" @click="saveDefault">保存</UiButton>
      </div>
    </section>

    <div v-if="loading && !storages.length" class="mt-4 grid grid-cols-[repeat(auto-fill,minmax(300px,1fr))] gap-3.5" aria-busy="true">
      <div v-for="n in 3" :key="n" class="h-[300px] animate-pulse rounded-oi-lg border border-oi-line bg-oi-panel" />
    </div>
    <div v-else class="mt-4 grid grid-cols-[repeat(auto-fill,minmax(300px,1fr))] gap-3.5 max-[360px]:grid-cols-1">
      <StorageCard
        v-for="storage in storages"
        :key="storage.id"
        :storage="storage"
        :is-default="storage.id === defaultStorage"
        :cdn-domain="cdnDomain"
        :can-update="canUpdate"
        :can-delete="canDelete"
        :can-edit-cdn="canSettings"
        :busy="busyId === storage.id"
        @edit="openForm(storage)"
        @domain="openDomain(storage)"
        @test="testConnection(storage)"
        @toggle="setDisabled(storage, !storage.disabled)"
        @delete="openDelete(storage)"
      />
      <button
        v-if="canCreate && !loadError"
        type="button"
        class="flex min-h-[200px] flex-col items-center justify-center gap-1 rounded-oi-lg border border-dashed border-oi-line-2 text-oi-muted transition-colors hover:border-oi-accent hover:text-oi-accent focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring"
        @click="openForm(null)"
      >
        <i class="ri-add-circle-line text-[26px]" aria-hidden="true" />
        <b class="font-[600]">添加存储</b>
        <span class="text-xs">S3 · R2 · FTP · WebDAV</span>
      </button>
    </div>

    <p v-if="storages.length" class="mt-[18px] flex items-start gap-1.5 text-[12.5px] text-oi-muted">
      <i class="ri-shield-keyhole-line leading-5" aria-hidden="true" />
      <span v-if="directCount">来源白名单只对「经 OneImg」的链接生效。当前有 {{ directCount }} 个存储是直连，它们的文件不受白名单保护。</span>
      <span v-else>来源白名单只对「经 OneImg」的链接生效。当前所有存储都经 OneImg 访问。</span>
    </p>

    <StorageFormDialog v-model:open="formOpen" :storage="formStorage" @saved="refresh" />
    <StorageDomainDialog v-model:open="domainOpen" :storage="domainStorage" :cdn-domain="cdnDomain ?? ''" @saved="refresh" />
    <StorageDeleteDialog
      v-model:open="deleteOpen"
      :storage="deleteStorage"
      :can-disable="canUpdate && !deleteStorage?.disabled"
      @deleted="refresh"
      @disable="deleteStorage && setDisabled(deleteStorage, true, true)"
    />
  </div>
</template>
