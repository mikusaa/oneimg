<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { api, ApiError, type StorageBucket, type StorageRequest } from '@/api/client.ts'
import { toast, UiButton, UiDialog, UiField, UiInput } from '@/components/ui'
import StorageTypeIcon from './StorageTypeIcon.vue'
import { domainError, normalizeDomain, REMOTE_TYPES, supportsDirectDomain, TYPE_LABELS } from './storageInfo'

/** 编辑时传入已有存储，添加时为 null */
const props = defineProps<{ storage: StorageBucket | null }>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ saved: [storage: StorageBucket] }>()

interface FieldDef {
  key: string
  label: string
  /** 后端加密保存，响应里只返回 <key>_configured */
  secret?: boolean
  optional?: boolean
  wide?: boolean
  placeholder?: string
  hint?: string
  numeric?: boolean
}

const directFields = (type: 's3' | 'r2'): FieldDef[] => [
  { key: `${type}_endpoint`, label: 'Endpoint', wide: true, placeholder: 'https://…' },
  { key: `${type}_bucket`, label: 'Bucket' },
  { key: 'public_domain', label: '直链域名', optional: true, placeholder: '留空则经 OneImg 访问', hint: '存储桶绑定的公开域名，浏览器直接访问' },
  { key: `${type}_access_key`, label: 'Access Key', secret: true },
  { key: `${type}_secret_key`, label: 'Secret Key', secret: true },
]
const FIELDS: Record<string, FieldDef[]> = {
  r2: directFields('r2'),
  s3: directFields('s3'),
  ftp: [
    { key: 'ftp_host', label: '主机', placeholder: 'ftp.example.com', hint: '不需要填写 ftp:// 或 sftp://' },
    { key: 'ftp_port', label: '端口', numeric: true, placeholder: '21' },
    { key: 'ftp_user', label: '用户名', secret: true },
    { key: 'ftp_pass', label: '密码', secret: true },
  ],
  webdav: [
    { key: 'webdav_url', label: 'WebDAV 地址', wide: true, placeholder: 'https://…' },
    { key: 'webdav_user', label: '用户名', secret: true },
    { key: 'webdav_pass', label: '密码', secret: true },
  ],
}

const GiB = 1024 ** 3
const editing = computed(() => !!props.storage)
const type = ref('r2')
const name = ref('')
const capacityGB = ref('')
let initialCapacityGB = ''
const values = reactive<Record<string, string>>({})
const errors = reactive<Record<string, string>>({})
const formError = ref('')
const saving = ref(false)
const testing = ref(false)
const testResult = ref<{ ok: boolean; text: string } | null>(null)

const fields = computed(() => FIELDS[type.value] ?? [])
const configured = (key: string) => props.storage?.config?.[`${key}_configured`] === true

const reset = () => {
  const storage = props.storage
  type.value = storage?.type ?? 'r2'
  name.value = storage?.name ?? ''
  initialCapacityGB = storage && storage.capacity_bytes > 0 ? String(Number((storage.capacity_bytes / GiB).toFixed(2))) : ''
  capacityGB.value = initialCapacityGB
  for (const key of Object.keys(values)) delete values[key]
  for (const list of Object.values(FIELDS)) {
    for (const field of list) {
      const raw = storage?.type === type.value ? storage.config?.[field.key] : undefined
      values[field.key] = field.secret ? '' : raw == null ? '' : String(raw)
    }
  }
  if (!values.ftp_port) values.ftp_port = '21'
  clearErrors()
  testResult.value = null
}
const clearErrors = () => {
  for (const key of Object.keys(errors)) delete errors[key]
  formError.value = ''
}
watch(open, isOpen => { if (isOpen) reset() })
watch(type, () => { clearErrors(); testResult.value = null })

const placeholderOf = (field: FieldDef) => (field.secret && editing.value && configured(field.key) ? '已配置，留空保持不变' : field.placeholder ?? '')

const validate = () => {
  clearErrors()
  if (!name.value.trim()) errors.name = '请填写名称'
  const gb = Number(capacityGB.value)
  if (!capacityGB.value.trim() || !Number.isFinite(gb) || gb <= 0) errors.capacity = '请填写大于 0 的容量'
  for (const field of fields.value) {
    const value = values[field.key]?.trim() ?? ''
    if (field.key === 'public_domain') {
      const message = domainError(value)
      if (message) errors[field.key] = message
      continue
    }
    if (field.key === 'ftp_port') {
      const port = Number(value)
      if (!Number.isInteger(port) || port < 1 || port > 65535) errors[field.key] = '端口范围是 1–65535'
      continue
    }
    if (field.optional || value) continue
    if (field.secret && editing.value && configured(field.key)) continue
    errors[field.key] = `请填写${/^[A-Za-z]/.test(field.label) ? ' ' : ''}${field.label}`
  }
  return Object.keys(errors).length === 0
}

const payload = (): StorageRequest => {
  const config: Record<string, unknown> = {}
  for (const field of fields.value) {
    const value = values[field.key]?.trim() ?? ''
    if (field.key === 'public_domain') config[field.key] = normalizeDomain(value)
    else if (field.key === 'ftp_port') config[field.key] = Number(value)
    else if (value || !field.secret) config[field.key] = value
  }
  const capacity = props.storage && capacityGB.value.trim() === initialCapacityGB
    ? props.storage.capacity_bytes
    : Math.round(Number(capacityGB.value) * GiB)
  return { name: name.value.trim(), type: type.value as StorageRequest['type'], capacity_bytes: capacity, config }
}

const showProblem = (error: unknown, fallback: string) => {
  if (!(error instanceof ApiError)) { formError.value = fallback; return }
  const problem = error.problem
  const domain = problem?.errors?.find(item => item.field === 'config.public_domain')
  if (domain) { errors.public_domain = domain.message; return }
  formError.value = problem?.detail || fallback
}

const save = async () => {
  if (saving.value || !validate()) return
  saving.value = true
  try {
    const result = props.storage ? await api.updateBucket(props.storage.id, payload()) : await api.createBucket(payload())
    toast.success(props.storage ? '存储已更新' : '存储已添加')
    open.value = false
    emit('saved', result)
  } catch (error) {
    showProblem(error, props.storage ? '保存失败，请稍后重试' : '添加失败，请稍后重试')
  } finally {
    saving.value = false
  }
}

const test = async () => {
  if (testing.value || !validate()) return
  testing.value = true
  testResult.value = null
  try {
    const result = await api.testBucketConnection({ ...payload(), ...(props.storage ? { id: props.storage.id } : {}) })
    testResult.value = { ok: true, text: result?.detail || '连接正常' }
  } catch (error) {
    testResult.value = { ok: false, text: error instanceof ApiError ? error.problem?.detail || '连接测试失败' : '连接测试失败，请稍后重试' }
  } finally {
    testing.value = false
  }
}

const dialogDescription = computed(() => (editing.value
  ? '密钥类字段留空表示保持不变；修改 Endpoint、主机或账户时必须重新填写密钥。'
  : '本地存储是内置的，不需要添加。'))
</script>

<template>
  <UiDialog v-model:open="open" :title="editing ? '编辑存储' : '添加存储'" :description="dialogDescription" icon="ri-database-2-line" wide>
    <form id="storage-form" class="flex flex-col gap-4" novalidate @submit.prevent="save">
      <div role="group" aria-label="存储类型" class="grid grid-cols-4 gap-2 max-[520px]:grid-cols-2">
        <button
          v-for="item in REMOTE_TYPES"
          :key="item"
          type="button"
          :aria-pressed="type === item"
          :disabled="editing && type !== item"
          :class="[
            'flex flex-col items-center gap-1.5 rounded-xl border px-1 py-3 text-xs font-[550] text-oi-ink transition-[border-color,box-shadow] disabled:opacity-40',
            'focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring',
            type === item ? 'border-oi-accent shadow-[0_0_0_3px_var(--oi-ring)]' : 'border-oi-line-2 hover:border-oi-faint',
          ]"
          @click="type = item"
        >
          <StorageTypeIcon :type="item" />{{ TYPE_LABELS[item] }}
        </button>
      </div>

      <div class="grid grid-cols-2 gap-x-3 gap-y-3.5 max-[520px]:grid-cols-1">
        <UiField label="名称" for="storage-name" :error="errors.name">
          <UiInput id="storage-name" v-model="name" :invalid="!!errors.name" placeholder="例如：R2 主存储" autocomplete="off" />
        </UiField>
        <UiField label="容量" for="storage-capacity" :error="errors.capacity" hint="达到容量后停止向这个存储上传">
          <div class="relative">
            <UiInput id="storage-capacity" v-model="capacityGB" :invalid="!!errors.capacity" inputmode="decimal" class="pr-10" autocomplete="off" />
            <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-oi-muted">GB</span>
          </div>
        </UiField>
        <UiField
          v-for="field in fields"
          :key="`${type}-${field.key}`"
          :for="`storage-${field.key}`"
          :error="errors[field.key]"
          :hint="field.hint"
          :class="field.wide && 'col-span-2 max-[520px]:col-span-1'"
        >
          <template #label>
            {{ field.label }}<span v-if="field.optional" class="ml-1 font-normal text-oi-muted">可选</span>
          </template>
          <UiInput
            :id="`storage-${field.key}`"
            v-model="values[field.key]"
            mono
            :invalid="!!errors[field.key]"
            :type="field.secret ? 'password' : 'text'"
            :inputmode="field.numeric ? 'numeric' : undefined"
            :placeholder="placeholderOf(field)"
            :autocomplete="field.secret ? 'new-password' : 'off'"
            spellcheck="false"
          />
        </UiField>
      </div>

      <p v-if="editing && supportsDirectDomain(type)" class="text-xs text-oi-muted">修改直链域名会改变这个存储里所有文件（含历史文件）显示的链接。</p>
      <p v-if="formError" class="flex items-start gap-1.5 text-[12.5px] text-oi-danger" role="alert">
        <i class="ri-error-warning-line leading-5" aria-hidden="true" />{{ formError }}
      </p>
      <p v-if="testResult" :class="['flex items-start gap-1.5 text-[12.5px]', testResult.ok ? 'text-oi-accent-2' : 'text-oi-danger']" role="status">
        <i :class="[testResult.ok ? 'ri-checkbox-circle-line' : 'ri-error-warning-line', 'leading-5']" aria-hidden="true" />{{ testResult.text }}
      </p>
    </form>
    <template #footer>
      <UiButton variant="ghost" class="mr-auto max-[420px]:mr-0" :disabled="testing || saving" @click="test">
        <i :class="testing ? 'ri-loader-4-line animate-spin' : 'ri-pulse-line'" aria-hidden="true" />测试连接
      </UiButton>
      <UiButton variant="ghost" @click="open = false">取消</UiButton>
      <UiButton type="submit" form="storage-form" variant="primary" :disabled="saving">
        <i v-if="saving" class="ri-loader-4-line animate-spin" aria-hidden="true" />{{ editing ? '保存' : '添加' }}
      </UiButton>
    </template>
  </UiDialog>
</template>
