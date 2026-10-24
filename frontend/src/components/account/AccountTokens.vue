<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { api, errorMessage, type CreateTokenRequest, type PersonalAccessToken } from '@/api/client.ts'
import LinkBox from '@/components/media/LinkBox.vue'
import SettingSection from '@/components/settings/SettingSection.vue'
import { confirm, toast, UiButton, UiInput, UiSelect } from '@/components/ui'
import { formatDateTime } from '@/utils/format'

type TokenScope = CreateTokenRequest['scopes'][number]
type Expiration = 0 | 30 | 90 | 365

const SCOPES: Array<{ value: TokenScope; label: string }> = [
  { value: 'images:read', label: '读取图片' },
  { value: 'images:write', label: '上传和修改图片' },
  { value: 'images:delete', label: '删除图片' },
  { value: 'tags:read', label: '读取标签' },
  { value: 'tags:write', label: '管理标签' },
  { value: 'storage:read', label: '读取存储配置' },
  { value: 'storage:write', label: '管理存储配置' },
  { value: 'users:read', label: '读取用户' },
  { value: 'users:write', label: '管理用户' },
  { value: 'settings:read', label: '读取系统设置' },
  { value: 'settings:write', label: '修改系统设置' },
  { value: 'stats:read', label: '读取统计数据' },
]
const EXPIRATIONS: Array<{ value: Expiration; label: string }> = [
  { value: 30, label: '30 天' },
  { value: 90, label: '90 天' },
  { value: 365, label: '365 天' },
  { value: 0, label: '永不过期' },
]
const API_DOCS = 'https://github.com/mikusaa/oneimg/blob/main/docs/api.md'

const props = defineProps<{
  tokens: PersonalAccessToken[]
  loading: boolean
  /** 管理员可以一键选中全部权限范围 */
  isAdmin: boolean
}>()
const emit = defineEmits<{ changed: [] }>()

const showForm = ref(false)
const form = reactive<{ name: string; expiration: Expiration; scopes: TokenScope[] }>({ name: '', expiration: 90, scopes: ['images:read'] })
const errors = reactive<Record<string, string>>({})
const creating = ref(false)
const newToken = ref('')

const allSelected = computed(() => form.scopes.length === SCOPES.length)
const toggleAll = () => { form.scopes = allSelected.value ? [] : SCOPES.map(scope => scope.value) }
const toggleScope = (scope: TokenScope) => {
  form.scopes = form.scopes.includes(scope) ? form.scopes.filter(item => item !== scope) : [...form.scopes, scope]
}

const openForm = () => {
  showForm.value = !showForm.value
  newToken.value = ''
  for (const key of Object.keys(errors)) delete errors[key]
}

const create = async () => {
  if (creating.value) return
  for (const key of Object.keys(errors)) delete errors[key]
  if (!form.name.trim()) errors.name = '请填写令牌名称'
  if (!form.scopes.length) errors.scopes = '至少选择一个权限范围'
  if (Object.keys(errors).length) return
  creating.value = true
  try {
    const created = await api.createToken({ name: form.name.trim(), expiration_days: form.expiration, scopes: form.scopes })
    newToken.value = created.token
    Object.assign(form, { name: '', expiration: 90, scopes: ['images:read'] })
    showForm.value = false
    emit('changed')
  } catch (error) {
    toast.error(errorMessage(error, '创建令牌失败'))
  } finally {
    creating.value = false
  }
}

const revoke = async (token: PersonalAccessToken) => {
  const ok = await confirm({
    title: `撤销令牌「${token.name}」？`,
    description: '使用这个令牌的客户端会立即无法访问，撤销后不能恢复。',
    danger: true,
    confirmText: '撤销',
  })
  if (!ok) return
  try {
    await api.revokeToken(token.id)
    toast.success('令牌已撤销')
    emit('changed')
  } catch (error) {
    toast.error(errorMessage(error, '撤销令牌失败'))
  }
}

const expired = (token: PersonalAccessToken) => !!token.expires_at && new Date(token.expires_at).getTime() < Date.now()
const status = (token: PersonalAccessToken): { text: string; tone: '' | 'danger' | 'warn' } => {
  if (token.revoked_at) return { text: '已撤销', tone: 'danger' }
  if (expired(token)) return { text: '已过期', tone: 'warn' }
  return { text: token.expires_at ? `${new Date(token.expires_at).toLocaleDateString('zh-CN')} 到期` : '永不过期', tone: '' }
}
const scopeLabel = (scope: string) => SCOPES.find(item => item.value === scope)?.label ?? scope
const sorted = computed(() => [...props.tokens].sort((a, b) => Number(!!a.revoked_at) - Number(!!b.revoked_at)))
</script>

<template>
  <SettingSection id="account-tokens" title="API 令牌">
    <template #actions>
      <UiButton size="sm" :variant="showForm ? 'ghost' : 'primary'" :aria-expanded="showForm" @click="openForm">
        <i :class="showForm ? 'ri-close-line' : 'ri-add-line'" aria-hidden="true" />{{ showForm ? '取消' : '创建令牌' }}
      </UiButton>
    </template>
    <template #description>
      供 PicGo、Typora、脚本等客户端调用接口。令牌权限不会超过你账号本身的权限。
      <a :href="API_DOCS" target="_blank" rel="noopener" class="whitespace-nowrap text-oi-accent-2 hover:underline">接口文档 <i class="ri-external-link-line" aria-hidden="true" /></a>
    </template>
    <template #before>
      <div v-if="newToken" class="mb-3 flex flex-col gap-2.5 rounded-xl bg-oi-warn-soft px-3.5 py-3" role="status">
        <div class="flex gap-1.5 text-[12.5px] leading-normal text-oi-warn">
          <i class="ri-error-warning-line" aria-hidden="true" />令牌只显示这一次，请立即复制保存。
        </div>
        <LinkBox :value="newToken" label="令牌" class="!bg-oi-raised" />
      </div>
    </template>

    <form v-if="showForm" class="flex flex-col gap-3.5 rounded-t-oi-lg bg-oi-panel px-4 py-4 min-[900px]:px-[18px]" novalidate @submit.prevent="create">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div class="flex flex-col gap-1.5">
          <label for="token-name" class="text-[12.5px] font-[550] text-oi-text">名称</label>
          <UiInput id="token-name" v-model="form.name" maxlength="50" placeholder="例如 PicGo · MacBook" :invalid="!!errors.name" />
          <p v-if="errors.name" class="text-xs text-oi-danger" role="alert">{{ errors.name }}</p>
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="token-expiration" class="text-[12.5px] font-[550] text-oi-text">有效期</label>
          <UiSelect id="token-expiration" v-model.number="form.expiration">
            <option v-for="option in EXPIRATIONS" :key="option.value" :value="option.value">{{ option.label }}</option>
          </UiSelect>
        </div>
      </div>
      <fieldset class="flex min-w-0 flex-col gap-2">
        <legend class="mb-2 flex w-full items-center justify-between text-[12.5px] font-[550] text-oi-text">
          权限范围
          <button
            v-if="isAdmin"
            type="button"
            class="inline-flex items-center gap-2 font-normal text-oi-text"
            :aria-pressed="allSelected"
            @click="toggleAll"
          >
            <span :class="['grid h-4 w-4 place-items-center rounded-[5px] border-[1.5px] text-[11px]', allSelected ? 'border-oi-accent bg-oi-accent text-white' : 'border-oi-line-2 bg-oi-raised text-transparent']" aria-hidden="true"><i class="ri-check-line" /></span>
            全部权限
          </button>
        </legend>
        <div class="grid grid-cols-1 gap-1.5 sm:grid-cols-[repeat(auto-fill,minmax(220px,1fr))]">
          <label
            v-for="scope in SCOPES"
            :key="scope.value"
            :class="['flex h-9 cursor-pointer items-center gap-2 rounded-[9px] border bg-oi-raised px-2.5 text-[13px] has-[:focus-visible]:shadow-[0_0_0_3px_var(--oi-ring)]', form.scopes.includes(scope.value) ? 'border-oi-accent' : 'border-oi-line-2']"
          >
            <input type="checkbox" class="sr-only" :checked="form.scopes.includes(scope.value)" @change="toggleScope(scope.value)">
            <span :class="['grid h-4 w-4 flex-none place-items-center rounded-[5px] border-[1.5px] text-[11px]', form.scopes.includes(scope.value) ? 'border-oi-accent bg-oi-accent text-white' : 'border-oi-line-2 bg-oi-raised text-transparent']" aria-hidden="true"><i class="ri-check-line" /></span>
            <span class="min-w-0 truncate">{{ scope.label }}</span>
            <code class="ml-auto font-oi-mono text-[10.5px] text-oi-faint">{{ scope.value }}</code>
          </label>
        </div>
        <p v-if="errors.scopes" class="text-xs text-oi-danger" role="alert">{{ errors.scopes }}</p>
      </fieldset>
      <div class="flex justify-end">
        <UiButton variant="primary" type="submit" :disabled="creating">
          <i v-if="creating" class="ri-loader-4-line animate-spin" aria-hidden="true" />创建令牌
        </UiButton>
      </div>
    </form>

    <div v-if="loading && !tokens.length" class="flex h-20 items-center justify-center text-oi-muted" aria-busy="true">
      <i class="ri-loader-4-line animate-spin text-lg" aria-hidden="true" /><span class="sr-only">正在读取</span>
    </div>
    <div v-else-if="!tokens.length" class="px-4 py-7 text-center text-[13px] text-oi-muted">尚未创建令牌</div>
    <div
      v-for="token in sorted"
      :key="token.id"
      :class="['flex items-center gap-3.5 px-4 py-3.5 min-[900px]:px-[18px]', (token.revoked_at || expired(token)) && 'opacity-55']"
    >
      <span class="grid h-[38px] w-[38px] flex-none place-items-center rounded-[11px] border border-oi-line bg-oi-panel text-lg text-oi-text" aria-hidden="true"><i class="ri-key-2-line" /></span>
      <div class="min-w-0 flex-1">
        <b class="flex min-w-0 items-center gap-2 font-[580] text-oi-ink">
          <span class="truncate">{{ token.name }}</span>
          <span
            v-if="status(token).tone"
            :class="['inline-flex h-[18px] flex-none items-center rounded-full px-1.5 text-[11px] font-[550]', status(token).tone === 'danger' ? 'bg-oi-danger-soft text-oi-danger' : 'bg-oi-warn-soft text-oi-warn']"
          >{{ status(token).text }}</span>
        </b>
        <div class="mt-0.5 break-words text-[12.5px] text-oi-muted"><span class="font-oi-mono">{{ token.prefix }}…</span> · {{ token.scopes.map(scopeLabel).join('、') }}</div>
        <div class="text-[12.5px] text-oi-muted">
          {{ status(token).tone ? '' : `${status(token).text} · ` }}{{ token.last_used_at ? `最近使用 ${formatDateTime(token.last_used_at)}` : '尚未使用' }}
        </div>
      </div>
      <UiButton v-if="!token.revoked_at" size="sm" variant="ghost" class="hover:!text-oi-danger" :aria-label="`撤销 ${token.name}`" @click="revoke(token)">撤销</UiButton>
    </div>
  </SettingSection>
</template>
