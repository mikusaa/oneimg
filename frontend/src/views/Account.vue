<template>
  <div class="page-shell text-slate-800 dark:text-slate-200">
    <PageHeader title="账户设置" description="管理账户资料、密码、Passkey 和 API 令牌" />

    <div class="grid grid-cols-1 gap-6 pb-16 xl:grid-cols-2">
      <section class="section-card !p-0 overflow-hidden xl:col-start-1 xl:row-start-1">
        <div class="panel-content p-6 md:p-8">
          <div class="mb-6 flex items-center gap-3">
            <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300">
              <i class="ri-user-settings-line text-lg" aria-hidden="true"></i>
            </div>
            <div>
              <h2 class="font-semibold text-slate-900 dark:text-white">账户资料</h2>
              <p class="mt-0.5 text-xs text-slate-500 dark:text-slate-400">修改后需要重新登录</p>
            </div>
          </div>

          <form class="space-y-6" @submit.prevent="updateAccount">
            <div v-if="isAdmin" class="setting-group">
              <label class="field-label" for="newUsername">新用户名</label>
              <input
                id="newUsername"
                v-model="accountForm.newUsername"
                type="text"
                class="input-modern"
                placeholder="留空则不修改"
                minlength="3"
                maxlength="64"
              />
            </div>

            <div class="setting-group">
              <label class="field-label" for="currentPassword">当前密码</label>
              <input
                id="currentPassword"
                v-model="accountForm.currentPassword"
                type="password"
                class="input-modern"
                placeholder="请输入当前密码"
                autocomplete="current-password"
                required
              />
            </div>

            <div class="setting-group">
              <label class="field-label" for="newPassword">新密码</label>
              <input
                id="newPassword"
                v-model="accountForm.newPassword"
                type="password"
                class="input-modern"
                placeholder="留空则不修改"
                autocomplete="new-password"
                minlength="6"
              />
            </div>

            <div class="setting-group">
              <label class="field-label" for="confirmPassword">确认新密码</label>
              <input
                id="confirmPassword"
                v-model="accountForm.confirmPassword"
                type="password"
                class="input-modern"
                placeholder="再次输入新密码"
                autocomplete="new-password"
              />
            </div>

            <button type="submit" :disabled="isUpdatingAccount" class="primary-button w-full px-6 py-3">
              <i v-if="isUpdatingAccount" class="ri-loader-4-line animate-spin" aria-hidden="true"></i>
              保存修改
            </button>
          </form>
        </div>
      </section>

      <section class="section-card !p-0 overflow-hidden xl:col-span-2 xl:col-start-1 xl:row-start-2">
        <div class="panel-content p-6 md:p-8">
          <div class="mb-6 flex items-center justify-between gap-3">
            <div class="flex min-w-0 items-center gap-3">
              <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-300">
                <i class="ri-key-2-line text-lg" aria-hidden="true"></i>
              </div>
              <div class="min-w-0">
                <h2 class="font-semibold text-slate-900 dark:text-white">API 令牌</h2>
                <p class="mt-0.5 text-xs text-slate-500 dark:text-slate-400">管理用于接口调用的个人访问令牌</p>
              </div>
            </div>
            <button type="button" class="primary-button shrink-0 px-4 py-2" :disabled="tokenBusy" @click="showTokenForm = !showTokenForm">{{ showTokenForm ? '取消' : '创建令牌' }}</button>
          </div>
          <form v-if="showTokenForm" class="mb-6 grid gap-3 border-b border-slate-200 pb-6 dark:border-white/10 md:grid-cols-2" @submit.prevent="createPersonalToken">
            <input v-model="tokenForm.name" class="input-modern" placeholder="令牌名称" maxlength="50" required />
            <select v-model.number="tokenForm.expirationDays" class="input-modern"><option :value="30">30 天</option><option :value="90">90 天</option><option :value="365">365 天</option><option :value="0">永不过期</option></select>
            <fieldset class="md:col-span-2">
              <legend class="field-label mb-2">权限范围</legend>
              <label v-if="isAdmin" class="mb-2 flex min-h-10 items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2 text-sm text-blue-900 dark:border-blue-500/30 dark:bg-blue-500/10 dark:text-blue-200">
                <input
                  type="checkbox"
                  class="h-4 w-4 rounded border-blue-300 text-primary focus:ring-primary"
                  :checked="allTokenScopesSelected"
                  :indeterminate="someTokenScopesSelected"
                  @change="toggleAllTokenScopes"
                />
                <span class="font-medium">全部权限</span>
                <code class="ml-auto text-[11px] text-blue-500 dark:text-blue-300">{{ tokenScopes.length }} scopes</code>
              </label>
              <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                <label v-for="scope in tokenScopes" :key="scope.value" class="flex min-h-10 items-center gap-2 rounded-lg border border-slate-200 px-3 py-2 text-sm dark:border-white/10">
                  <input v-model="tokenForm.scopes" type="checkbox" :value="scope.value" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                  <span>{{ scope.label }}</span>
                  <code class="ml-auto text-[11px] text-slate-400">{{ scope.value }}</code>
                </label>
              </div>
            </fieldset>
            <button type="submit" class="primary-button md:col-span-2" :disabled="tokenBusy">{{ tokenBusy ? '创建中...' : '确认创建' }}</button>
          </form>
          <div v-if="newToken" class="mb-4 flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
            <code class="min-w-0 flex-1 break-all">{{ newToken }}</code>
            <button type="button" class="icon-button shrink-0" title="复制令牌" aria-label="复制令牌" @click="copyNewToken"><i class="ri-file-copy-line"></i></button>
          </div>
          <div v-if="tokens.length === 0" class="text-sm text-slate-500">尚未创建令牌</div>
          <div v-else class="divide-y divide-slate-100 dark:divide-white/5">
            <div v-for="token in tokens" :key="token.id" class="flex items-center justify-between gap-3 py-3">
              <div class="min-w-0"><p class="text-sm font-medium">{{ token.name }}</p><p class="text-xs text-slate-500">{{ token.prefix }} · {{ token.revoked_at ? '已撤销' : (token.expires_at ? `到期 ${formatDate(token.expires_at)}` : '永不过期') }}</p><p class="mt-1 break-words text-[11px] text-slate-400">{{ token.scopes.join(' · ') }}</p></div>
              <button v-if="!token.revoked_at" type="button" class="danger-button px-3 py-1.5" :disabled="tokenBusy" @click="revokePersonalToken(token)">撤销</button>
            </div>
          </div>
        </div>
      </section>

      <section class="section-card !p-0 overflow-hidden xl:col-start-2 xl:row-start-1">
        <div class="panel-content p-6 md:p-8">
          <div class="mb-6 flex items-center justify-between gap-3">
            <div class="flex min-w-0 items-center gap-3">
              <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300">
                <i class="ri-fingerprint-line text-lg" aria-hidden="true"></i>
              </div>
              <div class="min-w-0">
                <h2 class="font-semibold text-slate-900 dark:text-white">Passkey</h2>
                <p class="mt-0.5 text-xs text-slate-500 dark:text-slate-400">已绑定 {{ passkeys.length }} / 10</p>
              </div>
            </div>
            <span
              class="shrink-0 rounded-full border px-2 py-1 text-[11px]"
              :class="passkeyReady
                ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/20 dark:bg-emerald-500/10 dark:text-emerald-300'
                : 'border-slate-200 bg-slate-50 text-slate-500 dark:border-white/10 dark:bg-slate-800 dark:text-slate-400'"
            >
              {{ passkeyReady ? '可用' : '不可用' }}
            </span>
          </div>

          <form v-if="passkeyReady" class="mb-6 space-y-4 border-b border-slate-200 pb-6 dark:border-white/10" @submit.prevent="addPasskey">
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <label class="field-label" for="passkeyName">设备名称</label>
                <input
                  id="passkeyName"
                  v-model="passkeyForm.name"
                  class="input-modern"
                  type="text"
                  maxlength="50"
                  placeholder="例如 MacBook Touch ID"
                  :disabled="isAddingPasskey || passkeys.length >= 10"
                />
              </div>
              <div>
                <label class="field-label" for="passkeyPassword">当前密码</label>
                <input
                  id="passkeyPassword"
                  v-model="passkeyForm.currentPassword"
                  class="input-modern"
                  type="password"
                  autocomplete="current-password"
                  placeholder="确认当前密码"
                  :disabled="isAddingPasskey || passkeys.length >= 10"
                />
              </div>
            </div>
            <button
              type="submit"
              class="primary-button w-full sm:w-auto"
              :disabled="isAddingPasskey || passkeys.length >= 10"
            >
              <i v-if="isAddingPasskey" class="ri-loader-4-line animate-spin" aria-hidden="true"></i>
              <i v-else class="ri-add-line" aria-hidden="true"></i>
              {{ isAddingPasskey ? '正在添加' : '添加 Passkey' }}
            </button>
          </form>

          <div v-if="loadingPasskeys" class="flex h-28 items-center justify-center text-slate-400">
            <i class="ri-loader-4-line animate-spin text-xl" aria-label="正在加载"></i>
          </div>
          <div v-else-if="passkeys.length === 0" class="rounded-lg border border-dashed border-slate-200 px-4 py-10 text-center dark:border-white/10">
            <i class="ri-key-2-line text-2xl text-slate-300 dark:text-slate-600" aria-hidden="true"></i>
            <p class="mt-2 text-sm text-slate-500 dark:text-slate-400">尚未绑定 Passkey</p>
          </div>
          <div v-else class="divide-y divide-slate-100 dark:divide-white/5">
            <div v-for="item in passkeys" :key="item.id" class="py-4 first:pt-0 last:pb-0">
              <div v-if="editingPasskeyID === item.id" class="flex gap-2">
                <input v-model="editingPasskeyName" class="input-modern min-h-0 flex-1 py-2" maxlength="50" @keyup.enter="renamePasskey(item)" @keyup.esc="cancelRename" />
                <button class="icon-button" type="button" title="保存名称" aria-label="保存名称" :disabled="passkeyActionID === item.id" @click="renamePasskey(item)">
                  <i class="ri-check-line"></i>
                </button>
                <button class="icon-button" type="button" title="取消" aria-label="取消" @click="cancelRename">
                  <i class="ri-close-line"></i>
                </button>
              </div>
              <template v-else>
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-medium text-slate-900 dark:text-white" :title="item.name">{{ item.name }}</p>
                    <p class="mt-1 text-xs text-slate-400 dark:text-slate-500">
                      添加于 {{ formatDate(item.created_at) }}<span class="mx-1.5">·</span>{{ item.last_used_at ? `最近使用 ${formatDate(item.last_used_at)}` : '尚未使用' }}
                    </p>
                  </div>
                  <div class="flex shrink-0 gap-1">
                    <button class="icon-button" type="button" title="重命名" aria-label="重命名" @click="startRename(item)">
                      <i class="ri-edit-line"></i>
                    </button>
                    <button class="icon-button text-red-500" type="button" title="删除" aria-label="删除" @click="beginDelete(item)">
                      <i class="ri-delete-bin-7-line"></i>
                    </button>
                  </div>
                </div>
                <form v-if="deletingPasskeyID === item.id" class="mt-3 flex flex-col gap-2 rounded-lg bg-slate-50 p-3 dark:bg-slate-800/60 sm:flex-row" @submit.prevent="deletePasskey(item)">
                  <input v-model="deletePassword" class="input-modern min-h-0 flex-1 py-2" type="password" autocomplete="current-password" placeholder="输入当前密码确认删除" autofocus />
                  <div class="flex gap-2">
                    <button type="button" class="soft-button flex-1 sm:flex-none" @click="cancelDelete">取消</button>
                    <button type="submit" class="danger-button flex-1 sm:flex-none" :disabled="passkeyActionID === item.id">
                      {{ passkeyActionID === item.id ? '删除中' : '确认删除' }}
                    </button>
                  </div>
                </form>
              </template>
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { apiFetch } from "@/api/client.ts"
import { computed, onMounted, reactive, ref } from 'vue'
import { browserSupportsWebAuthn, startRegistration } from '@simplewebauthn/browser'
import { useRouter } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'
import message from '@/utils/message.ts'
import { getStoredUser, ROLE_ADMIN } from '@/utils/permissions.ts'
import type { CreateTokenRequest } from '@/api/generated/types.gen'

type TokenScope = CreateTokenRequest['scopes'][number]

const router = useRouter()
const isAdmin = Number(getStoredUser()?.role) === ROLE_ADMIN
const browserSupportsPasskeys = ref(false)
const serverSupportsPasskeys = ref(false)
const passkeyReady = computed(() => browserSupportsPasskeys.value && serverSupportsPasskeys.value)

const accountForm = ref({ newUsername: '', currentPassword: '', newPassword: '', confirmPassword: '' })
const isUpdatingAccount = ref(false)
const passkeys = ref([])
const loadingPasskeys = ref(true)
const isAddingPasskey = ref(false)
const passkeyActionID = ref(null)
const editingPasskeyID = ref(null)
const editingPasskeyName = ref('')
const deletingPasskeyID = ref(null)
const deletePassword = ref('')
const passkeyForm = reactive({ name: '', currentPassword: '' })
const tokens = ref([])
const tokenBusy = ref(false)
const showTokenForm = ref(false)
const newToken = ref('')
const tokenScopes: Array<{ value: TokenScope; label: string }> = [
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
const tokenForm = reactive<{ name: string; expirationDays: 0 | 30 | 90 | 365; scopes: TokenScope[] }>({
  name: '', expirationDays: 90, scopes: ['images:read'],
})
const allTokenScopesSelected = computed(() => tokenForm.scopes.length === tokenScopes.length)
const someTokenScopesSelected = computed(() => tokenForm.scopes.length > 0 && !allTokenScopesSelected.value)

const toggleAllTokenScopes = (event: Event) => {
  tokenForm.scopes = (event.target as HTMLInputElement).checked ? tokenScopes.map(scope => scope.value) : []
}

const authHeaders = (json = false) => ({
  ...(json ? { 'Content-Type': 'application/json' } : {})
})

const readJSON = async (response) => {
  const data = await response.json()
  if (!response.ok || !Object.prototype.hasOwnProperty.call(data, 'data')) throw new Error(data.detail || '请求失败')
  return data
}

const isPasskeyCancellation = (error) => {
  const name = error?.cause?.name || error?.name
  return name === 'NotAllowedError' || name === 'AbortError' || error?.code === 'ERROR_CEREMONY_ABORTED'
}

const formatDate = (value) => {
  if (!value) return '--'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '--' : date.toLocaleString('zh-CN', { dateStyle: 'medium', timeStyle: 'short' })
}

const loadPasskeyAvailability = async () => {
  try {
    const response = await apiFetch('/api/v1/public/config')
    const data = await readJSON(response)
    serverSupportsPasskeys.value = Boolean(data.data?.passkey_available)
  } catch {
    serverSupportsPasskeys.value = false
  }
}

const loadPasskeys = async () => {
  loadingPasskeys.value = true
  try {
    const data = await readJSON(await apiFetch('/api/v1/me/passkeys', { headers: authHeaders() }))
    passkeys.value = data.data || []
  } catch (error) {
    message.error(error.message || '获取 Passkey 失败')
  } finally {
    loadingPasskeys.value = false
  }
}

const loadTokens = async () => {
  try {
    const response = await apiFetch('/api/v1/me/tokens')
    const data = await response.json()
    if (!response.ok) throw new Error(data.detail || '获取令牌失败')
    tokens.value = data.data || []
  } catch (error) { message.error(error.message || '获取令牌失败') }
}

const createPersonalToken = async () => {
  if (tokenForm.scopes.length === 0) return message.warning('请至少选择一个权限范围')
  tokenBusy.value = true; newToken.value = ''
  try {
    const response = await apiFetch('/api/v1/me/tokens', { method: 'POST', headers: authHeaders(true), body: JSON.stringify({ name: tokenForm.name, expiration_days: tokenForm.expirationDays, scopes: tokenForm.scopes }) })
    const data = await response.json(); if (!response.ok) throw new Error(data.detail || '创建令牌失败')
    newToken.value = data.data.token; tokens.value.unshift(data.data.record); tokenForm.name = ''; showTokenForm.value = false
  } catch (error) { message.error(error.message || '创建令牌失败') } finally { tokenBusy.value = false }
}

const copyNewToken = async () => {
  if (!newToken.value) return
  try {
    await navigator.clipboard.writeText(newToken.value)
    message.success('令牌已复制')
  } catch {
    message.error('复制失败')
  }
}

const revokePersonalToken = async (token) => {
  if (!window.confirm(`确认撤销令牌“${token.name}”？`)) return
  tokenBusy.value = true
  try {
    const response = await apiFetch(`/api/v1/me/tokens/${token.id}/revoke`, { method: 'POST' })
    if (!response.ok) { const data = await response.json(); throw new Error(data.detail || '撤销令牌失败') }
    token.revoked_at = new Date().toISOString()
    message.success('令牌已撤销')
  } catch (error) { message.error(error.message || '撤销令牌失败') } finally { tokenBusy.value = false }
}

const addPasskey = async () => {
  const name = passkeyForm.name.trim()
  if (!name) return message.warning('请输入设备名称')
  if (!passkeyForm.currentPassword) return message.warning('请输入当前密码')
  isAddingPasskey.value = true
  try {
    const begin = await readJSON(await apiFetch('/api/v1/me/passkeys/registration/options', {
      method: 'POST',
      headers: authHeaders(true),
      body: JSON.stringify({ name, current_password: passkeyForm.currentPassword })
    }))
    const registration = await startRegistration({ optionsJSON: begin.data.options })
    await readJSON(await apiFetch('/api/v1/me/passkeys/registration/verify', {
      method: 'POST',
      headers: authHeaders(true),
      body: JSON.stringify(registration)
    }))
    passkeyForm.name = ''
    passkeyForm.currentPassword = ''
    message.success('Passkey 添加成功')
    await loadPasskeys()
  } catch (error) {
    if (!isPasskeyCancellation(error)) message.error(error.message || '添加 Passkey 失败')
  } finally {
    passkeyForm.currentPassword = ''
    isAddingPasskey.value = false
  }
}

const startRename = (item) => {
  deletingPasskeyID.value = null
  editingPasskeyID.value = item.id
  editingPasskeyName.value = item.name
}

const cancelRename = () => {
  editingPasskeyID.value = null
  editingPasskeyName.value = ''
}

const renamePasskey = async (item) => {
  const name = editingPasskeyName.value.trim()
  if (!name) return message.warning('设备名称不能为空')
  passkeyActionID.value = item.id
  try {
    await readJSON(await apiFetch(`/api/v1/me/passkeys/${item.id}`, {
      method: 'PATCH',
      headers: authHeaders(true),
      body: JSON.stringify({ name })
    }))
    item.name = name
    cancelRename()
    message.success('名称已更新')
  } catch (error) {
    message.error(error.message || '重命名失败')
  } finally {
    passkeyActionID.value = null
  }
}

const beginDelete = (item) => {
  cancelRename()
  deletingPasskeyID.value = item.id
  deletePassword.value = ''
}

const cancelDelete = () => {
  deletingPasskeyID.value = null
  deletePassword.value = ''
}

const deletePasskey = async (item) => {
  if (!deletePassword.value) return message.warning('请输入当前密码')
  passkeyActionID.value = item.id
  try {
    const revokeResponse = await apiFetch(`/api/v1/me/passkeys/${item.id}/revoke`, {
      method: 'POST',
      headers: authHeaders(true),
      body: JSON.stringify({ current_password: deletePassword.value })
    })
    if (!revokeResponse.ok && revokeResponse.status !== 204) {
      const revokeError = await revokeResponse.json()
      throw new Error(revokeError.detail || '撤销 Passkey 失败')
    }
    passkeys.value = passkeys.value.filter(passkey => passkey.id !== item.id)
    cancelDelete()
    message.success('Passkey 已删除')
  } catch (error) {
    message.error(error.message || '删除 Passkey 失败')
  } finally {
    passkeyActionID.value = null
  }
}

const updateAccount = async () => {
  const { newUsername, currentPassword, newPassword, confirmPassword } = accountForm.value
  const hasUsernameChange = isAdmin && newUsername.trim() !== ''
  const hasPasswordChange = newPassword.trim() !== ''
  if (!hasUsernameChange && !hasPasswordChange) return message.error('请输入要修改的用户名或密码')
  if (hasUsernameChange && (newUsername.length < 3 || newUsername.length > 50)) return message.error('用户名长度必须在 3-50 位之间')
  if (hasPasswordChange && newPassword.length < 6) return message.error('新密码长度至少为 6 位')
  if (hasPasswordChange && newPassword !== confirmPassword) return message.error('两次输入的新密码不一致')
  if (!currentPassword) return message.error('请输入当前密码以确认修改')

  isUpdatingAccount.value = true
  try {
    await readJSON(await apiFetch('/api/v1/me', {
      method: 'PATCH',
      headers: authHeaders(true),
      body: JSON.stringify({ username: hasUsernameChange ? newUsername : undefined, current_password: currentPassword, password: hasPasswordChange ? newPassword : undefined })
    }))
    message.success('修改成功，请重新登录')
    localStorage.removeItem('userInfo')
    setTimeout(() => router.replace('/login'), 800)
  } catch (error) {
    message.error(error.message || '更新失败')
  } finally {
    isUpdatingAccount.value = false
  }
}

onMounted(async () => {
  browserSupportsPasskeys.value = browserSupportsWebAuthn()
  await Promise.all([loadPasskeyAvailability(), loadPasskeys()])
  await loadTokens()
})
</script>
