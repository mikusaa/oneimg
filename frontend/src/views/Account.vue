<template>
  <div class="page-shell text-slate-800 dark:text-slate-200">
    <PageHeader title="账户设置" description="管理账户资料、密码和 Passkey" />

    <div class="grid grid-cols-1 gap-6 pb-16 xl:grid-cols-2">
      <section class="section-card overflow-hidden">
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

      <section class="section-card overflow-hidden">
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

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { browserSupportsWebAuthn, startRegistration } from '@simplewebauthn/browser'
import { useRouter } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'
import message from '@/utils/message.js'
import { getStoredUser, ROLE_ADMIN } from '@/utils/permissions.js'

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

const authHeaders = (json = false) => ({
  ...(json ? { 'Content-Type': 'application/json' } : {}),
  'Authorization': `Bearer ${localStorage.getItem('authToken')}`
})

const readJSON = async (response) => {
  const data = await response.json()
  if (!response.ok || data.code !== 200) throw new Error(data.message || '请求失败')
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
    const response = await fetch('/api/settings/login')
    const data = await readJSON(response)
    serverSupportsPasskeys.value = Boolean(data.data?.passkey_available)
  } catch {
    serverSupportsPasskeys.value = false
  }
}

const loadPasskeys = async () => {
  loadingPasskeys.value = true
  try {
    const data = await readJSON(await fetch('/api/passkeys', { headers: authHeaders() }))
    passkeys.value = data.data?.passkeys || []
  } catch (error) {
    message.error(error.message || '获取 Passkey 失败')
  } finally {
    loadingPasskeys.value = false
  }
}

const addPasskey = async () => {
  const name = passkeyForm.name.trim()
  if (!name) return message.warning('请输入设备名称')
  if (!passkeyForm.currentPassword) return message.warning('请输入当前密码')
  isAddingPasskey.value = true
  try {
    const begin = await readJSON(await fetch('/api/passkeys/register/begin', {
      method: 'POST',
      headers: authHeaders(true),
      body: JSON.stringify({ name, current_password: passkeyForm.currentPassword })
    }))
    const registration = await startRegistration({ optionsJSON: begin.data.options })
    await readJSON(await fetch('/api/passkeys/register/finish', {
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
    await readJSON(await fetch(`/api/passkeys/${item.id}`, {
      method: 'PUT',
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
    await readJSON(await fetch(`/api/passkeys/${item.id}`, {
      method: 'DELETE',
      headers: authHeaders(true),
      body: JSON.stringify({ current_password: deletePassword.value })
    }))
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
  if (hasUsernameChange && (newUsername.length < 3 || newUsername.length > 64)) return message.error('用户名长度必须在 3-64 位之间')
  if (hasPasswordChange && newPassword.length < 6) return message.error('新密码长度至少为 6 位')
  if (hasPasswordChange && newPassword !== confirmPassword) return message.error('两次输入的新密码不一致')
  if (!currentPassword) return message.error('请输入当前密码以确认修改')

  isUpdatingAccount.value = true
  try {
    await readJSON(await fetch('/api/account/change', {
      method: 'POST',
      headers: authHeaders(true),
      body: JSON.stringify({ new_username: newUsername, current_password: currentPassword, new_password: newPassword })
    }))
    message.success('修改成功，请重新登录')
    localStorage.removeItem('userInfo')
    localStorage.removeItem('authToken')
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
})
</script>
